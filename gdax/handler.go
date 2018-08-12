package gdax

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chrischris292/go-gdax-orderbook/common"
	"github.com/chrischris292/go-gdax-orderbook/config/orderbook"
	raven "github.com/getsentry/raven-go"
	ws "github.com/gorilla/websocket"
	"github.com/pkg/errors"
	gdaxClient "github.com/preichenberger/go-gdax"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const retry = time.Minute * 1

type Handler struct {
	client           *gdaxClient.Client
	sequence         int64
	book             *Book
	bookListeners    []HandlerConsumer
	rawFeedListeners []RawFeedListener
}

func NewHandler(gdaxClient *gdaxClient.Client, book *Book) *Handler {
	return &Handler{
		client:           gdaxClient,
		book:             book,
		bookListeners:    []HandlerConsumer{},
		rawFeedListeners: []RawFeedListener{},
	}
}

func (handler *Handler) AddHandlerConsumer(listener HandlerConsumer) {
	handler.bookListeners = append(handler.bookListeners, listener)
}
func (handler *Handler) AddRawFeedListener(listener RawFeedListener) {
	handler.rawFeedListeners = append(handler.rawFeedListeners, listener)
}

func (handler *Handler) Run() {
	// Connect to socket and send subscribe message
	for {
		err := handler.startListening()
		zap.L().Error("Failed to listen to web socket", zap.Error(err))
		raven.CaptureErrorAndWait(fmt.Errorf("Failed to listen to web socket: %v", err), nil)
		handler.flushClear()
		handler.sequence = 0
		handler.book.Clear()
		time.Sleep(retry)
	}
}

func (handler *Handler) startListening() error {
	// Connect to socket and send subscribe message
	var wsDialer ws.Dialer
	wsConn, _, err := wsDialer.Dial(config.AppConfig.Coinbase.CB_WS, nil)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		zap.L().Error("Could start web socket", zap.Error(err))
		return err
	}

	subscribe := gdaxClient.Message{
		Type: "subscribe",
		Channels: []gdaxClient.MessageChannel{
			gdaxClient.MessageChannel{
				Name: "full",
				ProductIds: []string{
					handler.book.ID,
				},
			},
		},
	}
	if err := wsConn.WriteJSON(subscribe); err != nil {
		zap.L().Error("Could not write subscription message", zap.Error(err))
		return errors.New("Could not write subscription message")
	}
	err = handler.SyncBook()
	if err != nil {
		zap.L().Error("Could not sync book", zap.Error(err))
		raven.CaptureErrorAndWait(err, nil)
		return errors.New("Could not sync book")
	}
	return handler.listenToSocket(wsConn)
}

func (handler *Handler) listenToSocket(wsConn *ws.Conn) error {
	for true {
		zap.L().Info("fsdfds")
		// Parse message
		_, data, err := wsConn.ReadMessage()
		if err != nil {
			return errors.Wrap(err, "Could not read from wsConn...Closing connection")
		}

		message := gdaxClient.Message{}
		err = json.Unmarshal(data, &message)
		if err != nil {
			return errors.Wrap(err, "Could not unmarshal message from socket")
		}

		// Handle Message
		if message.Type == "subscriptions" {
			zap.L().Info("Subscription message", zap.Strings("productIds", message.ProductIds), zap.String("channels", fmt.Sprintf("%v", message.Channels)))
			continue
		}

		if message.Sequence <= handler.sequence {
			zap.L().Info("Received old messagef", zap.Int64("sequence number", message.Sequence), zap.Int64("bookSequence", handler.sequence))
			continue
		}

		if message.Sequence != (handler.sequence + 1) {
			zap.L().Info("Book is out of order. Resyncing", zap.Int64("sequence number", message.Sequence), zap.Int64("bookSequence", handler.sequence))
			err = handler.SyncBook()
			if err != nil {
				return errors.Wrap(err, "could not sync book")
			}
			continue
		}
		handler.flushRawFeedMessage(string(data))

		handler.sequence = message.Sequence
		err = handler.handleIncremental(message)
		if err != nil {
			return errors.Wrap(err, "Could not read incremental")
		}
		handler.book.PrintTopFive()
	}
	return errors.New("for loop should never end...")
}

func (handler *Handler) handleIncremental(message gdaxClient.Message) error {
	switch messageType := message.Type; messageType {
	case "open":
		order, err := NewOrder(message.OrderId, message.RemainingSize, message.Price, message.Side)
		if err != nil {
			return errors.Wrap(err, "Could not create open message")
		}
		zap.L().Debug("Open message", zap.String("Order", order.ToString()))
		handler.book.Add(order)
		handler.flushBookUpdate(message)
		return nil
	case "done":
		// ignore done messages with no price
		if message.Price == "" {
			return nil
		}
		order, err := NewOrder(message.OrderId, message.RemainingSize, message.Price, message.Side)
		if err != nil {
			return errors.Wrap(err, "Could not create done message")
		}
		zap.L().Debug("Done message", zap.String("Order", order.ToString()))
		err = handler.book.Remove(order)
		if err != nil {
			return errors.Wrap(err, "Could not process done message")
		}
		handler.flushBookUpdate(message)
		return nil
	case "match":
		matchMessage, err := NewMatchMessage(message.TradeId, message.Sequence, message.MakerOrderId, message.TakerOrderId, message.Time, message.ProductId, message.Size, message.Price, message.Side)
		if err != nil {
			return errors.Wrap(err, "Could not create match message")
		}
		zap.L().Debug("Match message", zap.String("Match", fmt.Sprintf("%v", matchMessage.ToString())), zap.String("price", matchMessage.Price.String()))
		err = handler.book.Match(matchMessage)
		if err != nil {
			return errors.Wrap(err, "Could not process match message")
		}
		handler.flushBookUpdate(message)
		handler.flushTradeTick(matchMessage)
		return nil
	case "change":
		changeMessage, err := NewChangeMessage(message.Time, message.Sequence, message.OrderId, message.ProductId, message.NewSize, message.OldSize, message.Price, message.Side)
		if err != nil {
			return errors.Wrap(err, "Could not create change message")
		}
		zap.L().Debug("Change message", zap.String("Change", fmt.Sprintf("%v", changeMessage)))
		err = handler.book.Change(changeMessage)
		if err != nil {
			return errors.Wrap(err, "Could not process change message")
		}
		handler.flushBookUpdate(message)
		return nil
	}
	return nil
}

func (handler *Handler) SyncBook() error {
	snapshotBook, err := handler.client.GetBook(handler.book.ID, 3)
	if err != nil {
		return errors.Wrap(err, "Could not get a snapshot of the book")
	}
	handler.sequence = int64(snapshotBook.Sequence)

	for _, bid := range snapshotBook.Bids {
		price, err := decimal.NewFromString(bid.Price)
		if err != nil {
			return errors.Wrap(err, "Could not convert Price to decimal")
		}
		size, err := decimal.NewFromString(bid.Size)
		if err != nil {
			return errors.Wrap(err, "Could not convert bid size to decimal")
		}
		order := &Order{
			ID:    bid.OrderId,
			Size:  size,
			Price: price,
			Side:  common.BidSide,
		}
		zap.L().Debug(fmt.Sprintf("Snapshot bid: %v", bid))
		handler.flushRawFeedMessage(order.ToJSON())
		handler.book.Add(order)
	}
	for _, ask := range snapshotBook.Asks {
		price, err := decimal.NewFromString(ask.Price)
		if err != nil {
			return errors.Wrap(err, "Could not convert ask Price to decimal")
		}
		size, err := decimal.NewFromString(ask.Size)
		if err != nil {
			return errors.Wrap(err, "Could not convert ask size to decimal")
		}
		order := &Order{
			ID:    ask.OrderId,
			Size:  size,
			Price: price,
			Side:  common.AskSide,
		}
		zap.L().Debug(fmt.Sprintf("Snapshot ask: %v", ask))
		handler.flushRawFeedMessage(order.ToJSON())
		handler.book.Add(order)
	}
	return nil
}

func (handler *Handler) flushRawFeedMessage(message string) {
	for _, listener := range handler.rawFeedListeners {
		listener.Message(message)
	}
}

func (handler *Handler) flushBookUpdate(message gdaxClient.Message) {
	for _, listener := range handler.bookListeners {
		listener.BookUpdate(message)
	}
}

func (handler *Handler) flushTradeTick(msg Match) {
	for _, listener := range handler.bookListeners {
		listener.TradeTick(msg)
	}
}

func (handler *Handler) flushClear() {
	for _, listener := range handler.bookListeners {
		listener.Clear()
	}
}

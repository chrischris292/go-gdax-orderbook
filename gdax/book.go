package gdax

import (
	"fmt"
	"strings"

	"github.com/chrischris292/go-gdax-orderbook/common"
	"github.com/jabong/florest-core/src/common/collections/maps/linkedhashmap"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type Book struct {
	ID     string
	Bid    *BookSide
	Ask    *BookSide
	Trades []*Order
}

func NewBook(id string) *Book {
	b := &Book{
		ID:     id,
		Bid:    NewBookSide(common.BidSide),
		Ask:    NewBookSide(common.AskSide),
		Trades: []*Order{},
	}
	return b
}

func (b *Book) Clear() {
	b.Bid = NewBookSide(common.BidSide)
	b.Ask = NewBookSide(common.AskSide)
}

func (b *Book) Add(order *Order) {
	level, found := b.FindLevel(order.Price, order.Side)
	if found {
		level.Add(order)
	} else {
		b.addLevel(order)
	}
}

// Handles match messages
func (b *Book) Remove(order *Order) error {
	if _, found := b.FindLevel(order.Price, order.Side); !found {
		zap.L().Debug("Match message for unknown level", zap.String("order", order.ToString()))
		return nil
	}

	// ignore orders that are not in order book
	level, _ := b.FindLevel(order.Price, order.Side)
	if !level.Has(order.ID) {
		zap.L().Debug("Match message for unknown id", zap.String("order", order.ToString()), zap.String("order", order.ToString()))
		return nil
	}
	err := level.Remove(order.ID)
	if err != nil {
		return fmt.Errorf("Could not remove order %v", order.ToString())
	}

	if level.Empty() {
		b.removeLevel(order.Side, level)
	}
	return nil
}

func (b *Book) FindLevel(price decimal.Decimal, side common.Side) (*BookLevel, bool) {
	if side == common.BidSide {
		return b.Bid.GetBookLevel(price)
	} else {
		return b.Ask.GetBookLevel(price)
	}

	return &BookLevel{}, false
}

func (b *Book) Match(message Match) error {
	bl, ok := b.FindLevel(message.Price, message.MatchSide)
	if !ok {
		zap.L().Warn("Could not find level at price", zap.String("price", message.Price.String()))
		return nil
	}
	order, err := bl.GetFirstOrder()
	if err != nil {
		return errors.Wrap(err, "match message is for an order ID that doesn't exist in book")
	}

	if order.ID != message.MakerOrderID {
		return errors.Wrap(err, "match message is for an order ID that doesn't exist in book")
	}

	if order.Size.Equal(message.Size) {
		b.Remove(order)
	}
	return nil
}

func (b *Book) Change(message Change) error {
	order, err := b.FindOrder(message.OrderID, message.Price, message.Side)
	if err != nil {
		zap.L().Error("Could not change order as we could not find order", zap.String("id", message.OrderID))
		return err
	}
	if order.Size != message.OldSize {
		zap.L().Error("change message indicates new size and old size are not consistent with order book",
			zap.String("id", message.OrderID),
			zap.String("order", order.ToString()),
			zap.String("oldsize", message.OldSize.String()))
		return errors.Wrap(err, "change message indicates new size and old size are not consistent with order book")
	}
	order.Size = message.NewSize
	return nil
}

func (b *Book) FindOrder(id string, price decimal.Decimal, side common.Side) (*Order, error) {
	level, ok := b.FindLevel(price, side)
	if !ok {
		zap.L().Error("Could not find level", zap.String("price", price.String()))
		return nil, errors.New("Could not find level")
	}
	order, err := level.Get(id)
	if err != nil {
		return nil, fmt.Errorf("Could not find order (%s) at price (%v) due to error (%s)", id, price, err.Error())
	}
	return order, nil
}

func (b *Book) addLevel(order *Order) {
	level := &BookLevel{Price: order.Price, Size: decimal.New(0, 0), Orders: linkedhashmap.New()}
	level.Add(order)
	if order.Side == common.BidSide {
		b.Bid.AddBookLevel(order.Price, level)
	} else {
		b.Ask.AddBookLevel(order.Price, level)
	}
}

func (b *Book) removeLevel(side common.Side, level *BookLevel) {
	if side == common.BidSide {
		b.Bid.RemoveBookLevel(level.Price)
	} else {
		b.Ask.RemoveBookLevel(level.Price)
	}
}

func (b *Book) PrintTopFive() {
	askString, bidString := b.GetTopFive()
	zap.L().Info(bidString)
	zap.L().Info(askString)
}

func (b *Book) GetTopFive() (string, string) {
	asks := []string{}
	bids := []string{}
	it := b.Ask.PriceToLevels.Iterator()
	i := 0
	for it.Next() {
		if i == 5 {
			break
		}
		key, value := it.Key(), it.Value()
		price, _ := key.(decimal.Decimal)
		level, _ := value.(*BookLevel)
		size := level.GetSize()
		orders := level.GetNumOrders()
		levelString := fmt.Sprintf("[Price: %s Size: %s Orders: %v]", price.String(), size.String(), orders)
		asks = append(asks, levelString)
		i += 1
	}
	i = 0
	it = b.Bid.PriceToLevels.Iterator()
	for it.End(); it.Prev(); {
		if i == 5 {
			break
		}
		key, value := it.Key(), it.Value()
		price, _ := key.(decimal.Decimal)
		level, _ := value.(*BookLevel)
		size := level.GetSize()
		orders := level.GetNumOrders()
		levelString := fmt.Sprintf("[Price: %s Size: %s Orders: %v]", price.String(), size.String(), orders)
		bids = append(bids, levelString)
		i += 1
	}
	askString := "Asks: " + strings.Join(asks, " ")
	bidString := "Bids: " + strings.Join(bids, " ")
	return askString, bidString
}

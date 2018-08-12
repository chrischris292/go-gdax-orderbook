package gdax

import (
	"encoding/json"
	"fmt"

	"github.com/chrischris292/go-gdax-orderbook/common"
	raven "github.com/getsentry/raven-go"
	"github.com/pkg/errors"
	gdaxClient "github.com/preichenberger/go-gdax"
	"github.com/shopspring/decimal"
)

type Order struct {
	ID    string
	Size  decimal.Decimal
	Price decimal.Decimal
	Side  common.Side
}

const SnapshotMessageType = "snapshot"

type HandlerConsumer interface {
	BookUpdate(message gdaxClient.Message)
	TradeTick(msg Match)
	Clear()
}

type RawFeedListener interface {
	Message(string)
}

func NewOrder(id string, size string, price string, side string) (*Order, error) {
	orderSide, err := ToSide(side)
	if err != nil {
		return &Order{}, errors.Wrap(err, "could not convert side string to side")
	}
	sizeDec, err := decimal.NewFromString(size)
	if err != nil {
		return nil, errors.Wrap(err, "Could not convert size to decimal")
	}
	priceDec, err := decimal.NewFromString(price)
	if err != nil {
		return nil, errors.Wrap(err, "Could not convert Price to decimal")
	}

	return &Order{ID: id, Size: sizeDec, Price: priceDec, Side: orderSide}, nil
}

func (order *Order) ToString() string {
	return fmt.Sprintf("ID: %s, Size: %s, Price: %s, Side: %s", order.ID, order.Size.String(), order.Price.String(), common.ToString(order.Side))
}

func (order *Order) ToJSON() string {
	orderMap := map[string]string{
		"order_id": order.ID,
		"size":     order.Size.String(),
		"price":    order.Price.String(),
		"side":     common.ToString(order.Side),
		"type":     SnapshotMessageType,
	}
	jsonStr, _ := json.Marshal(orderMap)
	return string(jsonStr)
}

func NewOrderFromDecimal(id string, size decimal.Decimal, price decimal.Decimal, side string) (*Order, error) {
	orderSide, err := ToSide(side)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		return &Order{}, err
	}
	return &Order{ID: id, Size: size, Price: price, Side: orderSide}, nil
}

func ToSide(side string) (common.Side, error) {
	if side == "buy" {
		return common.BidSide, nil
	} else if side == "sell" {
		return common.AskSide, nil
	} else {
		return common.BidSide, fmt.Errorf("Side %s is not supported", side)
	}
}

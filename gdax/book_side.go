package gdax

import (
	"fmt"

	"github.com/chrischris292/go-gdax-orderbook/common"
	"github.com/emirpasic/gods/maps/treemap"
	"github.com/jabong/florest-core/src/common/collections/maps/linkedhashmap"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type BookSide struct {
	PriceToLevels *treemap.Map
	side          common.Side
	volume        decimal.Decimal
}

var errOrderNotFound = errors.New("Could not find order")

func DecimalComparator(a, b interface{}) int {
	aDec, _ := a.(decimal.Decimal)
	bDec, _ := b.(decimal.Decimal)
	if aDec.GreaterThan(bDec) {
		return 1
	} else if aDec.LessThan(bDec) {
		return -1
	}
	return 0
}

func NewBookSide(side common.Side) *BookSide {
	return &BookSide{treemap.NewWith(DecimalComparator), side, decimal.Decimal{}}
}

func (b *BookSide) Add(order *Order) {
	level, found := b.GetBookLevel(order.Price)
	if found {
		level.Add(order)
	} else {
		b.addLevel(order)
	}
	b.volume.Add(order.Size)
}

func (b *BookSide) Change(message Change) error {
	order, err := b.FindOrder(message.OrderID, message.Price)
	if err != nil {
		return errors.Wrap(err, "Could not change order as we could not find order: "+message.OrderID)
	}
	if order.Size != message.OldSize {
		zap.L().Error("change message indicates new size and old size are not consistent with order book...logic for handler is incorrect.",
			zap.String("id", message.OrderID),
			zap.String("order", order.ToString()),
			zap.String("oldsize", message.OldSize.String()))
		return errors.New("change message indicates new size and old size are not consistent with order book...logic for handler is incorrect.")
	}

	// update bookside volume
	// new - old + curr_volume = newVolume
	diff := message.NewSize.Sub(order.Size)
	b.volume.Add(diff)

	// update order size
	order.Size = message.NewSize

	return nil
}

func (b *BookSide) Remove(order *Order) error {
	level, _ := b.GetBookLevel(order.Price)
	if !level.Has(order.ID) {
		return errOrderNotFound
	}
	err := level.Remove(order.ID)
	if err != nil {
		return fmt.Errorf("Could not remove order %v", order.ToString())
	}
	b.volume.Sub(order.Size)

	if level.Empty() {
		b.RemoveBookLevel(level.Price)
	}
	return nil
}

func (b *BookSide) addLevel(order *Order) {
	level := &BookLevel{Price: order.Price, Size: decimal.New(0, 0), Orders: linkedhashmap.New()}
	level.Add(order)
	b.AddBookLevel(order.Price, level)
	b.volume.Add(order.Size)
}

func (bookSide *BookSide) HasBookLevel(price decimal.Decimal) bool {
	_, found := bookSide.PriceToLevels.Get(price)
	return found
}

func (bookSide *BookSide) AddBookLevel(price decimal.Decimal, level *BookLevel) error {
	if bookSide.HasBookLevel(price) {
		return fmt.Errorf("BookSide %s already has book level %s", common.ToString(bookSide.side), price.String())
	}
	bookSide.PriceToLevels.Put(price, level)
	return nil
}

func (bookSide *BookSide) GetBookLevel(price decimal.Decimal) (*BookLevel, bool) {
	bookLevel, found := bookSide.PriceToLevels.Get(price)
	if !found {
		return nil, false
	}
	retVal, ok := bookLevel.(*BookLevel)
	if !ok {
		zap.L().Error(fmt.Sprintf("BookSide %s could not convert book level at price %s.", common.ToString(bookSide.side), price.String()))
		return nil, false
	}
	return retVal, true
}

func (bookSide *BookSide) RemoveBookLevel(price decimal.Decimal) error {
	if !bookSide.HasBookLevel(price) {
		return fmt.Errorf("BookSide %s could not remove book level as book level at price %s does not exist", common.ToString(bookSide.side), price.String())
	}
	bookSide.PriceToLevels.Remove(price)
	return nil
}

func (bookSide *BookSide) GetTopLevel() (*BookLevel, error) {
	if bookSide.PriceToLevels.Empty() {
		return nil, fmt.Errorf("BookSide %s is empty. Tried to get top level", common.ToString(bookSide.side))
	}
	if bookSide.side == common.BidSide {
		_, level := bookSide.PriceToLevels.Max()
		retVal, ok := level.(*BookLevel)
		if !ok {
			return nil, fmt.Errorf("PriceToLevels has a corrupt level for bookSide %s. Tried to get top level", common.ToString(bookSide.side))
		}
		return retVal, nil
	} else {
		_, level := bookSide.PriceToLevels.Min()
		retVal, ok := level.(*BookLevel)
		if !ok {
			return nil, fmt.Errorf("PriceToLevels has a corrupt level for bookSide %s. Tried to get top level", common.ToString(bookSide.side))
		}
		return retVal, nil
	}
}

func (bookSide *BookSide) FindOrder(id string, price decimal.Decimal) (*Order, error) {
	level, ok := bookSide.GetBookLevel(price)
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

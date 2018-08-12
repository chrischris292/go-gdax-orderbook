package gdax

import (
	"fmt"

	"github.com/jabong/florest-core/src/common/collections/maps/linkedhashmap"
	"github.com/shopspring/decimal"
)

type BookLevel struct {
	Price  decimal.Decimal
	Orders *linkedhashmap.Map
	Size   decimal.Decimal
}

func (bl *BookLevel) Add(order *Order) {
	bl.Orders.Put(order.ID, order)
	bl.Size = bl.Size.Add(order.Size)
}

func (bl *BookLevel) Remove(id string) error {
	order, err := bl.Get(id)
	if err != nil {
		return err
	}
	bl.Orders.Remove(order.ID)
	bl.Size = bl.Size.Sub(order.Size)
	return nil
}

func (bl *BookLevel) GetSize() decimal.Decimal {
	return bl.Size
}

func (bl *BookLevel) GetNumOrders() int {
	return bl.Orders.Size()
}

func (bl *BookLevel) Empty() bool {
	return bl.Orders.IsEmpty()
}

func (bl *BookLevel) Get(orderID string) (retVal *Order, _ error) {
	found := bl.Orders.Contains(orderID)
	if !found {
		return nil, fmt.Errorf("OrderID not found")
	}
	order, _ := bl.Orders.Get(orderID)
	retVal, ok := order.(*Order)
	if !ok {
		return nil, fmt.Errorf("Could not parse order pointer in book level map")
	}
	return retVal, nil
}

func (bl *BookLevel) Has(orderID string) bool {
	return bl.Orders.Contains(orderID)
}

func (bl *BookLevel) GetFirstOrder() (retVal *Order, _ error) {
	if bl.Orders.IsEmpty() {
		return nil, fmt.Errorf("book level is empty")
	}
	order := bl.Orders.Iterator().Next().GetValue()
	retVal, ok := order.(*Order)
	if !ok {
		return nil, fmt.Errorf("Could not parse order pointer in book level map")
	}
	return retVal, nil
}

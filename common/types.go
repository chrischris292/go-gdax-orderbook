package common

import (
	"github.com/shopspring/decimal"
)

type Side uint8

const BidSide Side = 0
const AskSide Side = 1

type Order struct {
	ID    string
	Size  decimal.Decimal
	Price decimal.Decimal
	Side  Side
}

func ToString(side Side) string {
	if side == BidSide {
		return "BID"
	} else {
		return "ASK"
	}
}

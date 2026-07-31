package models

import "time"

// 1.avoid floating point number in financial systems (ie 0.1 + 0.3 != 0.30000000000000004)
// instead represent prices as integers(uint64) uisng smallest currency unit (ie cents) e.g $100.50

//2.Price-Time Priority (FIFO at each Price Level)
//i).Price Priority: Higher buy offers match before lower ones; lower sell offers match before higher ones.
//ii).Time Priority: Among orders at the exact same price level, the earliest placed order matches first.
//iii).Bids(buys) are sorted from highest prices to lowest
//iv).Asks(sells) are sorted from lowest to highest
//-If prices are equal ,orders that arrive first are matched

type Side int

const (
	SideBuy Side = iota
	SideSell
)

func (s Side) String() string {
	if s == SideBuy {
		return "BUY"
	}
	return "SELL"
}

type Order struct {
	ID        uint64    `json:"id"`
	ClientId  uint64    `json:"client_id"`
	Symbol    string    `json:"symbol"`
	Side      Side      `json:"side"`
	Price     uint64    `json:"price"`
	Quantity  uint64    `json:"quantity"`
	Timestamp time.Time `json:"timestamp"`
}

// Trade reps an execution match between buyer and seller
// BuyOrder and SellOrder vs MakerOrder and TakerOrder
// MakerOrder and TakerOrder -> describe how order interact with the order book
// BuyOrder and SellOrder -> Basically describes what the trader wants
// BuyerOrderID -> who bought, SellerOrderID -> who sold MakerOrderId->identifies whose order was a
// already resting in the order book, TakerOrderID -> who arrived later and matched the order
type Trade struct {
	BuyOrderID   uint64 `json:"buyorderid"`
	SellOrderID  uint64 `json:"sellorderid"`
	MakerOrderID uint64
	TakerOrderID uint64
	Price        uint64
	Quantity     uint64
	Timestamp    time.Time
}

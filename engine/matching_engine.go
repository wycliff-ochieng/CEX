package engine

import (
	"sync"
	"time"
)

//matching engine
//purpose : 1.conduct trades by matching sellers and buyers
//2.Client specific sequencing -
// global sequence alignment providing a single.globally ordered list of all operations that every system components eventually agrees upon
//3.Orrder book operations( users should be able submit,modify,cancell orders )
//order types - limit orders and market orders

//Trades,Order,OrderType,OrderBook,bestBid(),bestAsk()

type Order struct {
	ID        string
	Side      string //buy or sell
	Type      OrderType
	Price     float32
	Quantity  float32
	Remaining float32
	TimeStamp time.Time
}

type OrderType struct {
	LimitOrders  float32 `json:"limitorders"`  //spcify maximum buy price / minimum sell price
	MarketOrders float32 `json:"marketorders"` //executed immediatelyn at best available price
}

type Trade struct {
	BuyOrderID  string
	SellOrderID string
	Price       float32
	Quantity    float32
	TimeStamp   time.Time
}

type OrderBook struct {
	mu   sync.Mutex
	Bids []*Order
	Asks []*Order
} //limit order book (LOB)

type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: make([]*Order, 0),
		Asks: make([]*Order, 0),
	}
}


func(ob *OrderBook) SubmitOrder(order *Order) {
	
}
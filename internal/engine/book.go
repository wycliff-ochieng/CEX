package engine

import (
	"github/wycliff-ochieng/internal/models"
	"sync"
)

type OrderBook struct {
	mu   sync.Mutex
	Bids []*models.Order
	Asks []*models.Order
}

//order book operations (CRUD) LOB operations

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: make([]*models.Order, 0),
		Asks: make([]*models.Order, 0),
	}
}

// process an incoming limit order and returns executed trades
func (ob *OrderBook) ProcessLimitOrder(order *models.Order) []models.Trade {
	var trades []models.Trade

	return trades
}

// matchBuyOrder - matches incoming BUY order (bids) agains existing SELL orders (asks)
func (ob *OrderBook) matchBuyOrder(takeOrder *models.Order) []models.Trade {
	var trades []string

	var executedTrades []models.Trade

	for len(ob.Asks) > 0 && ob.Asks[0].Price <= takeOrder.Price && takeOrder.Quantity > 0 {
		bestAskLevel := ob.Asks[0]
	}

	return trades
}

func min(x int64, y int64) int64 {

	if x < y {
		return x
	}
	return y
}

//matchSellOrder

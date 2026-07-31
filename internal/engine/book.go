package engine

import (
	"github/wycliff-ochieng/internal/models"
	"sync"
	"time"
)

type PriceLevel struct {
	Price  uint64
	Orders []*models.Order
}

type OrderBook struct {
	mu   sync.Mutex
	Bids []*PriceLevel
	Asks []*PriceLevel
}

//order book operations (CRUD) LOB operations

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: make([]*PriceLevel, 0),
		Asks: make([]*PriceLevel, 0),
	}
}

// process an incoming limit order and returns executed trades
func (ob *OrderBook) ProcessLimitOrder(order *models.Order) []models.Trade {
	var trades []models.Trade

	return trades
}

// matchBuyOrder - matches incoming BUY order (bids) against existing SELL orders (asks)
func (ob *OrderBook) matchBuyOrder(takerOrder *models.Order) []models.Trade {
	var trades []string

	_ = trades //debugging

	var executedTrades []models.Trade

	for len(ob.Asks) > 0 && ob.Asks[0].Price <= takerOrder.Price && takerOrder.Quantity > 0 {
		bestAskLevel := ob.Asks[0]

		for len(bestAskLevel.Orders) > 0 && takerOrder.Quantity > 0 {
			makerOrder := bestAskLevel.Orders[0]

			matchQuantity := min(makerOrder.Quantity, takerOrder.Quantity)

			executedTrades = append(executedTrades, models.Trade{
				MakerOrderID: makerOrder.ID,
				TakerOrderID: takerOrder.ID,
				Price:        makerOrder.Price,
				Quantity:     matchQuantity,
				Timestamp:    time.Now(),
			})

			takerOrder.Quantity -= matchQuantity
			makerOrder.Quantity -= matchQuantity

			if makerOrder.Quantity == 0 {
				bestAskLevel.Orders = bestAskLevel.Orders[1:]
			}
		}

		if len(bestAskLevel.Orders) == 0 {
			ob.Asks = ob.Asks[1:]
		}

	}

	return executedTrades
}

func (ob *OrderBook) matchSellOrder(takerOrder *models.Order) []models.Trade {

	var executedTrades []models.Trade

	// While there are bids, and the highest bid price >= our sell price, and we still want to sell...
	for len(ob.Bids) > 0 && ob.Bids[0].Price >= takerOrder.Price && takerOrder.Quantity > 0 {
		bestBidLevel := ob.Bids[0]

		for len(bestBidLevel.Orders) > 0 && takerOrder.Quantity > 0 {
			makerOrder := bestBidLevel.Orders[0]
			matchQty := min(takerOrder.Quantity, makerOrder.Quantity)

			executedTrades = append(executedTrades, models.Trade{
				MakerOrderID: makerOrder.ID,
				TakerOrderID: takerOrder.ID,
				Price:        makerOrder.Price,
				Quantity:     matchQty,
				Timestamp:    time.Now(),
			})

			takerOrder.Quantity -= matchQty
			makerOrder.Quantity -= matchQty

			if makerOrder.Quantity == 0 {
				bestBidLevel.Orders = bestBidLevel.Orders[1:]
			}
		}

		if len(bestBidLevel.Orders) == 0 {
			ob.Bids = ob.Bids[1:]
		}
	}
	return executedTrades
}

func min(x uint64, y uint64) uint64 {

	if x < y {
		return x
	}
	return y
}

//matchSellOrder

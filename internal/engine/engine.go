package engine

import (
	"github/wycliff-ochieng/internal/events"
	"github/wycliff-ochieng/internal/models"
)

// ReplayEvents rebuilds an OrderBook from a sequenced event log. Each event
// records a state change the engine already committed: an order that rested,
// a trade that executed, or an order that was cancelled. Applying the same
// transitions in the same order must reproduce the same book — that is what
// "determinism" buys: replay of the log is identical to the live run.
//
// The events must be presented in sequence-number order (ascending Seq).
func ReplayEvents(log []events.Event) *OrderBook {
	ob := NewOrderBook()
	for _, e := range log {
		switch e.Type {
		case events.OrderAccepted:
			ob.replayAccepted(e.Order)
		case events.TradeExecuted:
			ob.replayTrade(e.Trade)
		case events.OrderCancelled:
			ob.replayCancelled(e.Order)
		}
	}
	return ob
}

// replayAccepted puts the rested order back onto its side, exactly as the
// engine did when it first accepted it.
func (ob *OrderBook) replayAccepted(order *models.Order) {
	if order == nil {
		return
	}
	if order.Side == models.SideBuy {
		ob.addRestingOrder(&ob.Bids, order, false)
	} else {
		ob.addRestingOrder(&ob.Asks, order, true)
	}
}

// replayTrade applies a fill to the resting maker: its quantity drops, and if
// it is exhausted it leaves the book exactly as the live engine removed it.
func (ob *OrderBook) replayTrade(trade *models.Trade) {
	if trade == nil {
		return
	}
	node, ok := ob.index[trade.MakerOrderID]
	if !ok {
		return
	}
	node.Order.Quantity -= trade.Quantity
	if node.Order.Quantity == 0 {
		delete(ob.index, node.Order.ID)
		level := node.Level
		unlink(level, node)
		if level.Head == nil {
			if node.IsBid {
				removeLevel(&ob.Bids, level)
			} else {
				removeLevel(&ob.Asks, level)
			}
		}
	}
}

// replayCancelled removes the cancelled order from the book.
func (ob *OrderBook) replayCancelled(order *models.Order) {
	if order == nil {
		return
	}
	ob.CancelOrder(order.ID)
}

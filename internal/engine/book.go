package engine

import (
	"sync"

	"github/wycliff-ochieng/internal/events"
	"github/wycliff-ochieng/internal/models"
)

// OrderNode is a stable address for one resting order in a price level.
// The book's index maps order id → node, and unlink is pointer surgery:
// removing a node never disturbs its neighbours.
type OrderNode struct {
	Order *models.Order
	Level *PriceLevel
	IsBid bool
	Prev  *OrderNode
	Next  *OrderNode
}

// PriceLevel holds a FIFO queue of orders at one price, as a doubly linked
// list so that cancellation from the middle is O(1).
type PriceLevel struct {
	Price uint64
	Head  *OrderNode
	Tail  *OrderNode
}

type OrderBook struct {
	mu    sync.Mutex
	Bids  []*PriceLevel
	Asks  []*PriceLevel
	index map[uint64]*OrderNode
}

// NewOrderBook creates an empty book with an empty order index.
func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids:  make([]*PriceLevel, 0),
		Asks:  make([]*PriceLevel, 0),
		index: make(map[uint64]*OrderNode),
	}
}

// ProcessLimitOrder processes an incoming limit order: it crosses against the
// opposite side, then rests any unfilled remainder on its own side at its
// limit price. The caller's order is treated as a command — it is never
// mutated and never stored in the book.
//
// The returned events describe every state change caused by this one order:
// one TRADE event per fill, and one ACCEPTED event if a remainder rested.
// Events are returned unsequenced; the caller stamps them with a sequencer.
func (ob *OrderBook) ProcessLimitOrder(order *models.Order) []events.Event {
	working := *order

	if working.Side == models.SideBuy {
		evs := ob.matchBuyOrder(&working)
		if working.Quantity > 0 {
			ob.addRestingOrder(&ob.Bids, &working, false)
			evs = append(evs, acceptEvent(order, working.Quantity))
		}
		return evs
	}

	evs := ob.matchSellOrder(&working)
	if working.Quantity > 0 {
		ob.addRestingOrder(&ob.Asks, &working, true)
		evs = append(evs, acceptEvent(order, working.Quantity))
	}
	return evs
}

// ProcessMarketOrder processes an incoming market order: it crosses against
// the best opposite prices until fully filled or the book is dry. A market
// order never rests — any unfilled remainder is not placed (immediate-or-
// cancel semantics). The caller's order is treated as a command.
func (ob *OrderBook) ProcessMarketOrder(order *models.Order) []events.Event {
	working := *order
	working.Type = models.OrderTypeMarket
	working.Price = 0

	if working.Side == models.SideBuy {
		return ob.matchBuyOrder(&working)
	}
	return ob.matchSellOrder(&working)
}

// CancelOrder removes the resting order with the given id from the book in
// O(1). It reports whether an order was found and removed, and emits a single
// CANCELLED event for it. A cancelled order can never trade.
func (ob *OrderBook) CancelOrder(orderID uint64) (bool, []events.Event) {
	node, ok := ob.index[orderID]
	if !ok {
		return false, nil
	}

	level := node.Level
	unlink(level, node)
	if level.Head == nil {
		if node.IsBid {
			removeLevel(&ob.Bids, level)
		} else {
			removeLevel(&ob.Asks, level)
		}
	}
	delete(ob.index, orderID)

	cancelled := *node.Order
	return true, []events.Event{{Type: events.OrderCancelled, Order: &cancelled}}
}

// acceptEvent snapshots the order as it rests: same fields as the caller's
// command, but with the reduced (remainder) quantity.
func acceptEvent(order *models.Order, restQty uint64) events.Event {
	rest := *order
	rest.Quantity = restQty
	return events.Event{Type: events.OrderAccepted, Order: &rest}
}

// addRestingOrder places the unfilled remainder onto one side of the book,
// keeping the side sorted best-price-first (bids high→low, asks low→high) and
// FIFO within a price level. ascending selects the sort direction.
func (ob *OrderBook) addRestingOrder(levels *[]*PriceLevel, order *models.Order, ascending bool) {
	for i, level := range *levels {
		switch {
		case level.Price == order.Price:
			ob.appendOrder(level, order, ascending)
			return
		case ascending && level.Price > order.Price:
			ob.insertLevel(levels, i, order, ascending)
			return
		case !ascending && level.Price < order.Price:
			ob.insertLevel(levels, i, order, ascending)
			return
		}
	}
	ob.insertLevel(levels, len(*levels), order, ascending)
}

// insertLevel inserts a new price level at index i holding a copy of order.
func (ob *OrderBook) insertLevel(levels *[]*PriceLevel, i int, order *models.Order, ascending bool) {
	level := &PriceLevel{Price: order.Price}
	ob.appendOrder(level, order, ascending)
	*levels = append(*levels, nil)
	copy((*levels)[i+1:], (*levels)[i:])
	(*levels)[i] = level
}

// appendOrder copies order into a new node, appends it to the tail of the
// level's FIFO queue, and registers it in the order index.
func (ob *OrderBook) appendOrder(level *PriceLevel, order *models.Order, ascending bool) {
	rest := *order
	node := &OrderNode{Order: &rest, Level: level, IsBid: !ascending}
	if level.Tail == nil {
		level.Head, level.Tail = node, node
	} else {
		node.Prev = level.Tail
		level.Tail.Next = node
		level.Tail = node
	}
	ob.index[node.Order.ID] = node
}

// unlink removes node from its level's list in O(1); neighbours keep their
// links untouched.
func unlink(level *PriceLevel, node *OrderNode) {
	if node.Prev != nil {
		node.Prev.Next = node.Next
	} else {
		level.Head = node.Next
	}
	if node.Next != nil {
		node.Next.Prev = node.Prev
	} else {
		level.Tail = node.Prev
	}
	node.Prev, node.Next = nil, nil
}

// removeLevel drops a price level from a side's slice once it has no orders.
func removeLevel(levels *[]*PriceLevel, level *PriceLevel) {
	for i, l := range *levels {
		if l == level {
			*levels = append((*levels)[:i], (*levels)[i+1:]...)
			return
		}
	}
}

// matchBuyOrder matches an incoming BUY order against the best asks, emitting
// one TRADE event per fill.
func (ob *OrderBook) matchBuyOrder(takerOrder *models.Order) []events.Event {
	var evs []events.Event

	for len(ob.Asks) > 0 && takerOrder.Quantity > 0 &&
		(takerOrder.Type == models.OrderTypeMarket || ob.Asks[0].Price <= takerOrder.Price) {
		bestAskLevel := ob.Asks[0]

		for bestAskLevel.Head != nil && takerOrder.Quantity > 0 {
			maker := bestAskLevel.Head

			matchQuantity := min(maker.Order.Quantity, takerOrder.Quantity)

			evs = append(evs, events.Event{
				Type: events.TradeExecuted,
				Trade: &models.Trade{
					MakerOrderID: maker.Order.ID,
					TakerOrderID: takerOrder.ID,
					Price:        maker.Order.Price,
					Quantity:     matchQuantity,
				},
			})

			takerOrder.Quantity -= matchQuantity
			maker.Order.Quantity -= matchQuantity

			if maker.Order.Quantity == 0 {
				delete(ob.index, maker.Order.ID)
				bestAskLevel.Head = maker.Next
				if bestAskLevel.Head != nil {
					bestAskLevel.Head.Prev = nil
				} else {
					bestAskLevel.Tail = nil
				}
				maker.Prev, maker.Next = nil, nil
			}
		}

		if bestAskLevel.Head == nil {
			ob.Asks = ob.Asks[1:]
		}
	}

	return evs
}

// matchSellOrder matches an incoming SELL order against the best bids,
// emitting one TRADE event per fill.
func (ob *OrderBook) matchSellOrder(takerOrder *models.Order) []events.Event {
	var evs []events.Event

	for len(ob.Bids) > 0 && takerOrder.Quantity > 0 &&
		(takerOrder.Type == models.OrderTypeMarket || ob.Bids[0].Price >= takerOrder.Price) {
		bestBidLevel := ob.Bids[0]

		for bestBidLevel.Head != nil && takerOrder.Quantity > 0 {
			maker := bestBidLevel.Head

			matchQty := min(takerOrder.Quantity, maker.Order.Quantity)

			evs = append(evs, events.Event{
				Type: events.TradeExecuted,
				Trade: &models.Trade{
					MakerOrderID: maker.Order.ID,
					TakerOrderID: takerOrder.ID,
					Price:        maker.Order.Price,
					Quantity:     matchQty,
				},
			})

			takerOrder.Quantity -= matchQty
			maker.Order.Quantity -= matchQty

			if maker.Order.Quantity == 0 {
				delete(ob.index, maker.Order.ID)
				bestBidLevel.Head = maker.Next
				if bestBidLevel.Head != nil {
					bestBidLevel.Head.Prev = nil
				} else {
					bestBidLevel.Tail = nil
				}
				maker.Prev, maker.Next = nil, nil
			}
		}

		if bestBidLevel.Head == nil {
			ob.Bids = ob.Bids[1:]
		}
	}

	return evs
}

func min(x uint64, y uint64) uint64 {
	if x < y {
		return x
	}
	return y
}

package gateway

import (
	"log"

	"github/wycliff-ochieng/internal/engine"
	"github/wycliff-ochieng/internal/events"
	"github/wycliff-ochieng/internal/models"
	"github/wycliff-ochieng/internal/sequencer"
)

// Request wraps an incoming API request to the engine thread.
type Request struct {
	Order   *models.Order
	IsClose bool // Set to true to shutdown the gateway
}

type OrderGateway struct {
	ob          *engine.OrderBook
	seq         *sequencer.Sequencer
	inbox       chan Request
	eventStream chan<- events.Event
}

func NewOrderGateway(eventStream chan<- events.Event) *OrderGateway {
	return &OrderGateway{
		ob:          engine.NewOrderBook(),
		seq:         sequencer.New(),
		inbox:       make(chan Request, 1000),
		eventStream: eventStream,
	}
}

// Start runs the single-threaded Hot Path loop.
func (g *OrderGateway) Start() {
	log.Println("Order Gateway started. Engine is accepting orders.")
	for req := range g.inbox {
		if req.IsClose {
			close(g.eventStream)
			break
		}

		// Validate (basic)
		if req.Order.Price == 0 && req.Order.Type == models.OrderTypeLimit {
			log.Printf("Rejecting invalid limit order price=0")
			continue
		}
		if req.Order.Quantity == 0 {
			log.Printf("Rejecting zero quantity order")
			continue
		}

		// 1. Process against local In-Memory Engine
		var evs []events.Event
		if req.Order.Type == models.OrderTypeMarket {
			evs = g.ob.ProcessMarketOrder(req.Order)
		} else {
			evs = g.ob.ProcessLimitOrder(req.Order)
		}

		// 2. Global Sequence Assignment
		evs = g.seq.Stamp(evs)

		// 3. Publish sequentially to downstream event stream
		for _, e := range evs {
			g.eventStream <- e
		}
	}
}

// SubmitOrder puts an order into the inbox. Will be called by Fiber HTTP handlers.
func (g *OrderGateway) SubmitOrder(order *models.Order) {
	g.inbox <- Request{Order: order}
}

func (g *OrderGateway) Shutdown() {
	g.inbox <- Request{IsClose: true}
}

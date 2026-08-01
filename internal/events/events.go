package events

import "github/wycliff-ochieng/internal/models"

// Type is the kind of state change an event records.
type Type uint8

const (
	// OrderAccepted means an order (or its remainder) joined the book.
	OrderAccepted Type = iota
	// TradeExecuted means two orders crossed and a trade happened.
	TradeExecuted
	// OrderCancelled means a resting order was removed from the book.
	OrderCancelled
)

func (t Type) String() string {
	switch t {
	case OrderAccepted:
		return "ACCEPTED"
	case TradeExecuted:
		return "TRADE"
	case OrderCancelled:
		return "CANCELLED"
	}
	return "UNKNOWN"
}

// Event is one atomic, immutable unit of engine output. It carries either the
// order that changed (ACCEPTED / CANCELLED) or the trade that executed.
// Sequence numbers are assigned by the sequencer, not the engine.
type Event struct {
	Seq   uint64
	Type  Type
	Order *models.Order
	Trade *models.Trade
}

// Trades extracts just the executed trades from a batch of events.
func Trades(evs []Event) []models.Trade {
	out := make([]models.Trade, 0, len(evs))
	for _, e := range evs {
		if e.Type == TradeExecuted {
			out = append(out, *e.Trade)
		}
	}
	return out
}

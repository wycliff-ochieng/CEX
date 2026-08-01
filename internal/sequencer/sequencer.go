package sequencer

import (
	"sync"

	"github/wycliff-ochieng/internal/events"
)

// Sequencer hands out monotonically increasing global sequence numbers.
// It is the single point that decides "what happened in what order", so every
// component downstream (replay, replication, market data) can agree on a
// total order of events without ever consulting a wall clock.
type Sequencer struct {
	mu  sync.Mutex
	seq uint64
}

// New creates a sequencer that will assign 1, 2, 3, …
func New() *Sequencer {
	return &Sequencer{}
}

// Next returns the next sequence number.
func (s *Sequencer) Next() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return s.seq
}

// Stamp assigns consecutive sequence numbers to a batch of events, preserving
// their order. Stamping a whole order's events in one call keeps them atomic:
// no other goroutine can interleave a number into the middle of the batch.
func (s *Sequencer) Stamp(evs []events.Event) []events.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range evs {
		s.seq++
		evs[i].Seq = s.seq
	}
	return evs
}

package engine_test

import (
	"fmt"
	"strings"
	"testing"

	"github/wycliff-ochieng/internal/engine"
	"github/wycliff-ochieng/internal/events"
	"github/wycliff-ochieng/internal/models"
	"github/wycliff-ochieng/internal/sequencer"
)

func mkOrder(id uint64, side models.Side, price, qty uint64) *models.Order {
	return &models.Order{ID: id, ClientId: 1, Symbol: "AAPL", Side: side, Price: price, Quantity: qty}
}

// trades extracts just the executed trades from a batch of events.
func trades(evs []events.Event) []models.Trade {
	return events.Trades(evs)
}

// ordersAt walks a price level's queue head→tail, preserving FIFO order.
func ordersAt(level *engine.PriceLevel) []*models.Order {
	var out []*models.Order
	for n := level.Head; n != nil; n = n.Next {
		out = append(out, n.Order)
	}
	return out
}

// inBook reports whether an order id is currently resting anywhere.
func inBook(ob *engine.OrderBook, id uint64) bool {
	for _, level := range ob.Bids {
		for _, o := range ordersAt(level) {
			if o.ID == id {
				return true
			}
		}
	}
	for _, level := range ob.Asks {
		for _, o := range ordersAt(level) {
			if o.ID == id {
				return true
			}
		}
	}
	return false
}

// fingerprint is a canonical string of a book's entire state: every level on
// both sides, and every order in each level's FIFO queue. Two books are
// equivalent exactly when their fingerprints are equal.
func fingerprint(ob *engine.OrderBook) string {
	var b strings.Builder
	for _, side := range []struct {
		levels []*engine.PriceLevel
		label  string
	}{{ob.Bids, "B"}, {ob.Asks, "A"}} {
		for _, l := range side.levels {
			fmt.Fprintf(&b, "%s%d[", side.label, l.Price)
			for _, o := range ordersAt(l) {
				fmt.Fprintf(&b, "%d:%d,", o.ID, o.Quantity)
			}
			b.WriteString("];")
		}
	}
	return b.String()
}

// runWorkload replays a fixed script of commands through a fresh book,
// stamping every order's events through a fresh sequencer. It returns the
// book and the fully sequenced event log. Because both are built purely from
// the same input, they are the object of the determinism checks.
func runWorkload() (*engine.OrderBook, []events.Event) {
	ob := engine.NewOrderBook()
	seq := sequencer.New()
	var log []events.Event

	step := func(order *models.Order) {
		log = append(log, seq.Stamp(ob.ProcessLimitOrder(order))...)
	}
	step(mkOrder(1, models.SideSell, 10100, 10))
	step(mkOrder(2, models.SideSell, 10200, 30))
	step(mkOrder(3, models.SideSell, 10200, 5))
	step(mkOrder(4, models.SideBuy, 10000, 25))
	step(mkOrder(5, models.SideBuy, 9900, 15))
	step(mkOrder(6, models.SideBuy, 10200, 40))
	step(mkOrder(7, models.SideSell, 9900, 7))
	step(mkOrder(8, models.SideBuy, 9800, 3))
	return ob, log
}

func TestNewBookStartsEmpty(t *testing.T) {
	ob := engine.NewOrderBook()
	if len(ob.Bids) != 0 || len(ob.Asks) != 0 {
		t.Fatalf("expected empty book, got %d bids %d asks", len(ob.Bids), len(ob.Asks))
	}
}

func TestFullFillSingleLevel(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10100, 10))

	trades := trades(ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 10100, 10)))

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Price != 10100 || trades[0].Quantity != 10 {
		t.Fatalf("unexpected trade %+v", trades[0])
	}
	if trades[0].MakerOrderID != 1 || trades[0].TakerOrderID != 2 {
		t.Fatalf("maker/taker ids wrong: %+v", trades[0])
	}
	if len(ob.Asks) != 0 {
		t.Fatalf("expected empty asks after full fill, got %d levels", len(ob.Asks))
	}
}

func TestMultiLevelSweep(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10100, 20))
	ob.ProcessLimitOrder(mkOrder(2, models.SideSell, 10200, 30))
	ob.ProcessLimitOrder(mkOrder(3, models.SideBuy, 10000, 25))
	ob.ProcessLimitOrder(mkOrder(4, models.SideBuy, 9900, 15))

	trades := trades(ob.ProcessLimitOrder(mkOrder(5, models.SideBuy, 10200, 40)))

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if trades[0].Price != 10100 || trades[0].Quantity != 20 {
		t.Fatalf("first trade wrong (want maker's 10100 x20): %+v", trades[0])
	}
	if trades[1].Price != 10200 || trades[1].Quantity != 20 {
		t.Fatalf("second trade wrong (want maker's 10200 x20): %+v", trades[1])
	}
	if len(ob.Asks) != 1 || ob.Asks[0].Price != 10200 {
		t.Fatalf("asks after sweep wrong: %+v", ob.Asks)
	}
	if orders := ordersAt(ob.Asks[0]); len(orders) != 1 || orders[0].Quantity != 10 {
		t.Fatalf("remaining ask level wrong: %+v", ob.Asks[0])
	}
	if len(ob.Bids) != 2 {
		t.Fatalf("bids should be untouched, got %d levels", len(ob.Bids))
	}
}

func TestPartialFillRestsOnOwnSide(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 9900, 10))
	ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 9800, 20))

	trades := trades(ob.ProcessLimitOrder(mkOrder(3, models.SideBuy, 9900, 30)))

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Price != 9900 || trades[0].Quantity != 10 {
		t.Fatalf("trade wrong: %+v", trades[0])
	}
	if len(ob.Asks) != 0 {
		t.Fatalf("expected empty asks, got %d levels", len(ob.Asks))
	}
	if len(ob.Bids) != 2 || ob.Bids[0].Price != 9900 || ob.Bids[1].Price != 9800 {
		t.Fatalf("bids wrong (want 9900 then 9800): %+v", ob.Bids)
	}
	if orders := ordersAt(ob.Bids[0]); len(orders) != 1 || orders[0].Quantity != 20 {
		t.Fatalf("rested remainder wrong: %+v", ob.Bids[0])
	}
}

func TestTradePrintsAtMakerPrice(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10000, 10))

	trades := trades(ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 10500, 10)))

	if len(trades) != 1 || trades[0].Price != 10000 {
		t.Fatalf("expected trade at maker price 10000, got %+v", trades)
	}
}

func TestFIFOWithinPriceLevel(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10000, 5))
	ob.ProcessLimitOrder(mkOrder(2, models.SideSell, 10000, 10))

	trades := trades(ob.ProcessLimitOrder(mkOrder(3, models.SideBuy, 10000, 8)))

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if trades[0].MakerOrderID != 1 || trades[0].Quantity != 5 {
		t.Fatalf("expected maker 1 (earlier) to fill first, got %+v", trades[0])
	}
	if trades[1].MakerOrderID != 2 || trades[1].Quantity != 3 {
		t.Fatalf("expected maker 2 to fill second, got %+v", trades[1])
	}
}

func TestRestBecomesNewBestBid(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideBuy, 9950, 20))
	ob.ProcessLimitOrder(mkOrder(2, models.SideSell, 10050, 10))

	ob.ProcessLimitOrder(mkOrder(3, models.SideBuy, 10000, 10))

	if len(ob.Bids) != 2 || ob.Bids[0].Price != 10000 || ob.Bids[1].Price != 9950 {
		t.Fatalf("bids not sorted best-first: %+v", ob.Bids)
	}
	if orders := ordersAt(ob.Bids[0]); len(orders) != 1 || orders[0].ID != 3 {
		t.Fatalf("new rest should be the new best bid: %+v", ob.Bids[0])
	}
}

func TestRestersQueueFIFOBehindEarlierOrder(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideBuy, 10000, 10))
	ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 10000, 10))

	if len(ob.Bids) != 1 {
		t.Fatalf("expected one level, got %+v", ob.Bids)
	}
	if orders := ordersAt(ob.Bids[0]); len(orders) != 2 || orders[0].ID != 1 || orders[1].ID != 2 {
		t.Fatalf("FIFO order violated: %+v", ordersAt(ob.Bids[0]))
	}
}

func TestCallerOrderIsUntouched(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 9900, 10))

	buy := mkOrder(2, models.SideBuy, 9900, 30)
	ob.ProcessLimitOrder(buy)

	if buy.Quantity != 30 {
		t.Fatalf("caller's order mutated: got quantity %d, want 30", buy.Quantity)
	}
}

func TestCancelRemovesRestingOrder(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideBuy, 10000, 10))

	ok, _ := ob.CancelOrder(1)
	if !ok {
		t.Fatal("expected cancel to succeed")
	}
	if len(ob.Bids) != 0 {
		t.Fatalf("expected empty bids after cancel, got %+v", ob.Bids)
	}
	if inBook(ob, 1) {
		t.Fatal("cancelled order still resting in the book")
	}
	if ok, _ := ob.CancelOrder(1); ok {
		t.Fatal("expected second cancel of same order to fail")
	}
}

func TestCancelUnknownOrder(t *testing.T) {
	ob := engine.NewOrderBook()
	if ok, _ := ob.CancelOrder(99); ok {
		t.Fatal("expected cancel of unknown id to fail")
	}
}

func TestCancelMiddleOrderKeepsFIFO(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideBuy, 10000, 6))
	ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 10000, 6))
	ob.ProcessLimitOrder(mkOrder(3, models.SideBuy, 10000, 6))

	if ok, _ := ob.CancelOrder(2); !ok {
		t.Fatal("expected cancel of middle order to succeed")
	}

	trades := trades(ob.ProcessLimitOrder(mkOrder(4, models.SideSell, 10000, 10)))

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades (cancelled order must not trade), got %d", len(trades))
	}
	if trades[0].MakerOrderID != 1 || trades[0].Quantity != 6 {
		t.Fatalf("expected maker 1 to fill first: %+v", trades[0])
	}
	if trades[1].MakerOrderID != 3 || trades[1].Quantity != 4 {
		t.Fatalf("expected maker 3 to fill second (order 2 skipped): %+v", trades[1])
	}
}

func TestCancelledOrderNeverTrades(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10000, 10))
	ob.CancelOrder(1)

	trades := trades(ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 10000, 10)))

	if len(trades) != 0 {
		t.Fatalf("cancelled order traded: %+v", trades)
	}
}

func TestCancelEmptyLevelRemoved(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideBuy, 10000, 10))
	ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 9900, 5))

	if ok, _ := ob.CancelOrder(2); !ok {
		t.Fatal("expected cancel of lower level to succeed")
	}
	if len(ob.Bids) != 1 || ob.Bids[0].Price != 10000 {
		t.Fatalf("expected only the 10000 level to remain, got %+v", ob.Bids)
	}

	if ok, _ := ob.CancelOrder(1); !ok {
		t.Fatal("expected cancel of best bid to succeed")
	}
	if len(ob.Bids) != 0 {
		t.Fatalf("expected empty bids, got %+v", ob.Bids)
	}
}

func TestMarketBuyWalksBestAsks(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10000, 10))
	ob.ProcessLimitOrder(mkOrder(2, models.SideSell, 10100, 10))

	trades := trades(ob.ProcessMarketOrder(mkOrder(3, models.SideBuy, 0, 15)))

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if trades[0].Price != 10000 || trades[0].Quantity != 10 {
		t.Fatalf("first trade wrong (best ask first): %+v", trades[0])
	}
	if trades[1].Price != 10100 || trades[1].Quantity != 5 {
		t.Fatalf("second trade wrong: %+v", trades[1])
	}
	if orders := ordersAt(ob.Asks[0]); len(orders) != 1 || orders[0].Quantity != 5 {
		t.Fatalf("remaining ask wrong: %+v", orders)
	}
}

func TestMarketOrderNeverRests(t *testing.T) {
	ob := engine.NewOrderBook()

	trades := trades(ob.ProcessMarketOrder(mkOrder(1, models.SideBuy, 0, 10)))

	if len(trades) != 0 {
		t.Fatalf("expected no trades on an empty book, got %+v", trades)
	}
	if len(ob.Bids) != 0 || len(ob.Asks) != 0 {
		t.Fatalf("market order must never rest, book changed: %+v", ob.Bids)
	}
}

func TestMarketOrderExhaustsBookIOC(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10000, 10))

	trades := trades(ob.ProcessMarketOrder(mkOrder(2, models.SideBuy, 0, 999)))

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity != 10 {
		t.Fatalf("trade wrong: %+v", trades[0])
	}
	if len(ob.Asks) != 0 {
		t.Fatalf("asks should be exhausted, got %+v", ob.Asks)
	}
	if len(ob.Bids) != 0 {
		t.Fatalf("market remainder must not rest, got %+v", ob.Bids)
	}
}

func TestMarketSellWalksBestBids(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideBuy, 10000, 10))
	ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 9900, 10))

	trades := trades(ob.ProcessMarketOrder(mkOrder(3, models.SideSell, 0, 12)))

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if trades[0].Price != 10000 || trades[0].Quantity != 10 {
		t.Fatalf("first trade wrong (best bid first): %+v", trades[0])
	}
	if trades[1].Price != 9900 || trades[1].Quantity != 2 {
		t.Fatalf("second trade wrong: %+v", trades[1])
	}
}

// --- Phase 2: sequencing & determinism ---

func TestSequencerAssignsConsecutiveNumbers(t *testing.T) {
	seq := sequencer.New()
	if got := seq.Next(); got != 1 {
		t.Fatalf("first seq should be 1, got %d", got)
	}
	if got := seq.Next(); got != 2 {
		t.Fatalf("second seq should be 2, got %d", got)
	}
}

func TestOneOrderProducesOneAtomicSeqGroup(t *testing.T) {
	ob := engine.NewOrderBook()
	seq := sequencer.New()

	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 10000, 10))
	ob.ProcessLimitOrder(mkOrder(2, models.SideSell, 10050, 10))

	evs := seq.Stamp(ob.ProcessLimitOrder(mkOrder(3, models.SideBuy, 10050, 25)))

	if len(evs) != 3 {
		t.Fatalf("expected 3 events (2 trades + 1 accept), got %d", len(evs))
	}
	if evs[0].Seq != 1 || evs[1].Seq != 2 || evs[2].Seq != 3 {
		t.Fatalf("expected atomic group 1,2,3, got %v %v %v", evs[0].Seq, evs[1].Seq, evs[2].Seq)
	}
}

// sameEvent compares two events by value. Seq and Type are plain fields; the
// Order and Trade payloads are pointers, so they are dereferenced and compared
// field by field (pointer equality would vary between runs).
func sameEvent(a, b events.Event) bool {
	if a.Seq != b.Seq || a.Type != b.Type {
		return false
	}
	if a.Order == nil || b.Order == nil {
		return a.Order == b.Order
	}
	if *a.Order != *b.Order {
		return false
	}
	if a.Trade == nil || b.Trade == nil {
		return a.Trade == b.Trade
	}
	return *a.Trade == *b.Trade
}

func TestAcceptedEventCarriesRestedRemainder(t *testing.T) {
	ob := engine.NewOrderBook()
	ob.ProcessLimitOrder(mkOrder(1, models.SideSell, 9900, 10))

	evs := ob.ProcessLimitOrder(mkOrder(2, models.SideBuy, 9900, 30))

	if len(evs) != 2 {
		t.Fatalf("expected 2 events, got %d", len(evs))
	}
	accept := evs[1]
	if accept.Type != events.OrderAccepted {
		t.Fatalf("expected second event to be an accept, got %v", accept.Type)
	}
	if accept.Order.Quantity != 20 {
		t.Fatalf("accept event should carry the rested remainder 20, got %d", accept.Order.Quantity)
	}
	if accept.Order.ID != 2 {
		t.Fatalf("accept event should carry the taker's id, got %d", accept.Order.ID)
	}
}

func TestReplayMatchesLiveWorkload(t *testing.T) {
	ob, log := runWorkload()

	replayed := engine.ReplayEvents(log)

	if fingerprint(ob) != fingerprint(replayed) {
		t.Fatalf("replay diverged from live run:\n  live:     %s\n  replayed: %s",
			fingerprint(ob), fingerprint(replayed))
	}
}

func TestReplayIsDeterministicAcrossRuns(t *testing.T) {
	_, logA := runWorkload()
	_, logB := runWorkload()

	bookA := engine.ReplayEvents(logA)
	bookB := engine.ReplayEvents(logB)

	if fingerprint(bookA) != fingerprint(bookB) {
		t.Fatalf("identical workloads replayed to different books:\n  A: %s\n  B: %s",
			fingerprint(bookA), fingerprint(bookB))
	}
	if len(logA) != len(logB) {
		t.Fatalf("identical workloads produced different event counts: %d vs %d", len(logA), len(logB))
	}
}

func TestReplayProducesSameLogAcrossRuns(t *testing.T) {
	obA, logA := runWorkload()
	obB, logB := runWorkload()

	if fingerprint(obA) != fingerprint(obB) {
		t.Fatal("two runs of the same workload produced different live books")
	}
	for i := range logA {
		if !sameEvent(logA[i], logB[i]) {
			t.Fatalf("event %d differs between runs: %+v vs %+v", i, logA[i], logB[i])
		}
	}
}

func TestReplayWithCancelsAndMarkets(t *testing.T) {
	ob := engine.NewOrderBook()
	seq := sequencer.New()
	var log []events.Event

	step := func(order *models.Order) {
		log = append(log, seq.Stamp(ob.ProcessLimitOrder(order))...)
	}
	market := func(order *models.Order) {
		log = append(log, seq.Stamp(ob.ProcessMarketOrder(order))...)
	}

	step(mkOrder(1, models.SideBuy, 10000, 10))
	step(mkOrder(2, models.SideBuy, 9900, 5))
	step(mkOrder(3, models.SideSell, 10050, 8))
	ok, cancelEvs := ob.CancelOrder(2)
	if !ok {
		t.Fatal("expected cancel of order 2 to succeed")
	}
	log = append(log, seq.Stamp(cancelEvs)...)
	step(mkOrder(4, models.SideSell, 10000, 4))
	market(mkOrder(5, models.SideSell, 0, 20))

	replayed := engine.ReplayEvents(log)

	if fingerprint(ob) != fingerprint(replayed) {
		t.Fatalf("replay with cancels/markets diverged:\n  live:     %s\n  replayed: %s",
			fingerprint(ob), fingerprint(replayed))
	}
}

func TestBookInvariantsHeld(t *testing.T) {
	ob := engine.NewOrderBook()

	seed := uint64(1)
	next := func() uint64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return seed
	}

	for i := uint64(1); i <= 500; i++ {
		side := models.SideBuy
		if next()%2 == 0 {
			side = models.SideSell
		}
		price := 10000 + next()%500
		qty := 1 + next()%20
		ob.ProcessLimitOrder(mkOrder(i, side, price, qty))

		if len(ob.Bids) > 0 && len(ob.Asks) > 0 && ob.Bids[0].Price >= ob.Asks[0].Price {
			t.Fatalf("book crossed after order %d: best bid %d >= best ask %d", i, ob.Bids[0].Price, ob.Asks[0].Price)
		}
		for j := 1; j < len(ob.Bids); j++ {
			if ob.Bids[j-1].Price < ob.Bids[j].Price {
				t.Fatalf("bids out of order after order %d", i)
			}
			if ob.Bids[j].Head == nil {
				t.Fatalf("empty bid level after order %d", i)
			}
		}
		for j := 1; j < len(ob.Asks); j++ {
			if ob.Asks[j-1].Price > ob.Asks[j].Price {
				t.Fatalf("asks out of order after order %d", i)
			}
			if ob.Asks[j].Head == nil {
				t.Fatalf("empty ask level after order %d", i)
			}
		}
	}
}

func TestBookInvariantsWithCancels(t *testing.T) {
	ob := engine.NewOrderBook()

	seed := uint64(42)
	next := func() uint64 {
		seed = seed*6364136223846793005 + 1442695040888963407
		return seed
	}

	var resting []uint64

	checkInvariants := func(step uint64) {
		if len(ob.Bids) > 0 && len(ob.Asks) > 0 && ob.Bids[0].Price >= ob.Asks[0].Price {
			t.Fatalf("book crossed after step %d: best bid %d >= best ask %d", step, ob.Bids[0].Price, ob.Asks[0].Price)
		}
		for j := 1; j < len(ob.Bids); j++ {
			if ob.Bids[j-1].Price < ob.Bids[j].Price {
				t.Fatalf("bids out of order after step %d", step)
			}
			if ob.Bids[j].Head == nil {
				t.Fatalf("empty bid level after step %d", step)
			}
		}
		for j := 1; j < len(ob.Asks); j++ {
			if ob.Asks[j-1].Price > ob.Asks[j].Price {
				t.Fatalf("asks out of order after step %d", step)
			}
			if ob.Asks[j].Head == nil {
				t.Fatalf("empty ask level after step %d", step)
			}
		}
	}

	for i := uint64(1); i <= 1000; i++ {
		if next()%4 == 0 && len(resting) > 0 {
			j := next() % uint64(len(resting))
			id := resting[j]
			resting = append(resting[:j], resting[j+1:]...)
			if ok, _ := ob.CancelOrder(id); !ok {
				t.Fatalf("expected cancel of resting order %d to succeed", id)
			}
			if inBook(ob, id) {
				t.Fatalf("cancelled order %d still in book", id)
			}
		} else {
			side := models.SideBuy
			if next()%2 == 0 {
				side = models.SideSell
			}
			price := 10000 + next()%500
			qty := 1 + next()%20
			id := i + 10000
			ob.ProcessLimitOrder(mkOrder(id, side, price, qty))
			if inBook(ob, id) {
				resting = append(resting, id)
			}
		}
		checkInvariants(i)
	}
}

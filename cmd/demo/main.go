package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github/wycliff-ochieng/internal/engine"
	"github/wycliff-ochieng/internal/events"
	"github/wycliff-ochieng/internal/models"
	"github/wycliff-ochieng/internal/sequencer"
)

var nextID uint64 = 1

var seq = sequencer.New()

// log is the running, fully-sequenced event log for the session.
var log []events.Event

func newID() uint64 {
	id := nextID
	nextID++
	return id
}

func money(cents uint64) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

func levelQty(level *engine.PriceLevel) uint64 {
	var total uint64
	for n := level.Head; n != nil; n = n.Next {
		total += n.Order.Quantity
	}
	return total
}

// printEvents prints a sequenced batch of events, one line per event.
func printEvents(evs []events.Event) {
	for _, e := range evs {
		switch e.Type {
		case events.OrderAccepted:
			fmt.Printf("   #%-3d ACCEPTED  %s %d @ %s\n",
				e.Seq, e.Order.Side, e.Order.Quantity, money(e.Order.Price))
		case events.TradeExecuted:
			fmt.Printf("   #%-3d TRADE     %d @ %s  (maker #%d <-> taker #%d)\n",
				e.Seq, e.Trade.Quantity, money(e.Trade.Price), e.Trade.MakerOrderID, e.Trade.TakerOrderID)
		case events.OrderCancelled:
			fmt.Printf("   #%-3d CANCELLED #%d\n", e.Seq, e.Order.ID)
		}
	}
}

// submit runs one order through the engine, stamps and logs its events, then
// prints the events and the book.
func submit(ob *engine.OrderBook, label string, side models.Side, typ models.OrderType, price, qty uint64) uint64 {
	id := newID()
	order := &models.Order{ID: id, ClientId: 1, Symbol: "DEMO", Side: side, Type: typ, Price: price, Quantity: qty}

	fmt.Printf("\n> %s\n", label)

	var evs []events.Event
	if typ == models.OrderTypeMarket {
		evs = ob.ProcessMarketOrder(order)
	} else {
		evs = ob.ProcessLimitOrder(order)
	}
	evs = seq.Stamp(evs)
	log = append(log, evs...)
	printEvents(evs)
	printBook(ob)
	return id
}

// replayProof rebuilds the book from the session log and compares it to the
// live book — the whole point of Phase 2.
func replayProof(ob *engine.OrderBook) {
	fmt.Printf("\nREPLAY: rebuilding the book from %d sequenced events...\n", len(log))
	rebuilt := engine.ReplayEvents(log)
	if fingerprint(rebuilt) == fingerprint(ob) {
		fmt.Println("REPLAY OK — the rebuilt book is identical to the live book.")
	} else {
		fmt.Println("REPLAY MISMATCH — determinism is broken!")
	}
	printBook(rebuilt)
}

// fingerprint is a canonical string of a book's entire state, so two books can
// be compared for equality.
func fingerprint(ob *engine.OrderBook) string {
	var b strings.Builder
	for _, side := range []struct {
		levels []*engine.PriceLevel
		label  string
	}{{ob.Bids, "B"}, {ob.Asks, "A"}} {
		for _, l := range side.levels {
			fmt.Fprintf(&b, "%s%d[", side.label, l.Price)
			for n := l.Head; n != nil; n = n.Next {
				fmt.Fprintf(&b, "%d:%d,", n.Order.ID, n.Order.Quantity)
			}
			b.WriteString("];")
		}
	}
	return b.String()
}

func printBook(ob *engine.OrderBook) {
	fmt.Println("   ---- ASKS (sellers) ----")
	if len(ob.Asks) == 0 {
		fmt.Println("        (empty)")
	} else {
		for i := len(ob.Asks) - 1; i >= 0; i-- {
			marker := ""
			if i == 0 {
				marker = "    <- best ask"
			}
			fmt.Printf("        %8s x %5d%s\n", money(ob.Asks[i].Price), levelQty(ob.Asks[i]), marker)
		}
	}
	fmt.Println("        --------------------")
	if len(ob.Bids) == 0 {
		fmt.Println("        (empty)")
	} else {
		for i, l := range ob.Bids {
			marker := ""
			if i == 0 {
				marker = "    <- best bid"
			}
			fmt.Printf("        %8s x %5d%s\n", money(l.Price), levelQty(l), marker)
		}
	}
	fmt.Println("   ---- BIDS (buyers) ----")
}

func scripted() {
	ob := engine.NewOrderBook()

	submit(ob, "SEED:  sell 10 @ $101.00", models.SideSell, models.OrderTypeLimit, 10100, 10)
	submit(ob, "SEED:  sell 30 @ $101.50", models.SideSell, models.OrderTypeLimit, 10150, 30)
	submit(ob, "SEED:  sell 20 @ $102.00", models.SideSell, models.OrderTypeLimit, 10200, 20)
	submit(ob, "SEED:  buy 25 @ $100.00", models.SideBuy, models.OrderTypeLimit, 10000, 25)
	submit(ob, "SEED:  buy 15 @ $99.50", models.SideBuy, models.OrderTypeLimit, 9950, 15)
	submit(ob, "SEED:  buy 10 @ $99.00", models.SideBuy, models.OrderTypeLimit, 9900, 10)

	submit(ob, "LESSON 1 - sweep: BUY 40 @ $102.00 (walks two ask levels, prints at maker prices)",
		models.SideBuy, models.OrderTypeLimit, 10200, 40)

	restID := submit(ob, "LESSON 2 - rest: BUY 30 @ $101.00 (cannot cross, rests as the new best bid)",
		models.SideBuy, models.OrderTypeLimit, 10100, 30)

	submit(ob, "MARKET - walk: MARKET BUY 15 (takes the best ask at its price)",
		models.SideBuy, models.OrderTypeMarket, 0, 15)

	submit(ob, "MARKET - exhaust: MARKET BUY 999 (drains the book; the remainder is cancelled, never rests)",
		models.SideBuy, models.OrderTypeMarket, 0, 999)

	fmt.Printf("\n> CANCEL resting bid #%d\n", restID)
	ok, cancelEvs := ob.CancelOrder(restID)
	if ok {
		cancelEvs = seq.Stamp(cancelEvs)
		log = append(log, cancelEvs...)
		printEvents(cancelEvs)
	} else {
		fmt.Println("   not found")
	}
	printBook(ob)

	submit(ob, "FIFO:  sell 5 @ $103.00 (first)", models.SideSell, models.OrderTypeLimit, 10300, 5)
	submit(ob, "FIFO:  sell 10 @ $103.00 (second)", models.SideSell, models.OrderTypeLimit, 10300, 10)
	submit(ob, "FIFO:  BUY 8 @ $103.00 (earlier seller fills first)",
		models.SideBuy, models.OrderTypeLimit, 10300, 8)

	replayProof(ob)
}

func repl(ob *engine.OrderBook) {
	fmt.Println("Commands (prices are in cents, so $101.00 = 10100):")
	fmt.Println("  B <qty> <price>   buy limit     e.g. B 10 10100")
	fmt.Println("  S <qty> <price>   sell limit    e.g. S 5 10250")
	fmt.Println("  MB <qty>          market buy    e.g. MB 15")
	fmt.Println("  MS <qty>          market sell   e.g. MS 5")
	fmt.Println("  X <orderid>       cancel        e.g. X 3")
	fmt.Println("  R                 replay the log and check determinism")
	fmt.Println("  Q                 quit")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "Q":
			return
		case "B", "S":
			if len(fields) < 3 {
				fmt.Println("   usage: B <qty> <price>")
				continue
			}
			qty, err1 := strconv.ParseUint(fields[1], 10, 64)
			price, err2 := strconv.ParseUint(fields[2], 10, 64)
			if err1 != nil || err2 != nil {
				fmt.Println("   bad numbers")
				continue
			}
			side := models.SideBuy
			if strings.ToUpper(fields[0]) == "S" {
				side = models.SideSell
			}
			submit(ob, fmt.Sprintf("%s %d @ %s", side, qty, money(price)),
				side, models.OrderTypeLimit, price, qty)
		case "MB", "MS":
			if len(fields) < 2 {
				fmt.Println("   usage: MB <qty>")
				continue
			}
			qty, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				fmt.Println("   bad qty")
				continue
			}
			side := models.SideBuy
			label := "MARKET BUY"
			if strings.ToUpper(fields[0]) == "MS" {
				side = models.SideSell
				label = "MARKET SELL"
			}
			submit(ob, fmt.Sprintf("%s %d", label, qty), side, models.OrderTypeMarket, 0, qty)
		case "X":
			id, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				fmt.Println("   bad id")
				continue
			}
			if ok, cancelEvs := ob.CancelOrder(id); ok {
				cancelEvs = seq.Stamp(cancelEvs)
				log = append(log, cancelEvs...)
				printEvents(cancelEvs)
			} else {
				fmt.Printf("   #%d not resting\n", id)
			}
			printBook(ob)
		case "R":
			replayProof(ob)
		default:
			fmt.Println("   unknown command (try B / S / MB / MS / X / R / Q)")
		}
	}
}

func main() {
	ob := engine.NewOrderBook()

	if len(os.Args) > 1 && os.Args[1] == "-repl" {
		repl(ob)
		return
	}
	scripted()
}

package worker

import (
	"context"
	"fmt"
	"log"

	"github/wycliff-ochieng/internal/db"
	"github/wycliff-ochieng/internal/events"
)

// Start starts the async worker that listens to the sequencer event stream and writes to PostgreSQL.
func Start(eventStream <-chan events.Event, database *db.DB) {
	for ev := range eventStream {
		if err := processEvent(ev, database); err != nil {
			log.Printf("Worker Error processing event Seq #%d: %v", ev.Seq, err)
		}
	}
}

func processEvent(ev events.Event, database *db.DB) error {
	ctx := context.Background()
	
	switch ev.Type {
	case events.OrderAccepted:
		// Insert order into DB
		query := `
			INSERT INTO orders (id, user_id, symbol, side, price, quantity, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'PENDING')
			ON CONFLICT (id) DO NOTHING;
		`
		_, err := database.Pool.Exec(ctx, query,
			ev.Order.ID,
			ev.Order.ClientId,
			ev.Order.Symbol,
			ev.Order.Side.String(),
			ev.Order.Price,
			ev.Order.Quantity,
		)
		if err != nil {
			return fmt.Errorf("insert order: %v", err)
		}
		
	case events.TradeExecuted:
		// Insert trade and update order statuses
		tx, err := database.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		// 1. Insert trade
		_, err = tx.Exec(ctx, `
			INSERT INTO trades (maker_order_id, taker_order_id, price, quantity)
			VALUES ($1, $2, $3, $4)
		`, ev.Trade.MakerOrderID, ev.Trade.TakerOrderID, ev.Trade.Price, ev.Trade.Quantity)
		if err != nil {
			return fmt.Errorf("insert trade: %v", err)
		}

		// 2. Update order statuses (simplification: mark both as PARTIAL_FILL or FILLED based on quantity)
		// For a full implementation, we'd calculate exactly when they hit 0. 
		// Here we'll just set them to FILLED for demonstration purposes.
		_, err = tx.Exec(ctx, `UPDATE orders SET status = 'FILLED' WHERE id IN ($1, $2)`, ev.Trade.MakerOrderID, ev.Trade.TakerOrderID)
		if err != nil {
			return fmt.Errorf("update orders: %v", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit trade: %v", err)
		}

	case events.OrderCancelled:
		_, err := database.Pool.Exec(ctx, `UPDATE orders SET status = 'CANCELLED' WHERE id = $1`, ev.Order.ID)
		if err != nil {
			return fmt.Errorf("cancel order: %v", err)
		}
	}
	
	return nil
}

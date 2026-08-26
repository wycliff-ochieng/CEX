package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github/wycliff-ochieng/internal/db"
	"github/wycliff-ochieng/internal/events"
	"github/wycliff-ochieng/internal/gateway"
	"github/wycliff-ochieng/internal/models"
	"github/wycliff-ochieng/internal/worker"
)

var globalOrderID uint64 = 1

type OrderRequest struct {
	ClientID uint64 `json:"client_id"`
	Symbol   string `json:"symbol"`
	Side     string `json:"side"` // "BUY" or "SELL"
	Type     string `json:"type"` // "LIMIT" or "MARKET"
	Price    uint64 `json:"price"` // In cents
	Quantity uint64 `json:"quantity"`
}

func main() {
	log.Println("Booting Centralized Exchange Engine...")

	// 1. Connect to PostgreSQL
	connStr := "postgres://cex:password@localhost:5432/exchange"
	database, err := db.ConnectDB(connStr)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer database.Close()
	log.Println("Connected to PostgreSQL successfully.")

	// 2. Set up Event Stream Channel
	eventStream := make(chan events.Event, 10000)

	// 3. Start the Async DB Worker (CQRS Event Sourcing Pattern)
	go worker.Start(eventStream, database)

	// 4. Start the Hot-Path Order Gateway and Matching Engine Loop
	gw := gateway.NewOrderGateway(eventStream)
	go gw.Start()

	// 5. Setup REST API using gorilla/mux
	r := mux.NewRouter()

	r.HandleFunc("/api/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		var req OrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		side := models.SideBuy
		if req.Side == "SELL" {
			side = models.SideSell
		}

		orderType := models.OrderTypeLimit
		if req.Type == "MARKET" {
			orderType = models.OrderTypeMarket
		}

		order := &models.Order{
			ID:       globalOrderID,
			ClientId: req.ClientID,
			Symbol:   req.Symbol,
			Side:     side,
			Type:     orderType,
			Price:    req.Price,
			Quantity: req.Quantity,
		}
		globalOrderID++

		// Send to Hot Path Engine
		gw.SubmitOrder(order)

		// Note: For a real system, we'd wait for the engine to acknowledge the sequence number.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "Order submitted to matching engine",
			"order_id": order.ID,
		})
	}).Methods("POST")

	// Changing port to 8888 since 3000 and 8080 were in use
	log.Println("REST Gateway listening on :8888")
	if err := http.ListenAndServe(":8888", r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

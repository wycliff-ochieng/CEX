This is a comprehensive extraction of the system design concepts, architecture, and logic presented in the video by the content creator **Jordan has no life**.

---

### 1. Video Overview
* **Topic:** System Design of a Stock Exchange (Matching Engine)
* **Date of Presentation:** Wednesday, June 12, 2024
* **Speaker/Creator:** Jordan ("Jordan has no life" YouTube channel)
* **Disclaimer:** Jordan humorously disclaims that he works on frontend trader GUIs in his day-to-day job rather than backend exchange logic, and that this design is openly adapted from Jane Street’s public exchange architecture presentations. 

---

### 2. Core Problem Requirements
An exchange system must fulfill three primary goals:
1. **Match Buyers and Sellers:** Successfully pair buy orders (bids) and sell orders (asks) of a stock.
2. **Global Sequence Agreement:** Provide a single, globally ordered list of all operations that every system component eventually agrees upon. If the primary system fails, the backup must reconstruct the exact same sequence.
3. **Client-Specific Sequencing:** Provide an ordered, sequenced list of all activities (placed orders, cancels, trades) back to each individual client so they can track their state chronologically.

---

### 3. The Limit Order Book (LOB) & Matching Logic
Exchanges maintain an order book consisting of **Bids** (buy orders) and **Asks** (sell orders). 
* **Matching Rule:** A trade occurs when the highest bid price is greater than or equal to the lowest ask price ($\text{Bid Price} \ge \text{Ask Price}$).
* **Multiple Executions:** A single large incoming order can trigger multiple trades. For instance, if a buyer wants 110 units at $130, they might match with one seller offering 100 units at $120 and another offering 10 units at $130. 
* **Atomicity:** The engine must guarantee that either all triggered trades of a multi-match order succeed, or none do, ensuring fault tolerance and state consistency.

---

### 4. Matching Engine Performance & Determinism
Because exchanges process millions or billions of transactions daily, performance is critical.
* **In-Memory Execution:** To eliminate slow disk and network I/O bottlenecks during matching, the engine runs entirely **locally in memory**. 
* **Determinism:** The matching engine acts as a deterministic state machine. If the engine is in state $S$ and applies message $m$, it will **always** transition to the exact same resulting state $S'$. 

---

### 5. Networking Layer (UDP Multicast & Retransmitters)
To distribute market data fairly to all HFTs (High-Frequency Traders), banks, and clearing companies:
* **Why not TCP?** TCP requires individual, one-to-one connections. Sending updates sequentially to 50 clients means client #1 gets the data before client #50, introducing unfair latency arbitrage.
* **The Solution: UDP Multicast.** The exchange broadcasts a single packet over a private network, allowing all connected parties to receive it at virtually the same instant.
* **Handling UDP Loss (Retransmitters):** Since UDP is unreliable, packets can be dropped or arrive out of order.
  * *Sequence Numbers:* Every multicast message contains a sequence number.
  * *Gap Detection:* If a client receives message #3 and then message #5, it detects that it missed message #4.
  * *Retransmitters (RT):* The exchange deploys dedicated Retransmitter nodes that cache every multicast message. The client sends a targeted request to a Retransmitter: *"Give me all messages $\ge 4$,"* allowing the client to catch up without querying the main matching engine.

---

### 6. Fault Tolerance (Active-Passive State Machine Replication)
If the in-memory matching engine crashes, the state must be recovered instantly.
* **Why Active-Active (Option 1) Fails:** Sending client orders directly to both a Primary and a Backup engine simultaneously does not work. Because of network jitter, orders can arrive at the Primary in the order (A, then B) and at the Backup in the order (B, then A). Due to deterministic matching, this would cause their books to diverge.
* **State Machine Replication (Option 2):** The Backup engine does not listen to client inputs. Instead, it listens to the **sequenced output** of the Primary engine. Because the output is already sequenced, the Backup processes the events in the exact same chronological order, maintaining a mirrored state.
* **Reconstructing Atomic Batches:** If the Primary crashes mid-batch (e.g., it broadcasts trade #1 but goes down before broadcasting trades #2 and #3 of an atomic execution), the Backup can reconstruct the missing trades locally because it runs the exact same deterministic code. 
* **ZooKeeper:** Zookeeper coordinates consensus and manages heartbeats. If the Primary engine stops heartbeating, ZooKeeper promotes the Backup to the new Primary.

---

### 7. Sharding / Partitioning the Engine
A single matching engine is a physical bottleneck. To scale throughput, the system must be partitioned.
* **No Sharding Within a Symbol:** You cannot partition orders for a single stock (e.g., AAPL) across different shards because buy and sell orders must see each other to match.
* **Sharding by Symbol:** The system partitions the matching engines **by stock symbol** (e.g., Shard 1 handles AAPL, Shard 2 handles GOOGL, Shard 3 handles TSLA).
* **Cross-Symbol Transactions:** If a client wants an atomic transaction across symbols (e.g., *"buy AAPL and sell GOOGL simultaneously"*), it would require a distributed transaction protocol like Two-Phase Commit (2PC). Because 2PC is far too slow for low-latency trading, exchanges typically do not support atomic cross-symbol orders.

---

### 8. Client-Specific Message Ordering
To return ordered history back to individual clients:
* **The Lock Strategy:** When an operation (order, trade, or cancel) is finalized, the system grabs a lock associated with the specific `ClientID`. 
* **Monotonic Sequencing:** While holding the client lock, the system assigns a monotonically increasing sequence number specifically for that client's events before broadcasting or returning the response. This guarantees client-side historical consistency without locking the entire matching engine.

---

### 9. Final Stock Exchange Architecture
The complete system architecture diagram shown at the end of the video integrates these components:

```
                  ┌──────────────┐
                  │   Clients    │
                  └──────┬───────┘
                         │ (Private Network)
                  ┌──────▼───────┐
                  │Order Gateways│ <─── (Rate Limiting)
                  └──────┬───────┘
                         │
           ┌─────────────▼─────────────┐
           │ Matching Engine (Primary) │ <─── (In-Memory Engine)
           └─────────────┬─────────────┘
                         │ (UDP Multicast Output)
        ┌────────────────┼────────────────┬────────────────┐
        ▼                ▼                ▼                ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│Retransmitter │ │Retransmitter │ │Matching Eng. │ │Order Gateways│
│    Node 1    │ │    Node 2    │ │   (Backup)   │ │ (Sends to Cl)│
└──────────────┘ └──────────────┘ └──────┬───────┘ └──────────────┘
                                         ▲
                                         │ (Heartbeat/Consensus)
                                  ┌──────▼───────┐
                                  │  ZooKeeper   │
                                  └──────────────┘
```

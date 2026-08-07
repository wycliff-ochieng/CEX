# Ubiquitous Language

## Order Lifecycle

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Order** | A request to buy or sell an asset at a specific price and quantity. | Request, transaction |
| **Limit Order** | An **Order** that stays in the **Order Book** until it finds a match at its specified price or better. | Resting order |
| **Market Order** | An **Order** that executes immediately against the best available prices and is never added to the **Order Book**. | Immediate order |
| **Side** | The direction of an **Order**: either **Buy** (Bid) or **Sell** (Ask). | Direction |
| **Trade** | An execution resulting from the matching of a **Maker Order** and a **Taker Order**. | Match, fill |
| **Maker Order** | A resting **Order** already in the **Order Book** that "makes" liquidity. | Passive order |
| **Taker Order** | An incoming **Order** that matches against a **Maker Order**, "taking" liquidity. | Aggressive order |
| **Cancel** | The removal of a resting **Order** from the **Order Book**. | Delete, remove |

## Market Structure

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Order Book** | The collection of all resting **Limit Orders**, separated into **Bids** and **Asks**. | The book, L2 data |
| **Price Level** | A specific price in the **Order Book** containing all orders resting at that value. | Price point, tick |
| **Best Bid** | The highest price currently offered by buyers in the **Order Book**. | Top of book (buy) |
| **Best Ask** | The lowest price currently offered by sellers in the **Order Book**. | Top of book (sell) |
| **Spread** | The gap between the **Best Ask** and the **Best Bid**. | Gap |
| **Price-Time Priority** | The rule where **Orders** are matched first by best price, then by the time they were accepted. | FIFO matching |

## System Integrity

| Term | Definition | Aliases to avoid |
| --- | --- | --- |
| **Event** | An atomic, immutable record of a system state change (e.g., acceptance, trade, cancellation). | Log entry, message |
| **Sequence Number** | A unique, monotonically increasing number that establishes the total order of **Events**. | Seq, ID |
| **Sequencer** | The component that assigns **Sequence Numbers** to ensure a single source of truth for ordering. | Ordering service |
| **Event Log** | The sequenced history of all **Events**, used for persistence and recovery. | Journal, audit trail |
| **Determinism** | The guarantee that replaying the same **Event Log** will always produce the same **Order Book** state. | Reproducibility |
| **Replay** | The process of reconstructing the current system state by processing the **Event Log** from the beginning. | Recovery, hydrate |

## Relationships

- An **Order** is placed on a specific **Side** (**Buy** or **Sell**).
- A **Trade** involves exactly one **Maker Order** and one **Taker Order**.
- An **Order Book** consists of multiple **Price Levels**.
- Each **Price Level** follows **Time Priority** (FIFO) for its internal **Orders**.
- The **Sequencer** assigns a **Sequence Number** to every **Event**.
- **Determinism** is achieved by ensuring the **Engine** only depends on the **Event Log**.

## Example dialogue

> **Dev:** "If a **Market Order** arrives and only partially fills, does the remainder become a **Maker Order**?"
> **Domain expert:** "No, a **Market Order** is never added to the **Order Book**. Any unfilled quantity is immediately **Cancelled**."
> **Dev:** "So only **Limit Orders** can become **Maker Orders**?"
> **Domain expert:** "Exactly. A **Limit Order** that doesn't match immediately becomes a **Maker Order** resting at its **Price Level**."
> **Dev:** "And the **Sequencer** ensures that if two **Orders** arrive at the same time, we still have a deterministic **Best Bid**?"
> **Domain expert:** "Yes, the **Sequencer** picks the winner and assigns the next **Sequence Number**. The **Engine** then processes them one by one, maintaining **Price-Time Priority**."

## Flagged ambiguities

- **"Account" vs "Client"**: The code uses `ClientId`. We should stick to **Client** for the entity placing orders, and potentially **Account** for their balance/position (though not yet implemented).
- **"Bid/Ask" vs "Buy/Sell"**: **Buy/Sell** describes the action/side, while **Bid/Ask** refers to the entries in the **Order Book**. They are often used interchangeably, but we should use **Side** for orders and **Bids/Asks** for the book levels.

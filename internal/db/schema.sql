-- Schema for the Centralized Exchange (Step 2)

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS accounts (
    user_id INT REFERENCES users(id),
    currency VARCHAR(10) NOT NULL,
    available_balance BIGINT DEFAULT 0, -- Store as cents (uint64)
    locked_balance BIGINT DEFAULT 0,
    PRIMARY KEY (user_id, currency)
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGINT PRIMARY KEY,
    user_id INT REFERENCES users(id),
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL, -- BUY or SELL
    price BIGINT NOT NULL,     -- In cents
    quantity BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL -- PENDING, FILLED, PARTIAL_FILL, CANCELLED
);

CREATE TABLE IF NOT EXISTS trades (
    id SERIAL PRIMARY KEY,
    maker_order_id BIGINT REFERENCES orders(id),
    taker_order_id BIGINT REFERENCES orders(id),
    price BIGINT NOT NULL,
    quantity BIGINT NOT NULL,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Basic setup for testing
INSERT INTO users (email) VALUES ('test1@example.com'), ('test2@example.com') ON CONFLICT DO NOTHING;
INSERT INTO accounts (user_id, currency, available_balance) VALUES (1, 'USD', 1000000) ON CONFLICT DO NOTHING; -- $10,000 for User 1
INSERT INTO accounts (user_id, currency, available_balance) VALUES (2, 'AAPL', 500) ON CONFLICT DO NOTHING; -- 500 AAPL shares for User 2

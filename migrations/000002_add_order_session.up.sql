ALTER TABLE orders ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_orders_session ON orders(session_id);

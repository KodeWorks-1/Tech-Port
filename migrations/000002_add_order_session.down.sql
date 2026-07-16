DROP INDEX IF EXISTS idx_orders_session;
ALTER TABLE orders DROP COLUMN IF EXISTS session_id;

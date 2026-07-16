CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          TEXT NOT NULL,
    phone         TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    address       TEXT NOT NULL DEFAULT '',
    city          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_sessions (
    session_id TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_user_sessions_user ON user_sessions(user_id);

ALTER TABLE orders ADD COLUMN user_id BIGINT REFERENCES users(id);
CREATE INDEX idx_orders_user ON orders(user_id);

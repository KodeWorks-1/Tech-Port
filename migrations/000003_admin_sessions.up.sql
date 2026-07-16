CREATE TABLE admin_sessions (
    token      TEXT PRIMARY KEY,
    admin_id   BIGINT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_admin_sessions_expiry ON admin_sessions(expires_at);

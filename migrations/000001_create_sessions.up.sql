CREATE TABLE sessions (
    id                 UUID PRIMARY KEY,
    user_id            TEXT        NOT NULL,
    refresh_token_hash TEXT        NOT NULL,
    user_agent         TEXT        NOT NULL DEFAULT '',
    ip_address         TEXT        NOT NULL DEFAULT '',
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX sessions_refresh_token_hash_idx ON sessions (refresh_token_hash);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

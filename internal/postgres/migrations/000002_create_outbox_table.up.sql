CREATE TABLE outbox
(
    id         BIGSERIAL PRIMARY KEY,
    topic      VARCHAR(255) NOT NULL,
    key        BYTEA,
    value      BYTEA        NOT NULL,
    headers    JSONB,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

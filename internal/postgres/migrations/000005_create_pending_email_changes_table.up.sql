CREATE TABLE pending_email_changes
(
    user_id    UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    email      VARCHAR(100) NOT NULL CHECK (email = lower(email)),
    token_hash CHAR(64)     NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ  NOT NULL
);

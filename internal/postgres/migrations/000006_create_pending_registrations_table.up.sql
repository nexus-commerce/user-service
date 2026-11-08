CREATE TABLE pending_registrations
(
    email         VARCHAR(100) PRIMARY KEY CHECK (email = lower(email)),
    user_id       UUID         NOT NULL UNIQUE,
    first_name    VARCHAR(50)  NOT NULL,
    last_name     VARCHAR(50)  NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    token_hash    CHAR(64)     NOT NULL UNIQUE,
    expires_at    TIMESTAMPTZ  NOT NULL
);

CREATE TABLE users
(
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name    VARCHAR(50)  NOT NULL,
    last_name     VARCHAR(50)  NOT NULL,
    email         VARCHAR(100) NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users (first_name, last_name, email, password_hash)
VALUES ('Peter', 'Parker', 'peter.parker@email.com', '$2a$10$sMn.IWt9q3EiisAecQoOLOsvnA0wsl2oMRDGcHIrAR6XNOBVpxILK');

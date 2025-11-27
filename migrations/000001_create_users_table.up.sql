CREATE TABLE users
(
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users (first_name, last_name, email, password_hash)
VALUES (
           'Peter',
           'Parker',
           'peter.parker@email.com',
           '$2a$10$sMn.IWt9q3EiisAecQoOLOsvnA0wsl2oMRDGcHIrAR6XNOBVpxILK'
       );
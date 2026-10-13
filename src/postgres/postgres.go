package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	pool *sql.DB
}

func New(ctx context.Context, dsn string) (*DB, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool := stdlib.OpenDB(*config)

	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(25)
	pool.SetConnMaxLifetime(5 * time.Minute)
	pool.SetConnMaxIdleTime(5 * time.Minute)

	if err = pool.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("ping: %w", err), pool.Close())
	}

	return &DB{pool: pool}, nil
}

func (d *DB) Close() error {
	return d.pool.Close()
}

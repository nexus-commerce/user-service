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

type Database struct {
	pool *sql.DB
}

func New(ctx context.Context, dsn string) (*Database, error) {
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

	return &Database{pool: pool}, nil
}

func (d *Database) Close() error {
	return d.pool.Close()
}

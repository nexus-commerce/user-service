package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
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

//go:embed migrations/*.sql
var migrations embed.FS

func (d *Database) Migrate(ctx context.Context) error {
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			log.Printf("close migration source: %v", err)
		}
	}()

	conn, err := d.pool.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("close migration connection: %v", err)
		}
	}()

	driver, err := postgres.WithConnection(ctx, conn, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	stop := context.AfterFunc(ctx, func() { m.GracefulStop <- true })
	defer stop()

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		log.Println("No new migrations to apply")
		return nil
	}
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("interrupted: %w", err)
	}

	log.Println("Migrations applied successfully")
	return nil
}

func (d *Database) runInTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback tx: %w", rbErr))
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

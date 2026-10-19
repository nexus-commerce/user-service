package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

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

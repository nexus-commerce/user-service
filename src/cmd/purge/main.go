package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/nexus-commerce/user-service/src/config"
	"github.com/nexus-commerce/user-service/src/lifecycle"
	"github.com/nexus-commerce/user-service/src/postgres"
	"github.com/nexus-commerce/user-service/src/purge"
)

const dbConnectTimeout = 10 * time.Second

type purgeConfig struct {
	Postgres config.Postgres
	Purge    config.Purge
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := loadDotEnv(); err != nil {
		return err
	}

	cfg, err := config.Load[purgeConfig]()
	if err != nil {
		return fmt.Errorf("load config (check env vars or .env): %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pingCtx, cancel := context.WithTimeout(ctx, dbConnectTimeout)
	defer cancel()
	db, err := postgres.New(pingCtx, cfg.Postgres.DSN())
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close postgres: %v", err)
		}
	}()

	purger := purge.New(postgres.NewPendingRequests(db), purge.Options{
		Interval: cfg.Purge.Interval,
		Grace:    cfg.Purge.Grace,
	})

	return lifecycle.Run(ctx, purger)
}

func loadDotEnv() error {
	if err := godotenv.Load(".env"); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("parse .env: %w", err)
		}
		log.Println("no .env file found, relying on environment variables")
	}
	return nil
}

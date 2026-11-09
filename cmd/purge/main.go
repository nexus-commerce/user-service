package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/nexus-commerce/user-service/internal/config"
	"github.com/nexus-commerce/user-service/internal/lifecycle"
	"github.com/nexus-commerce/user-service/internal/postgres"
	"github.com/nexus-commerce/user-service/internal/purge"
)

const dbConnectTimeout = 10 * time.Second

type purgeConfig struct {
	Postgres config.Postgres
	Purge    config.Purge
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("purge stopped", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := loadDotEnv(logger); err != nil {
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
	db, err := postgres.New(pingCtx, cfg.Postgres.DSN(), logger)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close postgres", "err", err)
		}
	}()

	retention := postgres.NewRetention(db)
	purger := purge.New(retention, retention, purge.Options{
		Interval: cfg.Purge.Interval,
		Grace:    cfg.Purge.Grace,
		Logger:   logger,
	})

	return lifecycle.Run(ctx, purger)
}

func loadDotEnv(logger *slog.Logger) error {
	if err := godotenv.Load(".env"); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("parse .env: %w", err)
		}
		logger.Info("no .env file found, relying on environment variables")
	}
	return nil
}

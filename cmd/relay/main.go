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
	"github.com/nexus-commerce/user-service/internal/kafka"
	"github.com/nexus-commerce/user-service/internal/lifecycle"
	"github.com/nexus-commerce/user-service/internal/postgres"
	"github.com/nexus-commerce/user-service/internal/relay"
)

const dbConnectTimeout = 10 * time.Second

type relayConfig struct {
	Postgres config.Postgres
	Kafka    config.Kafka
	Outbox   config.Outbox
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("relay stopped", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if err := loadDotEnv(logger); err != nil {
		return err
	}

	cfg, err := config.Load[relayConfig]()
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

	producer := kafka.NewProducer(cfg.Kafka.Addr())
	defer func() {
		if err := producer.Close(); err != nil {
			logger.Error("close kafka producer", "err", err)
		}
	}()

	outboxRelay := relay.NewWorker(postgres.NewOutbox(db), producer, relay.Options{
		Interval:  cfg.Outbox.Interval,
		BatchSize: cfg.Outbox.BatchSize,
		Logger:    logger,
	})

	return lifecycle.Run(ctx, outboxRelay)
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

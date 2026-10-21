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
	"github.com/nexus-commerce/user-service/src/kafka"
	"github.com/nexus-commerce/user-service/src/lifecycle"
	"github.com/nexus-commerce/user-service/src/postgres"
	"github.com/nexus-commerce/user-service/src/relay"
)

const dbConnectTimeout = 10 * time.Second

type relayConfig struct {
	Postgres config.Postgres
	Kafka    config.Kafka
	Outbox   config.Outbox
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

	cfg, err := config.Load[relayConfig]()
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

	producer := kafka.NewProducer(cfg.Kafka.Addr())
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("close kafka producer: %v", err)
		}
	}()

	outboxRelay := relay.New(postgres.NewOutbox(db), producer, relay.Options{
		Interval:  cfg.Outbox.Interval,
		BatchSize: cfg.Outbox.BatchSize,
	})

	return lifecycle.Run(ctx, outboxRelay)
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

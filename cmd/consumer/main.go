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

	"github.com/nexus-commerce/user-service/internal/config"
	"github.com/nexus-commerce/user-service/internal/kafka"
	"github.com/nexus-commerce/user-service/internal/lifecycle"
	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/nacl"
	"github.com/nexus-commerce/user-service/internal/postgres"
	"github.com/nexus-commerce/user-service/internal/usecase"
)

const dbConnectTimeout = 10 * time.Second

type consumerConfig struct {
	Postgres     config.Postgres
	Kafka        config.Kafka
	Consumer     config.Consumer
	Notification config.Notification
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

	cfg, err := config.Load[consumerConfig]()
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

	sealer, err := nacl.NewSealer(cfg.Notification.PublicKey, cfg.Notification.KeyID)
	if err != nil {
		return fmt.Errorf("create token sealer: %w", err)
	}
	publisher, err := messaging.NewPublisher(sealer)
	if err != nil {
		return fmt.Errorf("create event publisher: %w", err)
	}

	completeRegistration := usecase.NewCompleteRegistration(
		postgres.NewRegistrations(db, publisher),
		postgres.NewUsers(db, publisher),
	)

	consumer, err := kafka.NewConsumer(kafka.ConsumerOptions{
		Addr:    cfg.Kafka.Addr(),
		GroupID: cfg.Consumer.GroupID,
		Topic:   messaging.TopicUsersInternal,
	}, kafka.NewRegistrationConfirmed(completeRegistration))
	if err != nil {
		return fmt.Errorf("create kafka consumer: %w", err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Printf("close kafka consumer: %v", err)
		}
	}()

	return lifecycle.Run(ctx, consumer)
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

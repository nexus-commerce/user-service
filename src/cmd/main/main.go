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
	httpctrl "github.com/nexus-commerce/user-service/src/controller/http"
	"github.com/nexus-commerce/user-service/src/docs"
	"github.com/nexus-commerce/user-service/src/kafka"
	"github.com/nexus-commerce/user-service/src/lifecycle"
	"github.com/nexus-commerce/user-service/src/postgres"
	"github.com/nexus-commerce/user-service/src/publisher"
	"github.com/nexus-commerce/user-service/src/token"
	"github.com/nexus-commerce/user-service/src/usecase"
)

const dbConnectTimeout = 10 * time.Second

// @title           User Service API
// @version         0.1.0
// @description		This is the API documentation for the User Service
// @BasePath        /api
//
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                JWT token in the format "Bearer {token}"
//
// @securityDefinitions.basic BasicAuth
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := loadDotEnv(); err != nil {
		return err
	}

	cfg, err := config.New()
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

	if err = db.Migrate(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	producer := kafka.NewProducer(cfg.Kafka.Addr())
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("close kafka producer: %v", err)
		}
	}()

	outbox := postgres.NewOutbox(db)
	events := publisher.NewEvents(outbox, publisher.Topics{
		Users: cfg.Kafka.Topics.Users,
	})

	relay := publisher.NewRelay(outbox, producer, publisher.RelayOptions{
		Interval:  cfg.Outbox.Interval,
		BatchSize: cfg.Outbox.BatchSize,
	})

	users := postgres.NewUsers(db)
	tokens := token.New(token.Options{
		Secret: cfg.JWT.Secret,
		Issuer: cfg.JWT.Issuer,
	})

	login := usecase.NewLogin(users, tokens)
	register := usecase.NewRegister(db, users, events)
	getProfile := usecase.NewGetProfile(users)
	updateProfile := usecase.NewUpdateProfile(users)

	server := httpctrl.New(httpctrl.Options{
		Port:       cfg.HTTP.Port,
		BasicAuth:  httpctrl.BasicAuth(cfg.Docs.Username, cfg.Docs.Password),
		BearerAuth: httpctrl.BearerAuth(tokens),
	})

	server.Register(
		httpctrl.NewDocs(),
		httpctrl.NewOpenAPI(docs.SwaggerInfo.ReadDoc()),
		httpctrl.NewHealth(),
		httpctrl.NewLogin(login),
		httpctrl.NewRegister(register),
		httpctrl.NewGetProfile(getProfile),
		httpctrl.NewUpdateProfile(updateProfile),
	)

	return lifecycle.Run(ctx, server, relay)
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

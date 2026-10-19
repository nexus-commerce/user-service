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
	"github.com/nexus-commerce/user-service/src/controller/httpapi"
	"github.com/nexus-commerce/user-service/src/docs"
	"github.com/nexus-commerce/user-service/src/jwt"
	"github.com/nexus-commerce/user-service/src/messaging"
	"github.com/nexus-commerce/user-service/src/postgres"
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

	users := postgres.NewUsers(db, messaging.NewPublisher())
	tokens := jwt.NewTokens(jwt.Options{
		Secret: cfg.JWT.Secret,
		Issuer: cfg.JWT.Issuer,
	})

	login, err := usecase.NewLogin(users, tokens)
	if err != nil {
		return fmt.Errorf("create login use case: %w", err)
	}
	register := usecase.NewRegister(users, tokens)
	getProfile := usecase.NewGetProfile(users)
	updateProfile := usecase.NewUpdateProfile(users)

	server := httpapi.New(httpapi.Options{
		Port:       cfg.HTTP.Port,
		BasicAuth:  httpapi.BasicAuth(cfg.Docs.Username, cfg.Docs.Password),
		BearerAuth: httpapi.BearerAuth(tokens),
	})

	server.Register(
		httpapi.NewOpenAPIDocs(),
		httpapi.NewOpenAPI(docs.SwaggerInfo.ReadDoc()),
		httpapi.NewAsyncAPIDocs(),
		httpapi.NewAsyncAPI(docs.AsyncAPI),
		httpapi.NewHealth(),
		httpapi.NewLogin(login),
		httpapi.NewRegister(register),
		httpapi.NewGetProfile(getProfile),
		httpapi.NewUpdateProfile(updateProfile),
	)

	return server.Run(ctx)
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

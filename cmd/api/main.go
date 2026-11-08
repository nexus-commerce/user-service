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

	"github.com/nexus-commerce/user-service/internal/bcrypt"
	"github.com/nexus-commerce/user-service/internal/config"
	"github.com/nexus-commerce/user-service/internal/docs"
	"github.com/nexus-commerce/user-service/internal/http"
	"github.com/nexus-commerce/user-service/internal/jwt"
	"github.com/nexus-commerce/user-service/internal/nacl"
	"github.com/nexus-commerce/user-service/internal/postgres"
	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/userevent"
)

const dbConnectTimeout = 10 * time.Second

type appConfig struct {
	Postgres     config.Postgres
	HTTP         config.HTTP
	JWT          config.JWT
	Docs         config.Docs
	Notification config.Notification
}

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

	cfg, err := config.Load[appConfig]()
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

	sealer, err := nacl.NewSealer(cfg.Notification.PublicKey, cfg.Notification.KeyID)
	if err != nil {
		return fmt.Errorf("create token sealer: %w", err)
	}

	publisher, err := userevent.NewPublisher(sealer)
	if err != nil {
		return fmt.Errorf("create event publisher: %w", err)
	}
	users := postgres.NewUsers(db, publisher)
	registrations := postgres.NewRegistrations(db, publisher)
	tokens := jwt.NewTokens(jwt.Options{
		Secret: cfg.JWT.Secret,
		Issuer: cfg.JWT.Issuer,
	})

	hasher := bcrypt.NewHasher(bcrypt.DefaultCost)

	login, err := usecase.NewLogin(users, tokens, hasher)
	if err != nil {
		return fmt.Errorf("create login use case: %w", err)
	}
	requestRegistration := usecase.NewRequestRegistration(users, registrations, hasher)
	confirmRegistration := usecase.NewConfirmRegistration(registrations)
	fetchUser := usecase.NewFetchUser(users)
	rename := usecase.NewRename(users)
	requestEmailChange := usecase.NewRequestEmailChange(users, hasher)
	confirmEmailChange := usecase.NewConfirmEmailChange(users)

	server := http.New(http.Options{
		Port:       cfg.HTTP.Port,
		BasicAuth:  http.BasicAuth(cfg.Docs.Username, cfg.Docs.Password),
		BearerAuth: http.BearerAuth(tokens),
	})

	server.Register(
		http.NewOpenAPIDocs(),
		http.NewOpenAPI(docs.SwaggerInfo.ReadDoc()),
		http.NewAsyncAPIDocs(),
		http.NewAsyncAPI(docs.AsyncAPI),
		http.NewHealth(),
		http.NewLogin(login),
		http.NewRequestRegistration(requestRegistration),
		http.NewConfirmRegistration(confirmRegistration),
		http.NewFetchUser(fetchUser),
		http.NewRename(rename),
		http.NewRequestEmailChange(requestEmailChange),
		http.NewConfirmEmailChange(confirmEmailChange),
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

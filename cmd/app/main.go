package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"user-service/internal/config"
	"user-service/internal/repository"
	"user-service/internal/server"
	"user-service/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	_ "github.com/jackc/pgx/v5/stdlib"
	pb "github.com/nexus-commerce/nexus-contracts-go/user/v1"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	secret = "my-secret-key"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Config
	cfg, err := config.New()
	if err != nil {
		return err
	}

	// Database connection
	addr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host,
		cfg.DB.Port, cfg.DB.Name,
	)
	conn, err := setupDatabase(addr, cfg.DB.MigrationsPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.Server.Port))
	if err != nil {
		return err
	}

	// Repository
	repo := repository.New(conn)

	// Kafka writer
	kafkaAddr := fmt.Sprintf("%s:%s", cfg.Kafka.Host, cfg.Kafka.Port)
	userRegisteredWriter := &kafka.Writer{
		Addr:                   kafka.TCP(kafkaAddr),
		Topic:                  "users.registered",
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	defer userRegisteredWriter.Close()

	// Service
	svc := service.New(repo, userRegisteredWriter)

	// gRPC server
	s := grpc.NewServer(grpc.ChainUnaryInterceptor(
		selector.UnaryServerInterceptor(
			AuthInterceptor,
			selector.MatchFunc(func(ctx context.Context, callMeta interceptors.CallMeta) bool {
				return callMeta.FullMethod() == pb.UserService_GetProfile_FullMethodName ||
					callMeta.FullMethod() == pb.UserService_UpdateProfile_FullMethodName
			}),
		)))
	pb.RegisterUserServiceServer(s, server.NewServer(svc))

	// Serving gRPC server
	go func() {
		log.Printf("Starting gRPC user service server on port %s\n", cfg.Server.Port)
		if err = s.Serve(listener); err != nil {
			log.Println(err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("Received shutdown signal, stopping server...")

	s.GracefulStop()

	return nil
}

func setupDatabase(addr, path string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", addr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = conn.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	driver, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		return nil, err
	}

	migrationsPath := fmt.Sprintf("file://%s", filepath.ToSlash(path))

	m, err := migrate.NewWithDatabaseInstance(migrationsPath, "postgres", driver)
	if err != nil {
		return nil, err
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	} else if errors.Is(err, migrate.ErrNoChange) {
		log.Println("No new migrations to apply")
	} else {
		log.Println("Migrations applied successfully")
	}

	return conn, nil
}

func AuthInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	mt, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "metadata is not provided")
	}

	bearedToken := mt["authorization"]
	fmt.Println(bearedToken)
	if len(bearedToken) == 0 {
		return nil, status.Error(codes.Unauthenticated, "bearer token not found")
	}

	tokenString := strings.TrimPrefix(bearedToken[0], "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	claims := token.Claims.(jwt.MapClaims)

	userID, ok := claims["user-id"]
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user-id not found")
	}

	userIDFloat, ok := userID.(float64)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user-id is not float")
	}

	userIDInt := int(userIDFloat)

	ctx = context.WithValue(ctx, "user-id", userIDInt)

	return handler(ctx, req)
}

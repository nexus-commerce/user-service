package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"golang.org/x/crypto/bcrypt"
	"strconv"
	"time"
	"user-service/internal/model"
	"user-service/internal/repository"
)

const (
	secret      = "my-secret-key"
	issuer      = "my-unique-issuer-key"
	expiryHours = 24
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrWrongPassword          = errors.New("wrong password")
	ErrMismatchPassword       = errors.New("passwords do not match")
	ErrUserEmailAlreadyExists = errors.New("user email already exists")
	ErrSendingEvent           = errors.New("error sending event")
)

type UserRegisteredEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Data      UserData  `json:"data"`
}

type UserData struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	LoginURL  string `json:"login_url"`
}

type Service struct {
	repo                 *repository.Repository
	userRegisteredWriter *kafka.Writer
}

func New(repo *repository.Repository, userRegisteredWriter *kafka.Writer) *Service {
	return &Service{
		repo:                 repo,
		userRegisteredWriter: userRegisteredWriter,
	}
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", ErrWrongPassword
	}

	claims := jwt.MapClaims{
		"user-id": user.ID,
		"iss":     issuer,
		"exp":     time.Now().Add(expiryHours * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

func (s *Service) Register(ctx context.Context, firstName, lastName, email, password1, password2 string) (int64, error) {
	if password1 != password2 {
		return 0, ErrMismatchPassword
	}

	_, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil {
		return 0, ErrUserEmailAlreadyExists
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password1), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	user := &model.User{
		PasswordHash: string(hashedPassword),
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
		CreatedAt:    time.Now(),
	}

	userID, err := s.repo.CreateUser(ctx, user)
	if err != nil {
		return 0, err
	}

	eventData := UserData{
		UserID:    userID,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		LoginURL:  "http://my-ecom-project.dynv6.net/auth/login",
	}

	err = s.sendUserRegisteredEvent(ctx, eventData)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

func (s *Service) GetProfile(ctx context.Context, userID int64) (*model.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, email, firstName, lastName string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}

	user.FirstName = firstName
	user.LastName = lastName
	user.Email = email

	return s.repo.UpdateUser(ctx, user)
}

func (s *Service) sendUserRegisteredEvent(ctx context.Context, eventData UserData) error {
	event := UserRegisteredEvent{
		EventID:   uuid.NewString(),
		EventType: "users.registered",
		Timestamp: time.Now(),
		Version:   "1.0",
		Data:      eventData,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(strconv.Itoa(int(eventData.UserID))),
		Value: eventBytes,
	}

	err = s.userRegisteredWriter.WriteMessages(ctx, msg)

	return err
}

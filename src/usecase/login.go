package usecase

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

type loginUsers interface {
	ByEmail(ctx context.Context, email entity.Email) (*entity.User, error)
}

type tokenIssuer interface {
	Issue(userID uuid.UUID) (string, error)
}

type Login struct {
	users  loginUsers
	tokens tokenIssuer
	dummy  dummyCredentials
}

func NewLogin(users loginUsers, tokens tokenIssuer) (*Login, error) {
	d, err := newDummyCredentials()
	if err != nil {
		return nil, fmt.Errorf("dummy credentials: %w", err)
	}
	return &Login{
		users:  users,
		tokens: tokens,
		dummy:  d,
	}, nil
}

type Credentials struct {
	Email    string
	Password string
}

func (l *Login) Execute(ctx context.Context, creds Credentials) (string, error) {
	user, err := l.authenticate(ctx, creds)
	if err != nil {
		return "", err
	}
	return l.tokens.Issue(user.ID())
}

func (l *Login) authenticate(ctx context.Context, creds Credentials) (*entity.User, error) {
	email, err := entity.NewEmail(creds.Email)
	if err != nil {
		return nil, l.rejectTimingSafe()
	}
	password, err := entity.NewPassword(creds.Password)
	if err != nil {
		return nil, l.rejectTimingSafe()
	}

	user, err := l.users.ByEmail(ctx, email)
	if errors.Is(err, entity.ErrUserNotFound) {
		return nil, l.rejectTimingSafe()
	}
	if err != nil {
		return nil, err
	}

	if err = user.Authenticate(password); err != nil {
		return nil, err
	}
	return user, nil
}

type dummyCredentials struct {
	password entity.Password
	hash     string
}

func newDummyCredentials() (dummyCredentials, error) {
	password, err := entity.NewPassword("dummy")
	if err != nil {
		return dummyCredentials{}, fmt.Errorf("create password: %w", err)
	}
	hash, err := password.Hash()
	if err != nil {
		return dummyCredentials{}, err
	}
	return dummyCredentials{password: password, hash: hash}, nil
}

func (l *Login) rejectTimingSafe() error {
	_, _ = l.dummy.password.Matches(l.dummy.hash)
	return entity.ErrInvalidCredentials
}

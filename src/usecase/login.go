package usecase

import (
	"context"
	"fmt"

	"github.com/nexus-commerce/user-service/src/entity"
)

type LoginUsers interface {
	ByEmail(ctx context.Context, email entity.Email) (*entity.User, error)
}

type Tokens interface {
	Issue(userID string) (string, error)
}

type Login struct {
	users  LoginUsers
	tokens Tokens
}

func NewLogin(users LoginUsers, tokens Tokens) *Login {
	return &Login{
		users:  users,
		tokens: tokens,
	}
}

func (l *Login) Execute(ctx context.Context, email entity.Email, password entity.Password) (string, error) {
	if err := email.Validate(); err != nil {
		return "", fmt.Errorf("email: %w", err)
	}

	if err := password.Validate(); err != nil {
		return "", fmt.Errorf("password: %w", err)
	}

	user, err := l.users.ByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	ok, err := user.CheckPassword(password)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrWrongPassword
	}

	return l.tokens.Issue(user.ID)
}

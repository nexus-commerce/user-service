package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/src/entity"
)

type loginUsers interface {
	ByEmail(ctx context.Context, email entity.Email) (*entity.User, error)
}

type tokenIssuer interface {
	Issue(userID string) (string, error)
}

type Login struct {
	users  loginUsers
	tokens tokenIssuer
}

func NewLogin(users loginUsers, tokens tokenIssuer) *Login {
	return &Login{
		users:  users,
		tokens: tokens,
	}
}

func (l *Login) Execute(ctx context.Context, email entity.Email, password entity.Password) (string, error) {
	user, err := l.users.ByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		password.MatchesNothing()
		return "", err
	}
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

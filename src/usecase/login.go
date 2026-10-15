package usecase

import (
	"context"
	"errors"
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
}

func NewLogin(users loginUsers, tokens tokenIssuer) *Login {
	return &Login{
		users:  users,
		tokens: tokens,
	}
}

func (l *Login) Execute(ctx context.Context, rawEmail, rawPassword string) (string, error) {
	email, err := entity.NewEmail(rawEmail)
	if err != nil {
		entity.SpendPasswordCheck()
		return "", entity.ErrUserNotFound
	}
	password, err := entity.NewPassword(rawPassword)
	if err != nil {
		entity.SpendPasswordCheck()
		return "", ErrWrongPassword
	}

	user, err := l.users.ByEmail(ctx, email)
	if errors.Is(err, entity.ErrUserNotFound) {
		entity.SpendPasswordCheck()
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

	return l.tokens.Issue(user.ID())
}

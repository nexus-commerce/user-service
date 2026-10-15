package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	user, err := l.authenticate(ctx, rawEmail, rawPassword)
	if err != nil {
		return "", err
	}
	return l.tokens.Issue(user.ID())
}

func (l *Login) authenticate(ctx context.Context, rawEmail, rawPassword string) (*entity.User, error) {
	email, err := entity.NewEmail(rawEmail)
	if err != nil {
		return nil, rejectWithoutUser()
	}
	password, err := entity.NewPassword(rawPassword)
	if err != nil {
		return nil, rejectWithoutUser()
	}

	user, err := l.users.ByEmail(ctx, email)
	if errors.Is(err, entity.ErrUserNotFound) {
		return nil, rejectWithoutUser()
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

var dummy = sync.OnceValue(func() dummyCredentials {
	password, err := entity.NewPassword("dummy")
	if err != nil {
		panic(fmt.Sprintf("create dummy password: %v", err))
	}
	hash, err := password.Hash()
	if err != nil {
		panic(fmt.Sprintf("hash dummy password: %v", err))
	}
	return dummyCredentials{password: password, hash: hash}
})

func rejectWithoutUser() error {
	d := dummy()
	_, _ = d.password.Matches(d.hash)
	return entity.ErrInvalidCredentials
}

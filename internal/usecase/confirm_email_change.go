package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/internal/user"
)

type confirmEmailChangeUsers interface {
	UpdateByEmailChangeToken(ctx context.Context, hash user.ConfirmationTokenHash, update func(*user.User) error) (*user.User, error)
}

type ConfirmEmailChange struct {
	users confirmEmailChangeUsers
}

func NewConfirmEmailChange(users confirmEmailChangeUsers) *ConfirmEmailChange {
	return &ConfirmEmailChange{
		users: users,
	}
}

func (c *ConfirmEmailChange) Execute(ctx context.Context, rawToken string) error {
	token, err := user.ParseConfirmationToken(rawToken)
	if err != nil {
		return err
	}

	_, err = c.users.UpdateByEmailChangeToken(ctx, token.Hash(), func(u *user.User) error {
		return u.ConfirmEmailChange(token)
	})
	switch {
	case errors.Is(err, user.ErrNotFound):
		return user.ErrInvalidConfirmationToken
	case errors.Is(err, user.ErrEmailTaken):
		return c.reject(ctx, token)
	}
	return err
}

func (c *ConfirmEmailChange) reject(ctx context.Context, token user.ConfirmationToken) error {
	_, err := c.users.UpdateByEmailChangeToken(ctx, token.Hash(), func(u *user.User) error {
		return u.RejectEmailChange(token)
	})
	if err != nil && !errors.Is(err, user.ErrNotFound) {
		return err
	}
	return user.ErrEmailTaken
}

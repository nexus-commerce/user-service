package usecase

import (
	"context"
	"errors"

	"github.com/nexus-commerce/user-service/internal/user"
)

type confirmRegistrationRegistrations interface {
	UpdateByToken(ctx context.Context, hash user.ConfirmationTokenHash, update func(*user.PendingRegistration) error) (*user.PendingRegistration, error)
}

type ConfirmRegistration struct {
	registrations confirmRegistrationRegistrations
}

func NewConfirmRegistration(registrations confirmRegistrationRegistrations) *ConfirmRegistration {
	return &ConfirmRegistration{
		registrations: registrations,
	}
}

func (c *ConfirmRegistration) Execute(ctx context.Context, rawToken string) error {
	token, err := user.ParseConfirmationToken(rawToken)
	if err != nil {
		return err
	}

	_, err = c.registrations.UpdateByToken(ctx, token.Hash(), func(p *user.PendingRegistration) error {
		return p.Confirm(token)
	})
	if errors.Is(err, user.ErrRegistrationNotFound) {
		return user.ErrInvalidConfirmationToken
	}
	return err
}

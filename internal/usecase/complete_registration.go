package usecase

import (
	"context"
	"errors"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
)

type completeRegistrationRegistrations interface {
	ByUserID(ctx context.Context, id uuid.UUID) (*user.PendingRegistration, error)
	Reject(ctx context.Context, id uuid.UUID, rejectFn func(*user.PendingRegistration) error) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type completeRegistrationUsers interface {
	Add(ctx context.Context, usr *user.User) error
}

type CompleteRegistration struct {
	registrations completeRegistrationRegistrations
	users         completeRegistrationUsers
}

func NewCompleteRegistration(registrations completeRegistrationRegistrations, users completeRegistrationUsers) *CompleteRegistration {
	return &CompleteRegistration{
		registrations: registrations,
		users:         users,
	}
}

func (c *CompleteRegistration) Execute(ctx context.Context, userID uuid.UUID) error {
	pending, err := c.registrations.ByUserID(ctx, userID)
	if errors.Is(err, user.ErrRegistrationNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	usr, err := pending.Complete()
	if err != nil {
		return err
	}

	err = c.users.Add(ctx, usr)
	if errors.Is(err, user.ErrEmailTaken) {
		return c.reject(ctx, userID)
	}
	if err != nil {
		return err
	}
	return c.registrations.Delete(ctx, userID)
}

func (c *CompleteRegistration) reject(ctx context.Context, userID uuid.UUID) error {
	err := c.registrations.Reject(ctx, userID, (*user.PendingRegistration).Reject)
	if err != nil && !errors.Is(err, user.ErrRegistrationNotFound) {
		return err
	}
	return user.ErrEmailTaken
}

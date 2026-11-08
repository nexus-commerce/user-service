package kafka

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/user"
	"github.com/nexus-commerce/user-service/internal/userevent"
)

type registrationCompleter interface {
	Execute(ctx context.Context, userID uuid.UUID) error
}

type RegistrationConfirmed struct {
	uc registrationCompleter
}

func NewRegistrationConfirmed(uc registrationCompleter) *RegistrationConfirmed {
	return &RegistrationConfirmed{uc: uc}
}

func (h *RegistrationConfirmed) EventType() string { return userevent.TypeRegistrationConfirmed }

func (h *RegistrationConfirmed) Handle(ctx context.Context, msg messaging.Message) error {
	userID, err := userevent.DecodeRegistrationConfirmed(msg.Value)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPermanent, err)
	}

	err = h.uc.Execute(ctx, userID)
	switch {
	case errors.Is(err, user.ErrEmailTaken):
		return nil
	case errors.Is(err, user.ErrRegistrationNotConfirmed):
		return fmt.Errorf("%w: %w", ErrPermanent, err)
	}
	return err
}

package kafka

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/user"
	"github.com/nexus-commerce/user-service/internal/userevent"
)

type fakeCompleter struct {
	err error
	got uuid.UUID
}

func (f *fakeCompleter) Execute(_ context.Context, id uuid.UUID) error {
	f.got = id
	return f.err
}

func registrationConfirmedMessage(id uuid.UUID) messaging.Message {
	return messaging.Message{
		Type:  userevent.TypeRegistrationConfirmed,
		Value: []byte(`{"eventType":"user.registration_confirmed","data":{"userId":"` + id.String() + `"}}`),
	}
}

func TestRegistrationConfirmedHandle(t *testing.T) {
	transient := errors.New("db unavailable")
	tests := []struct {
		name          string
		ucErr         error
		wantNil       bool
		wantPermanent bool
		wantErr       error
	}{
		{name: "completed", wantNil: true},
		{name: "email taken", ucErr: user.ErrEmailTaken, wantNil: true},
		{name: "not confirmed", ucErr: user.ErrRegistrationNotConfirmed, wantPermanent: true},
		{name: "transient", ucErr: transient, wantErr: transient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			uc := &fakeCompleter{err: tt.ucErr}

			err := NewRegistrationConfirmed(uc).Handle(t.Context(), registrationConfirmedMessage(id))

			if uc.got != id {
				t.Errorf("Execute() got user %v, want %v", uc.got, id)
			}
			switch {
			case tt.wantNil && err != nil:
				t.Errorf("Handle() error = %v, want nil", err)
			case tt.wantPermanent && !errors.Is(err, ErrPermanent):
				t.Errorf("Handle() error = %v, want %v", err, ErrPermanent)
			case tt.wantErr != nil && (!errors.Is(err, tt.wantErr) || errors.Is(err, ErrPermanent)):
				t.Errorf("Handle() error = %v, want transient %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistrationConfirmedMalformedIsPermanent(t *testing.T) {
	uc := &fakeCompleter{}
	err := NewRegistrationConfirmed(uc).Handle(t.Context(), messaging.Message{Value: []byte("{")})
	if !errors.Is(err, ErrPermanent) || !errors.Is(err, userevent.ErrMalformedMessage) {
		t.Errorf("Handle() error = %v, want %v wrapping %v", err, ErrPermanent, userevent.ErrMalformedMessage)
	}
	if uc.got != (uuid.UUID{}) {
		t.Error("use case called for malformed message")
	}
}

package user_test

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
)

func mustPending(t *testing.T) (*user.PendingRegistration, user.ConfirmationToken) {
	t.Helper()
	p, err := user.RequestRegistration(mustEmail(t, janeEmail), mustFullName(t, janeDoe), mustPassword(t, "s3cretpass"), fakeHasher{})
	if err != nil {
		t.Fatalf("RequestRegistration() error: %v", err)
	}
	for _, e := range p.Events() {
		if r, ok := e.(user.RegistrationRequested); ok {
			return p, r.Token
		}
	}
	t.Fatal("RequestRegistration() recorded no RegistrationRequested event")
	return nil, user.ConfirmationToken{}
}

func TestRequestRegistrationRecordsEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		events := p.Events()
		if len(events) != 1 {
			t.Fatalf("Events() len = %d, want 1", len(events))
		}
		e, ok := events[0].(user.RegistrationRequested)
		if !ok {
			t.Fatalf("Events()[0] is %T, want RegistrationRequested", events[0])
		}
		if e.ID == uuid.Nil() || e.AggregateID != p.UserID() || e.Email != p.Email() || e.FullName != p.FullName() {
			t.Errorf("RegistrationRequested = %+v does not match pending registration", e)
		}
		if e.Token.Hash() != p.TokenHash() || p.TokenHash() != tok.Hash() {
			t.Errorf("token hash = %q, pending hash = %q, want equal", e.Token.Hash().Encoded(), p.TokenHash().Encoded())
		}
		if !e.OccurredAt.Equal(time.Now()) || !e.ExpiresAt.Equal(p.ExpiresAt()) || !e.ExpiresAt.Equal(e.OccurredAt.Add(24*time.Hour)) {
			t.Errorf("OccurredAt = %v, ExpiresAt = %v, pending ExpiresAt = %v", e.OccurredAt, e.ExpiresAt, p.ExpiresAt())
		}
		if got := p.PasswordHash().Encoded(); got != "hashed:s3cretpass" {
			t.Errorf("PasswordHash() = %q, want %q", got, "hashed:s3cretpass")
		}
	})
}

func TestConfirmMarksRegistrationConfirmed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		time.Sleep(time.Hour)
		if err := p.Confirm(tok); err != nil {
			t.Fatalf("Confirm() error: %v", err)
		}
		at, ok := p.ConfirmedAt()
		if !ok || !p.Confirmed() || !at.Equal(time.Now()) {
			t.Errorf("ConfirmedAt() = %v, %v; want %v, true", at, ok, time.Now())
		}
		events := p.Events()
		if len(events) != 2 {
			t.Fatalf("Events() len = %d, want 2", len(events))
		}
		e, ok := events[1].(user.RegistrationConfirmed)
		if !ok {
			t.Fatalf("Events()[1] is %T, want RegistrationConfirmed", events[1])
		}
		if e.ID == uuid.Nil() || e.AggregateID != p.UserID() || !e.OccurredAt.Equal(at) {
			t.Errorf("RegistrationConfirmed = %+v, want aggregate %v at %v", e, p.UserID(), at)
		}
	})
}

func TestConfirmRejectsWrongToken(t *testing.T) {
	p, _ := mustPending(t)
	if err := p.Confirm(user.NewConfirmationToken()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("Confirm(wrong) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
	if p.Confirmed() {
		t.Error("registration confirmed with a wrong token")
	}
}

func TestConfirmTwiceRejectsToken(t *testing.T) {
	p, tok := mustPending(t)
	if err := p.Confirm(tok); err != nil {
		t.Fatalf("first Confirm() error: %v", err)
	}
	if err := p.Confirm(tok); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("second Confirm() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
	if n := len(p.Events()); n != 2 {
		t.Errorf("Events() len = %d, want 2", n)
	}
}

func TestConfirmAcceptsTokenAtExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		time.Sleep(24 * time.Hour)
		if err := p.Confirm(tok); err != nil {
			t.Errorf("Confirm() at expiry instant error: %v", err)
		}
	})
}

func TestConfirmRejectsExpiredToken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		time.Sleep(25 * time.Hour)
		if err := p.Confirm(tok); !errors.Is(err, user.ErrConfirmationTokenExpired) {
			t.Errorf("Confirm(expired) error = %v, want %v", err, user.ErrConfirmationTokenExpired)
		}
		if p.Confirmed() {
			t.Error("expired registration confirmed")
		}
	})
}

func TestEnsureSupersedableAllowsUnconfirmedRegistration(t *testing.T) {
	existing, _ := mustPending(t)
	if err := existing.EnsureSupersedable(); err != nil {
		t.Errorf("EnsureSupersedable() error: %v", err)
	}
}

func TestEnsureSupersedableRejectsConfirmedRegistration(t *testing.T) {
	existing, tok := mustPending(t)
	if err := existing.Confirm(tok); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	if err := existing.EnsureSupersedable(); !errors.Is(err, user.ErrRegistrationAlreadyConfirmed) {
		t.Errorf("EnsureSupersedable() error = %v, want %v", err, user.ErrRegistrationAlreadyConfirmed)
	}
}

func TestRejectRequiresConfirmation(t *testing.T) {
	p, _ := mustPending(t)
	if err := p.Reject(); !errors.Is(err, user.ErrRegistrationNotConfirmed) {
		t.Errorf("Reject() error = %v, want %v", err, user.ErrRegistrationNotConfirmed)
	}
}

func TestRejectRecordsEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		if err := p.Confirm(tok); err != nil {
			t.Fatalf("Confirm() error: %v", err)
		}
		if err := p.Reject(); err != nil {
			t.Fatalf("Reject() error: %v", err)
		}
		events := p.Events()
		e, ok := events[len(events)-1].(user.RegistrationRejected)
		if !ok {
			t.Fatalf("last event is %T, want RegistrationRejected", events[len(events)-1])
		}
		if e.AggregateID != p.UserID() || e.Email != p.Email() || !e.OccurredAt.Equal(time.Now()) {
			t.Errorf("RegistrationRejected = %+v, want aggregate %v, email %q", e, p.UserID(), p.Email())
		}
	})
}

func TestCompleteRequiresConfirmation(t *testing.T) {
	p, _ := mustPending(t)
	if _, err := p.Complete(); !errors.Is(err, user.ErrRegistrationNotConfirmed) {
		t.Errorf("Complete() error = %v, want %v", err, user.ErrRegistrationNotConfirmed)
	}
}

func TestCompleteCreatesRegisteredUser(t *testing.T) {
	p, tok := mustPending(t)
	if err := p.Confirm(tok); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	u, err := p.Complete()
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if u.ID() != p.UserID() || u.Email() != p.Email() || u.FullName() != p.FullName() || u.PasswordHash() != p.PasswordHash() {
		t.Errorf("Complete() = {id: %v, email: %q, hash: %q}, want {id: %v, email: %q, hash: %q}",
			u.ID(), u.Email(), u.PasswordHash().Encoded(), p.UserID(), p.Email(), p.PasswordHash().Encoded())
	}
	if _, pending := u.PendingEmailChange(); pending {
		t.Error("new user has a pending email change")
	}
	if err := u.Authenticate(mustPassword(t, "s3cretpass"), fakeHasher{}); err != nil {
		t.Errorf("Authenticate() error: %v", err)
	}

	events := u.Events()
	if len(events) != 1 {
		t.Fatalf("Events() returned %d events, want 1", len(events))
	}
	reg, ok := events[0].(user.Registered)
	if !ok {
		t.Fatalf("Events()[0] is %T, want Registered", events[0])
	}
	if reg.ID == uuid.Nil() {
		t.Errorf("Events()[0].ID = nil, want non-nil")
	}
	want := user.Registered{
		EventMeta: user.EventMeta{ID: reg.ID, AggregateID: p.UserID(), OccurredAt: u.CreatedAt()},
		Email:     p.Email(),
		FullName:  p.FullName(),
	}
	if reg != want {
		t.Errorf("Events()[0] = %+v, want %+v", reg, want)
	}
}

func TestRestorePendingRegistration(t *testing.T) {
	id := uuid.New()
	email := mustEmail(t, janeEmail)
	name := mustFullName(t, janeDoe)
	hash := mustHash(t)
	tok := user.NewConfirmationToken()
	expiresAt := time.Now().UTC().Add(time.Hour)

	p := user.RestorePendingRegistration(id, email, name, hash, tok.Hash(), expiresAt, nil)
	matches := p.UserID() == id && p.Email() == email && p.FullName() == name && p.PasswordHash() == hash && p.TokenHash() == tok.Hash() && p.ExpiresAt().Equal(expiresAt)
	if !matches {
		t.Errorf("RestorePendingRegistration() = {id: %v, email: %q, hash: %q, expiresAt: %v}, want {id: %v, email: %q, hash: %q, expiresAt: %v}",
			p.UserID(), p.Email(), p.PasswordHash().Encoded(), p.ExpiresAt(), id, email, hash.Encoded(), expiresAt)
	}
	if p.Confirmed() {
		t.Error("restored unconfirmed registration reports Confirmed()")
	}
	if events := p.Events(); len(events) != 0 {
		t.Errorf("Events() after RestorePendingRegistration returned %d events, want 0", len(events))
	}
	if err := p.Confirm(tok); err != nil {
		t.Fatalf("Confirm() on restored registration error: %v", err)
	}
}

func TestRestoreConfirmedRegistration(t *testing.T) {
	tok := user.NewConfirmationToken()
	confirmedAt := time.Now().UTC()
	p := user.RestorePendingRegistration(uuid.New(), mustEmail(t, janeEmail), mustFullName(t, janeDoe), mustHash(t),
		tok.Hash(), confirmedAt.Add(time.Hour), &confirmedAt)

	if at, ok := p.ConfirmedAt(); !ok || !at.Equal(confirmedAt) {
		t.Errorf("ConfirmedAt() = %v, %v; want %v, true", at, ok, confirmedAt)
	}
	if err := p.Confirm(tok); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("Confirm() on confirmed registration error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
	if _, err := p.Complete(); err != nil {
		t.Errorf("Complete() on restored confirmed registration error: %v", err)
	}
}

package user_test

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
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

func TestConfirmCreatesRegisteredUser(t *testing.T) {
	p, tok := mustPending(t)
	u, err := p.Confirm(tok)
	if err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	if u.ID() != p.UserID() || u.Email() != p.Email() || u.FullName() != p.FullName() || u.PasswordHash() != p.PasswordHash() {
		t.Errorf("Confirm() = {id: %v, email: %q, hash: %q}, want {id: %v, email: %q, hash: %q}",
			u.ID(), u.Email(), u.PasswordHash().Encoded(), p.UserID(), p.Email(), p.PasswordHash().Encoded())
	}
	if _, pending := u.PendingEmailChange(); pending {
		t.Error("confirmed user has a pending email change")
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

func TestConfirmRejectsWrongToken(t *testing.T) {
	p, _ := mustPending(t)
	if _, err := p.Confirm(user.NewConfirmationToken()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("Confirm(wrong) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestConfirmAcceptsTokenAtExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		time.Sleep(24 * time.Hour)
		if _, err := p.Confirm(tok); err != nil {
			t.Errorf("Confirm() at expiry instant error: %v", err)
		}
	})
}

func TestConfirmRejectsExpiredToken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		time.Sleep(25 * time.Hour)
		if _, err := p.Confirm(tok); !errors.Is(err, user.ErrConfirmationTokenExpired) {
			t.Errorf("Confirm(expired) error = %v, want %v", err, user.ErrConfirmationTokenExpired)
		}
	})
}

func TestRestorePendingRegistration(t *testing.T) {
	id := uuid.New()
	email := mustEmail(t, janeEmail)
	name := mustFullName(t, janeDoe)
	hash := mustHash(t)
	tok := user.NewConfirmationToken()
	expiresAt := time.Now().UTC().Add(time.Hour)

	p := user.RestorePendingRegistration(id, email, name, hash, tok.Hash(), expiresAt)
	matches := p.UserID() == id && p.Email() == email && p.FullName() == name && p.PasswordHash() == hash && p.TokenHash() == tok.Hash() && p.ExpiresAt().Equal(expiresAt)
	if !matches {
		t.Errorf("RestorePendingRegistration() = {id: %v, email: %q, hash: %q, expiresAt: %v}, want {id: %v, email: %q, hash: %q, expiresAt: %v}",
			p.UserID(), p.Email(), p.PasswordHash().Encoded(), p.ExpiresAt(), id, email, hash.Encoded(), expiresAt)
	}
	if events := p.Events(); len(events) != 0 {
		t.Errorf("Events() after RestorePendingRegistration returned %d events, want 0", len(events))
	}
	u, err := p.Confirm(tok)
	if err != nil {
		t.Fatalf("Confirm() on restored registration error: %v", err)
	}
	if u.ID() != id {
		t.Errorf("Confirm() user ID = %v, want %v", u.ID(), id)
	}
}

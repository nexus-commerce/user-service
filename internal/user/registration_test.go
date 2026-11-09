package user_test

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/nexus-commerce/user-service/internal/user"
)

type registrationFields struct {
	UserID       uuid.UUID
	Email        string
	FullName     nameFields
	PasswordHash string
	TokenHash    string
	ExpiresAt    time.Time
}

func registrationOf(p *user.PendingRegistration) registrationFields {
	return registrationFields{
		UserID:       p.UserID(),
		Email:        p.Email().String(),
		FullName:     fieldsOf(p.FullName()),
		PasswordHash: p.PasswordHash().Encoded(),
		TokenHash:    p.TokenHash().Encoded(),
		ExpiresAt:    p.ExpiresAt(),
	}
}

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

func lastRequested(t *testing.T, p *user.PendingRegistration) user.RegistrationRequested {
	t.Helper()
	events := p.Events()
	e, ok := events[len(events)-1].(user.RegistrationRequested)
	if !ok {
		t.Fatalf("Events() last event is %T, want RegistrationRequested", events[len(events)-1])
	}
	return e
}

func TestRequestRegistrationRecordsEvent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, tok := mustPending(t)
		events := p.Events()
		if len(events) != 1 {
			t.Fatalf("RequestRegistration() Events() len = %d, want 1", len(events))
		}
		e, ok := events[0].(user.RegistrationRequested)
		if !ok {
			t.Fatalf("RequestRegistration() Events()[0] is %T, want RegistrationRequested", events[0])
		}
		if e.ID == uuid.Nil() {
			t.Error("RequestRegistration() RegistrationRequested.ID = nil, want non-nil")
		}
		want := user.RegistrationRequested{
			AggregateID: p.UserID(),
			OccurredAt:  time.Now(),
			Email:       mustEmail(t, janeEmail),
			FullName:    mustFullName(t, janeDoe),
			Token:       tok,
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}
		if diff := cmp.Diff(want, e, valueObjects, ignoreEventID); diff != "" {
			t.Errorf("RequestRegistration() RegistrationRequested event mismatch (-want +got):\n%s", diff)
		}
		wantPending := registrationFields{
			Email:        janeEmail,
			FullName:     janeDoe,
			PasswordHash: "hashed:s3cretpass",
			TokenHash:    tok.Hash().Encoded(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		ignoreUserID := cmpopts.IgnoreFields(registrationFields{}, "UserID")
		if diff := cmp.Diff(wantPending, registrationOf(p), ignoreUserID); diff != "" {
			t.Errorf("RequestRegistration() mismatch (-want +got):\n%s", diff)
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
		if !p.Confirmed() {
			t.Error("Confirm() left Confirmed() = false, want true")
		}
		if at, ok := p.ConfirmedAt(); !ok || !at.Equal(time.Now()) {
			t.Errorf("Confirm() ConfirmedAt() = %v, %v; want %v, true", at, ok, time.Now())
		}
		events := p.Events()
		if len(events) != 2 {
			t.Fatalf("Confirm() Events() len = %d, want 2", len(events))
		}
		e, ok := events[1].(user.RegistrationConfirmed)
		if !ok {
			t.Fatalf("Confirm() Events()[1] is %T, want RegistrationConfirmed", events[1])
		}
		if e.ID == uuid.Nil() {
			t.Error("Confirm() RegistrationConfirmed.ID = nil, want non-nil")
		}
		want := user.RegistrationConfirmed{AggregateID: p.UserID(), OccurredAt: time.Now()}
		if diff := cmp.Diff(want, e, ignoreEventID); diff != "" {
			t.Errorf("Confirm() RegistrationConfirmed event mismatch (-want +got):\n%s", diff)
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
		t.Errorf("second Confirm() Events() len = %d, want 2", n)
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

var janetRoe = nameFields{FirstName: "Janet", LastName: "Roe"}

func newerRequest(t *testing.T, email string) *user.PendingRegistration {
	t.Helper()
	p, err := user.RequestRegistration(mustEmail(t, email), mustFullName(t, janetRoe), mustPassword(t, "n3wpassword"), fakeHasher{})
	if err != nil {
		t.Fatalf("RequestRegistration() error: %v", err)
	}
	return p
}

func TestSupersedeRenewsRegistration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, _ := mustPending(t)
		id, email := p.UserID(), p.Email()
		time.Sleep(23 * time.Hour)
		newer := newerRequest(t, janeEmail)
		name, hash := newer.FullName(), newer.PasswordHash()

		if err := p.Supersede(newer); err != nil {
			t.Fatalf("Supersede() error: %v", err)
		}
		want := registrationFields{
			UserID:       id,
			Email:        email.String(),
			FullName:     fieldsOf(name),
			PasswordHash: hash.Encoded(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		ignoreToken := cmpopts.IgnoreFields(registrationFields{}, "TokenHash")
		if diff := cmp.Diff(want, registrationOf(p), ignoreToken); diff != "" {
			t.Errorf("Supersede() mismatch (-want +got):\n%s", diff)
		}
		if n := len(p.Events()); n != 2 {
			t.Fatalf("Supersede() Events() len = %d, want 2", n)
		}
		e := lastRequested(t, p)
		wantEvent := user.RegistrationRequested{
			AggregateID: id,
			OccurredAt:  time.Now(),
			Email:       email,
			FullName:    name,
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}
		ignoreEventToken := cmpopts.IgnoreFields(user.RegistrationRequested{}, "Token")
		if diff := cmp.Diff(wantEvent, e, valueObjects, ignoreEventID, ignoreEventToken); diff != "" {
			t.Errorf("Supersede() RegistrationRequested event mismatch (-want +got):\n%s", diff)
		}
		if got, want := e.Token.Hash().Encoded(), p.TokenHash().Encoded(); got != want {
			t.Errorf("Supersede() event token hash = %q, want pending token hash %q", got, want)
		}
	})
}

func TestSupersedeIssuesFreshToken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, old := mustPending(t)
		time.Sleep(25 * time.Hour)
		if err := p.Supersede(newerRequest(t, janeEmail)); err != nil {
			t.Fatalf("Supersede() error: %v", err)
		}
		fresh := lastRequested(t, p).Token

		if err := p.Confirm(old); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("Confirm(superseded token) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
		}
		if err := p.Confirm(fresh); err != nil {
			t.Errorf("Confirm(fresh token) error: %v", err)
		}
	})
}

func TestSupersedeRejectsConfirmedRegistration(t *testing.T) {
	p, tok := mustPending(t)
	if err := p.Confirm(tok); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	before := registrationOf(p)
	if err := p.Supersede(newerRequest(t, janeEmail)); !errors.Is(err, user.ErrRegistrationAlreadyConfirmed) {
		t.Errorf("Supersede() error = %v, want %v", err, user.ErrRegistrationAlreadyConfirmed)
	}
	if diff := cmp.Diff(before, registrationOf(p)); diff != "" {
		t.Errorf("Supersede() changed a confirmed registration (-want +got):\n%s", diff)
	}
	if n := len(p.Events()); n != 2 {
		t.Errorf("Supersede() Events() len = %d, want 2", n)
	}
}

func TestSupersedeRejectsOtherEmail(t *testing.T) {
	p, _ := mustPending(t)
	before := registrationOf(p)
	if err := p.Supersede(newerRequest(t, "other@example.com")); !errors.Is(err, user.ErrRegistrationEmailMismatch) {
		t.Errorf("Supersede() error = %v, want %v", err, user.ErrRegistrationEmailMismatch)
	}
	if diff := cmp.Diff(before, registrationOf(p)); diff != "" {
		t.Errorf("Supersede(other email) changed the registration (-want +got):\n%s", diff)
	}
	if n := len(p.Events()); n != 1 {
		t.Errorf("Supersede(other email) Events() len = %d, want 1", n)
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
			t.Fatalf("Reject() last event is %T, want RegistrationRejected", events[len(events)-1])
		}
		want := user.RegistrationRejected{
			AggregateID: p.UserID(),
			OccurredAt:  time.Now(),
			Email:       mustEmail(t, janeEmail),
		}
		if diff := cmp.Diff(want, e, valueObjects, ignoreEventID); diff != "" {
			t.Errorf("Reject() RegistrationRejected event mismatch (-want +got):\n%s", diff)
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
	wantUser := userFields{
		ID:           p.UserID(),
		Email:        p.Email().String(),
		FullName:     fieldsOf(p.FullName()),
		PasswordHash: p.PasswordHash().Encoded(),
	}
	ignoreCreatedAt := cmpopts.IgnoreFields(userFields{}, "CreatedAt")
	if diff := cmp.Diff(wantUser, userOf(u), ignoreCreatedAt); diff != "" {
		t.Errorf("Complete() mismatch (-want +got):\n%s", diff)
	}
	if _, pending := u.PendingEmailChange(); pending {
		t.Error("new user has a pending email change")
	}
	if err := u.Authenticate(mustPassword(t, "s3cretpass"), fakeHasher{}); err != nil {
		t.Errorf("Authenticate() error: %v", err)
	}

	events := u.Events()
	if len(events) != 1 {
		t.Fatalf("Complete() Events() returned %d events, want 1", len(events))
	}
	reg, ok := events[0].(user.Registered)
	if !ok {
		t.Fatalf("Complete() Events()[0] is %T, want Registered", events[0])
	}
	if reg.ID == uuid.Nil() {
		t.Error("Complete() Registered.ID = nil, want non-nil")
	}
	want := user.Registered{
		AggregateID: p.UserID(),
		OccurredAt:  u.CreatedAt(),
		Email:       mustEmail(t, janeEmail),
		FullName:    mustFullName(t, janeDoe),
	}
	if diff := cmp.Diff(want, reg, valueObjects, ignoreEventID); diff != "" {
		t.Errorf("Complete() Registered event mismatch (-want +got):\n%s", diff)
	}
}

func TestRestorePendingRegistration(t *testing.T) {
	id := uuid.New()
	email := mustEmail(t, janeEmail)
	name := mustFullName(t, janeDoe)
	hash := mustHash(t)
	tok := user.NewConfirmationToken()
	expiresAt := time.Now().UTC().Add(time.Hour)

	p := user.RestorePendingRegistration(user.PendingRegistrationSnapshot{
		UserID:       id,
		Email:        email,
		FullName:     name,
		PasswordHash: hash,
		TokenHash:    tok.Hash(),
		ExpiresAt:    expiresAt,
	})
	want := registrationFields{
		UserID:       id,
		Email:        email.String(),
		FullName:     fieldsOf(name),
		PasswordHash: hash.Encoded(),
		TokenHash:    tok.Hash().Encoded(),
		ExpiresAt:    expiresAt,
	}
	if diff := cmp.Diff(want, registrationOf(p)); diff != "" {
		t.Errorf("RestorePendingRegistration() mismatch (-want +got):\n%s", diff)
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
	email := mustEmail(t, janeEmail)
	name := mustFullName(t, janeDoe)
	hash := mustHash(t)

	p := user.RestorePendingRegistration(user.PendingRegistrationSnapshot{
		UserID:       uuid.New(),
		Email:        email,
		FullName:     name,
		PasswordHash: hash,
		TokenHash:    tok.Hash(),
		ExpiresAt:    confirmedAt.Add(time.Hour),
		ConfirmedAt:  &confirmedAt,
	})

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

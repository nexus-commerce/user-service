package usecase_test

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

func requestedJane(t *testing.T) (*user.User, *fakeUsers, user.ConfirmationToken) {
	t.Helper()
	usr, users := newJane(t)
	email, err := user.NewEmail("new@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error: %v", err)
	}
	password, err := user.NewPassword("s3cretpass")
	if err != nil {
		t.Fatalf("NewPassword() error: %v", err)
	}
	if err = usr.RequestEmailChange(email, password, fakeHasher{}); err != nil {
		t.Fatalf("RequestEmailChange() error: %v", err)
	}
	for _, e := range usr.Events() {
		if r, ok := e.(user.EmailChangeRequested); ok {
			return usr, users, r.Token
		}
	}
	t.Fatal("no EmailChangeRequested event")
	return nil, nil, user.ConfirmationToken{}
}

func TestConfirmEmailChange(t *testing.T) {
	usr, users, tok := requestedJane(t)

	if err := usecase.NewConfirmEmailChange(users).Execute(t.Context(), tok.Plaintext()); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if got := usr.Email().String(); got != "new@example.com" {
		t.Errorf("Email() = %q, want new@example.com", got)
	}
	var changed int
	for _, e := range usr.Events() {
		if _, ok := e.(user.EmailChanged); ok {
			changed++
		}
	}
	if changed != 1 {
		t.Errorf("recorded %d EmailChanged events, want 1", changed)
	}
}

func TestConfirmEmailChangeRejectsBadToken(t *testing.T) {
	_, users, _ := requestedJane(t)
	for _, token := range []string{"", "abc", strings.Repeat("!", 43), user.NewConfirmationToken().Plaintext()} {
		err := usecase.NewConfirmEmailChange(users).Execute(t.Context(), token)
		if !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("Execute(%q) error = %v, want %v", token, err, user.ErrInvalidConfirmationToken)
		}
	}
}

func TestConfirmEmailChangeEmailTakenMeanwhile(t *testing.T) {
	_, users, tok := requestedJane(t)
	users.updateErr = user.ErrEmailTaken

	err := usecase.NewConfirmEmailChange(users).Execute(t.Context(), tok.Plaintext())
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("Execute() error = %v, want %v", err, user.ErrEmailTaken)
	}
}

func TestConfirmEmailChangeExpired(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		usr, users, tok := requestedJane(t)
		time.Sleep(25 * time.Hour)

		err := usecase.NewConfirmEmailChange(users).Execute(t.Context(), tok.Plaintext())
		if !errors.Is(err, user.ErrConfirmationTokenExpired) {
			t.Errorf("Execute() error = %v, want %v", err, user.ErrConfirmationTokenExpired)
		}
		if got := usr.Email().String(); got != "jane@example.com" {
			t.Errorf("Email() = %q, want unchanged", got)
		}
	})
}

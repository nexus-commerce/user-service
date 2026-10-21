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

func registeredJane(t *testing.T) (*fakeUsers, *fakeRegistrations, user.ConfirmationToken) {
	t.Helper()
	users := &fakeUsers{}
	regs := &fakeRegistrations{users: users}
	if err := usecase.NewRegister(users, regs, fakeHasher{}).Execute(t.Context(), validRegistration("jane@example.com")); err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	return users, regs, tokenFor(t, regs, "jane@example.com")
}

func TestConfirmRegistration(t *testing.T) {
	users, regs, tok := registeredJane(t)

	token, err := usecase.NewConfirmRegistration(regs, fakeTokens{}).Execute(t.Context(), tok.Plaintext())
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	email, err := user.NewEmail("jane@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error: %v", err)
	}
	usr, err := users.ByEmail(t.Context(), email)
	if err != nil {
		t.Fatalf("ByEmail() error: %v", err)
	}
	if token != "token:"+usr.ID().String() {
		t.Errorf("Execute() token = %q, want token for %v", token, usr.ID())
	}
	if len(regs.byEmail) != 0 {
		t.Errorf("%d pending registrations left, want 0", len(regs.byEmail))
	}
}

func TestConfirmRegistrationRejectsBadToken(t *testing.T) {
	_, regs, _ := registeredJane(t)
	for _, token := range []string{"", "abc", strings.Repeat("!", 43), user.NewConfirmationToken().Plaintext()} {
		_, err := usecase.NewConfirmRegistration(regs, fakeTokens{}).Execute(t.Context(), token)
		if !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("Execute(%q) error = %v, want %v", token, err, user.ErrInvalidConfirmationToken)
		}
	}
}

func TestConfirmRegistrationTokenSingleUse(t *testing.T) {
	_, regs, tok := registeredJane(t)
	confirm := usecase.NewConfirmRegistration(regs, fakeTokens{})

	if _, err := confirm.Execute(t.Context(), tok.Plaintext()); err != nil {
		t.Fatalf("first Execute() error: %v", err)
	}
	if _, err := confirm.Execute(t.Context(), tok.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("second Execute() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestConfirmRegistrationExpired(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		users, regs, tok := registeredJane(t)
		time.Sleep(25 * time.Hour)

		token, err := usecase.NewConfirmRegistration(regs, fakeTokens{}).Execute(t.Context(), tok.Plaintext())
		if !errors.Is(err, user.ErrConfirmationTokenExpired) {
			t.Errorf("Execute() error = %v, want %v", err, user.ErrConfirmationTokenExpired)
		}
		if token != "" {
			t.Errorf("Execute() token = %q, want empty", token)
		}
		if len(users.byID) != 0 {
			t.Errorf("users has %d entries, want 0", len(users.byID))
		}
	})
}

func TestConfirmRegistrationEmailTakenMeanwhile(t *testing.T) {
	_, regs, tok := registeredJane(t)
	regs.confirmErr = user.ErrEmailTaken

	token, err := usecase.NewConfirmRegistration(regs, fakeTokens{}).Execute(t.Context(), tok.Plaintext())
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("Execute() error = %v, want %v", err, user.ErrEmailTaken)
	}
	if token != "" {
		t.Errorf("Execute() token = %q, want empty", token)
	}
}

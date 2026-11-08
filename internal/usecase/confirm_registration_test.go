package usecase_test

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

func registeredJane(t *testing.T) (*fakeUsers, *fakeRegistrations, user.ConfirmationToken) {
	t.Helper()
	users := &fakeUsers{}
	regs := &fakeRegistrations{}
	if err := usecase.NewRequestRegistration(users, regs, fakeHasher{}).Execute(t.Context(), validRegistrationRequest("jane@example.com")); err != nil {
		t.Fatalf("RequestRegistration() error: %v", err)
	}
	return users, regs, tokenFor(t, regs, "jane@example.com")
}

func pendingJane(t *testing.T, regs *fakeRegistrations) *user.PendingRegistration {
	t.Helper()
	p, ok := regs.byEmail[mustEmail(t, "jane@example.com")]
	if !ok {
		t.Fatal("no pending registration for jane@example.com")
	}
	return p
}

func mustEmail(t *testing.T, s string) user.Email {
	t.Helper()
	e, err := user.NewEmail(s)
	if err != nil {
		t.Fatalf("NewEmail(%q) error: %v", s, err)
	}
	return e
}

func publishedOf[E user.Event](events []user.Event) []E {
	var out []E
	for _, e := range events {
		if v, ok := e.(E); ok {
			out = append(out, v)
		}
	}
	return out
}

func TestConfirmRegistration(t *testing.T) {
	users, regs, tok := registeredJane(t)

	if err := usecase.NewConfirmRegistration(regs).Execute(t.Context(), tok.Plaintext()); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	p := pendingJane(t, regs)
	if !p.Confirmed() {
		t.Error("registration is not confirmed")
	}
	confirmed := publishedOf[user.RegistrationConfirmed](regs.published)
	if len(confirmed) != 1 || confirmed[0].AggregateID != p.UserID() {
		t.Errorf("RegistrationConfirmed events = %+v, want one for %v", confirmed, p.UserID())
	}
	if len(users.byID) != 0 || users.addCalls != 0 {
		t.Errorf("users = %d, Add calls = %d; want 0, 0", len(users.byID), users.addCalls)
	}
}

func TestConfirmRegistrationMalformedTokenSkipsRepository(t *testing.T) {
	_, regs, _ := registeredJane(t)
	regs.readErr = errors.New("repository must not be called")
	if err := usecase.NewConfirmRegistration(regs).Execute(t.Context(), "bad"); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Fatalf("Execute() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestConfirmRegistrationRejectsBadToken(t *testing.T) {
	_, regs, _ := registeredJane(t)
	for _, token := range []string{"", "abc", strings.Repeat("!", 43), user.NewConfirmationToken().Plaintext()} {
		err := usecase.NewConfirmRegistration(regs).Execute(t.Context(), token)
		if !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("Execute(%q) error = %v, want %v", token, err, user.ErrInvalidConfirmationToken)
		}
	}
}

func TestConfirmRegistrationTokenSingleUse(t *testing.T) {
	users, regs, tok := registeredJane(t)
	confirm := usecase.NewConfirmRegistration(regs)

	if err := confirm.Execute(t.Context(), tok.Plaintext()); err != nil {
		t.Fatalf("first Execute() error: %v", err)
	}
	if err := confirm.Execute(t.Context(), tok.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("Execute() before completion error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}

	if err := usecase.NewCompleteRegistration(regs, users).Execute(t.Context(), pendingJane(t, regs).UserID()); err != nil {
		t.Fatalf("CompleteRegistration error: %v", err)
	}
	if err := confirm.Execute(t.Context(), tok.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("Execute() after completion error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestConfirmRegistrationExpired(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, regs, tok := registeredJane(t)
		time.Sleep(25 * time.Hour)

		if err := usecase.NewConfirmRegistration(regs).Execute(t.Context(), tok.Plaintext()); !errors.Is(err, user.ErrConfirmationTokenExpired) {
			t.Errorf("Execute() error = %v, want %v", err, user.ErrConfirmationTokenExpired)
		}
		if pendingJane(t, regs).Confirmed() {
			t.Error("expired registration was confirmed")
		}
	})
}

func TestConfirmRegistrationUpdateFailure(t *testing.T) {
	_, regs, tok := registeredJane(t)
	want := errors.New("update failed")
	regs.updateErr = want

	if err := usecase.NewConfirmRegistration(regs).Execute(t.Context(), tok.Plaintext()); !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
	if pendingJane(t, regs).Confirmed() {
		t.Error("registration confirmed after failed update")
	}
	if n := len(publishedOf[user.RegistrationConfirmed](regs.published)); n != 0 {
		t.Errorf("published %d RegistrationConfirmed events, want 0", n)
	}
}

func TestConfirmRegistrationDoesNotMaskOtherErrors(t *testing.T) {
	_, regs, tok := registeredJane(t)
	regs.readErr = user.ErrEmailTaken

	if err := usecase.NewConfirmRegistration(regs).Execute(t.Context(), tok.Plaintext()); !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("Execute() error = %v, want original %v", err, user.ErrEmailTaken)
	}
}

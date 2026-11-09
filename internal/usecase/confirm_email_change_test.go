package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

func requestedJane(t *testing.T) (*user.User, *fakeUsers, user.ConfirmationToken) {
	t.Helper()
	u, users := newJane(t)
	email, err := user.NewEmail("new@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error: %v", err)
	}
	password, err := user.NewPassword("s3cretpass")
	if err != nil {
		t.Fatalf("NewPassword() error: %v", err)
	}
	proof, err := u.Reauthenticate(password, fakeHasher{})
	if err != nil {
		t.Fatalf("Reauthenticate() error: %v", err)
	}
	if err = u.RequestEmailChange(email, false, proof); err != nil {
		t.Fatalf("RequestEmailChange() error: %v", err)
	}
	for _, e := range u.Events() {
		if r, ok := e.(user.EmailChangeRequested); ok {
			return u, users, r.Token
		}
	}
	t.Fatal("no EmailChangeRequested event")
	return nil, nil, user.ConfirmationToken{}
}

func TestConfirmEmailChange(t *testing.T) {
	u, users, tok := requestedJane(t)

	if err := usecase.NewConfirmEmailChange(users).Execute(t.Context(), tok.Plaintext()); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if got := u.Email().String(); got != "new@example.com" {
		t.Errorf("after Execute() Email() = %q, want new@example.com", got)
	}
	var changed int
	for _, e := range u.Events() {
		if _, ok := e.(user.EmailChanged); ok {
			changed++
		}
	}
	if changed != 1 {
		t.Errorf("Execute() recorded %d EmailChanged events, want 1", changed)
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
	u, users, tok := requestedJane(t)
	newUser(t, users, "new@example.com", "John", "Roe")
	confirm := usecase.NewConfirmEmailChange(users)

	if err := confirm.Execute(t.Context(), tok.Plaintext()); !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("Execute() error = %v, want %v", err, user.ErrEmailTaken)
	}
	if got := u.Email().String(); got != "jane@example.com" {
		t.Errorf("Execute() Email() = %q, want %q", got, "jane@example.com")
	}
	if _, ok := u.PendingEmailChange(); ok {
		t.Error("Execute() left a pending email change, want none")
	}
	rejected := publishedOf[user.EmailChangeRejected](u.Events())
	want := []user.EmailChangeRejected{{AggregateID: u.ID(), NewEmail: mustEmail(t, "new@example.com")}}
	if diff := cmp.Diff(want, rejected, valueObjects, ignoreEventMeta); diff != "" {
		t.Errorf("Execute() EmailChangeRejected events mismatch (-want +got):\n%s", diff)
	}
	if n := len(publishedOf[user.EmailChanged](u.Events())); n != 0 {
		t.Errorf("Execute() recorded %d EmailChanged events, want 0", n)
	}
	if err := confirm.Execute(t.Context(), tok.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("repeated Execute() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestConfirmEmailChangeRejectFailure(t *testing.T) {
	u, users, tok := requestedJane(t)
	newUser(t, users, "new@example.com", "John", "Roe")
	want := errors.New("reject failed")
	users.updateErr = want

	if err := usecase.NewConfirmEmailChange(users).Execute(t.Context(), tok.Plaintext()); !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
	if _, ok := u.PendingEmailChange(); !ok {
		t.Error("pending email change lost after failed rejection")
	}
}

type vanishingEmailChange struct {
	*fakeUsers
	calls int
}

func (v *vanishingEmailChange) UpdateByEmailChangeToken(ctx context.Context, hash user.ConfirmationTokenHash, update user.Update) (*user.User, error) {
	v.calls++
	if v.calls > 1 {
		return nil, user.ErrNotFound
	}
	return v.fakeUsers.UpdateByEmailChangeToken(ctx, hash, update)
}

func TestConfirmEmailChangeVanishedBeforeReject(t *testing.T) {
	_, users, tok := requestedJane(t)
	newUser(t, users, "new@example.com", "John", "Roe")

	err := usecase.NewConfirmEmailChange(&vanishingEmailChange{fakeUsers: users}).Execute(t.Context(), tok.Plaintext())
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("Execute() error = %v, want %v", err, user.ErrEmailTaken)
	}
}

func TestConfirmEmailChangeExpired(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		u, users, tok := requestedJane(t)
		time.Sleep(25 * time.Hour)

		err := usecase.NewConfirmEmailChange(users).Execute(t.Context(), tok.Plaintext())
		if !errors.Is(err, user.ErrConfirmationTokenExpired) {
			t.Errorf("Execute() error = %v, want %v", err, user.ErrConfirmationTokenExpired)
		}
		if got := u.Email().String(); got != "jane@example.com" {
			t.Errorf("Email() = %q, want unchanged", got)
		}
	})
}

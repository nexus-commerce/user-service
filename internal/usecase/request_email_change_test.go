package usecase_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

func TestRequestEmailChange(t *testing.T) {
	tests := []struct {
		name      string
		newEmail  string
		password  string
		wantErr   error
		wantField usecase.Field
	}{
		{name: "success", newEmail: "new@example.com", password: "s3cretpass"},
		{name: "malformed email", newEmail: "x", password: "s3cretpass", wantErr: user.ErrMalformedEmail, wantField: usecase.FieldEmail},
		{name: "same email", newEmail: "JANE@example.com", password: "s3cretpass", wantErr: user.ErrEmailUnchanged, wantField: usecase.FieldEmail},
		{name: "wrong password", newEmail: "new@example.com", password: "wrongpass", wantErr: user.ErrInvalidCredentials},
		{name: "password fails format", newEmail: "new@example.com", password: "short", wantErr: user.ErrInvalidCredentials},
		{name: "taken with wrong password", newEmail: "john@example.com", password: "wrongpass", wantErr: user.ErrInvalidCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, users := newJane(t)
			newUser(t, users, "john@example.com", "John", "Roe")

			err := usecase.NewRequestEmailChange(users, fakeHasher{}).Execute(t.Context(), usecase.EmailChangeRequest{
				UserID:   u.ID(),
				NewEmail: tt.newEmail,
				Password: tt.password,
			})

			if tt.wantErr != nil {
				checkRequestError(t, err, tt.wantErr, tt.wantField)
				return
			}
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			checkEmailChangeRequested(t, u, tt.newEmail)
		})
	}
}

func checkRequestError(t *testing.T, err, want error, wantField usecase.Field) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
	if wantField == 0 {
		return
	}
	if fe, ok := errors.AsType[*usecase.FieldError](err); !ok || fe.Field != wantField {
		t.Errorf("Execute() error = %v, want FieldError for %v", err, wantField)
	}
}

func checkEmailChangeRequested(t *testing.T, u *user.User, wantEmail string) {
	t.Helper()
	p, ok := u.PendingEmailChange()
	if !ok {
		t.Fatal("Execute() left no pending email change, want one")
	}
	if got := p.Email().String(); got != wantEmail {
		t.Errorf("Execute() PendingEmailChange().Email() = %q, want %q", got, wantEmail)
	}
	if n := len(publishedOf[user.EmailChangeRequested](u.Events())); n != 1 {
		t.Errorf("Execute() recorded %d EmailChangeRequested events, want 1", n)
	}
}

type hashFreeUpdates struct {
	*fakeUsers
	t      *testing.T
	hasher *countingHasher
}

func (hf hashFreeUpdates) UpdateByID(ctx context.Context, id uuid.UUID, update user.Update) (*user.User, error) {
	hf.t.Helper()
	before := hf.hasher.verifies
	u, err := hf.fakeUsers.UpdateByID(ctx, id, update)
	if hf.hasher.verifies != before {
		hf.t.Errorf("hasher ran %d times during UpdateByID, want 0", hf.hasher.verifies-before)
	}
	return u, err
}

func TestRequestEmailChangeVerifiesPasswordOutsideUpdate(t *testing.T) {
	for _, newEmail := range []string{"new@example.com", "john@example.com"} {
		t.Run(newEmail, func(t *testing.T) {
			u, users := newJane(t)
			newUser(t, users, "john@example.com", "John", "Roe")
			hasher := &countingHasher{}

			uc := usecase.NewRequestEmailChange(hashFreeUpdates{fakeUsers: users, t: t, hasher: hasher}, hasher)
			err := uc.Execute(t.Context(), usecase.EmailChangeRequest{
				UserID:   u.ID(),
				NewEmail: newEmail,
				Password: "s3cretpass",
			})
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if hasher.verifies != 1 {
				t.Errorf("hasher verified %d times, want 1", hasher.verifies)
			}
		})
	}
}

func TestRequestEmailChangeUnknownUser(t *testing.T) {
	_, users := newJane(t)

	err := usecase.NewRequestEmailChange(users, fakeHasher{}).Execute(t.Context(), usecase.EmailChangeRequest{
		UserID:   uuid.New(),
		NewEmail: "new@example.com",
		Password: "s3cretpass",
	})
	if !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Execute() error = %v, want %v", err, user.ErrNotFound)
	}
}

func TestRequestEmailChangeToTakenAddress(t *testing.T) {
	u, users := newJane(t)
	john := newUser(t, users, "john@example.com", "John", "Roe")

	err := usecase.NewRequestEmailChange(users, fakeHasher{}).Execute(t.Context(), usecase.EmailChangeRequest{
		UserID:   u.ID(),
		NewEmail: "john@example.com",
		Password: "s3cretpass",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil so a taken email is not revealed", err)
	}
	if _, ok := u.PendingEmailChange(); ok {
		t.Error("PendingEmailChange() exists, want none for a taken email")
	}
	if n := len(publishedOf[user.EmailChangeRequested](u.Events())); n != 0 {
		t.Errorf("Execute() recorded %d EmailChangeRequested events for a taken email, want 0", n)
	}
	attempts := publishedOf[user.EmailChangeToTakenAddressAttempted](u.Events())
	want := []user.EmailChangeToTakenAddressAttempted{{AggregateID: u.ID(), NewEmail: john.Email()}}
	if diff := cmp.Diff(want, attempts, valueObjects, ignoreEventMeta); diff != "" {
		t.Errorf("Execute() EmailChangeToTakenAddressAttempted events mismatch (-want +got):\n%s", diff)
	}
}

package usecase_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

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
			usr, users := newJane(t)
			newUser(t, users, "john@example.com", "John", "Roe")

			err := usecase.NewRequestEmailChange(users, fakeHasher{}).Execute(t.Context(), usecase.EmailChangeRequest{
				UserID:   usr.ID(),
				NewEmail: tt.newEmail,
				Password: tt.password,
			})

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Execute() error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantField != 0 {
					if fe, ok := errors.AsType[*usecase.FieldError](err); !ok || fe.Field != tt.wantField {
						t.Errorf("Execute() error = %v, want FieldError for %v", err, tt.wantField)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			p, ok := usr.PendingEmailChange()
			if !ok || p.Email().String() != "new@example.com" {
				t.Errorf("PendingEmailChange() = %+v, %v; want new@example.com", p, ok)
			}
			var requested int
			for _, e := range usr.Events() {
				if _, ok := e.(user.EmailChangeRequested); ok {
					requested++
				}
			}
			if requested != 1 {
				t.Errorf("recorded %d EmailChangeRequested events, want 1", requested)
			}
		})
	}
}

type hashFreeUpdates struct {
	*fakeUsers
	t      *testing.T
	hasher *countingHasher
}

func (u hashFreeUpdates) UpdateByID(ctx context.Context, id uuid.UUID, updateFn func(*user.User) error) (*user.User, error) {
	u.t.Helper()
	before := u.hasher.verifies
	usr, err := u.fakeUsers.UpdateByID(ctx, id, updateFn)
	if u.hasher.verifies != before {
		u.t.Errorf("hasher ran %d times during UpdateByID, want 0", u.hasher.verifies-before)
	}
	return usr, err
}

func TestRequestEmailChangeVerifiesPasswordOutsideUpdate(t *testing.T) {
	for _, newEmail := range []string{"new@example.com", "john@example.com"} {
		t.Run(newEmail, func(t *testing.T) {
			usr, users := newJane(t)
			newUser(t, users, "john@example.com", "John", "Roe")
			hasher := &countingHasher{}

			err := usecase.NewRequestEmailChange(hashFreeUpdates{fakeUsers: users, t: t, hasher: hasher}, hasher).Execute(t.Context(), usecase.EmailChangeRequest{
				UserID:   usr.ID(),
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
	usr, users := newJane(t)
	john := newUser(t, users, "john@example.com", "John", "Roe")

	err := usecase.NewRequestEmailChange(users, fakeHasher{}).Execute(t.Context(), usecase.EmailChangeRequest{
		UserID:   usr.ID(),
		NewEmail: "john@example.com",
		Password: "s3cretpass",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil so a taken email is not revealed", err)
	}
	if _, ok := usr.PendingEmailChange(); ok {
		t.Error("PendingEmailChange() exists, want none for a taken email")
	}
	var attempts []user.EmailChangeToTakenAddressAttempted
	for _, e := range usr.Events() {
		if a, ok := e.(user.EmailChangeToTakenAddressAttempted); ok {
			attempts = append(attempts, a)
		}
		if _, ok := e.(user.EmailChangeRequested); ok {
			t.Error("recorded EmailChangeRequested for a taken email")
		}
	}
	if len(attempts) != 1 || attempts[0].NewEmail != john.Email() {
		t.Errorf("EmailChangeToTakenAddressAttempted events = %+v, want one for %q", attempts, john.Email())
	}
}

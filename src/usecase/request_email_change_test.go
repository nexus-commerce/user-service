package usecase_test

import (
	"errors"
	"testing"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
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
		{name: "taken", newEmail: "john@example.com", password: "s3cretpass", wantErr: user.ErrEmailTaken},
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

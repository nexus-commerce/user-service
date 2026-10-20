package usecase_test

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type fakeTokens struct{}

func (fakeTokens) Issue(userID uuid.UUID) (string, error) { return "token:" + userID.String(), nil }

type addedUsers struct {
	added []*user.User
}

func (a *addedUsers) Add(_ context.Context, usr *user.User) error {
	a.added = append(a.added, usr)
	return nil
}

func TestRegister(t *testing.T) {
	valid := usecase.Registration{
		FirstName:            "Jane",
		LastName:             "Doe",
		Email:                "Jane@Example.com",
		Password:             "s3cretpass",
		PasswordConfirmation: "s3cretpass",
	}
	tests := []struct {
		name      string
		modify    func(*usecase.Registration)
		wantErr   error
		wantField usecase.Field
	}{
		{name: "success", modify: func(*usecase.Registration) {}},
		{name: "password mismatch", modify: func(r *usecase.Registration) { r.PasswordConfirmation = "otherpass" }, wantErr: usecase.ErrPasswordMismatch, wantField: usecase.FieldPasswordConfirmation},
		{name: "empty first name", modify: func(r *usecase.Registration) { r.FirstName = "" }, wantErr: user.ErrEmptyName, wantField: usecase.FieldFirstName},
		{name: "blank last name", modify: func(r *usecase.Registration) { r.LastName = " " }, wantErr: user.ErrEmptyName, wantField: usecase.FieldLastName},
		{name: "malformed email", modify: func(r *usecase.Registration) { r.Email = "x" }, wantErr: user.ErrMalformedEmail, wantField: usecase.FieldEmail},
		{name: "short password", modify: func(r *usecase.Registration) { r.Password, r.PasswordConfirmation = "short", "short" }, wantErr: user.ErrPasswordTooShort, wantField: usecase.FieldPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := valid
			tt.modify(&reg)
			users := &addedUsers{}

			token, err := usecase.NewRegister(users, fakeTokens{}, fakeHasher{}).Execute(t.Context(), reg)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, user.ErrValidation) {
					t.Fatalf("Execute() error = %v, want %v", err, tt.wantErr)
				}
				if fe, ok := errors.AsType[*usecase.FieldError](err); !ok || fe.Field != tt.wantField {
					t.Errorf("Execute() error = %v, want FieldError for %v", err, tt.wantField)
				}
				if len(users.added) != 0 {
					t.Errorf("Add called %d times, want 0", len(users.added))
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if len(users.added) != 1 {
				t.Fatalf("Add called %d times, want 1", len(users.added))
			}
			usr := users.added[0]
			if usr.Email().String() != "jane@example.com" || usr.FullName().FirstName().String() != "Jane" || usr.FullName().LastName().String() != "Doe" {
				t.Errorf("added user = %s %s <%s>, want Jane Doe <jane@example.com>", usr.FullName().FirstName(), usr.FullName().LastName(), usr.Email())
			}
			if token != "token:"+usr.ID().String() {
				t.Errorf("Execute() token = %q, want token for %v", token, usr.ID())
			}
		})
	}
}

package usecase_test

import (
	"errors"
	"testing"
	"uuid"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type fakeTokens struct{}

func (fakeTokens) Issue(userID uuid.UUID) (string, error) { return "token:" + userID.String(), nil }

func validRegistration(email string) usecase.Registration {
	return usecase.Registration{
		FirstName:            "Jane",
		LastName:             "Doe",
		Email:                email,
		Password:             "s3cretpass",
		PasswordConfirmation: "s3cretpass",
	}
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*usecase.Registration)
		wantErr   error
		wantField usecase.Field
	}{
		{name: "password mismatch", modify: func(r *usecase.Registration) { r.PasswordConfirmation = "otherpass" }, wantErr: usecase.ErrPasswordMismatch, wantField: usecase.FieldPasswordConfirmation},
		{name: "empty first name", modify: func(r *usecase.Registration) { r.FirstName = "" }, wantErr: user.ErrEmptyName, wantField: usecase.FieldFirstName},
		{name: "blank last name", modify: func(r *usecase.Registration) { r.LastName = " " }, wantErr: user.ErrEmptyName, wantField: usecase.FieldLastName},
		{name: "malformed email", modify: func(r *usecase.Registration) { r.Email = "x" }, wantErr: user.ErrMalformedEmail, wantField: usecase.FieldEmail},
		{name: "short password", modify: func(r *usecase.Registration) { r.Password, r.PasswordConfirmation = "short", "short" }, wantErr: user.ErrPasswordTooShort, wantField: usecase.FieldPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := validRegistration("Jane@Example.com")
			tt.modify(&reg)
			users := &fakeUsers{}
			regs := &fakeRegistrations{users: users}

			err := usecase.NewRegister(users, regs, fakeHasher{}).Execute(t.Context(), reg)

			if !errors.Is(err, tt.wantErr) || !errors.Is(err, user.ErrValidation) {
				t.Fatalf("Execute() error = %v, want %v", err, tt.wantErr)
			}
			if fe, ok := errors.AsType[*usecase.FieldError](err); !ok || fe.Field != tt.wantField {
				t.Errorf("Execute() error = %v, want FieldError for %v", err, tt.wantField)
			}
			if len(regs.byEmail) != 0 {
				t.Errorf("Save stored %d pending registrations, want 0", len(regs.byEmail))
			}
		})
	}
}

func TestRegisterNewEmailSavesPending(t *testing.T) {
	users := &fakeUsers{}
	regs := &fakeRegistrations{users: users}

	if err := usecase.NewRegister(users, regs, fakeHasher{}).Execute(t.Context(), validRegistration("Jane@Example.com")); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	email, err := user.NewEmail("jane@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error: %v", err)
	}
	p, ok := regs.byEmail[email]
	if !ok {
		t.Fatal("no pending registration for jane@example.com")
	}
	if p.FullName().FirstName().String() != "Jane" || p.FullName().LastName().String() != "Doe" {
		t.Errorf("pending name = %s %s, want Jane Doe", p.FullName().FirstName(), p.FullName().LastName())
	}
	if len(users.byID) != 0 {
		t.Errorf("users has %d entries, want 0", len(users.byID))
	}
}

func TestRegisterTakenEmailRecordsDuplicate(t *testing.T) {
	jane, users := newJane(t)
	regs := &fakeRegistrations{users: users}

	if err := usecase.NewRegister(users, regs, fakeHasher{}).Execute(t.Context(), validRegistration("Jane@Example.com")); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if len(regs.byEmail) != 0 {
		t.Errorf("saved %d pending registrations, want 0", len(regs.byEmail))
	}
	events := jane.Events()
	last := events[len(events)-1]
	if _, ok := last.(user.DuplicateRegistrationAttempted); !ok {
		t.Errorf("last event = %T, want user.DuplicateRegistrationAttempted", last)
	}
}

func TestRegisterHashesOnceInBothBranches(t *testing.T) {
	for _, email := range []string{"new@example.com", "jane@example.com"} {
		_, users := newJane(t)
		regs := &fakeRegistrations{users: users}
		hasher := &countingHasher{}

		if err := usecase.NewRegister(users, regs, hasher).Execute(t.Context(), validRegistration(email)); err != nil {
			t.Fatalf("Execute(%q) error: %v", email, err)
		}
		if hasher.hashes != 1 {
			t.Errorf("Execute(%q) hashed %d times, want 1", email, hasher.hashes)
		}
	}
}

func TestRegisterDuplicateUserVanished(t *testing.T) {
	_, users := newJane(t)
	users.vanishOnRecord = true
	regs := &fakeRegistrations{users: users}

	if err := usecase.NewRegister(users, regs, fakeHasher{}).Execute(t.Context(), validRegistration("jane@example.com")); err != nil {
		t.Errorf("Execute() error: %v", err)
	}
	if len(regs.byEmail) != 0 {
		t.Errorf("saved %d pending registrations, want 0", len(regs.byEmail))
	}
}

func TestRegisterAgainReplacesPending(t *testing.T) {
	users := &fakeUsers{}
	regs := &fakeRegistrations{users: users}
	register := usecase.NewRegister(users, regs, fakeHasher{})

	if err := register.Execute(t.Context(), validRegistration("new@example.com")); err != nil {
		t.Fatalf("first Execute() error: %v", err)
	}
	first := tokenFor(t, regs, "new@example.com")
	if err := register.Execute(t.Context(), validRegistration("new@example.com")); err != nil {
		t.Fatalf("second Execute() error: %v", err)
	}
	second := tokenFor(t, regs, "new@example.com")
	if len(regs.byEmail) != 1 {
		t.Fatalf("saved %d pending registrations, want 1", len(regs.byEmail))
	}

	confirm := usecase.NewConfirmRegistration(regs, fakeTokens{})
	if _, err := confirm.Execute(t.Context(), first.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("Execute(first) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
	if _, err := confirm.Execute(t.Context(), second.Plaintext()); err != nil {
		t.Errorf("Execute(second) error: %v", err)
	}
}

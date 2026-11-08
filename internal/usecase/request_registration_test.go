package usecase_test

import (
	"errors"
	"testing"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

func validRegistrationRequest(email string) usecase.RegistrationRequest {
	return usecase.RegistrationRequest{
		FirstName:            "Jane",
		LastName:             "Doe",
		Email:                email,
		Password:             "s3cretpass",
		PasswordConfirmation: "s3cretpass",
	}
}

func TestRequestRegistration(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*usecase.RegistrationRequest)
		wantErr   error
		wantField usecase.Field
	}{
		{name: "password mismatch", modify: func(r *usecase.RegistrationRequest) { r.PasswordConfirmation = "otherpass" }, wantErr: usecase.ErrPasswordMismatch, wantField: usecase.FieldPasswordConfirmation},
		{name: "empty first name", modify: func(r *usecase.RegistrationRequest) { r.FirstName = "" }, wantErr: user.ErrEmptyName, wantField: usecase.FieldFirstName},
		{name: "blank last name", modify: func(r *usecase.RegistrationRequest) { r.LastName = " " }, wantErr: user.ErrEmptyName, wantField: usecase.FieldLastName},
		{name: "malformed email", modify: func(r *usecase.RegistrationRequest) { r.Email = "x" }, wantErr: user.ErrMalformedEmail, wantField: usecase.FieldEmail},
		{name: "short password", modify: func(r *usecase.RegistrationRequest) { r.Password, r.PasswordConfirmation = "short", "short" }, wantErr: user.ErrPasswordTooShort, wantField: usecase.FieldPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := validRegistrationRequest("Jane@Example.com")
			tt.modify(&reg)
			users := &fakeUsers{}
			regs := &fakeRegistrations{}

			err := usecase.NewRequestRegistration(users, regs, fakeHasher{}).Execute(t.Context(), reg)

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

func TestRequestRegistrationNewEmailSavesPending(t *testing.T) {
	users := &fakeUsers{}
	regs := &fakeRegistrations{}

	if err := usecase.NewRequestRegistration(users, regs, fakeHasher{}).Execute(t.Context(), validRegistrationRequest("Jane@Example.com")); err != nil {
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

func TestRequestRegistrationTakenEmailRecordsDuplicate(t *testing.T) {
	jane, users := newJane(t)
	regs := &fakeRegistrations{}

	if err := usecase.NewRequestRegistration(users, regs, fakeHasher{}).Execute(t.Context(), validRegistrationRequest("Jane@Example.com")); err != nil {
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

func TestRequestRegistrationHashesOnceInBothBranches(t *testing.T) {
	for _, email := range []string{"new@example.com", "jane@example.com"} {
		_, users := newJane(t)
		regs := &fakeRegistrations{}
		hasher := &countingHasher{}

		if err := usecase.NewRequestRegistration(users, regs, hasher).Execute(t.Context(), validRegistrationRequest(email)); err != nil {
			t.Fatalf("Execute(%q) error: %v", email, err)
		}
		if hasher.hashes != 1 {
			t.Errorf("Execute(%q) hashed %d times, want 1", email, hasher.hashes)
		}
	}
}

func TestRequestRegistrationDuplicateUserVanished(t *testing.T) {
	_, users := newJane(t)
	users.vanishOnRecord = true
	regs := &fakeRegistrations{}

	if err := usecase.NewRequestRegistration(users, regs, fakeHasher{}).Execute(t.Context(), validRegistrationRequest("jane@example.com")); err != nil {
		t.Errorf("Execute() error: %v", err)
	}
	if len(regs.byEmail) != 0 {
		t.Errorf("saved %d pending registrations, want 0", len(regs.byEmail))
	}
}

func TestRequestRegistrationAgainReplacesPending(t *testing.T) {
	users := &fakeUsers{}
	regs := &fakeRegistrations{}
	requestRegistration := usecase.NewRequestRegistration(users, regs, fakeHasher{})

	if err := requestRegistration.Execute(t.Context(), validRegistrationRequest("new@example.com")); err != nil {
		t.Fatalf("first Execute() error: %v", err)
	}
	first := tokenFor(t, regs, "new@example.com")
	if err := requestRegistration.Execute(t.Context(), validRegistrationRequest("new@example.com")); err != nil {
		t.Fatalf("second Execute() error: %v", err)
	}
	second := tokenFor(t, regs, "new@example.com")
	if len(regs.byEmail) != 1 {
		t.Fatalf("saved %d pending registrations, want 1", len(regs.byEmail))
	}

	confirm := usecase.NewConfirmRegistration(regs)
	if err := confirm.Execute(t.Context(), first.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("Execute(first) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
	if err := confirm.Execute(t.Context(), second.Plaintext()); err != nil {
		t.Errorf("Execute(second) error: %v", err)
	}
}

func TestRequestRegistrationKeepsConfirmedRegistration(t *testing.T) {
	users, regs, id := confirmedJane(t)
	published := len(regs.published)

	if err := usecase.NewRequestRegistration(users, regs, fakeHasher{}).Execute(t.Context(), validRegistrationRequest("jane@example.com")); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	p := pendingJane(t, regs)
	if p.UserID() != id || !p.Confirmed() {
		t.Errorf("registration = {id: %v, confirmed: %v}, want {id: %v, confirmed: true}", p.UserID(), p.Confirmed(), id)
	}
	if len(regs.published) != published {
		t.Errorf("published %d new events, want 0", len(regs.published)-published)
	}
}

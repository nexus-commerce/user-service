package entity_test

import (
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

func TestNewProfile(t *testing.T) {
	tests := []struct {
		name      string
		fields    entity.ProfileFields
		wantErr   error
		wantField string
	}{
		{name: "valid", fields: entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}},
		{name: "bad first name", fields: entity.ProfileFields{Email: "jane@example.com", FirstName: "", LastName: "Doe"}, wantErr: entity.ErrEmptyName, wantField: "firstName"},
		{name: "bad last name", fields: entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: " "}, wantErr: entity.ErrEmptyName, wantField: "lastName"},
		{name: "bad email", fields: entity.ProfileFields{Email: "x", FirstName: "Jane", LastName: "Doe"}, wantErr: entity.ErrMalformedEmail, wantField: "email"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := entity.NewProfile(tt.fields)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, entity.ErrValidation) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				if fe, ok := errors.AsType[*entity.FieldError](err); !ok || fe.Field != tt.wantField {
					t.Errorf("error = %v, want field %q", err, tt.wantField)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Email().String() != tt.fields.Email || p.FirstName().String() != tt.fields.FirstName || p.LastName().String() != tt.fields.LastName {
				t.Errorf("profile = %s %s %s", p.Email(), p.FirstName(), p.LastName())
			}
		})
	}
}

func mustProfile(t *testing.T, f entity.ProfileFields) entity.Profile {
	t.Helper()
	p, err := entity.NewProfile(f)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewUserRecordsRegistration(t *testing.T) {
	profile := mustProfile(t, entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"})
	password, _ := entity.NewPassword("s3cret")

	u, err := entity.NewUser(profile, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Authenticate(password); err != nil {
		t.Errorf("Authenticate = %v, want nil", err)
	}

	events := u.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	reg, ok := events[0].(entity.UserRegistered)
	if !ok {
		t.Fatalf("event %T, want UserRegistered", events[0])
	}
	if reg.ID == uuid.Nil() || reg.AggregateID != u.ID() || reg.Profile != profile || !reg.OccurredAt.Equal(u.CreatedAt()) {
		t.Errorf("event = %+v", reg)
	}

	again := u.Events()
	if len(again) != 1 || again[0].(entity.UserRegistered).ID != reg.ID {
		t.Error("Events must return the same recorded events with stable IDs")
	}
}

func TestEventsReturnsCopy(t *testing.T) {
	password, _ := entity.NewPassword("s3cret")
	u, err := entity.NewUser(mustProfile(t, entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}), password)
	if err != nil {
		t.Fatal(err)
	}

	events := u.Events()
	events[0] = nil

	if u.Events()[0] == nil {
		t.Error("modifying the returned slice changed the aggregate's events")
	}
}

func TestChangeProfile(t *testing.T) {
	password, _ := entity.NewPassword("s3cret")
	u, err := entity.NewUser(mustProfile(t, entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}), password)
	if err != nil {
		t.Fatal(err)
	}

	changed := mustProfile(t, entity.ProfileFields{Email: "jane.smith@example.com", FirstName: "Jane", LastName: "Smith"})
	if !u.ChangeProfile(changed) {
		t.Error("ChangeProfile(new profile) = false, want true")
	}
	if !hasProfile(u, changed) {
		t.Errorf("profile = %s %s %s, want %+v", u.Email(), u.FirstName(), u.LastName(), changed)
	}

	same := mustProfile(t, entity.ProfileFields{Email: "jane.smith@example.com", FirstName: "Jane", LastName: "Smith"})
	if u.ChangeProfile(same) {
		t.Error("ChangeProfile(same profile) = true, want false")
	}
}

func hasProfile(u *entity.User, p entity.Profile) bool {
	return u.Email() == p.Email() && u.FirstName() == p.FirstName() && u.LastName() == p.LastName()
}

func TestRestoreProfile(t *testing.T) {
	longName := strings.Repeat("a", 51)
	fields := entity.ProfileFields{Email: "Legacy@Example.com", FirstName: longName, LastName: "Doe"}
	p := entity.RestoreProfile(fields)

	if _, err := entity.NewProfile(fields); err == nil {
		t.Fatal("NewProfile accepted data that should fail current rules")
	}
	if p.Email().String() != "Legacy@Example.com" || p.FirstName().String() != longName || p.LastName().String() != "Doe" {
		t.Errorf("restored profile = %s %s %s", p.Email(), p.FirstName(), p.LastName())
	}
}

func TestRestoreUser(t *testing.T) {
	password, _ := entity.NewPassword("s3cret")
	hash, err := password.Hash()
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	profile := mustProfile(t, entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"})
	createdAt := time.Date(2026, 10, 14, 19, 30, 0, 0, time.UTC)

	t.Run("valid", func(t *testing.T) {
		u, err := entity.RestoreUser(id, profile, hash, createdAt)
		if err != nil {
			t.Fatal(err)
		}
		if u.ID() != id || !hasProfile(u, profile) || u.PasswordHash() != hash || !u.CreatedAt().Equal(createdAt) {
			t.Errorf("restored user = %v %s %s %s %s %v", u.ID(), u.Email(), u.FirstName(), u.LastName(), u.PasswordHash(), u.CreatedAt())
		}
		if err := u.Authenticate(password); err != nil {
			t.Errorf("Authenticate = %v, want nil", err)
		}
		if events := u.Events(); len(events) != 0 {
			t.Errorf("got %d events, want none", len(events))
		}
	})

	t.Run("malformed hash", func(t *testing.T) {
		_, err := entity.RestoreUser(id, profile, "not-a-bcrypt-hash", createdAt)
		if !errors.Is(err, entity.ErrMalformedPasswordHash) {
			t.Fatalf("error = %v, want %v", err, entity.ErrMalformedPasswordHash)
		}
		if errors.Is(err, entity.ErrValidation) {
			t.Error("malformed hash must not be a validation error")
		}
	})
}

func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	password, _ := entity.NewPassword("s3cret")
	other, _ := entity.NewPassword("other")
	u, err := entity.NewUser(mustProfile(t, entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}), password)
	if err != nil {
		t.Fatal(err)
	}

	if err := u.Authenticate(other); !errors.Is(err, entity.ErrInvalidCredentials) {
		t.Errorf("Authenticate(wrong) = %v, want %v", err, entity.ErrInvalidCredentials)
	}
}

package entity_test

import (
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

var janeDoe = entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}

func TestNewProfile(t *testing.T) {
	tests := []struct {
		name      string
		fields    entity.ProfileFields
		wantErr   error
		wantField string
	}{
		{
			name:   "valid",
			fields: janeDoe,
		},
		{
			name:      "bad first name",
			fields:    entity.ProfileFields{Email: "jane@example.com", FirstName: "", LastName: "Doe"},
			wantErr:   entity.ErrEmptyName,
			wantField: "firstName",
		},
		{
			name:      "bad last name",
			fields:    entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: " "},
			wantErr:   entity.ErrEmptyName,
			wantField: "lastName",
		},
		{
			name:      "bad email",
			fields:    entity.ProfileFields{Email: "x", FirstName: "Jane", LastName: "Doe"},
			wantErr:   entity.ErrMalformedEmail,
			wantField: "email",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := entity.NewProfile(tt.fields)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, entity.ErrValidation) {
					t.Fatalf("NewProfile(%+v) error = %v, want %v", tt.fields, err, tt.wantErr)
				}
				if fe, ok := errors.AsType[*entity.FieldError](err); !ok || fe.Field != tt.wantField {
					t.Errorf("NewProfile(%+v) error = %v, want FieldError for %q", tt.fields, err, tt.wantField)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProfile(%+v) unexpected error: %v", tt.fields, err)
			}
			if got := fieldsOf(p); got != tt.fields {
				t.Errorf("NewProfile(%+v) = %+v, want %+v", tt.fields, got, tt.fields)
			}
		})
	}
}

func mustProfile(t *testing.T, f entity.ProfileFields) entity.Profile {
	t.Helper()
	p, err := entity.NewProfile(f)
	if err != nil {
		t.Fatalf("NewProfile(%+v) error: %v", f, err)
	}
	return p
}

func mustPassword(t *testing.T, s string) entity.Password {
	t.Helper()
	p, err := entity.NewPassword(s)
	if err != nil {
		t.Fatalf("NewPassword(%q) error: %v", s, err)
	}
	return p
}

type fakeHasher struct{}

func (fakeHasher) Hash(p entity.Password) (entity.PasswordHash, error) {
	return entity.NewPasswordHash("hashed:" + p.Plaintext())
}

func (fakeHasher) Verify(h entity.PasswordHash, p entity.Password) (bool, error) {
	return h.Encoded() == "hashed:"+p.Plaintext(), nil
}

type profileView interface {
	Email() entity.Email
	FirstName() entity.Name
	LastName() entity.Name
}

func fieldsOf(p profileView) entity.ProfileFields {
	return entity.ProfileFields{
		Email:     p.Email().String(),
		FirstName: p.FirstName().String(),
		LastName:  p.LastName().String(),
	}
}

func TestNewUserRecordsRegistration(t *testing.T) {
	profile := mustProfile(t, janeDoe)
	password := mustPassword(t, "s3cret")

	u, err := entity.NewUser(profile, password, fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}
	if err := u.Authenticate(password, fakeHasher{}); err != nil {
		t.Errorf("Authenticate = %v, want nil", err)
	}

	events := u.Events()
	if len(events) != 1 {
		t.Fatalf("Events() returned %d events, want 1", len(events))
	}
	reg, ok := events[0].(entity.UserRegistered)
	if !ok {
		t.Fatalf("Events()[0] is %T, want UserRegistered", events[0])
	}
	if reg.ID == uuid.Nil() {
		t.Errorf("Events()[0].ID = nil, want non-nil")
	}
	want := entity.UserRegistered{
		ID:          reg.ID,
		AggregateID: u.ID(),
		OccurredAt:  u.CreatedAt(),
		Profile:     profile,
	}
	if reg != want {
		t.Errorf("Events()[0] = %+v, want %+v", reg, want)
	}

	again := u.Events()
	if len(again) != 1 || again[0].(entity.UserRegistered).ID != reg.ID {
		t.Error("Events must return the same recorded events with stable IDs")
	}
}

func TestEventsReturnsCopy(t *testing.T) {
	password := mustPassword(t, "s3cret")
	u, err := entity.NewUser(mustProfile(t, janeDoe), password, fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}

	events := u.Events()
	events[0] = nil

	if u.Events()[0] == nil {
		t.Error("modifying the returned slice changed the aggregate's events")
	}
}

func mustUser(t *testing.T) *entity.User {
	t.Helper()
	u, err := entity.NewUser(mustProfile(t, janeDoe), mustPassword(t, "s3cret"), fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}
	return u
}

func mustHash(t *testing.T) entity.PasswordHash {
	t.Helper()
	h, err := fakeHasher{}.Hash(mustPassword(t, "s3cret"))
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	return h
}

func TestProfileApply(t *testing.T) {
	base := mustProfile(t, janeDoe)
	smith := "Smith"
	empty := ""
	bad := "x"

	tests := []struct {
		name      string
		change    entity.ProfileChange
		want      entity.ProfileFields
		wantErr   error
		wantField string
	}{
		{
			name:   "no fields",
			change: entity.ProfileChange{},
			want:   janeDoe,
		},
		{
			name:   "only last name",
			change: entity.ProfileChange{LastName: &smith},
			want:   entity.ProfileFields{Email: janeDoe.Email, FirstName: janeDoe.FirstName, LastName: "Smith"},
		},
		{
			name:      "invalid first name",
			change:    entity.ProfileChange{FirstName: &empty},
			wantErr:   entity.ErrEmptyName,
			wantField: "firstName",
		},
		{
			name:      "invalid email",
			change:    entity.ProfileChange{Email: &bad},
			wantErr:   entity.ErrMalformedEmail,
			wantField: "email",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := base.Apply(tt.change)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, entity.ErrValidation) {
					t.Fatalf("Apply() error = %v, want %v", err, tt.wantErr)
				}
				if fe, ok := errors.AsType[*entity.FieldError](err); !ok || fe.Field != tt.wantField {
					t.Errorf("Apply() error = %v, want FieldError for %q", err, tt.wantField)
				}
				return
			}
			if err != nil {
				t.Fatalf("Apply() unexpected error: %v", err)
			}
			if got := fieldsOf(got); got != tt.want {
				t.Errorf("Apply() = %+v, want %+v", got, tt.want)
			}
			if fieldsOf(base) != janeDoe {
				t.Error("Apply() mutated the receiver")
			}
		})
	}
}

func TestChangeProfile(t *testing.T) {
	smith := "Smith"
	empty := ""

	t.Run("changed", func(t *testing.T) {
		u := mustUser(t)
		changed, err := u.ChangeProfile(entity.ProfileChange{LastName: &smith})
		if err != nil || !changed {
			t.Fatalf("ChangeProfile() = %v, %v; want true, nil", changed, err)
		}
		if got := u.LastName().String(); got != "Smith" {
			t.Errorf("LastName() = %q, want %q", got, "Smith")
		}
	})

	t.Run("same values", func(t *testing.T) {
		u := mustUser(t)
		first := janeDoe.FirstName
		changed, err := u.ChangeProfile(entity.ProfileChange{FirstName: &first})
		if err != nil || changed {
			t.Errorf("ChangeProfile(same) = %v, %v; want false, nil", changed, err)
		}
	})

	t.Run("invalid leaves profile intact", func(t *testing.T) {
		u := mustUser(t)
		_, err := u.ChangeProfile(entity.ProfileChange{LastName: &smith, FirstName: &empty})
		if !errors.Is(err, entity.ErrEmptyName) {
			t.Fatalf("ChangeProfile() error = %v, want %v", err, entity.ErrEmptyName)
		}
		if got := fieldsOf(u); got != janeDoe {
			t.Errorf("profile after failed change = %+v, want %+v", got, janeDoe)
		}
	})

	t.Run("legacy fields are not revalidated", func(t *testing.T) {
		legacy := entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: strings.Repeat("a", 51)}
		u := entity.RestoreUser(uuid.New(), entity.RestoreProfile(legacy), mustHash(t), time.Now())
		first := "Janet"
		changed, err := u.ChangeProfile(entity.ProfileChange{FirstName: &first})
		if err != nil || !changed {
			t.Fatalf("ChangeProfile() = %v, %v; want true, nil", changed, err)
		}
		if got := u.LastName().String(); got != legacy.LastName {
			t.Errorf("LastName() = %q, want legacy value kept", got)
		}
	})
}

func TestRestoreProfile(t *testing.T) {
	longName := strings.Repeat("a", 51)
	fields := entity.ProfileFields{Email: "Legacy@Example.com", FirstName: longName, LastName: "Doe"}
	p := entity.RestoreProfile(fields)

	if _, err := entity.NewProfile(fields); err == nil {
		t.Fatal("NewProfile accepted data that should fail current rules")
	}
	if got := fieldsOf(p); got != fields {
		t.Errorf("RestoreProfile(%+v) = %+v, want %+v", fields, got, fields)
	}
}

func TestRestoreUser(t *testing.T) {
	password := mustPassword(t, "s3cret")
	hash, err := fakeHasher{}.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	id := uuid.New()
	fields := janeDoe
	profile := mustProfile(t, fields)
	createdAt := time.Date(2026, 10, 14, 19, 30, 0, 0, time.UTC)

	u := entity.RestoreUser(id, profile, hash, createdAt)
	got := fieldsOf(u)
	matches := u.ID() == id && got == fields && u.PasswordHash() == hash && u.CreatedAt().Equal(createdAt)
	if !matches {
		t.Errorf("RestoreUser() = {id: %v, profile: %+v, hash: %q, createdAt: %v}, want {id: %v, profile: %+v, hash: %q, createdAt: %v}",
			u.ID(), got, u.PasswordHash().Encoded(), u.CreatedAt(), id, fields, hash.Encoded(), createdAt)
	}
	if err := u.Authenticate(password, fakeHasher{}); err != nil {
		t.Errorf("Authenticate = %v, want nil", err)
	}
	if events := u.Events(); len(events) != 0 {
		t.Errorf("Events() after RestoreUser returned %d events, want 0", len(events))
	}
}

func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	password := mustPassword(t, "s3cret")
	other := mustPassword(t, "other")
	u, err := entity.NewUser(mustProfile(t, janeDoe), password, fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}

	if err := u.Authenticate(other, fakeHasher{}); !errors.Is(err, entity.ErrInvalidCredentials) {
		t.Errorf("Authenticate(wrong) = %v, want %v", err, entity.ErrInvalidCredentials)
	}
}

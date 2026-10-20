package user_test

import (
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

var janeDoe = user.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}

func TestNewProfile(t *testing.T) {
	tests := []struct {
		name      string
		fields    user.ProfileFields
		wantErr   error
		wantField string
	}{
		{
			name:   "valid",
			fields: janeDoe,
		},
		{
			name:      "bad first name",
			fields:    user.ProfileFields{Email: "jane@example.com", FirstName: "", LastName: "Doe"},
			wantErr:   user.ErrEmptyName,
			wantField: "firstName",
		},
		{
			name:      "bad last name",
			fields:    user.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: " "},
			wantErr:   user.ErrEmptyName,
			wantField: "lastName",
		},
		{
			name:      "bad email",
			fields:    user.ProfileFields{Email: "x", FirstName: "Jane", LastName: "Doe"},
			wantErr:   user.ErrMalformedEmail,
			wantField: "email",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := user.NewProfile(tt.fields)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, user.ErrValidation) {
					t.Fatalf("NewProfile(%+v) error = %v, want %v", tt.fields, err, tt.wantErr)
				}
				if fe, ok := errors.AsType[*user.FieldError](err); !ok || fe.Field != tt.wantField {
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

func mustProfile(t *testing.T, f user.ProfileFields) user.Profile {
	t.Helper()
	p, err := user.NewProfile(f)
	if err != nil {
		t.Fatalf("NewProfile(%+v) error: %v", f, err)
	}
	return p
}

func mustPassword(t *testing.T, s string) user.Password {
	t.Helper()
	p, err := user.NewPassword(s)
	if err != nil {
		t.Fatalf("NewPassword(%q) error: %v", s, err)
	}
	return p
}

type fakeHasher struct{}

func (fakeHasher) Hash(p user.Password) (user.PasswordHash, error) {
	return user.NewPasswordHash("hashed:" + p.Plaintext())
}

func (fakeHasher) Verify(h user.PasswordHash, p user.Password) (bool, error) {
	return h.Encoded() == "hashed:"+p.Plaintext(), nil
}

type profileView interface {
	Email() user.Email
	FirstName() user.Name
	LastName() user.Name
}

func fieldsOf(p profileView) user.ProfileFields {
	return user.ProfileFields{
		Email:     p.Email().String(),
		FirstName: p.FirstName().String(),
		LastName:  p.LastName().String(),
	}
}

func TestNewUserRecordsRegistration(t *testing.T) {
	profile := mustProfile(t, janeDoe)
	password := mustPassword(t, "s3cret")

	u, err := user.NewUser(profile, password, fakeHasher{})
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
	reg, ok := events[0].(user.UserRegistered)
	if !ok {
		t.Fatalf("Events()[0] is %T, want UserRegistered", events[0])
	}
	if reg.ID == uuid.Nil() {
		t.Errorf("Events()[0].ID = nil, want non-nil")
	}
	want := user.UserRegistered{
		ID:          reg.ID,
		AggregateID: u.ID(),
		OccurredAt:  u.CreatedAt(),
		Profile:     profile,
	}
	if reg != want {
		t.Errorf("Events()[0] = %+v, want %+v", reg, want)
	}

	again := u.Events()
	if len(again) != 1 || again[0].(user.UserRegistered).ID != reg.ID {
		t.Error("Events must return the same recorded events with stable IDs")
	}
}

func TestEventsReturnsCopy(t *testing.T) {
	password := mustPassword(t, "s3cret")
	u, err := user.NewUser(mustProfile(t, janeDoe), password, fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}

	events := u.Events()
	events[0] = nil

	if u.Events()[0] == nil {
		t.Error("modifying the returned slice changed the aggregate's events")
	}
}

func mustUser(t *testing.T) *user.User {
	t.Helper()
	u, err := user.NewUser(mustProfile(t, janeDoe), mustPassword(t, "s3cret"), fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}
	return u
}

func mustHash(t *testing.T) user.PasswordHash {
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
		change    user.ProfileChange
		want      user.ProfileFields
		wantErr   error
		wantField string
	}{
		{
			name:   "no fields",
			change: user.ProfileChange{},
			want:   janeDoe,
		},
		{
			name:   "only last name",
			change: user.ProfileChange{LastName: &smith},
			want:   user.ProfileFields{Email: janeDoe.Email, FirstName: janeDoe.FirstName, LastName: "Smith"},
		},
		{
			name:      "invalid first name",
			change:    user.ProfileChange{FirstName: &empty},
			wantErr:   user.ErrEmptyName,
			wantField: "firstName",
		},
		{
			name:      "invalid email",
			change:    user.ProfileChange{Email: &bad},
			wantErr:   user.ErrMalformedEmail,
			wantField: "email",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := base.Apply(tt.change)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || !errors.Is(err, user.ErrValidation) {
					t.Fatalf("Apply() error = %v, want %v", err, tt.wantErr)
				}
				if fe, ok := errors.AsType[*user.FieldError](err); !ok || fe.Field != tt.wantField {
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
		changed, err := u.ChangeProfile(user.ProfileChange{LastName: &smith})
		if err != nil || !changed {
			t.Fatalf("ChangeProfile() = %v, %v; want true, nil", changed, err)
		}
		if got := u.LastName().String(); got != "Smith" {
			t.Errorf("LastName() = %q, want %q", got, "Smith")
		}
		events := profileChangedEvents(u)
		if len(events) != 1 {
			t.Fatalf("recorded %d UserProfileChanged events, want 1", len(events))
		}
		if e := events[0]; e.AggregateID != u.ID() || e.ID == uuid.Nil() || fieldsOf(e.Profile) != fieldsOf(u) {
			t.Errorf("UserProfileChanged = %+v, want aggregate %v with profile %+v", e, u.ID(), fieldsOf(u))
		}
	})

	t.Run("same values", func(t *testing.T) {
		u := mustUser(t)
		first := janeDoe.FirstName
		changed, err := u.ChangeProfile(user.ProfileChange{FirstName: &first})
		if err != nil || changed {
			t.Errorf("ChangeProfile(same) = %v, %v; want false, nil", changed, err)
		}
		if n := len(profileChangedEvents(u)); n != 0 {
			t.Errorf("recorded %d UserProfileChanged events, want 0", n)
		}
	})

	t.Run("invalid leaves profile intact", func(t *testing.T) {
		u := mustUser(t)
		_, err := u.ChangeProfile(user.ProfileChange{LastName: &smith, FirstName: &empty})
		if !errors.Is(err, user.ErrEmptyName) {
			t.Fatalf("ChangeProfile() error = %v, want %v", err, user.ErrEmptyName)
		}
		if got := fieldsOf(u); got != janeDoe {
			t.Errorf("profile after failed change = %+v, want %+v", got, janeDoe)
		}
		if n := len(profileChangedEvents(u)); n != 0 {
			t.Errorf("recorded %d UserProfileChanged events, want 0", n)
		}
	})

	t.Run("legacy fields are not revalidated", func(t *testing.T) {
		legacy := user.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: strings.Repeat("a", 51)}
		u := user.RestoreUser(uuid.New(), user.RestoreProfile(legacy), mustHash(t), time.Now())
		first := "Janet"
		changed, err := u.ChangeProfile(user.ProfileChange{FirstName: &first})
		if err != nil || !changed {
			t.Fatalf("ChangeProfile() = %v, %v; want true, nil", changed, err)
		}
		if got := u.LastName().String(); got != legacy.LastName {
			t.Errorf("LastName() = %q, want legacy value kept", got)
		}
		events := profileChangedEvents(u)
		if len(events) != 1 || events[0].Profile.LastName().String() != legacy.LastName {
			t.Errorf("UserProfileChanged events = %+v, want one carrying the full profile", events)
		}
	})
}

func profileChangedEvents(u *user.User) []user.UserProfileChanged {
	var out []user.UserProfileChanged
	for _, e := range u.Events() {
		if pc, ok := e.(user.UserProfileChanged); ok {
			out = append(out, pc)
		}
	}
	return out
}

func TestRestoreProfile(t *testing.T) {
	longName := strings.Repeat("a", 51)
	fields := user.ProfileFields{Email: "Legacy@Example.com", FirstName: longName, LastName: "Doe"}
	p := user.RestoreProfile(fields)

	if _, err := user.NewProfile(fields); err == nil {
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

	u := user.RestoreUser(id, profile, hash, createdAt)
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
	u, err := user.NewUser(mustProfile(t, janeDoe), password, fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}

	if err := u.Authenticate(other, fakeHasher{}); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Errorf("Authenticate(wrong) = %v, want %v", err, user.ErrInvalidCredentials)
	}
}

package user_test

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

const janeEmail = "jane@example.com"

type nameFields struct {
	FirstName string
	LastName  string
}

var janeDoe = nameFields{FirstName: "Jane", LastName: "Doe"}

func mustFullName(t *testing.T, f nameFields) user.FullName {
	t.Helper()
	return user.NewFullName(mustName(t, f.FirstName), mustName(t, f.LastName))
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

func fieldsOf(n user.FullName) nameFields {
	return nameFields{
		FirstName: n.FirstName().String(),
		LastName:  n.LastName().String(),
	}
}

func TestNewUserRecordsRegistration(t *testing.T) {
	email := mustEmail(t, janeEmail)
	name := mustFullName(t, janeDoe)
	password := mustPassword(t, "s3cretpass")

	u, err := user.NewUser(email, name, password, fakeHasher{})
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
	reg, ok := events[0].(user.Registered)
	if !ok {
		t.Fatalf("Events()[0] is %T, want Registered", events[0])
	}
	if reg.ID == uuid.Nil() {
		t.Errorf("Events()[0].ID = nil, want non-nil")
	}
	want := user.Registered{
		ID:          reg.ID,
		AggregateID: u.ID(),
		OccurredAt:  u.CreatedAt(),
		Email:       email,
		FullName:    name,
	}
	if reg != want {
		t.Errorf("Events()[0] = %+v, want %+v", reg, want)
	}

	again := u.Events()
	if len(again) != 1 || again[0].(user.Registered).ID != reg.ID {
		t.Error("Events must return the same recorded events with stable IDs")
	}
}

func TestEventsReturnsCopy(t *testing.T) {
	password := mustPassword(t, "s3cretpass")
	u, err := user.NewUser(mustEmail(t, janeEmail), mustFullName(t, janeDoe), password, fakeHasher{})
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
	u, err := user.NewUser(mustEmail(t, janeEmail), mustFullName(t, janeDoe), mustPassword(t, "s3cretpass"), fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}
	return u
}

func mustHash(t *testing.T) user.PasswordHash {
	t.Helper()
	h, err := fakeHasher{}.Hash(mustPassword(t, "s3cretpass"))
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	return h
}

func TestRestoreName(t *testing.T) {
	longName := strings.Repeat("a", 51)

	if _, err := user.NewName(longName); err == nil {
		t.Fatal("NewName accepted data that should fail current rules")
	}
	if got := user.RestoreName(longName).String(); got != longName {
		t.Errorf("RestoreName(%q) = %q, want %q", longName, got, longName)
	}
}

func TestRestoreUser(t *testing.T) {
	password := mustPassword(t, "s3cretpass")
	hash, err := fakeHasher{}.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	id := uuid.New()
	const legacyEmail = "Legacy@Example.com"
	fields := janeDoe
	name := mustFullName(t, fields)
	createdAt := time.Date(2026, 10, 14, 19, 30, 0, 0, time.UTC)

	u := user.RestoreUser(id, user.RestoreEmail(legacyEmail), name, hash, createdAt, nil)
	got := fieldsOf(u.FullName())
	matches := u.ID() == id && u.Email().String() == legacyEmail && got == fields && u.PasswordHash() == hash && u.CreatedAt().Equal(createdAt)
	if !matches {
		t.Errorf("RestoreUser() = {id: %v, email: %q, fullName: %+v, hash: %q, createdAt: %v}, want {id: %v, email: %q, fullName: %+v, hash: %q, createdAt: %v}",
			u.ID(), u.Email(), got, u.PasswordHash().Encoded(), u.CreatedAt(), id, legacyEmail, fields, hash.Encoded(), createdAt)
	}
	if err := u.Authenticate(password, fakeHasher{}); err != nil {
		t.Errorf("Authenticate = %v, want nil", err)
	}
	if events := u.Events(); len(events) != 0 {
		t.Errorf("Events() after RestoreUser returned %d events, want 0", len(events))
	}
}

func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	password := mustPassword(t, "s3cretpass")
	other := mustPassword(t, "otherpass")
	u, err := user.NewUser(mustEmail(t, janeEmail), mustFullName(t, janeDoe), password, fakeHasher{})
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}

	if err := u.Authenticate(other, fakeHasher{}); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Errorf("Authenticate(wrong) = %v, want %v", err, user.ErrInvalidCredentials)
	}
}

func mustEmail(t *testing.T, s string) user.Email {
	t.Helper()
	e, err := user.NewEmail(s)
	if err != nil {
		t.Fatalf("NewEmail(%q) error: %v", s, err)
	}
	return e
}

func mustName(t *testing.T, s string) user.Name {
	t.Helper()
	n, err := user.NewName(s)
	if err != nil {
		t.Fatalf("NewName(%q) error: %v", s, err)
	}
	return n
}

func eventsOf[E user.Event](u *user.User) []E {
	var out []E
	for _, e := range u.Events() {
		if typed, ok := e.(E); ok {
			out = append(out, typed)
		}
	}
	return out
}

func TestRename(t *testing.T) {
	t.Run("changed", func(t *testing.T) {
		u := mustUser(t)
		u.Rename(user.NewFullName(mustName(t, "Jane"), mustName(t, "Smith")))
		events := eventsOf[user.Renamed](u)
		if u.FullName().LastName().String() != "Smith" || len(events) != 1 {
			t.Fatalf("after Rename: lastName=%q, %d Renamed; want Smith, 1", u.FullName().LastName(), len(events))
		}
		if e := events[0]; e.AggregateID != u.ID() || e.FullName.FirstName().String() != "Jane" || e.FullName.LastName().String() != "Smith" {
			t.Errorf("Renamed = %+v", e)
		}
	})
	t.Run("same values", func(t *testing.T) {
		u := mustUser(t)
		u.Rename(user.NewFullName(mustName(t, "Jane"), mustName(t, "Doe")))
		if n := len(eventsOf[user.Renamed](u)); n != 0 {
			t.Errorf("recorded %d Renamed, want 0", n)
		}
	})
}

func requestChange(t *testing.T, u *user.User, email, password string) error {
	t.Helper()
	return u.RequestEmailChange(mustEmail(t, email), mustPassword(t, password), fakeHasher{})
}

func TestRequestEmailChange(t *testing.T) {
	t.Run("records pending change and event", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			u := mustUser(t)
			now := time.Now()
			if err := requestChange(t, u, "new@example.com", "s3cretpass"); err != nil {
				t.Fatalf("RequestEmailChange() error: %v", err)
			}
			if u.Email().String() != "jane@example.com" {
				t.Errorf("Email() = %q, want unchanged until confirmation", u.Email())
			}
			p, ok := u.PendingEmailChange()
			events := eventsOf[user.EmailChangeRequested](u)
			if !ok || p.Email().String() != "new@example.com" || !p.ExpiresAt().Equal(now.Add(24*time.Hour)) || len(events) != 1 {
				t.Fatalf("pending = %+v, %v; %d events", p, ok, len(events))
			}
			if e := events[0]; e.Token.Hash() != p.TokenHash() || e.NewEmail != p.Email() || !e.ExpiresAt.Equal(p.ExpiresAt()) || !e.OccurredAt.Equal(now) {
				t.Errorf("EmailChangeRequested = %+v does not match pending %+v", e, p)
			}
		})
	})
	t.Run("wrong password", func(t *testing.T) {
		u := mustUser(t)
		if err := requestChange(t, u, "new@example.com", "wrongpass"); !errors.Is(err, user.ErrInvalidCredentials) {
			t.Errorf("error = %v, want %v", err, user.ErrInvalidCredentials)
		}
		if _, ok := u.PendingEmailChange(); ok || len(u.Events()) != 1 {
			t.Error("failed request left pending change or extra events")
		}
	})
	t.Run("same email in other case", func(t *testing.T) {
		u := mustUser(t)
		err := requestChange(t, u, "Jane@Example.com", "s3cretpass")
		if !errors.Is(err, user.ErrEmailUnchanged) || !errors.Is(err, user.ErrValidation) {
			t.Errorf("error = %v, want %v", err, user.ErrEmailUnchanged)
		}
	})
}

func TestConfirmEmailChange(t *testing.T) {
	requested := func(t *testing.T) (*user.User, user.EmailChangeToken) {
		u := mustUser(t)
		if err := requestChange(t, u, "new@example.com", "s3cretpass"); err != nil {
			t.Fatalf("RequestEmailChange() error: %v", err)
		}
		return u, eventsOf[user.EmailChangeRequested](u)[0].Token
	}
	t.Run("valid at expiry instant", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			u, tok := requested(t)
			time.Sleep(24 * time.Hour)
			if err := u.ConfirmEmailChange(tok); err != nil {
				t.Fatalf("ConfirmEmailChange() error: %v", err)
			}
			events := eventsOf[user.EmailChanged](u)
			if _, ok := u.PendingEmailChange(); ok || u.Email().String() != "new@example.com" || len(events) != 1 {
				t.Fatalf("after confirm: email=%q pending=%v events=%d", u.Email(), ok, len(events))
			}
			if e := events[0]; e.OldEmail.String() != "jane@example.com" || e.NewEmail.String() != "new@example.com" || !e.OccurredAt.Equal(time.Now()) {
				t.Errorf("EmailChanged = %+v", e)
			}
		})
	})
	t.Run("expired", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			u, tok := requested(t)
			time.Sleep(24*time.Hour + time.Nanosecond)
			err := u.ConfirmEmailChange(tok)
			if !errors.Is(err, user.ErrEmailChangeTokenExpired) || u.Email().String() != "jane@example.com" {
				t.Errorf("error = %v, email = %q; want %v, unchanged", err, u.Email(), user.ErrEmailChangeTokenExpired)
			}
		})
	})
	t.Run("superseded token", func(t *testing.T) {
		u, old := requested(t)
		if err := requestChange(t, u, "other@example.com", "s3cretpass"); err != nil {
			t.Fatalf("second RequestEmailChange() error: %v", err)
		}
		if err := u.ConfirmEmailChange(old); !errors.Is(err, user.ErrInvalidEmailChangeToken) {
			t.Errorf("confirm with superseded token = %v, want %v", err, user.ErrInvalidEmailChangeToken)
		}
	})
	t.Run("no pending change", func(t *testing.T) {
		u := mustUser(t)
		if err := u.ConfirmEmailChange(user.NewEmailChangeToken()); !errors.Is(err, user.ErrInvalidEmailChangeToken) {
			t.Errorf("error = %v, want %v", err, user.ErrInvalidEmailChangeToken)
		}
	})
}

func TestRestoreUserWithPendingEmailChange(t *testing.T) {
	tok := user.NewEmailChangeToken()
	expiresAt := time.Now().Add(time.Hour)
	pending := user.RestorePendingEmailChange(user.RestoreEmail("new@example.com"), tok.Hash(), expiresAt)
	u := user.RestoreUser(uuid.New(), user.RestoreEmail(janeEmail), mustFullName(t, janeDoe), mustHash(t), time.Now(), &pending)
	if err := u.ConfirmEmailChange(tok); err != nil {
		t.Errorf("ConfirmEmailChange() on restored user error: %v", err)
	}
}

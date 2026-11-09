package user_test

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
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

func TestEventsReturnsCopy(t *testing.T) {
	u := mustUser(t)

	events := u.Events()
	events[0] = nil

	if u.Events()[0] == nil {
		t.Error("modifying the returned slice changed the aggregate's events")
	}
}

func mustUser(t *testing.T) *user.User {
	t.Helper()
	p, tok := mustPending(t)
	if err := p.Confirm(tok); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	u, err := p.Complete()
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
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

func TestRestore(t *testing.T) {
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

	u := user.Restore(user.Snapshot{
		ID:           id,
		Email:        user.RestoreEmail(legacyEmail),
		FullName:     name,
		PasswordHash: hash,
		CreatedAt:    createdAt,
	})
	got := fieldsOf(u.FullName())
	matches := u.ID() == id && u.Email().String() == legacyEmail && got == fields && u.PasswordHash() == hash && u.CreatedAt().Equal(createdAt)
	if !matches {
		t.Errorf("Restore() = {id: %v, email: %q, fullName: %+v, hash: %q, createdAt: %v}, want {id: %v, email: %q, fullName: %+v, hash: %q, createdAt: %v}",
			u.ID(), u.Email(), got, u.PasswordHash().Encoded(), u.CreatedAt(), id, legacyEmail, fields, hash.Encoded(), createdAt)
	}
	if err := u.Authenticate(password, fakeHasher{}); err != nil {
		t.Errorf("Authenticate = %v, want nil", err)
	}
	if events := u.Events(); len(events) != 0 {
		t.Errorf("Events() after Restore returned %d events, want 0", len(events))
	}
}

func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	other := mustPassword(t, "otherpass")
	u := mustUser(t)

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
	proof, err := u.Reauthenticate(mustPassword(t, password), fakeHasher{})
	if err != nil {
		return err
	}
	return u.RequestEmailChange(mustEmail(t, email), false, proof)
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

func TestRequestEmailChangeToTakenAddress(t *testing.T) {
	attempt := func(t *testing.T, u *user.User, email, password string) error {
		t.Helper()
		proof, err := u.Reauthenticate(mustPassword(t, password), fakeHasher{})
		if err != nil {
			return err
		}
		return u.RequestEmailChange(mustEmail(t, email), true, proof)
	}
	t.Run("records event without pending change", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			u := mustUser(t)
			if err := attempt(t, u, "taken@example.com", "s3cretpass"); err != nil {
				t.Fatalf("RequestEmailChange() error: %v", err)
			}
			if _, ok := u.PendingEmailChange(); ok || u.Email().String() != janeEmail {
				t.Errorf("email = %q, pending = %v; want unchanged, no pending", u.Email(), ok)
			}
			if n := len(eventsOf[user.EmailChangeRequested](u)); n != 0 {
				t.Errorf("recorded %d EmailChangeRequested, want 0 so no link goes to a taken address", n)
			}
			events := eventsOf[user.EmailChangeToTakenAddressAttempted](u)
			if len(events) != 1 {
				t.Fatalf("recorded %d EmailChangeToTakenAddressAttempted, want 1", len(events))
			}
			if e := events[0]; e.AggregateID != u.ID() || e.NewEmail.String() != "taken@example.com" || !e.OccurredAt.Equal(time.Now()) {
				t.Errorf("EmailChangeToTakenAddressAttempted = %+v", e)
			}
		})
	})
	t.Run("invalidates earlier token", func(t *testing.T) {
		u, earlier := requestedChange(t)
		if err := attempt(t, u, "taken@example.com", "s3cretpass"); err != nil {
			t.Fatalf("RequestEmailChange() error: %v", err)
		}
		if err := u.ConfirmEmailChange(earlier); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("confirm with earlier token = %v, want %v so it cannot reveal that the address is taken", err, user.ErrInvalidConfirmationToken)
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		u := mustUser(t)
		if err := attempt(t, u, "taken@example.com", "wrongpass"); !errors.Is(err, user.ErrInvalidCredentials) {
			t.Errorf("error = %v, want %v", err, user.ErrInvalidCredentials)
		}
		if n := len(eventsOf[user.EmailChangeToTakenAddressAttempted](u)); n != 0 {
			t.Errorf("recorded %d EmailChangeToTakenAddressAttempted, want 0", n)
		}
	})
	t.Run("own email", func(t *testing.T) {
		u := mustUser(t)
		err := attempt(t, u, "Jane@Example.com", "s3cretpass")
		if !errors.Is(err, user.ErrEmailUnchanged) {
			t.Errorf("error = %v, want %v", err, user.ErrEmailUnchanged)
		}
	})
}

func TestEmailChangeRejectsInvalidProof(t *testing.T) {
	addresses := map[string]bool{
		"free":  false,
		"taken": true,
	}
	proofs := map[string]func(*testing.T, *user.User) user.Reauthentication{
		"missing": func(*testing.T, *user.User) user.Reauthentication {
			return user.Reauthentication{}
		},
		"other user": func(t *testing.T, u *user.User) user.Reauthentication {
			other := user.Restore(user.Snapshot{
				ID:           uuid.New(),
				Email:        u.Email(),
				FullName:     u.FullName(),
				PasswordHash: u.PasswordHash(),
				CreatedAt:    u.CreatedAt(),
			})
			proof, err := other.Reauthenticate(mustPassword(t, "s3cretpass"), fakeHasher{})
			if err != nil {
				t.Fatalf("Reauthenticate() error: %v", err)
			}
			return proof
		},
		"password changed since": func(t *testing.T, u *user.User) user.Reauthentication {
			hash := user.RestorePasswordHash("hashed:otherpass1")
			earlier := user.Restore(user.Snapshot{
				ID:           u.ID(),
				Email:        u.Email(),
				FullName:     u.FullName(),
				PasswordHash: hash,
				CreatedAt:    u.CreatedAt(),
			})
			proof, err := earlier.Reauthenticate(mustPassword(t, "otherpass1"), fakeHasher{})
			if err != nil {
				t.Fatalf("Reauthenticate() error: %v", err)
			}
			return proof
		},
	}
	for addressName, taken := range addresses {
		for proofName, proof := range proofs {
			t.Run(addressName+"/"+proofName, func(t *testing.T) {
				u := mustUser(t)
				events := len(u.Events())

				err := u.RequestEmailChange(mustEmail(t, "new@example.com"), taken, proof(t, u))
				if !errors.Is(err, user.ErrInvalidCredentials) {
					t.Errorf("error = %v, want %v", err, user.ErrInvalidCredentials)
				}
				if _, ok := u.PendingEmailChange(); ok || len(u.Events()) != events {
					t.Error("rejected proof left pending change or extra events")
				}
			})
		}
	}
}

func requestedChange(t *testing.T) (*user.User, user.ConfirmationToken) {
	t.Helper()
	u := mustUser(t)
	if err := requestChange(t, u, "new@example.com", "s3cretpass"); err != nil {
		t.Fatalf("RequestEmailChange() error: %v", err)
	}
	return u, eventsOf[user.EmailChangeRequested](u)[0].Token
}

func TestConfirmEmailChange(t *testing.T) {
	t.Run("valid at expiry instant", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			u, tok := requestedChange(t)
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
			u, tok := requestedChange(t)
			time.Sleep(24*time.Hour + time.Nanosecond)
			err := u.ConfirmEmailChange(tok)
			if !errors.Is(err, user.ErrConfirmationTokenExpired) || u.Email().String() != "jane@example.com" {
				t.Errorf("error = %v, email = %q; want %v, unchanged", err, u.Email(), user.ErrConfirmationTokenExpired)
			}
		})
	})
	t.Run("superseded token", func(t *testing.T) {
		u, old := requestedChange(t)
		if err := requestChange(t, u, "other@example.com", "s3cretpass"); err != nil {
			t.Fatalf("second RequestEmailChange() error: %v", err)
		}
		if err := u.ConfirmEmailChange(old); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("confirm with superseded token = %v, want %v", err, user.ErrInvalidConfirmationToken)
		}
	})
	t.Run("no pending change", func(t *testing.T) {
		u := mustUser(t)
		if err := u.ConfirmEmailChange(user.NewConfirmationToken()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("error = %v, want %v", err, user.ErrInvalidConfirmationToken)
		}
	})
}

func TestRejectEmailChange(t *testing.T) {
	t.Run("discards pending change and records event", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			u, tok := requestedChange(t)
			if err := u.RejectEmailChange(tok); err != nil {
				t.Fatalf("RejectEmailChange() error: %v", err)
			}
			if _, ok := u.PendingEmailChange(); ok || u.Email().String() != janeEmail {
				t.Errorf("email = %q, pending = %v; want unchanged, no pending", u.Email(), ok)
			}
			events := eventsOf[user.EmailChangeRejected](u)
			if len(events) != 1 {
				t.Fatalf("recorded %d EmailChangeRejected, want 1", len(events))
			}
			if e := events[0]; e.AggregateID != u.ID() || e.NewEmail.String() != "new@example.com" || !e.OccurredAt.Equal(time.Now()) {
				t.Errorf("EmailChangeRejected = %+v", e)
			}
		})
	})
	t.Run("superseded token", func(t *testing.T) {
		u, old := requestedChange(t)
		if err := requestChange(t, u, "other@example.com", "s3cretpass"); err != nil {
			t.Fatalf("second RequestEmailChange() error: %v", err)
		}
		if err := u.RejectEmailChange(old); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("reject with superseded token = %v, want %v", err, user.ErrInvalidConfirmationToken)
		}
		p, ok := u.PendingEmailChange()
		if !ok || p.Email().String() != "other@example.com" || len(eventsOf[user.EmailChangeRejected](u)) != 0 {
			t.Errorf("pending = %+v, %v; want other@example.com kept and no EmailChangeRejected", p, ok)
		}
	})
	t.Run("no pending change", func(t *testing.T) {
		u := mustUser(t)
		if err := u.RejectEmailChange(user.NewConfirmationToken()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("error = %v, want %v", err, user.ErrInvalidConfirmationToken)
		}
	})
}

func TestRestoreWithPendingEmailChange(t *testing.T) {
	tok := user.NewConfirmationToken()
	expiresAt := time.Now().Add(time.Hour)
	pending := user.RestorePendingEmailChange(user.RestoreEmail("new@example.com"), tok.Hash(), expiresAt)
	u := user.Restore(user.Snapshot{
		ID:                 uuid.New(),
		Email:              user.RestoreEmail(janeEmail),
		FullName:           mustFullName(t, janeDoe),
		PasswordHash:       mustHash(t),
		CreatedAt:          time.Now(),
		PendingEmailChange: &pending,
	})
	if err := u.ConfirmEmailChange(tok); err != nil {
		t.Errorf("ConfirmEmailChange() on restored user error: %v", err)
	}
}

func TestRecordDuplicateRegistration(t *testing.T) {
	u := mustUser(t)
	email, name, hash := u.Email(), u.FullName(), u.PasswordHash()
	before := len(u.Events())
	u.RecordDuplicateRegistration()
	events := u.Events()
	if len(events) != before+1 {
		t.Fatalf("recorded %d events, want %d", len(events), before+1)
	}
	e, ok := events[len(events)-1].(user.DuplicateRegistrationAttempted)
	if !ok {
		t.Fatalf("last event = %T, want user.DuplicateRegistrationAttempted", events[len(events)-1])
	}
	if e.AggregateID != u.ID() || e.Email != u.Email() {
		t.Errorf("DuplicateRegistrationAttempted = %+v", e)
	}
	if u.Email() != email || u.FullName() != name || u.PasswordHash() != hash {
		t.Errorf("user state changed after RecordDuplicateRegistration")
	}
}

package user_test

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/nexus-commerce/user-service/internal/user"
)

const janeEmail = "jane@example.com"

var (
	valueObjects      = cmp.AllowUnexported(user.Email{}, user.Name{}, user.FullName{}, user.ConfirmationToken{}, user.ConfirmationTokenHash{}, user.PasswordHash{})
	ignoreEventID     = cmpopts.IgnoreFields(user.EventMeta{}, "ID")
	ignoreChangeToken = cmpopts.IgnoreFields(user.EmailChangeRequested{}, "Token")
)

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

type userFields struct {
	ID           uuid.UUID
	Email        string
	FullName     nameFields
	PasswordHash string
	CreatedAt    time.Time
}

func userOf(u *user.User) userFields {
	return userFields{
		ID:           u.ID(),
		Email:        u.Email().String(),
		FullName:     fieldsOf(u.FullName()),
		PasswordHash: u.PasswordHash().Encoded(),
		CreatedAt:    u.CreatedAt(),
	}
}

type pendingEmailFields struct {
	Email     string
	TokenHash string
	ExpiresAt time.Time
}

func pendingEmailOf(u *user.User) (pendingEmailFields, bool) {
	p, ok := u.PendingEmailChange()
	return pendingEmailFields{
		Email:     p.Email().String(),
		TokenHash: p.TokenHash().Encoded(),
		ExpiresAt: p.ExpiresAt(),
	}, ok
}

func TestEventsReturnsCopy(t *testing.T) {
	u := mustUser(t)

	events := u.Events()
	events[0] = nil

	if u.Events()[0] == nil {
		t.Error("modifying the slice returned by Events() changed the aggregate's events")
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
		t.Fatalf("NewName(%d chars) error = nil, want error", len(longName))
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
	want := userFields{
		ID:           id,
		Email:        legacyEmail,
		FullName:     fields,
		PasswordHash: hash.Encoded(),
		CreatedAt:    createdAt,
	}
	if diff := cmp.Diff(want, userOf(u)); diff != "" {
		t.Errorf("Restore() mismatch (-want +got):\n%s", diff)
	}
	if err := u.Authenticate(password, fakeHasher{}); err != nil {
		t.Errorf("Authenticate() error = %v, want nil", err)
	}
	if events := u.Events(); len(events) != 0 {
		t.Errorf("Events() after Restore returned %d events, want 0", len(events))
	}
}

func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	other := mustPassword(t, "otherpass")
	u := mustUser(t)

	if err := u.Authenticate(other, fakeHasher{}); !errors.Is(err, user.ErrInvalidCredentials) {
		t.Errorf("Authenticate(wrong) error = %v, want %v", err, user.ErrInvalidCredentials)
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
		synctest.Test(t, func(t *testing.T) {
			u := mustUser(t)
			name := user.NewFullName(mustName(t, "Jane"), mustName(t, "Smith"))
			u.Rename(name)
			wantName := nameFields{FirstName: "Jane", LastName: "Smith"}
			if diff := cmp.Diff(wantName, fieldsOf(u.FullName())); diff != "" {
				t.Errorf("Rename() FullName() mismatch (-want +got):\n%s", diff)
			}
			events := eventsOf[user.Renamed](u)
			if len(events) != 1 {
				t.Fatalf("Rename() recorded %d Renamed events, want 1", len(events))
			}
			want := user.Renamed{
				AggregateID: u.ID(),
				OccurredAt:  time.Now(),
				FullName:    name,
			}
			if diff := cmp.Diff(want, events[0], valueObjects, ignoreEventID); diff != "" {
				t.Errorf("Rename() Renamed event mismatch (-want +got):\n%s", diff)
			}
		})
	})
	t.Run("same values", func(t *testing.T) {
		u := mustUser(t)
		u.Rename(user.NewFullName(mustName(t, "Jane"), mustName(t, "Doe")))
		if n := len(eventsOf[user.Renamed](u)); n != 0 {
			t.Errorf("Rename(same name) recorded %d Renamed events, want 0", n)
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
			if got := u.Email().String(); got != janeEmail {
				t.Errorf("RequestEmailChange() Email() = %q, want %q until confirmation", got, janeEmail)
			}
			events := eventsOf[user.EmailChangeRequested](u)
			if len(events) != 1 {
				t.Fatalf("RequestEmailChange() recorded %d EmailChangeRequested events, want 1", len(events))
			}
			e := events[0]
			want := user.EmailChangeRequested{
				AggregateID: u.ID(),
				OccurredAt:  now,
				FullName:    u.FullName(),
				NewEmail:    mustEmail(t, "new@example.com"),
				ExpiresAt:   now.Add(24 * time.Hour),
			}
			if diff := cmp.Diff(want, e, valueObjects, ignoreEventID, ignoreChangeToken); diff != "" {
				t.Errorf("RequestEmailChange() EmailChangeRequested event mismatch (-want +got):\n%s", diff)
			}
			got, ok := pendingEmailOf(u)
			if !ok {
				t.Fatal("RequestEmailChange() left no pending email change, want one")
			}
			wantPending := pendingEmailFields{Email: "new@example.com", TokenHash: e.Token.Hash().Encoded(), ExpiresAt: now.Add(24 * time.Hour)}
			if diff := cmp.Diff(wantPending, got); diff != "" {
				t.Errorf("RequestEmailChange() pending change mismatch (-want +got):\n%s", diff)
			}
		})
	})
	t.Run("wrong password", func(t *testing.T) {
		u := mustUser(t)
		if err := requestChange(t, u, "new@example.com", "wrongpass"); !errors.Is(err, user.ErrInvalidCredentials) {
			t.Errorf("Reauthenticate(wrong) error = %v, want %v", err, user.ErrInvalidCredentials)
		}
		if got, ok := pendingEmailOf(u); ok {
			t.Errorf("Reauthenticate(wrong) left pending change %+v, want none", got)
		}
		if n := len(u.Events()); n != 1 {
			t.Errorf("Reauthenticate(wrong) Events() len = %d, want 1", n)
		}
	})
	t.Run("same email in other case", func(t *testing.T) {
		u := mustUser(t)
		err := requestChange(t, u, "Jane@Example.com", "s3cretpass")
		if !errors.Is(err, user.ErrEmailUnchanged) {
			t.Errorf("RequestEmailChange(own email) error = %v, want %v", err, user.ErrEmailUnchanged)
		}
		if !errors.Is(err, user.ErrValidation) {
			t.Errorf("RequestEmailChange(own email) error = %v, want %v", err, user.ErrValidation)
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
				t.Fatalf("RequestEmailChange(taken) error: %v", err)
			}
			if got := u.Email().String(); got != janeEmail {
				t.Errorf("RequestEmailChange(taken) Email() = %q, want %q", got, janeEmail)
			}
			if got, ok := pendingEmailOf(u); ok {
				t.Errorf("RequestEmailChange(taken) left pending change %+v, want none", got)
			}
			if n := len(eventsOf[user.EmailChangeRequested](u)); n != 0 {
				t.Errorf("RequestEmailChange(taken) recorded %d EmailChangeRequested events, want 0 so no link goes to a taken address", n)
			}
			events := eventsOf[user.EmailChangeToTakenAddressAttempted](u)
			if len(events) != 1 {
				t.Fatalf("RequestEmailChange(taken) recorded %d EmailChangeToTakenAddressAttempted events, want 1", len(events))
			}
			want := user.EmailChangeToTakenAddressAttempted{AggregateID: u.ID(), OccurredAt: time.Now(), NewEmail: mustEmail(t, "taken@example.com")}
			if diff := cmp.Diff(want, events[0], valueObjects, ignoreEventID); diff != "" {
				t.Errorf("RequestEmailChange(taken) event mismatch (-want +got):\n%s", diff)
			}
		})
	})
	t.Run("invalidates earlier token", func(t *testing.T) {
		u, earlier := requestedChange(t)
		if err := attempt(t, u, "taken@example.com", "s3cretpass"); err != nil {
			t.Fatalf("RequestEmailChange(taken) error: %v", err)
		}
		if err := u.ConfirmEmailChange(earlier); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("ConfirmEmailChange(earlier token) error = %v, want %v so it cannot reveal that the address is taken", err, user.ErrInvalidConfirmationToken)
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		u := mustUser(t)
		if err := attempt(t, u, "taken@example.com", "wrongpass"); !errors.Is(err, user.ErrInvalidCredentials) {
			t.Errorf("Reauthenticate(wrong) error = %v, want %v", err, user.ErrInvalidCredentials)
		}
		if n := len(eventsOf[user.EmailChangeToTakenAddressAttempted](u)); n != 0 {
			t.Errorf("Reauthenticate(wrong) recorded %d EmailChangeToTakenAddressAttempted events, want 0", n)
		}
	})
	t.Run("own email", func(t *testing.T) {
		u := mustUser(t)
		err := attempt(t, u, "Jane@Example.com", "s3cretpass")
		if !errors.Is(err, user.ErrEmailUnchanged) {
			t.Errorf("RequestEmailChange(own email) error = %v, want %v", err, user.ErrEmailUnchanged)
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
					t.Errorf("RequestEmailChange() error = %v, want %v", err, user.ErrInvalidCredentials)
				}
				if got, ok := pendingEmailOf(u); ok {
					t.Errorf("RequestEmailChange() left pending change %+v, want none", got)
				}
				if n := len(u.Events()); n != events {
					t.Errorf("RequestEmailChange() Events() len = %d, want %d", n, events)
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
			if got := u.Email().String(); got != "new@example.com" {
				t.Errorf("ConfirmEmailChange() Email() = %q, want %q", got, "new@example.com")
			}
			if got, ok := pendingEmailOf(u); ok {
				t.Errorf("ConfirmEmailChange() left pending change %+v, want none", got)
			}
			events := eventsOf[user.EmailChanged](u)
			if len(events) != 1 {
				t.Fatalf("ConfirmEmailChange() recorded %d EmailChanged events, want 1", len(events))
			}
			want := user.EmailChanged{
				AggregateID: u.ID(),
				OccurredAt:  time.Now(),
				FullName:    u.FullName(),
				OldEmail:    mustEmail(t, janeEmail),
				NewEmail:    mustEmail(t, "new@example.com"),
			}
			if diff := cmp.Diff(want, events[0], valueObjects, ignoreEventID); diff != "" {
				t.Errorf("ConfirmEmailChange() EmailChanged event mismatch (-want +got):\n%s", diff)
			}
		})
	})
	t.Run("expired", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			u, tok := requestedChange(t)
			time.Sleep(24*time.Hour + time.Nanosecond)
			if err := u.ConfirmEmailChange(tok); !errors.Is(err, user.ErrConfirmationTokenExpired) {
				t.Errorf("ConfirmEmailChange(expired) error = %v, want %v", err, user.ErrConfirmationTokenExpired)
			}
			if got := u.Email().String(); got != janeEmail {
				t.Errorf("ConfirmEmailChange(expired) Email() = %q, want %q", got, janeEmail)
			}
		})
	})
	t.Run("superseded token", func(t *testing.T) {
		u, old := requestedChange(t)
		if err := requestChange(t, u, "other@example.com", "s3cretpass"); err != nil {
			t.Fatalf("second RequestEmailChange() error: %v", err)
		}
		if err := u.ConfirmEmailChange(old); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("ConfirmEmailChange(superseded token) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
		}
	})
	t.Run("no pending change", func(t *testing.T) {
		u := mustUser(t)
		if err := u.ConfirmEmailChange(user.NewConfirmationToken()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("ConfirmEmailChange() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
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
			if got := u.Email().String(); got != janeEmail {
				t.Errorf("RejectEmailChange() Email() = %q, want %q", got, janeEmail)
			}
			if got, ok := pendingEmailOf(u); ok {
				t.Errorf("RejectEmailChange() left pending change %+v, want none", got)
			}
			events := eventsOf[user.EmailChangeRejected](u)
			if len(events) != 1 {
				t.Fatalf("RejectEmailChange() recorded %d EmailChangeRejected events, want 1", len(events))
			}
			want := user.EmailChangeRejected{
				AggregateID: u.ID(),
				OccurredAt:  time.Now(),
				NewEmail:    mustEmail(t, "new@example.com"),
			}
			if diff := cmp.Diff(want, events[0], valueObjects, ignoreEventID); diff != "" {
				t.Errorf("RejectEmailChange() EmailChangeRejected event mismatch (-want +got):\n%s", diff)
			}
		})
	})
	t.Run("superseded token", func(t *testing.T) {
		u, old := requestedChange(t)
		if err := requestChange(t, u, "other@example.com", "s3cretpass"); err != nil {
			t.Fatalf("second RequestEmailChange() error: %v", err)
		}
		before, _ := pendingEmailOf(u)
		if err := u.RejectEmailChange(old); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("RejectEmailChange(superseded token) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
		}
		got, ok := pendingEmailOf(u)
		if !ok {
			t.Error("RejectEmailChange(superseded token) discarded the pending change, want it kept")
		}
		if diff := cmp.Diff(before, got); diff != "" {
			t.Errorf("RejectEmailChange(superseded token) changed the pending change (-want +got):\n%s", diff)
		}
		if n := len(eventsOf[user.EmailChangeRejected](u)); n != 0 {
			t.Errorf("RejectEmailChange(superseded token) recorded %d EmailChangeRejected events, want 0", n)
		}
	})
	t.Run("no pending change", func(t *testing.T) {
		u := mustUser(t)
		if err := u.RejectEmailChange(user.NewConfirmationToken()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
			t.Errorf("RejectEmailChange() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
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
	synctest.Test(t, func(t *testing.T) {
		u := mustUser(t)
		before := userOf(u)
		count := len(u.Events())
		u.RecordDuplicateRegistration()
		events := u.Events()
		if len(events) != count+1 {
			t.Fatalf("RecordDuplicateRegistration() Events() len = %d, want %d", len(events), count+1)
		}
		e, ok := events[len(events)-1].(user.DuplicateRegistrationAttempted)
		if !ok {
			t.Fatalf("RecordDuplicateRegistration() last event = %T, want user.DuplicateRegistrationAttempted", events[len(events)-1])
		}
		want := user.DuplicateRegistrationAttempted{
			AggregateID: u.ID(),
			OccurredAt:  time.Now(),
			FullName:    u.FullName(),
			Email:       u.Email(),
		}
		if diff := cmp.Diff(want, e, valueObjects, ignoreEventID); diff != "" {
			t.Errorf("RecordDuplicateRegistration() event mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(before, userOf(u)); diff != "" {
			t.Errorf("RecordDuplicateRegistration() changed user state (-want +got):\n%s", diff)
		}
	})
}

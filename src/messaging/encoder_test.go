package messaging

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type unroutedEvent struct {
	user.Registered
}

func newUserRegistered(t *testing.T) user.Registered {
	t.Helper()
	email, err := user.NewEmail("jane@example.com")
	if err != nil {
		t.Fatalf("NewEmail() error: %v", err)
	}
	first, err := user.NewName("Jane")
	if err != nil {
		t.Fatalf("NewName() error: %v", err)
	}
	last, err := user.NewName("Doe")
	if err != nil {
		t.Fatalf("NewName() error: %v", err)
	}
	return user.Registered{
		EventMeta: newMeta(),
		Email:     email,
		FullName:  user.NewFullName(first, last),
	}
}

func TestEncodeUserRegistered(t *testing.T) {
	enc := newEncoder(fakeSealer{})
	e := newUserRegistered(t)

	msg, err := enc.encode(e)
	if err != nil {
		t.Fatalf("Encode = _, %v; want _, nil", err)
	}
	type header struct{ ID, Topic, Key string }
	gotHeader := header{msg.ID, msg.Topic, msg.Key}
	wantHeader := header{e.ID.String(), "users", e.AggregateID.String()}
	if gotHeader != wantHeader {
		t.Errorf("encode(Registered) header = %+v, want %+v", gotHeader, wantHeader)
	}

	type payload struct {
		ID      string `json:"eventId"`
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID string `json:"userId"`
			Email  string `json:"email"`
		} `json:"data"`
	}
	var got payload
	if err := json.Unmarshal(msg.Value, &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error: %v", msg.Value, err)
	}
	want := payload{ID: e.ID.String(), Type: "user.registered", Version: "1.0"}
	want.Data.UserID = e.AggregateID.String()
	want.Data.Email = "jane@example.com"
	if got != want {
		t.Errorf("encode(Registered) payload = %+v, want %+v", got, want)
	}
}

func TestEncodeRejectsUnroutedEvent(t *testing.T) {
	enc := newEncoder(fakeSealer{})

	if _, err := enc.encode(unroutedEvent{newUserRegistered(t)}); err == nil {
		t.Error("Encode(unroutedEvent) = _, nil; want error")
	}
}

type fakeSealer struct {
	out []byte
	err error
}

func (s fakeSealer) Seal(p []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.out != nil {
		return s.out, nil
	}
	return append([]byte("sealed:"), p...), nil
}

func (fakeSealer) KeyID() string { return "test-key" }

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

func newMeta() user.EventMeta {
	return user.EventMeta{
		ID:          uuid.New(),
		AggregateID: uuid.New(),
		OccurredAt:  time.Date(2026, 10, 20, 10, 15, 0, 0, time.UTC),
	}
}

func assertHeader(t *testing.T, msg Message, meta user.EventMeta) {
	t.Helper()
	type header struct{ ID, Topic, Key string }
	got := header{msg.ID, msg.Topic, msg.Key}
	want := header{meta.ID.String(), "users", meta.AggregateID.String()}
	if got != want {
		t.Errorf("header = %+v, want %+v", got, want)
	}
}

func decode[T any](t *testing.T, msg Message) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(msg.Value, &v); err != nil {
		t.Fatalf("json.Unmarshal(%s) error: %v", msg.Value, err)
	}
	return v
}

func TestEncodeUserRenamed(t *testing.T) {
	e := user.Renamed{EventMeta: newMeta(), FullName: user.NewFullName(mustName(t, "Jane"), mustName(t, "Smith"))}

	msg, err := newEncoder(fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(Renamed) error: %v", err)
	}
	assertHeader(t, msg, e.EventMeta)

	type payload struct {
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID    string    `json:"userId"`
			FirstName string    `json:"firstName"`
			LastName  string    `json:"lastName"`
			RenamedAt time.Time `json:"renamedAt"`
		} `json:"data"`
	}
	got := decode[payload](t, msg)
	want := payload{Type: "user.renamed", Version: "1.0"}
	want.Data.UserID = e.AggregateID.String()
	want.Data.FirstName = "Jane"
	want.Data.LastName = "Smith"
	want.Data.RenamedAt = e.OccurredAt
	if got != want {
		t.Errorf("encode(Renamed) payload = %+v, want %+v", got, want)
	}
}

func newUserEmailChangeRequested(t *testing.T) user.EmailChangeRequested {
	t.Helper()
	meta := newMeta()
	return user.EmailChangeRequested{
		EventMeta: meta,
		NewEmail:  mustEmail(t, "new@example.com"),
		Token:     user.NewConfirmationToken(),
		ExpiresAt: meta.OccurredAt.Add(24 * time.Hour),
	}
}

func TestEncodeUserEmailChangeRequested(t *testing.T) {
	e := newUserEmailChangeRequested(t)

	msg, err := newEncoder(fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChangeRequested) error: %v", err)
	}
	assertHeader(t, msg, e.EventMeta)

	type payload struct {
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID         string    `json:"userId"`
			NewEmail       string    `json:"newEmail"`
			EncryptedToken []byte    `json:"encryptedToken"`
			KeyID          string    `json:"keyId"`
			ExpiresAt      time.Time `json:"expiresAt"`
			RequestedAt    time.Time `json:"requestedAt"`
		} `json:"data"`
	}
	got := decode[payload](t, msg)
	if got.Type != "user.email_change_requested" || got.Version != "1.0" {
		t.Errorf("type, version = %q, %q; want user.email_change_requested, 1.0", got.Type, got.Version)
	}
	d := got.Data
	if d.UserID != e.AggregateID.String() || d.NewEmail != "new@example.com" || d.KeyID != "test-key" ||
		!d.ExpiresAt.Equal(e.ExpiresAt) || !d.RequestedAt.Equal(e.OccurredAt) {
		t.Errorf("data = %+v", d)
	}
	if want := "sealed:" + e.Token.Plaintext(); string(d.EncryptedToken) != want {
		t.Errorf("encryptedToken = %q, want %q", d.EncryptedToken, want)
	}
}

func TestEncodeUserEmailChangeRequestedHidesPlaintextToken(t *testing.T) {
	e := newUserEmailChangeRequested(t)

	msg, err := newEncoder(fakeSealer{out: []byte("opaque")}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChangeRequested) error: %v", err)
	}
	if bytes.Contains(msg.Value, []byte(e.Token.Plaintext())) {
		t.Errorf("encoded message %s contains plaintext token", msg.Value)
	}
}

func TestEncodeReturnsSealError(t *testing.T) {
	sealErr := errors.New("boom")

	_, err := newEncoder(fakeSealer{err: sealErr}).encode(newUserEmailChangeRequested(t))
	if !errors.Is(err, sealErr) {
		t.Errorf("encode() error = %v, want %v", err, sealErr)
	}
}

func newUserRegistrationRequested(t *testing.T) user.RegistrationRequested {
	t.Helper()
	meta := newMeta()
	return user.RegistrationRequested{
		EventMeta: meta,
		Email:     mustEmail(t, "jane@example.com"),
		FullName:  user.NewFullName(mustName(t, "Jane"), mustName(t, "Doe")),
		Token:     user.NewConfirmationToken(),
		ExpiresAt: meta.OccurredAt.Add(24 * time.Hour),
	}
}

func TestEncodeUserRegistrationRequested(t *testing.T) {
	e := newUserRegistrationRequested(t)

	msg, err := newEncoder(fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(RegistrationRequested) error: %v", err)
	}
	assertHeader(t, msg, e.EventMeta)

	type payload struct {
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID         string    `json:"userId"`
			Email          string    `json:"email"`
			FirstName      string    `json:"firstName"`
			LastName       string    `json:"lastName"`
			EncryptedToken []byte    `json:"encryptedToken"`
			KeyID          string    `json:"keyId"`
			ExpiresAt      time.Time `json:"expiresAt"`
			RequestedAt    time.Time `json:"requestedAt"`
		} `json:"data"`
	}
	got := decode[payload](t, msg)
	if got.Type != "user.registration_requested" || got.Version != "1.0" {
		t.Errorf("type, version = %q, %q; want user.registration_requested, 1.0", got.Type, got.Version)
	}
	d := got.Data
	if d.UserID != e.AggregateID.String() || d.Email != "jane@example.com" || d.FirstName != "Jane" ||
		d.LastName != "Doe" || d.KeyID != "test-key" ||
		!d.ExpiresAt.Equal(e.ExpiresAt) || !d.RequestedAt.Equal(e.OccurredAt) {
		t.Errorf("data = %+v", d)
	}
	if want := "sealed:" + e.Token.Plaintext(); string(d.EncryptedToken) != want {
		t.Errorf("encryptedToken = %q, want %q", d.EncryptedToken, want)
	}
}

func TestEncodeUserRegistrationRequestedHidesPlaintextToken(t *testing.T) {
	e := newUserRegistrationRequested(t)

	msg, err := newEncoder(fakeSealer{out: []byte("opaque")}).encode(e)
	if err != nil {
		t.Fatalf("encode(RegistrationRequested) error: %v", err)
	}
	if bytes.Contains(msg.Value, []byte(e.Token.Plaintext())) {
		t.Errorf("encoded message %s contains plaintext token", msg.Value)
	}
}

func TestEncodeUserDuplicateRegistrationAttempted(t *testing.T) {
	e := user.DuplicateRegistrationAttempted{EventMeta: newMeta(), Email: mustEmail(t, "jane@example.com")}

	msg, err := newEncoder(fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(DuplicateRegistrationAttempted) error: %v", err)
	}
	assertHeader(t, msg, e.EventMeta)

	type payload struct {
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID      string    `json:"userId"`
			Email       string    `json:"email"`
			AttemptedAt time.Time `json:"attemptedAt"`
		} `json:"data"`
	}
	got := decode[payload](t, msg)
	want := payload{Type: "user.duplicate_registration_attempted", Version: "1.0"}
	want.Data.UserID = e.AggregateID.String()
	want.Data.Email = "jane@example.com"
	want.Data.AttemptedAt = e.OccurredAt
	if got != want {
		t.Errorf("encode(DuplicateRegistrationAttempted) payload = %+v, want %+v", got, want)
	}
}

func TestEncodeUserEmailChangeToTakenAddressAttempted(t *testing.T) {
	e := user.EmailChangeToTakenAddressAttempted{EventMeta: newMeta(), NewEmail: mustEmail(t, "taken@example.com")}

	msg, err := newEncoder(fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChangeToTakenAddressAttempted) error: %v", err)
	}
	assertHeader(t, msg, e.EventMeta)

	type payload struct {
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID      string    `json:"userId"`
			NewEmail    string    `json:"newEmail"`
			AttemptedAt time.Time `json:"attemptedAt"`
		} `json:"data"`
	}
	got := decode[payload](t, msg)
	want := payload{Type: "user.email_change_to_taken_address_attempted", Version: "1.0"}
	want.Data.UserID = e.AggregateID.String()
	want.Data.NewEmail = "taken@example.com"
	want.Data.AttemptedAt = e.OccurredAt
	if got != want {
		t.Errorf("encode(EmailChangeToTakenAddressAttempted) payload = %+v, want %+v", got, want)
	}
}

func TestEncodeUserEmailChanged(t *testing.T) {
	e := user.EmailChanged{EventMeta: newMeta(), OldEmail: mustEmail(t, "jane@example.com"), NewEmail: mustEmail(t, "new@example.com")}

	msg, err := newEncoder(fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChanged) error: %v", err)
	}
	assertHeader(t, msg, e.EventMeta)

	type payload struct {
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID    string    `json:"userId"`
			OldEmail  string    `json:"oldEmail"`
			NewEmail  string    `json:"newEmail"`
			ChangedAt time.Time `json:"changedAt"`
		} `json:"data"`
	}
	got := decode[payload](t, msg)
	want := payload{Type: "user.email_changed", Version: "1.0"}
	want.Data.UserID = e.AggregateID.String()
	want.Data.OldEmail = "jane@example.com"
	want.Data.NewEmail = "new@example.com"
	want.Data.ChangedAt = e.OccurredAt
	if got != want {
		t.Errorf("encode(EmailChanged) payload = %+v, want %+v", got, want)
	}
}

package userevent

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/user"
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
	enc := mustEncoder(t, fakeSealer{})
	e := newUserRegistered(t)

	msg, err := enc.encode(e)
	if err != nil {
		t.Fatalf("encode(Registered) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(Registered) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID string `json:"userId"`
		Email  string `json:"email"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.registered",
		Version: "1.0",
		Data:    data{UserID: e.AggregateID.String(), Email: "jane@example.com"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(Registered) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeRejectsUnroutedEvent(t *testing.T) {
	enc := mustEncoder(t, fakeSealer{})

	if _, err := enc.encode(unroutedEvent{newUserRegistered(t)}); err == nil {
		t.Error("encode(unroutedEvent) error = nil, want error")
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

func mustEncoder(t *testing.T, s Sealer) *encoder {
	t.Helper()
	enc, err := newEncoder(s)
	if err != nil {
		t.Fatalf("newEncoder() error: %v", err)
	}
	return enc
}

func TestRegisterRejectsDuplicateRoute(t *testing.T) {
	enc := mustEncoder(t, fakeSealer{})
	r := newRoute(destination{topic: TopicUsers, typ: "user.renamed", version: "1.0"}, convertRenamed)

	if err := enc.register(r); err == nil {
		t.Error("register(duplicate Renamed route) = nil; want error")
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

func newMeta() user.EventMeta {
	return user.EventMeta{
		ID:          uuid.New(),
		AggregateID: uuid.New(),
		OccurredAt:  time.Date(2026, 10, 20, 10, 15, 0, 0, time.UTC),
	}
}

type msgHeader struct{ ID, Topic, Key string }

func headerOf(msg messaging.Message) msgHeader {
	return msgHeader{msg.ID, msg.Topic, msg.Key}
}

func usersHeader(meta user.EventMeta) msgHeader {
	return msgHeader{meta.ID.String(), "users", meta.AggregateID.String()}
}

type decodedEnvelope[D any] struct {
	ID      string `json:"eventId"`
	Type    string `json:"eventType"`
	Version string `json:"version"`
	Data    D      `json:"data"`
}

func decode[T any](t *testing.T, msg messaging.Message) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(msg.Value, &v); err != nil {
		t.Fatalf("json.Unmarshal(%s) error: %v", msg.Value, err)
	}
	return v
}

func TestEncodeUserRenamed(t *testing.T) {
	e := user.Renamed{EventMeta: newMeta(), FullName: user.NewFullName(mustName(t, "Jane"), mustName(t, "Smith"))}

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(Renamed) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(Renamed) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID    string    `json:"userId"`
		FirstName string    `json:"firstName"`
		LastName  string    `json:"lastName"`
		RenamedAt time.Time `json:"renamedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.renamed",
		Version: "1.0",
		Data:    data{UserID: e.AggregateID.String(), FirstName: "Jane", LastName: "Smith", RenamedAt: e.OccurredAt},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(Renamed) payload mismatch (-want +got):\n%s", diff)
	}
}

func newUserEmailChangeRequested(t *testing.T) user.EmailChangeRequested {
	t.Helper()
	meta := newMeta()
	return user.EmailChangeRequested{
		EventMeta: meta,
		FullName:  user.NewFullName(mustName(t, "Jane"), mustName(t, "Doe")),
		NewEmail:  mustEmail(t, "new@example.com"),
		Token:     user.NewConfirmationToken(),
		ExpiresAt: meta.OccurredAt.Add(24 * time.Hour),
	}
}

func TestEncodeUserEmailChangeRequested(t *testing.T) {
	e := newUserEmailChangeRequested(t)

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChangeRequested) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(EmailChangeRequested) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID         string    `json:"userId"`
		FirstName      string    `json:"firstName"`
		LastName       string    `json:"lastName"`
		NewEmail       string    `json:"newEmail"`
		EncryptedToken []byte    `json:"encryptedToken"`
		KeyID          string    `json:"keyId"`
		ExpiresAt      time.Time `json:"expiresAt"`
		RequestedAt    time.Time `json:"requestedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.email_change_requested",
		Version: "1.0",
		Data: data{
			UserID:         e.AggregateID.String(),
			FirstName:      "Jane",
			LastName:       "Doe",
			NewEmail:       "new@example.com",
			EncryptedToken: []byte("sealed:" + e.Token.Plaintext()),
			KeyID:          "test-key",
			ExpiresAt:      e.ExpiresAt,
			RequestedAt:    e.OccurredAt,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(EmailChangeRequested) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeUserEmailChangeRequestedHidesPlaintextToken(t *testing.T) {
	e := newUserEmailChangeRequested(t)

	msg, err := mustEncoder(t, fakeSealer{out: []byte("opaque")}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChangeRequested) error: %v", err)
	}
	if bytes.Contains(msg.Value, []byte(e.Token.Plaintext())) {
		t.Errorf("encoded message %s contains plaintext token", msg.Value)
	}
}

func TestEncodeReturnsSealError(t *testing.T) {
	sealErr := errors.New("boom")

	_, err := mustEncoder(t, fakeSealer{err: sealErr}).encode(newUserEmailChangeRequested(t))
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

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(RegistrationRequested) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(RegistrationRequested) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID         string    `json:"userId"`
		Email          string    `json:"email"`
		FirstName      string    `json:"firstName"`
		LastName       string    `json:"lastName"`
		EncryptedToken []byte    `json:"encryptedToken"`
		KeyID          string    `json:"keyId"`
		ExpiresAt      time.Time `json:"expiresAt"`
		RequestedAt    time.Time `json:"requestedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.registration_requested",
		Version: "1.0",
		Data: data{
			UserID:         e.AggregateID.String(),
			Email:          "jane@example.com",
			FirstName:      "Jane",
			LastName:       "Doe",
			EncryptedToken: []byte("sealed:" + e.Token.Plaintext()),
			KeyID:          "test-key",
			ExpiresAt:      e.ExpiresAt,
			RequestedAt:    e.OccurredAt,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(RegistrationRequested) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeUserRegistrationRequestedHidesPlaintextToken(t *testing.T) {
	e := newUserRegistrationRequested(t)

	msg, err := mustEncoder(t, fakeSealer{out: []byte("opaque")}).encode(e)
	if err != nil {
		t.Fatalf("encode(RegistrationRequested) error: %v", err)
	}
	if bytes.Contains(msg.Value, []byte(e.Token.Plaintext())) {
		t.Errorf("encoded message %s contains plaintext token", msg.Value)
	}
}

func TestEncodeUserDuplicateRegistrationAttempted(t *testing.T) {
	e := user.DuplicateRegistrationAttempted{
		EventMeta: newMeta(),
		FullName:  user.NewFullName(mustName(t, "Jane"), mustName(t, "Doe")),
		Email:     mustEmail(t, "jane@example.com"),
	}

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(DuplicateRegistrationAttempted) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(DuplicateRegistrationAttempted) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID      string    `json:"userId"`
		FirstName   string    `json:"firstName"`
		LastName    string    `json:"lastName"`
		Email       string    `json:"email"`
		AttemptedAt time.Time `json:"attemptedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.duplicate_registration_attempted",
		Version: "1.0",
		Data: data{
			UserID:      e.AggregateID.String(),
			FirstName:   "Jane",
			LastName:    "Doe",
			Email:       "jane@example.com",
			AttemptedAt: e.OccurredAt,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(DuplicateRegistrationAttempted) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeUserEmailChangeToTakenAddressAttempted(t *testing.T) {
	e := user.EmailChangeToTakenAddressAttempted{EventMeta: newMeta(), NewEmail: mustEmail(t, "taken@example.com")}

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChangeToTakenAddressAttempted) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(EmailChangeToTakenAddressAttempted) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID      string    `json:"userId"`
		NewEmail    string    `json:"newEmail"`
		AttemptedAt time.Time `json:"attemptedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.email_change_to_taken_address_attempted",
		Version: "1.0",
		Data:    data{UserID: e.AggregateID.String(), NewEmail: "taken@example.com", AttemptedAt: e.OccurredAt},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(EmailChangeToTakenAddressAttempted) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeUserEmailChanged(t *testing.T) {
	e := user.EmailChanged{
		EventMeta: newMeta(),
		FullName:  user.NewFullName(mustName(t, "Jane"), mustName(t, "Doe")),
		OldEmail:  mustEmail(t, "jane@example.com"),
		NewEmail:  mustEmail(t, "new@example.com"),
	}

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChanged) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(EmailChanged) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID    string    `json:"userId"`
		FirstName string    `json:"firstName"`
		LastName  string    `json:"lastName"`
		OldEmail  string    `json:"oldEmail"`
		NewEmail  string    `json:"newEmail"`
		ChangedAt time.Time `json:"changedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.email_changed",
		Version: "1.0",
		Data: data{
			UserID:    e.AggregateID.String(),
			FirstName: "Jane",
			LastName:  "Doe",
			OldEmail:  "jane@example.com",
			NewEmail:  "new@example.com",
			ChangedAt: e.OccurredAt,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(EmailChanged) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeUserRegistrationConfirmed(t *testing.T) {
	e := user.RegistrationConfirmed{EventMeta: newMeta()}

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(RegistrationConfirmed) error: %v", err)
	}
	type header struct{ ID, Topic, Key, Type string }
	gotHeader := header{msg.ID, msg.Topic, msg.Key, msg.Type}
	wantHeader := header{e.ID.String(), TopicUsersInternal, e.AggregateID.String(), TypeRegistrationConfirmed}
	if diff := cmp.Diff(wantHeader, gotHeader); diff != "" {
		t.Errorf("encode(RegistrationConfirmed) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID      string    `json:"userId"`
		ConfirmedAt time.Time `json:"confirmedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    TypeRegistrationConfirmed,
		Version: "1.0",
		Data:    data{UserID: e.AggregateID.String(), ConfirmedAt: e.OccurredAt},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(RegistrationConfirmed) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeUserRegistrationRejected(t *testing.T) {
	e := user.RegistrationRejected{EventMeta: newMeta(), Email: mustEmail(t, "jane@example.com")}

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(RegistrationRejected) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(RegistrationRejected) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID     string    `json:"userId"`
		Email      string    `json:"email"`
		RejectedAt time.Time `json:"rejectedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.registration_rejected",
		Version: "1.0",
		Data:    data{UserID: e.AggregateID.String(), Email: "jane@example.com", RejectedAt: e.OccurredAt},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(RegistrationRejected) payload mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeUserEmailChangeRejected(t *testing.T) {
	e := user.EmailChangeRejected{EventMeta: newMeta(), NewEmail: mustEmail(t, "new@example.com")}

	msg, err := mustEncoder(t, fakeSealer{}).encode(e)
	if err != nil {
		t.Fatalf("encode(EmailChangeRejected) error: %v", err)
	}
	if diff := cmp.Diff(usersHeader(e.EventMeta), headerOf(msg)); diff != "" {
		t.Errorf("encode(EmailChangeRejected) header mismatch (-want +got):\n%s", diff)
	}

	type data struct {
		UserID     string    `json:"userId"`
		NewEmail   string    `json:"newEmail"`
		RejectedAt time.Time `json:"rejectedAt"`
	}
	got := decode[decodedEnvelope[data]](t, msg)
	want := decodedEnvelope[data]{
		ID:      e.ID.String(),
		Type:    "user.email_change_rejected",
		Version: "1.0",
		Data:    data{UserID: e.AggregateID.String(), NewEmail: "new@example.com", RejectedAt: e.OccurredAt},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("encode(EmailChangeRejected) payload mismatch (-want +got):\n%s", diff)
	}
}

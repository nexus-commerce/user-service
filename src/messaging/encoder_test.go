package messaging

import (
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

type unknownEvent struct{}

func (unknownEvent) EventType() entity.EventType {
	return "Unknown"
}

type mismatchedEvent struct {
	entity.UserRegistered
}

func newUserRegistered(t *testing.T) entity.UserRegistered {
	t.Helper()
	profile, err := entity.NewProfile(entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"})
	if err != nil {
		t.Fatal(err)
	}
	return entity.UserRegistered{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		Profile:    profile,
		OccurredAt: time.Date(2026, 10, 14, 19, 30, 0, 0, time.UTC),
	}
}

func TestEncodeUserRegistered(t *testing.T) {
	enc := newEncoder()
	e := newUserRegistered(t)

	msg, err := enc.encode(e)
	if err != nil {
		t.Fatalf("Encode = _, %v; want _, nil", err)
	}
	if msg.ID != e.ID.String() || msg.Topic != "users" || msg.Key != e.UserID.String() {
		t.Errorf("id, topic, key = %q, %q, %q", msg.ID, msg.Topic, msg.Key)
	}

	var got struct {
		ID      string `json:"eventId"`
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID string `json:"userId"`
			Email  string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID.String() || got.Type != "user.registered" || got.Version != "1.0" || got.Data.UserID != e.UserID.String() || got.Data.Email != "jane@example.com" {
		t.Errorf("payload = %s", msg.Value)
	}
}

func TestEncodeRejectsUnknownEvent(t *testing.T) {
	enc := newEncoder()

	if _, err := enc.encode(unknownEvent{}); err == nil {
		t.Error("Encode(unknownEvent) = _, nil; want error")
	}
}

func TestEncodeRejectsMismatchedType(t *testing.T) {
	enc := newEncoder()

	if _, err := enc.encode(mismatchedEvent{newUserRegistered(t)}); err == nil {
		t.Error("Encode(mismatchedEvent) = _, nil; want error")
	}
}

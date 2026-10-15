package messaging_test

import (
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/messaging"
)

type unpublishedEvent struct {
	entity.UserRegistered
}

func TestEncodeUserRegistered(t *testing.T) {
	enc := messaging.NewEncoder(messaging.Topics{Users: "users"})
	profile, err := entity.NewProfile("jane@example.com", "Jane", "Doe")
	if err != nil {
		t.Fatal(err)
	}
	e := entity.UserRegistered{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		Profile:    profile,
		OccurredAt: time.Date(2026, 10, 14, 19, 30, 0, 0, time.UTC),
	}

	msg, ok, err := enc.Encode(e)
	if err != nil || !ok {
		t.Fatalf("Encode = _, %v, %v; want _, true, nil", ok, err)
	}
	if msg.ID != e.ID.String() || msg.Topic != "users" || msg.Key != e.UserID.String() {
		t.Errorf("id, topic, key = %q, %q, %q", msg.ID, msg.Topic, msg.Key)
	}

	var got struct {
		ID   string `json:"eventId"`
		Type string `json:"eventType"`
		Data struct {
			UserID string `json:"userId"`
			Email  string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != e.ID.String() || got.Type != "user.registered" || got.Data.UserID != e.UserID.String() || got.Data.Email != "jane@example.com" {
		t.Errorf("payload = %s", msg.Value)
	}
}

func TestEncodeSkipsUnpublishedEvents(t *testing.T) {
	enc := messaging.NewEncoder(messaging.Topics{Users: "users"})

	_, ok, err := enc.Encode(unpublishedEvent{})
	if err != nil || ok {
		t.Errorf("Encode = _, %v, %v; want _, false, nil", ok, err)
	}
}

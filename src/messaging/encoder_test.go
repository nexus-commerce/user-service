package messaging

import (
	"encoding/json"
	"testing"
	"time"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

type unroutedEvent struct {
	entity.UserRegistered
}

func newUserRegistered(t *testing.T) entity.UserRegistered {
	t.Helper()
	fields := entity.ProfileFields{Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}
	profile, err := entity.NewProfile(fields)
	if err != nil {
		t.Fatalf("NewProfile(%+v) error: %v", fields, err)
	}
	return entity.UserRegistered{
		ID:          uuid.New(),
		AggregateID: uuid.New(),
		OccurredAt:  time.Date(2026, 10, 14, 19, 30, 0, 0, time.UTC),
		Profile:     profile,
	}
}

func TestEncodeUserRegistered(t *testing.T) {
	enc := newEncoder()
	e := newUserRegistered(t)

	msg, err := enc.encode(e)
	if err != nil {
		t.Fatalf("Encode = _, %v; want _, nil", err)
	}
	type header struct{ ID, Topic, Key string }
	gotHeader := header{msg.ID, msg.Topic, msg.Key}
	wantHeader := header{e.ID.String(), "users", e.AggregateID.String()}
	if gotHeader != wantHeader {
		t.Errorf("encode(UserRegistered) header = %+v, want %+v", gotHeader, wantHeader)
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
		t.Errorf("encode(UserRegistered) payload = %+v, want %+v", got, want)
	}
}

func TestEncodeRejectsUnroutedEvent(t *testing.T) {
	enc := newEncoder()

	if _, err := enc.encode(unroutedEvent{newUserRegistered(t)}); err == nil {
		t.Error("Encode(unroutedEvent) = _, nil; want error")
	}
}

func TestEncodeUserProfileChanged(t *testing.T) {
	enc := newEncoder()
	reg := newUserRegistered(t)
	e := entity.UserProfileChanged{
		ID:          uuid.New(),
		AggregateID: reg.AggregateID,
		OccurredAt:  time.Date(2026, 10, 20, 10, 15, 0, 0, time.UTC),
		Profile:     reg.Profile,
	}

	msg, err := enc.encode(e)
	if err != nil {
		t.Fatalf("Encode = _, %v; want _, nil", err)
	}
	type header struct{ ID, Topic, Key string }
	gotHeader := header{msg.ID, msg.Topic, msg.Key}
	wantHeader := header{e.ID.String(), "users", e.AggregateID.String()}
	if gotHeader != wantHeader {
		t.Errorf("encode(UserProfileChanged) header = %+v, want %+v", gotHeader, wantHeader)
	}

	type payload struct {
		ID      string `json:"eventId"`
		Type    string `json:"eventType"`
		Version string `json:"version"`
		Data    struct {
			UserID    string    `json:"userId"`
			Email     string    `json:"email"`
			ChangedAt time.Time `json:"changedAt"`
		} `json:"data"`
	}
	var got payload
	if err := json.Unmarshal(msg.Value, &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error: %v", msg.Value, err)
	}
	want := payload{ID: e.ID.String(), Type: "user.profile_changed", Version: "1.0"}
	want.Data.UserID = e.AggregateID.String()
	want.Data.Email = "jane@example.com"
	want.Data.ChangedAt = e.OccurredAt
	if got != want {
		t.Errorf("encode(UserProfileChanged) payload = %+v, want %+v", got, want)
	}
}

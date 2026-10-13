package publisher

import (
	"time"
	"uuid"
)

type eventType string

type event struct {
	ID        string    `json:"eventId"`
	Type      eventType `json:"eventType"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Data      any       `json:"data"`
}

func newEvent(typ eventType, version string, payload any) *event {
	return &event{
		ID:        uuid.New().String(),
		Type:      typ,
		Timestamp: time.Now().UTC(),
		Version:   version,
		Data:      payload,
	}
}

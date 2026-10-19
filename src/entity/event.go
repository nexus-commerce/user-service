package entity

import (
	"time"
	"uuid"
)

type EventType string

const EventUserRegistered EventType = "UserRegistered"

type Event interface {
	EventType() EventType
}

type UserRegistered struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Profile    Profile
	OccurredAt time.Time
}

func (UserRegistered) EventType() EventType {
	return EventUserRegistered
}

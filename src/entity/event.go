package entity

import (
	"time"
	"uuid"
)

type Event interface {
	event()
}

type UserRegistered struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Profile    Profile
	OccurredAt time.Time
}

func (UserRegistered) event() {}

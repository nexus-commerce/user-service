package user

import (
	"time"
	"uuid"
)

type EventMeta struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	OccurredAt  time.Time
}

func (m EventMeta) Meta() EventMeta {
	return m
}

type Event interface {
	Meta() EventMeta
	isEvent()
}

type UserRegistered struct {
	EventMeta
	Profile Profile
}

func (UserRegistered) isEvent() {}

type UserProfileChanged struct {
	EventMeta
	Profile Profile
}

func (UserProfileChanged) isEvent() {}

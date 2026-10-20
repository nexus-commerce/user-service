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

type Registered struct {
	EventMeta
	Profile Profile
}

func (Registered) isEvent() {}

type Renamed struct {
	EventMeta
	FirstName Name
	LastName  Name
}

func (Renamed) isEvent() {}

type EmailChangeRequested struct {
	EventMeta
	NewEmail  Email
	Token     EmailChangeToken
	ExpiresAt time.Time
}

func (EmailChangeRequested) isEvent() {}

type EmailChanged struct {
	EventMeta
	OldEmail Email
	NewEmail Email
}

func (EmailChanged) isEvent() {}

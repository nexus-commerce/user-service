package entity

import (
	"time"
	"uuid"
)

type EventType string

const EventUserRegistered EventType = "UserRegistered"

type EventMeta struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	OccurredAt  time.Time
}

func (m EventMeta) Meta() EventMeta {
	return m
}

type Event interface {
	EventType() EventType
	Meta() EventMeta
}

type UserRegistered struct {
	EventMeta
	Profile Profile
}

func (UserRegistered) EventType() EventType {
	return EventUserRegistered
}

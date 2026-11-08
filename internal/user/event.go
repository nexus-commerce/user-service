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

func newEventMeta(aggregateID uuid.UUID, at time.Time) EventMeta {
	return EventMeta{ID: uuid.New(), AggregateID: aggregateID, OccurredAt: at}
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
	Email    Email
	FullName FullName
}

func (Registered) isEvent() {}

type RegistrationRequested struct {
	EventMeta
	Email     Email
	FullName  FullName
	Token     ConfirmationToken
	ExpiresAt time.Time
}

func (RegistrationRequested) isEvent() {}

type RegistrationConfirmed struct {
	EventMeta
}

func (RegistrationConfirmed) isEvent() {}

type RegistrationRejected struct {
	EventMeta
	Email Email
}

func (RegistrationRejected) isEvent() {}

type DuplicateRegistrationAttempted struct {
	EventMeta
	FullName FullName
	Email    Email
}

func (DuplicateRegistrationAttempted) isEvent() {}

type Renamed struct {
	EventMeta
	FullName FullName
}

func (Renamed) isEvent() {}

type EmailChangeRequested struct {
	EventMeta
	FullName  FullName
	NewEmail  Email
	Token     ConfirmationToken
	ExpiresAt time.Time
}

func (EmailChangeRequested) isEvent() {}

type EmailChangeToTakenAddressAttempted struct {
	EventMeta
	NewEmail Email
}

func (EmailChangeToTakenAddressAttempted) isEvent() {}

type EmailChanged struct {
	EventMeta
	FullName FullName
	OldEmail Email
	NewEmail Email
}

func (EmailChanged) isEvent() {}

type EmailChangeRejected struct {
	EventMeta
	NewEmail Email
}

func (EmailChangeRejected) isEvent() {}

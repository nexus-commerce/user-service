package entity

import "time"

type Event interface {
	event()
}

type UserRegistered struct {
	UserID     string
	Profile    Profile
	OccurredAt time.Time
}

func (UserRegistered) event() {}

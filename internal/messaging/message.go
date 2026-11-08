package messaging

import "context"

const HeaderEventType = "event-type"

type Message struct {
	ID    string
	Type  string
	Topic string
	Key   string
	Value []byte
}

type BatchFunc func(ctx context.Context, msgs []Message) error

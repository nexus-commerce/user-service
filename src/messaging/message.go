package messaging

import "context"

type Message struct {
	ID    string
	Topic string
	Key   string
	Value []byte
}

type BatchFunc func(ctx context.Context, msgs []Message) error

package publisher

import "context"

type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

type Producer interface {
	Publish(ctx context.Context, msgs ...Message) error
}

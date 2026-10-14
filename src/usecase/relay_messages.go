package usecase

import (
	"context"
)

// Message is an encoded event ready to be stored in the outbox and published.
type Message struct {
	Topic string
	Key   string
	Value []byte
}

type relayOutbox interface {
	Dispatch(ctx context.Context, limit int, send func(ctx context.Context, msgs []Message) error) (int, error)
}

type relayProducer interface {
	Publish(ctx context.Context, msgs ...Message) error
}

type RelayMessages struct {
	outbox   relayOutbox
	producer relayProducer
}

func NewRelayMessages(outbox relayOutbox, producer relayProducer) *RelayMessages {
	return &RelayMessages{
		outbox:   outbox,
		producer: producer,
	}
}

func (rm *RelayMessages) Execute(ctx context.Context, limit int) (int, error) {
	return rm.outbox.Dispatch(ctx, limit, func(ctx context.Context, msgs []Message) error {
		return rm.producer.Publish(ctx, msgs...)
	})
}

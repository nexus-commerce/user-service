package usecase

import (
	"context"
)

type Message struct {
	Topic string
	Key   string
	Value []byte
}

type relayOutbox interface {
	ProcessBatch(ctx context.Context, limit int, fn func(ctx context.Context, msgs []Message) error) (int, error)
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
	return rm.outbox.ProcessBatch(ctx, limit, func(ctx context.Context, msgs []Message) error {
		return rm.producer.Publish(ctx, msgs...)
	})
}

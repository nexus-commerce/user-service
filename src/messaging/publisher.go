package messaging

import (
	"context"
	"fmt"

	"github.com/nexus-commerce/user-service/src/user"
)

type Producer interface {
	Produce(ctx context.Context, msgs ...Message) error
}

type Publisher struct {
	encoder *encoder
}

func NewPublisher(sealer Sealer) (*Publisher, error) {
	enc, err := newEncoder(sealer)
	if err != nil {
		return nil, fmt.Errorf("create encoder: %w", err)
	}
	return &Publisher{encoder: enc}, nil
}

func (p *Publisher) Publish(ctx context.Context, pr Producer, events []user.Event) error {
	if len(events) == 0 {
		return nil
	}
	msgs := make([]Message, 0, len(events))
	for _, e := range events {
		msg, err := p.encoder.encode(e)
		if err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
		msgs = append(msgs, msg)
	}
	return pr.Produce(ctx, msgs...)
}

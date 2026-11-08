package userevent

import (
	"context"
	"fmt"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/user"
)

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

func (p *Publisher) Publish(ctx context.Context, pr messaging.Producer, events []user.Event) error {
	if len(events) == 0 {
		return nil
	}
	msgs := make([]messaging.Message, 0, len(events))
	for _, e := range events {
		msg, err := p.encoder.encode(e)
		if err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
		msgs = append(msgs, msg)
	}
	return pr.Produce(ctx, msgs...)
}

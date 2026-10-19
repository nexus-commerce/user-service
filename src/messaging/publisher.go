package messaging

import (
	"context"
	"fmt"

	"github.com/nexus-commerce/user-service/src/entity"
)

type Producer interface {
	Produce(ctx context.Context, msgs ...Message) error
}

type Publisher struct {
	encoder *Encoder
}

func NewPublisher(encoder *Encoder) *Publisher {
	return &Publisher{encoder: encoder}
}

func (p *Publisher) Publish(ctx context.Context, pr Producer, events []entity.Event) error {
	if len(events) == 0 {
		return nil
	}
	msgs := make([]Message, 0, len(events))
	for _, e := range events {
		msg, err := p.encoder.Encode(e)
		if err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
		msgs = append(msgs, msg)
	}
	return pr.Produce(ctx, msgs...)
}

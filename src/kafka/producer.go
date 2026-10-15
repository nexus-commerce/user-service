package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/nexus-commerce/user-service/src/messaging"
)

const batchTimeout = 10 * time.Millisecond

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(addr string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(addr),
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			BatchTimeout: batchTimeout,
		},
	}
}

func (p *Producer) Publish(ctx context.Context, msgs ...messaging.Message) error {
	kmsgs := make([]kafka.Message, len(msgs))
	for i, m := range msgs {
		kmsgs[i] = kafka.Message{
			Topic: m.Topic,
			Key:   []byte(m.Key),
			Value: m.Value,
		}
	}
	if err := p.writer.WriteMessages(ctx, kmsgs...); err != nil {
		return fmt.Errorf("publish %d messages: %w", len(kmsgs), err)
	}
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

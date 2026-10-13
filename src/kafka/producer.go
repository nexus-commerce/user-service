package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/nexus-commerce/user-service/src/publisher"
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

func (p *Producer) Publish(ctx context.Context, msgs ...publisher.Message) error {
	kmsgs := make([]kafka.Message, 0, len(msgs))
	for _, m := range msgs {
		kmsgs = append(kmsgs, kafka.Message{
			Topic:   m.Topic,
			Key:     m.Key,
			Value:   m.Value,
			Headers: toHeaders(m.Headers),
		})
	}
	return p.writer.WriteMessages(ctx, kmsgs...)
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func toHeaders(h map[string]string) []kafka.Header {
	if len(h) == 0 {
		return nil
	}
	headers := make([]kafka.Header, 0, len(h))
	for k, v := range h {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	return headers
}

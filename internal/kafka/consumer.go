package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/segmentio/kafka-go"

	"github.com/nexus-commerce/user-service/internal/messaging"
)

var ErrPermanent = errors.New("permanent message failure")

type Handler interface {
	EventType() string
	Handle(ctx context.Context, msg messaging.Message) error
}

type ConsumerOptions struct {
	Addr    string
	GroupID string
	Topic   string
	Logger  *slog.Logger
}

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type Consumer struct {
	reader  messageReader
	router  router
	retry   retrier
	groupID string
	topic   string
	logger  *slog.Logger
}

func NewConsumer(opts ConsumerOptions, handlers ...Handler) (*Consumer, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	r, err := newRouter(logger, handlers...)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{opts.Addr},
			GroupID: opts.GroupID,
			Topic:   opts.Topic,
		}),
		router:  r,
		retry:   retrier{logger: logger, retryable: temporary},
		groupID: opts.GroupID,
		topic:   opts.Topic,
		logger:  logger,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("starting kafka consumer", "group", c.groupID, "topic", c.topic)

	for {
		var km kafka.Message
		err := c.retry.do(ctx, "fetch message", func() error {
			var err error
			km, err = c.reader.FetchMessage(ctx)
			return err
		})
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("stopping kafka consumer")
				return nil
			}
			return fmt.Errorf("fetch message: %w", err)
		}

		if err = c.router.route(ctx, toMessage(km)); err != nil {
			if ctx.Err() != nil {
				c.logger.Info("stopping kafka consumer")
				return nil
			}
			return err
		}

		err = c.retry.do(ctx, "commit message", func() error {
			return c.reader.CommitMessages(ctx, km)
		})
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("stopping kafka consumer")
				return nil
			}
			return fmt.Errorf("commit message %d/%d: %w", km.Partition, km.Offset, err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func toMessage(km kafka.Message) messaging.Message {
	msg := messaging.Message{
		Topic: km.Topic,
		Key:   string(km.Key),
		Value: km.Value,
	}
	for _, h := range km.Headers {
		if h.Key == messaging.HeaderEventType {
			msg.Type = string(h.Value)
		}
	}
	return msg
}

func temporary(err error) bool {
	if kerr, ok := errors.AsType[kafka.Error](err); ok {
		return kerr.Temporary()
	}
	_, ok := errors.AsType[net.Error](err)
	return ok
}

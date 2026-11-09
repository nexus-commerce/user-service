package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/nexus-commerce/user-service/internal/messaging"
)

const (
	minRetryDelay = 100 * time.Millisecond
	maxRetryDelay = 30 * time.Second
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
		groupID: opts.GroupID,
		topic:   opts.Topic,
		logger:  logger,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("starting kafka consumer", "group", c.groupID, "topic", c.topic)

	for {
		var km kafka.Message
		err := retry(ctx, c.logger, "fetch message", temporary, func() error {
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

		err = retry(ctx, c.logger, "commit message", temporary, func() error {
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

type router struct {
	handlers map[string]Handler
	logger   *slog.Logger
}

func newRouter(logger *slog.Logger, handlers ...Handler) (router, error) {
	r := router{handlers: make(map[string]Handler, len(handlers)), logger: logger}
	for _, h := range handlers {
		if _, ok := r.handlers[h.EventType()]; ok {
			return router{}, fmt.Errorf("duplicate handler for event type %q", h.EventType())
		}
		r.handlers[h.EventType()] = h
	}
	return r, nil
}

func (r router) route(ctx context.Context, msg messaging.Message) error {
	h, ok := r.handlers[msg.Type]
	if !ok {
		return nil
	}

	op := fmt.Sprintf("handle %q message with key %q", msg.Type, msg.Key)
	err := retry(ctx, r.logger, op, func(err error) bool { return !errors.Is(err, ErrPermanent) }, func() error {
		return h.Handle(ctx, msg)
	})
	if errors.Is(err, ErrPermanent) {
		r.logger.Warn("skip message", "type", msg.Type, "key", msg.Key, "err", err)
		return nil
	}
	return err
}

func temporary(err error) bool {
	if kerr, ok := errors.AsType[kafka.Error](err); ok {
		return kerr.Temporary()
	}
	_, ok := errors.AsType[net.Error](err)
	return ok
}

func retry(ctx context.Context, logger *slog.Logger, op string, retryable func(error) bool, fn func() error) error {
	delay := minRetryDelay
	for {
		err := fn()
		if err == nil || !retryable(err) {
			return err
		}
		logger.Warn("retrying", "op", op, "delay", delay, "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = min(delay*2, maxRetryDelay)
	}
}

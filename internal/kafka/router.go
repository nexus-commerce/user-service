package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nexus-commerce/user-service/internal/messaging"
)

type router struct {
	handlers map[string]Handler
	logger   *slog.Logger
	retry    retrier
}

func newRouter(logger *slog.Logger, handlers ...Handler) (router, error) {
	r := router{
		handlers: make(map[string]Handler, len(handlers)),
		logger:   logger,
		retry:    retrier{logger: logger, retryable: notPermanent},
	}
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
	err := r.retry.do(ctx, op, func() error {
		return h.Handle(ctx, msg)
	})
	if errors.Is(err, ErrPermanent) {
		r.logger.Warn("skip message", "type", msg.Type, "key", msg.Key, "err", err)
		return nil
	}
	return err
}

func notPermanent(err error) bool {
	return !errors.Is(err, ErrPermanent)
}

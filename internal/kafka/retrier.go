package kafka

import (
	"context"
	"log/slog"
	"time"
)

const (
	minRetryDelay = 100 * time.Millisecond
	maxRetryDelay = 30 * time.Second
)

type retrier struct {
	logger    *slog.Logger
	retryable func(error) bool
}

func (r retrier) do(ctx context.Context, op string, fn func() error) error {
	delay := minRetryDelay
	for {
		err := fn()
		if err == nil || !r.retryable(err) {
			return err
		}
		r.logger.Warn("retrying", "op", op, "delay", delay, "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = min(delay*2, maxRetryDelay)
	}
}

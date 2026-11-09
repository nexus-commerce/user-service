package relay

import (
	"context"
	"log/slog"
	"time"

	"github.com/nexus-commerce/user-service/internal/messaging"
)

type Outbox interface {
	ProcessBatch(ctx context.Context, limit int, fn messaging.BatchFunc) (int, error)
}

type Options struct {
	Interval  time.Duration
	BatchSize int
	Logger    *slog.Logger
}

type Relay struct {
	outbox   Outbox
	producer messaging.Producer
	opts     Options
	logger   *slog.Logger
}

func New(outbox Outbox, producer messaging.Producer, opts Options) *Relay {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Relay{outbox: outbox, producer: producer, opts: opts, logger: logger}
}

func (r *Relay) Run(ctx context.Context) error {
	r.logger.Info("starting outbox relay")

	ticker := time.NewTicker(r.opts.Interval)
	defer ticker.Stop()

	for {
		r.flush(ctx)

		select {
		case <-ctx.Done():
			r.logger.Info("stopping outbox relay")
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Relay) flush(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := r.processBatch(ctx)
		if err != nil {
			r.logger.Error("relay outbox messages", "err", err)
			return
		}
		if n < r.opts.BatchSize {
			return
		}
	}
}

func (r *Relay) processBatch(ctx context.Context) (int, error) {
	return r.outbox.ProcessBatch(ctx, r.opts.BatchSize, r.produce)
}

func (r *Relay) produce(ctx context.Context, msgs []messaging.Message) error {
	return r.producer.Produce(ctx, msgs...)
}

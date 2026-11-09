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

type Worker struct {
	outbox   Outbox
	producer messaging.Producer
	opts     Options
	logger   *slog.Logger
}

func NewWorker(outbox Outbox, producer messaging.Producer, opts Options) *Worker {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Worker{outbox: outbox, producer: producer, opts: opts, logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("starting outbox relay")

	ticker := time.NewTicker(w.opts.Interval)
	defer ticker.Stop()

	for {
		w.flush(ctx)

		select {
		case <-ctx.Done():
			w.logger.Info("stopping outbox relay")
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) flush(ctx context.Context) {
	for ctx.Err() == nil {
		n, err := w.processBatch(ctx)
		if err != nil {
			w.logger.Error("relay outbox messages", "err", err)
			return
		}
		if n < w.opts.BatchSize {
			return
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) (int, error) {
	return w.outbox.ProcessBatch(ctx, w.opts.BatchSize, w.produce)
}

func (w *Worker) produce(ctx context.Context, msgs []messaging.Message) error {
	return w.producer.Produce(ctx, msgs...)
}

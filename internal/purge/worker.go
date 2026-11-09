package purge

import (
	"context"
	"log/slog"
	"time"
)

type abandonedRegistrations interface {
	DeleteAbandonedRegistrations(ctx context.Context, before time.Time) (int64, error)
}

type expiredEmailChanges interface {
	DeleteExpiredEmailChanges(ctx context.Context, before time.Time) (int64, error)
}

type Options struct {
	Interval time.Duration
	Grace    time.Duration
	Logger   *slog.Logger
}

type Worker struct {
	registrations abandonedRegistrations
	users         expiredEmailChanges
	opts          Options
	logger        *slog.Logger
}

func NewWorker(registrations abandonedRegistrations, users expiredEmailChanges, opts Options) *Worker {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Worker{registrations: registrations, users: users, opts: opts, logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("starting purge")

	ticker := time.NewTicker(w.opts.Interval)
	defer ticker.Stop()

	for {
		w.purge(ctx)

		select {
		case <-ctx.Done():
			w.logger.Info("stopping purge")
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) purge(ctx context.Context) {
	before := time.Now().Add(-w.opts.Grace)
	w.runDeletion(ctx, "abandoned registrations", before, w.registrations.DeleteAbandonedRegistrations)
	w.runDeletion(ctx, "expired email changes", before, w.users.DeleteExpiredEmailChanges)
}

func (w *Worker) runDeletion(ctx context.Context, what string, before time.Time, fn func(context.Context, time.Time) (int64, error)) {
	n, err := fn(ctx, before)
	if err != nil {
		w.logger.Error("purge failed", "what", what, "err", err)
		return
	}
	if n > 0 {
		w.logger.Info("purged", "what", what, "count", n)
	}
}

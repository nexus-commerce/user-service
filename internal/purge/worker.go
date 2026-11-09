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

func (p *Worker) Run(ctx context.Context) error {
	p.logger.Info("starting purge")

	ticker := time.NewTicker(p.opts.Interval)
	defer ticker.Stop()

	for {
		p.purge(ctx)

		select {
		case <-ctx.Done():
			p.logger.Info("stopping purge")
			return nil
		case <-ticker.C:
		}
	}
}

func (p *Worker) purge(ctx context.Context) {
	before := time.Now().Add(-p.opts.Grace)
	p.runDeletion(ctx, "abandoned registrations", before, p.registrations.DeleteAbandonedRegistrations)
	p.runDeletion(ctx, "expired email changes", before, p.users.DeleteExpiredEmailChanges)
}

func (p *Worker) runDeletion(ctx context.Context, what string, before time.Time, fn func(context.Context, time.Time) (int64, error)) {
	n, err := fn(ctx, before)
	if err != nil {
		p.logger.Error("purge failed", "what", what, "err", err)
		return
	}
	if n > 0 {
		p.logger.Info("purged", "what", what, "count", n)
	}
}

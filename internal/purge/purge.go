package purge

import (
	"context"
	"log"
	"time"
)

type abandonedRegistrations interface {
	DeleteAbandoned(ctx context.Context, before time.Time) (int64, error)
}

type expiredEmailChanges interface {
	DeleteExpiredEmailChanges(ctx context.Context, before time.Time) (int64, error)
}

type Options struct {
	Interval time.Duration
	Grace    time.Duration
}

type Purger struct {
	registrations abandonedRegistrations
	users         expiredEmailChanges
	opts          Options
}

func New(registrations abandonedRegistrations, users expiredEmailChanges, opts Options) *Purger {
	return &Purger{registrations: registrations, users: users, opts: opts}
}

func (p *Purger) Run(ctx context.Context) error {
	log.Println("Starting purge")

	ticker := time.NewTicker(p.opts.Interval)
	defer ticker.Stop()

	for {
		p.purge(ctx)

		select {
		case <-ctx.Done():
			log.Println("Stopping purge")
			return nil
		case <-ticker.C:
		}
	}
}

func (p *Purger) purge(ctx context.Context) {
	before := time.Now().Add(-p.opts.Grace)
	p.delete(ctx, "abandoned registrations", before, p.registrations.DeleteAbandoned)
	p.delete(ctx, "expired email changes", before, p.users.DeleteExpiredEmailChanges)
}

func (p *Purger) delete(ctx context.Context, what string, before time.Time, fn func(context.Context, time.Time) (int64, error)) {
	n, err := fn(ctx, before)
	if err != nil {
		log.Printf("purge %s: %v", what, err)
		return
	}
	if n > 0 {
		log.Printf("Purged %d %s", n, what)
	}
}

package purge

import (
	"context"
	"log"
	"time"
)

type purgeRegistrations interface {
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

type purgeUsers interface {
	DeleteExpiredEmailChanges(ctx context.Context, before time.Time) (int64, error)
}

type Options struct {
	Interval time.Duration
	Grace    time.Duration
}

type Purger struct {
	registrations purgeRegistrations
	users         purgeUsers
	opts          Options
}

func New(registrations purgeRegistrations, users purgeUsers, opts Options) *Purger {
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
	p.delete(ctx, "registrations", before, p.registrations.DeleteExpired)
	p.delete(ctx, "email changes", before, p.users.DeleteExpiredEmailChanges)
}

func (p *Purger) delete(ctx context.Context, what string, before time.Time, fn func(context.Context, time.Time) (int64, error)) {
	n, err := fn(ctx, before)
	if err != nil {
		log.Printf("purge expired %s: %v", what, err)
		return
	}
	if n > 0 {
		log.Printf("Purged %d expired %s", n, what)
	}
}

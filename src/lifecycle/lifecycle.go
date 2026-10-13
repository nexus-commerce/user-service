package lifecycle

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Runner interface {
	Run(ctx context.Context) error
}

func Run(ctx context.Context, runners ...Runner) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, r := range runners {
		g.Go(func() error { return r.Run(ctx) })
	}
	return g.Wait()
}

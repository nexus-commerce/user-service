package purge_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nexus-commerce/user-service/src/purge"
)

type fakeRegistrations struct {
	purged []time.Time
	err    error
}

func (f *fakeRegistrations) DeleteExpired(_ context.Context, before time.Time) (int64, error) {
	f.purged = append(f.purged, before)
	return 0, f.err
}

type fakeUsers struct {
	purged []time.Time
}

func (f *fakeUsers) DeleteExpiredEmailChanges(_ context.Context, before time.Time) (int64, error) {
	f.purged = append(f.purged, before)
	return 0, nil
}

func TestRunPurgesOnStartAndEveryInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registrations, users := &fakeRegistrations{}, &fakeUsers{}
		opts := purge.Options{Interval: time.Hour, Grace: 7 * 24 * time.Hour}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)
		start := time.Now()

		go func() { done <- purge.New(registrations, users, opts).Run(ctx) }()
		synctest.Wait()
		time.Sleep(opts.Interval)
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		want := []time.Time{start.Add(-opts.Grace), start.Add(opts.Interval - opts.Grace)}
		for name, got := range map[string][]time.Time{"registrations": registrations.purged, "email changes": users.purged} {
			if len(got) != len(want) || !got[0].Equal(want[0]) || !got[1].Equal(want[1]) {
				t.Errorf("%s purged with before = %v, want %v", name, got, want)
			}
		}
	})
}

func TestRunContinuesAfterError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registrations, users := &fakeRegistrations{err: errors.New("boom")}, &fakeUsers{}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)

		go func() { done <- purge.New(registrations, users, purge.Options{Interval: time.Hour}).Run(ctx) }()
		synctest.Wait()
		time.Sleep(time.Hour)
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if len(registrations.purged) != 2 || len(users.purged) != 2 {
			t.Errorf("purged registrations %d times, email changes %d times; want 2 each", len(registrations.purged), len(users.purged))
		}
	})
}

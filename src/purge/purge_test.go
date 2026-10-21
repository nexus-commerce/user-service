package purge_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/nexus-commerce/user-service/src/purge"
)

type fakeStore struct {
	registrations    []time.Time
	emailChanges     []time.Time
	registrationsErr error
}

func (f *fakeStore) DeleteExpiredRegistrations(_ context.Context, before time.Time) (int64, error) {
	f.registrations = append(f.registrations, before)
	return 0, f.registrationsErr
}

func (f *fakeStore) DeleteExpiredEmailChanges(_ context.Context, before time.Time) (int64, error) {
	f.emailChanges = append(f.emailChanges, before)
	return 0, nil
}

func TestRunPurgesOnStartAndEveryInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &fakeStore{}
		opts := purge.Options{Interval: time.Hour, Grace: 7 * 24 * time.Hour}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)
		start := time.Now()

		go func() { done <- purge.New(store, opts).Run(ctx) }()
		synctest.Wait()
		time.Sleep(opts.Interval)
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		want := []time.Time{start.Add(-opts.Grace), start.Add(opts.Interval - opts.Grace)}
		for name, got := range map[string][]time.Time{"registrations": store.registrations, "email changes": store.emailChanges} {
			if len(got) != len(want) || !got[0].Equal(want[0]) || !got[1].Equal(want[1]) {
				t.Errorf("%s purged with before = %v, want %v", name, got, want)
			}
		}
	})
}

func TestRunContinuesAfterError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &fakeStore{registrationsErr: errors.New("boom")}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error)

		go func() { done <- purge.New(store, purge.Options{Interval: time.Hour}).Run(ctx) }()
		synctest.Wait()
		time.Sleep(time.Hour)
		synctest.Wait()
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if len(store.registrations) != 2 || len(store.emailChanges) != 2 {
			t.Errorf("purged registrations %d times, email changes %d times; want 2 each", len(store.registrations), len(store.emailChanges))
		}
	})
}

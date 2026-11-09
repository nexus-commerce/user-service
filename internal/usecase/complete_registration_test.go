package usecase_test

import (
	"errors"
	"testing"
	"uuid"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

func confirmedJane(t *testing.T) (*fakeUsers, *fakeRegistrations, uuid.UUID) {
	t.Helper()
	users, regs, tok := registeredJane(t)
	p, err := regs.UpdateByToken(t.Context(), tok.Hash(), func(p *user.PendingRegistration) error {
		return p.Confirm(tok)
	})
	if err != nil {
		t.Fatalf("UpdateByToken() error: %v", err)
	}
	return users, regs, p.UserID()
}

func TestCompleteRegistrationCreatesUser(t *testing.T) {
	users, regs, id := confirmedJane(t)

	if err := usecase.NewCompleteRegistration(regs, users).Execute(t.Context(), id); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	u, err := users.ByID(t.Context(), id)
	if err != nil {
		t.Fatalf("ByID() error: %v", err)
	}
	if n := len(publishedOf[user.Registered](u.Events())); n != 1 {
		t.Errorf("user has %d Registered events, want 1", n)
	}
	if len(regs.byEmail) != 0 {
		t.Errorf("%d pending registrations left, want 0", len(regs.byEmail))
	}
}

func TestCompleteRegistrationIsIdempotent(t *testing.T) {
	users, regs, id := confirmedJane(t)
	complete := usecase.NewCompleteRegistration(regs, users)

	for i := range 2 {
		if err := complete.Execute(t.Context(), id); err != nil {
			t.Fatalf("Execute() #%d error: %v", i+1, err)
		}
	}
	if len(users.byID) != 1 || users.addCalls != 1 {
		t.Errorf("users = %d, Add calls = %d; want 1, 1", len(users.byID), users.addCalls)
	}
}

func TestCompleteRegistrationRetriesAfterRemoveFailure(t *testing.T) {
	users, regs, id := confirmedJane(t)
	complete := usecase.NewCompleteRegistration(regs, users)
	want := errors.New("remove failed")
	regs.removeErr = want

	if err := complete.Execute(t.Context(), id); !errors.Is(err, want) {
		t.Fatalf("first Execute() error = %v, want %v", err, want)
	}
	regs.removeErr = nil
	if err := complete.Execute(t.Context(), id); err != nil {
		t.Fatalf("retried Execute() error: %v", err)
	}
	if len(users.byID) != 1 || len(regs.byEmail) != 0 {
		t.Errorf("users = %d, registrations = %d; want 1, 0", len(users.byID), len(regs.byEmail))
	}
}

func TestCompleteRegistrationMissingRegistration(t *testing.T) {
	users, regs := &fakeUsers{}, &fakeRegistrations{}
	if err := usecase.NewCompleteRegistration(regs, users).Execute(t.Context(), uuid.New()); err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if users.addCalls != 0 {
		t.Errorf("Add calls = %d, want 0", users.addCalls)
	}
}

func TestCompleteRegistrationUnconfirmed(t *testing.T) {
	users, regs, _ := registeredJane(t)
	id := pendingJane(t, regs).UserID()

	if err := usecase.NewCompleteRegistration(regs, users).Execute(t.Context(), id); !errors.Is(err, user.ErrRegistrationNotConfirmed) {
		t.Errorf("Execute() error = %v, want %v", err, user.ErrRegistrationNotConfirmed)
	}
	if len(users.byID) != 0 || len(regs.byEmail) != 1 {
		t.Errorf("users = %d, registrations = %d; want 0, 1", len(users.byID), len(regs.byEmail))
	}
}

func TestCompleteRegistrationEmailTaken(t *testing.T) {
	users, regs, id := confirmedJane(t)
	newUser(t, users, "jane@example.com", "Other", "Jane")

	if err := usecase.NewCompleteRegistration(regs, users).Execute(t.Context(), id); !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("Execute() error = %v, want %v", err, user.ErrEmailTaken)
	}
	if _, err := users.ByID(t.Context(), id); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("ByID() error = %v, want %v", err, user.ErrNotFound)
	}
	if len(regs.byEmail) != 0 {
		t.Errorf("Execute() left %d pending registrations, want 0", len(regs.byEmail))
	}
	rejected := publishedOf[user.RegistrationRejected](regs.published)
	want := []user.RegistrationRejected{{AggregateID: id, Email: mustEmail(t, "jane@example.com")}}
	if diff := cmp.Diff(want, rejected, valueObjects, ignoreEventMeta); diff != "" {
		t.Errorf("Execute() RegistrationRejected events mismatch (-want +got):\n%s", diff)
	}
}

func TestCompleteRegistrationRejectFailure(t *testing.T) {
	users, regs, id := confirmedJane(t)
	newUser(t, users, "jane@example.com", "Other", "Jane")
	want := errors.New("reject failed")
	regs.removeErr = want

	if err := usecase.NewCompleteRegistration(regs, users).Execute(t.Context(), id); !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
	if len(regs.byEmail) != 1 {
		t.Errorf("%d pending registrations left, want 1", len(regs.byEmail))
	}
}

func TestCompleteRegistrationAddFailureKeepsRegistration(t *testing.T) {
	users, regs, id := confirmedJane(t)
	want := errors.New("users unavailable")
	users.addErr = want

	if err := usecase.NewCompleteRegistration(regs, users).Execute(t.Context(), id); !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
	if !pendingJane(t, regs).Confirmed() {
		t.Error("confirmed registration lost after Add failure")
	}
}

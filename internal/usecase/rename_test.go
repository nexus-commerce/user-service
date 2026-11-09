package usecase_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

func TestRenameKeepsOmittedName(t *testing.T) {
	u, users := newJane(t)
	first := "Janet"
	view, err := usecase.NewRename(users).Execute(t.Context(), usecase.NameChange{UserID: u.ID(), FirstName: &first})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	want := usecase.UserView{ID: u.ID(), Email: "jane@example.com", FirstName: "Janet", LastName: "Doe"}
	if diff := cmp.Diff(want, view); diff != "" {
		t.Errorf("Execute() view mismatch (-want +got):\n%s", diff)
	}
}

func TestRenameRejectsInvalidName(t *testing.T) {
	u, users := newJane(t)
	empty := ""
	_, err := usecase.NewRename(users).Execute(t.Context(), usecase.NameChange{UserID: u.ID(), LastName: &empty})
	if !errors.Is(err, user.ErrEmptyName) {
		t.Errorf("Execute() error = %v, want %v", err, user.ErrEmptyName)
	}
	if fe, ok := errors.AsType[*usecase.FieldError](err); !ok || fe.Field != usecase.FieldLastName {
		t.Errorf("Execute() error = %v, want FieldError for %v", err, usecase.FieldLastName)
	}
}

package usecase_test

import (
	"errors"
	"testing"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

func TestRenameKeepsOmittedName(t *testing.T) {
	usr, users := newJane(t)
	first := "Janet"
	view, err := usecase.NewRename(users).Execute(t.Context(), usecase.NameChange{UserID: usr.ID(), FirstName: &first})
	if err != nil || view.FirstName != "Janet" || view.LastName != "Doe" {
		t.Errorf("Execute() = %+v, %v; want Janet Doe, nil", view, err)
	}
}

func TestRenameRejectsInvalidName(t *testing.T) {
	usr, users := newJane(t)
	empty := ""
	_, err := usecase.NewRename(users).Execute(t.Context(), usecase.NameChange{UserID: usr.ID(), LastName: &empty})
	if fe, ok := errors.AsType[*user.FieldError](err); !ok || fe.Field != "lastName" || !errors.Is(err, user.ErrEmptyName) {
		t.Errorf("Execute() error = %v, want FieldError lastName / %v", err, user.ErrEmptyName)
	}
}

package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type renameUsers interface {
	Update(ctx context.Context, id uuid.UUID, updateFn func(*user.User) error) (*user.User, error)
}

type Rename struct {
	users renameUsers
}

func NewRename(users renameUsers) *Rename {
	return &Rename{
		users: users,
	}
}

type NameChange struct {
	UserID    uuid.UUID
	FirstName *string
	LastName  *string
}

func (r *Rename) Execute(ctx context.Context, c NameChange) (UserView, error) {
	first, err := parseName(c.FirstName, FieldFirstName)
	if err != nil {
		return UserView{}, err
	}
	last, err := parseName(c.LastName, FieldLastName)
	if err != nil {
		return UserView{}, err
	}

	usr, err := r.users.Update(ctx, c.UserID, func(usr *user.User) error {
		if first == nil && last == nil {
			return nil
		}
		usr.Rename(valueOr(first, usr.FirstName()), valueOr(last, usr.LastName()))
		return nil
	})
	if err != nil {
		return UserView{}, err
	}
	return toUserView(usr), nil
}

func parseName(s *string, field Field) (*user.Name, error) {
	if s == nil {
		return nil, nil
	}
	n, err := user.NewName(*s)
	if err != nil {
		return nil, &FieldError{Field: field, Err: err}
	}
	return &n, nil
}

func valueOr[T any](v *T, fallback T) T {
	if v == nil {
		return fallback
	}
	return *v
}

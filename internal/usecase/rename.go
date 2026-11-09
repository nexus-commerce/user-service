package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/user"
)

type renameUsers interface {
	UpdateByID(ctx context.Context, id uuid.UUID, update user.Update) (*user.User, error)
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

func (r *Rename) Execute(ctx context.Context, change NameChange) (UserView, error) {
	first, err := parseName(change.FirstName, FieldFirstName)
	if err != nil {
		return UserView{}, err
	}
	last, err := parseName(change.LastName, FieldLastName)
	if err != nil {
		return UserView{}, err
	}

	renamed, err := r.users.UpdateByID(ctx, change.UserID, func(u *user.User) error {
		name := u.FullName()
		if first != nil {
			name = name.WithFirstName(*first)
		}
		if last != nil {
			name = name.WithLastName(*last)
		}
		u.Rename(name)
		return nil
	})
	if err != nil {
		return UserView{}, err
	}
	return toUserView(renamed), nil
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

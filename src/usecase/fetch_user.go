package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type fetchUserUsers interface {
	ByID(ctx context.Context, id uuid.UUID) (*user.User, error)
}

type FetchUser struct {
	users fetchUserUsers
}

func NewFetchUser(users fetchUserUsers) *FetchUser {
	return &FetchUser{
		users: users,
	}
}

type UserView struct {
	ID        uuid.UUID
	Email     string
	FirstName string
	LastName  string
}

func toUserView(u *user.User) UserView {
	return UserView{
		ID:        u.ID(),
		Email:     u.Email().String(),
		FirstName: u.FullName().FirstName().String(),
		LastName:  u.FullName().LastName().String(),
	}
}

func (fu *FetchUser) Execute(ctx context.Context, userID uuid.UUID) (UserView, error) {
	usr, err := fu.users.ByID(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	return toUserView(usr), nil
}

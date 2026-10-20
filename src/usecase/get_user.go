package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type getUserUsers interface {
	ByID(ctx context.Context, id uuid.UUID) (*user.User, error)
}

type GetUser struct {
	users getUserUsers
}

func NewGetUser(users getUserUsers) *GetUser {
	return &GetUser{
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

func (gu *GetUser) Execute(ctx context.Context, userID uuid.UUID) (UserView, error) {
	usr, err := gu.users.ByID(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	return toUserView(usr), nil
}

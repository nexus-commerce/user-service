package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type getProfileUsers interface {
	ByID(ctx context.Context, id uuid.UUID) (*user.User, error)
}

type GetProfile struct {
	users getProfileUsers
}

func NewGetProfile(users getProfileUsers) *GetProfile {
	return &GetProfile{
		users: users,
	}
}

type ProfileView struct {
	ID        uuid.UUID
	Email     string
	FirstName string
	LastName  string
}

func toProfileView(u *user.User) ProfileView {
	return ProfileView{
		ID:        u.ID(),
		Email:     u.Email().String(),
		FirstName: u.FirstName().String(),
		LastName:  u.LastName().String(),
	}
}

func (gp *GetProfile) Execute(ctx context.Context, userID uuid.UUID) (ProfileView, error) {
	usr, err := gp.users.ByID(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	return toProfileView(usr), nil
}

package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/entity"
)

type getProfileUsers interface {
	ByID(ctx context.Context, id string) (*entity.User, error)
}

type GetProfile struct {
	users getProfileUsers
}

func NewGetProfile(users getProfileUsers) *GetProfile {
	return &GetProfile{
		users: users,
	}
}

func (gp *GetProfile) Execute(ctx context.Context, userID string) (*entity.User, error) {
	return gp.users.ByID(ctx, userID)
}

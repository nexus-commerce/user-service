package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/entity"
)

type GetProfileUsers interface {
	ByID(ctx context.Context, id string) (*entity.User, error)
}

type GetProfile struct {
	users GetProfileUsers
}

func NewGetProfile(users GetProfileUsers) *GetProfile {
	return &GetProfile{
		users: users,
	}
}

func (gp *GetProfile) Execute(ctx context.Context, userID string) (*entity.User, error) {
	return gp.users.ByID(ctx, userID)
}

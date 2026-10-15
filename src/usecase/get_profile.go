package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

type getProfileUsers interface {
	ByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
}

type GetProfile struct {
	users getProfileUsers
}

func NewGetProfile(users getProfileUsers) *GetProfile {
	return &GetProfile{
		users: users,
	}
}

func (gp *GetProfile) Execute(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	return gp.users.ByID(ctx, userID)
}

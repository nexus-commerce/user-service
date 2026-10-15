package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/entity"
)

type updateProfileUsers interface {
	ByID(ctx context.Context, id string) (*entity.User, error)
	UpdateByID(ctx context.Context, id string, updateFn func(user *entity.User) (bool, error)) (*entity.User, error)
}

type UpdateProfile struct {
	users updateProfileUsers
}

func NewUpdateProfile(users updateProfileUsers) *UpdateProfile {
	return &UpdateProfile{
		users: users,
	}
}

type ProfileUpdate struct {
	UserID    string
	FirstName *entity.Name
	LastName  *entity.Name
	Email     *entity.Email
}

func (pu ProfileUpdate) isZero() bool {
	return pu.FirstName == nil && pu.LastName == nil && pu.Email == nil
}

func (up *UpdateProfile) Execute(ctx context.Context, update ProfileUpdate) (*entity.User, error) {
	if update.isZero() {
		return up.users.ByID(ctx, update.UserID)
	}

	return up.users.UpdateByID(ctx, update.UserID, func(user *entity.User) (bool, error) {
		p := user.Profile
		if update.FirstName != nil {
			p.FirstName = *update.FirstName
		}
		if update.LastName != nil {
			p.LastName = *update.LastName
		}
		if update.Email != nil {
			p.Email = *update.Email
		}

		if err := user.ChangeProfile(p); err != nil {
			return false, err
		}
		return true, nil
	})
}

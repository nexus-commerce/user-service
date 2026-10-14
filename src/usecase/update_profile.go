package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/entity"
)

type updateProfileUsers interface {
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

func (pu ProfileUpdate) empty() bool {
	return pu.FirstName == nil && pu.LastName == nil && pu.Email == nil
}

func (up *UpdateProfile) Execute(ctx context.Context, update ProfileUpdate) (*entity.User, error) {
	return up.users.UpdateByID(ctx, update.UserID, func(user *entity.User) (bool, error) {
		if update.empty() {
			return false, nil
		}
		if update.FirstName != nil {
			user.FirstName = *update.FirstName
		}
		if update.LastName != nil {
			user.LastName = *update.LastName
		}
		if update.Email != nil {
			user.Email = *update.Email
		}

		if err := user.Profile.Validate(); err != nil {
			return false, err
		}
		return true, nil
	})
}

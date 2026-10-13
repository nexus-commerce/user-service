package usecase

import (
	"context"

	"github.com/nexus-commerce/user-service/src/entity"
)

type UpdateProfileUsers interface {
	ByID(ctx context.Context, id string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
}

type UpdateProfile struct {
	users UpdateProfileUsers
}

func NewUpdateProfile(users UpdateProfileUsers) *UpdateProfile {
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

func (up *UpdateProfile) Execute(ctx context.Context, update ProfileUpdate) (*entity.User, error) {
	user, err := up.users.ByID(ctx, update.UserID)
	if err != nil {
		return nil, err
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

	if err = user.Profile.Validate(); err != nil {
		return nil, err
	}

	if err = up.users.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

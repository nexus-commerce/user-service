package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

type updateProfileUsers interface {
	Update(ctx context.Context, id uuid.UUID, updateFn func(*entity.User) (bool, error)) (*entity.User, error)
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
	UserID    uuid.UUID
	FirstName *string
	LastName  *string
	Email     *string
}

func (pu *ProfileUpdate) toProfileChange() entity.ProfileChange {
	return entity.ProfileChange{
		Email:     pu.Email,
		FirstName: pu.FirstName,
		LastName:  pu.LastName,
	}
}

func (up *UpdateProfile) Execute(ctx context.Context, update ProfileUpdate) (ProfileView, error) {
	user, err := up.users.Update(ctx, update.UserID, func(user *entity.User) (bool, error) {
		return user.ChangeProfile(update.toProfileChange())
	})
	if err != nil {
		return ProfileView{}, err
	}
	return toProfileView(user), nil
}

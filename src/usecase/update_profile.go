package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/entity"
)

type updateProfileUsers interface {
	Update(ctx context.Context, id uuid.UUID, updateFn func(user *entity.User) (bool, error)) (*entity.User, error)
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

func (up *UpdateProfile) Execute(ctx context.Context, update ProfileUpdate) (*entity.User, error) {
	return up.users.Update(ctx, update.UserID, func(user *entity.User) (bool, error) {
		p, err := entity.NewProfile(
			valueOr(update.Email, user.Email().String()),
			valueOr(update.FirstName, user.FirstName().String()),
			valueOr(update.LastName, user.LastName().String()),
		)
		if err != nil {
			return false, err
		}

		return user.ChangeProfile(p), nil
	})
}

func valueOr(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

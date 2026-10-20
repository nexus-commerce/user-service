package usecase

import (
	"context"
	"uuid"

	"github.com/nexus-commerce/user-service/src/user"
)

type changeProfileUsers interface {
	Update(ctx context.Context, id uuid.UUID, updateFn func(*user.User) (bool, error)) (*user.User, error)
}

type ChangeProfile struct {
	users changeProfileUsers
}

func NewChangeProfile(users changeProfileUsers) *ChangeProfile {
	return &ChangeProfile{
		users: users,
	}
}

type ProfileChange struct {
	UserID    uuid.UUID
	FirstName *string
	LastName  *string
	Email     *string
}

func (pc *ProfileChange) toProfileChange() user.ProfileChange {
	return user.ProfileChange{
		Email:     pc.Email,
		FirstName: pc.FirstName,
		LastName:  pc.LastName,
	}
}

func (cp *ChangeProfile) Execute(ctx context.Context, change ProfileChange) (ProfileView, error) {
	usr, err := cp.users.Update(ctx, change.UserID, func(usr *user.User) (bool, error) {
		return usr.ChangeProfile(change.toProfileChange())
	})
	if err != nil {
		return ProfileView{}, err
	}
	return toProfileView(usr), nil
}

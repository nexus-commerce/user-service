package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type fetchUserUsersFunc func(context.Context, uuid.UUID) (*user.User, error)

func (f fetchUserUsersFunc) ByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return f(ctx, id)
}

func TestFetchUser(t *testing.T) {
	dbErr := errors.New("database unavailable")
	tests := []struct {
		name    string
		readErr error
		wantErr error
	}{
		{name: "success"},
		{name: "not found", readErr: user.ErrNotFound, wantErr: user.ErrNotFound},
		{name: "wrapped not found", readErr: fmt.Errorf("lookup: %w", user.ErrNotFound), wantErr: user.ErrNotFound},
		{name: "repository failure", readErr: dbErr, wantErr: dbErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, users := newJane(t)
			calls := 0
			repo := fetchUserUsersFunc(func(ctx context.Context, id uuid.UUID) (*user.User, error) {
				calls++
				if ctx != t.Context() || id != u.ID() {
					t.Errorf("ByID() got context %v, ID %v; want test context, %v", ctx, id, u.ID())
				}
				if tt.readErr != nil {
					return nil, tt.readErr
				}
				return users.ByID(ctx, id)
			})

			view, err := usecase.NewFetchUser(repo).Execute(t.Context(), u.ID())
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Execute() error = %v, want %v", err, tt.wantErr)
			}
			wantView := usecase.UserView{}
			if tt.wantErr == nil {
				wantView = usecase.UserView{ID: u.ID(), Email: "jane@example.com", FirstName: "Jane", LastName: "Doe"}
			}
			if view != wantView {
				t.Errorf("Execute() view = %+v, want %+v", view, wantView)
			}
			if calls != 1 {
				t.Errorf("ByID calls = %d, want 1", calls)
			}
		})
	}
}

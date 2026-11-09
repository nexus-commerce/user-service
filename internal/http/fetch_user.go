package http

import (
	"context"
	"errors"
	"net/http"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type userFetcher interface {
	Execute(ctx context.Context, userID uuid.UUID) (usecase.UserView, error)
}

type FetchUser struct {
	uc userFetcher
}

func NewFetchUser(uc userFetcher) *FetchUser {
	return &FetchUser{
		uc: uc,
	}
}

type userEnvelope struct {
	User *userResponse `json:"user"`
}

type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func toUserEnvelope(v usecase.UserView) userEnvelope {
	return userEnvelope{
		User: &userResponse{
			ID:        v.ID.String(),
			Email:     v.Email,
			FirstName: v.FirstName,
			LastName:  v.LastName,
		},
	}
}

// ServeHTTP returns the authenticated user.
//
// @Summary     Get current user
// @Description Returns the authenticated user
// @Tags        Users
// @Produce     json
// @Security    BearerAuth
// @Success     200  {object}  userEnvelope
// @Failure     401  {object}  errorResponse  "Missing or invalid token"
// @Failure     404  {object}  errorResponse  "User not found"
// @Failure     500  {object}  errorResponse  "Internal server error"
// @Router      /users/me [get]
func (fu *FetchUser) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	view, err := fu.uc.Execute(r.Context(), userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			writeError(rw, r, http.StatusNotFound, msgUserNotFound)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, toUserEnvelope(view))
}

func (fu *FetchUser) Route() Route {
	return Route{
		Pattern: "GET /users/me",
		Handler: fu,
		Auth:    AuthBearer,
	}
}

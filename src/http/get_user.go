package http

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type GetUser struct {
	uc *usecase.GetUser
}

func NewGetUser(uc *usecase.GetUser) *GetUser {
	return &GetUser{
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

// ServeHTTP godoc
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
func (h *GetUser) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	usr, err := h.uc.Execute(r.Context(), userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			writeError(rw, r, http.StatusNotFound, msgUserNotFound)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, toUserEnvelope(usr))
}

func (h *GetUser) Route() Route {
	return Route{
		Pattern: "GET /users/me",
		Handler: h,
		Auth:    AuthBearer,
	}
}

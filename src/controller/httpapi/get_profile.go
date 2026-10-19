package httpapi

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/usecase"
)

type GetProfile struct {
	uc *usecase.GetProfile
}

func NewGetProfile(uc *usecase.GetProfile) *GetProfile {
	return &GetProfile{
		uc: uc,
	}
}

type profileResponse struct {
	User *userResponse `json:"user"`
}

type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// ServeHTTP godoc
// @Summary     Get profile
// @Description Returns the profile of the authenticated user
// @Tags        Users
// @Produce     json
// @Security    BearerAuth
// @Success     200  {object}  profileResponse
// @Failure     401  {object}  errorResponse  "Missing or invalid token"
// @Failure     404  {object}  errorResponse  "User not found"
// @Failure     500  {object}  errorResponse  "Internal server error"
// @Router      /users/profile [get]
func (h *GetProfile) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	user, err := h.uc.Execute(r.Context(), userID)
	if err != nil {
		if errors.Is(err, entity.ErrUserNotFound) {
			writeError(rw, r, http.StatusNotFound, err.Error())
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, profileResponse{
		User: &userResponse{
			ID:        user.ID().String(),
			Email:     user.Email().String(),
			FirstName: user.FirstName().String(),
			LastName:  user.LastName().String(),
		},
	})
}

func (h *GetProfile) Route() Route {
	return Route{
		Pattern: "GET /users/profile",
		Handler: h,
		Auth:    AuthBearer,
	}
}

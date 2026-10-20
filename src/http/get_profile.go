package http

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
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

func toProfileResponse(v usecase.ProfileView) profileResponse {
	return profileResponse{
		User: &userResponse{
			ID:        v.ID.String(),
			Email:     v.Email,
			FirstName: v.FirstName,
			LastName:  v.LastName,
		},
	}
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

	profile, err := h.uc.Execute(r.Context(), userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			writeError(rw, r, http.StatusNotFound, err.Error())
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, toProfileResponse(profile))
}

func (h *GetProfile) Route() Route {
	return Route{
		Pattern: "GET /users/profile",
		Handler: h,
		Auth:    AuthBearer,
	}
}

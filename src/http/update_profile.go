package http

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type UpdateProfile struct {
	uc *usecase.UpdateProfile
}

func NewUpdateProfile(uc *usecase.UpdateProfile) *UpdateProfile {
	return &UpdateProfile{
		uc: uc,
	}
}

type updateProfileRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Email     *string `json:"email"`
}

func (r *updateProfileRequest) toProfileUpdate(userID uuid.UUID) usecase.ProfileUpdate {
	return usecase.ProfileUpdate{
		UserID:    userID,
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Email:     r.Email,
	}
}

// ServeHTTP godoc
// @Summary     Update profile
// @Description Partially updates the profile of the authenticated user; only provided fields are changed
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request  body      updateProfileRequest  true  "Fields to update"
// @Success     200      {object}  profileResponse
// @Failure     400      {object}  errorResponse  "Invalid request, empty or too long fields or malformed email"
// @Failure     401      {object}  errorResponse  "Missing or invalid token"
// @Failure     404      {object}  errorResponse  "User not found"
// @Failure     409      {object}  errorResponse  "User with this email already exists"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /users/profile [patch]
func (h *UpdateProfile) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	reqBody, err := readJSON[updateProfileRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	profile, err := h.uc.Execute(r.Context(), reqBody.toProfileUpdate(userID))
	if err != nil {
		switch {
		case errors.Is(err, user.ErrValidation):
			writeError(rw, r, http.StatusBadRequest, err.Error())
			return
		case errors.Is(err, user.ErrUserNotFound):
			writeError(rw, r, http.StatusNotFound, err.Error())
			return
		case errors.Is(err, user.ErrEmailTaken):
			writeError(rw, r, http.StatusConflict, err.Error())
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, toProfileResponse(profile))
}

func (h *UpdateProfile) Route() Route {
	return Route{
		Pattern: "PATCH /users/profile",
		Handler: h,
		Auth:    AuthBearer,
	}
}

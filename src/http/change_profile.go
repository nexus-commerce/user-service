package http

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type ChangeProfile struct {
	uc *usecase.ChangeProfile
}

func NewChangeProfile(uc *usecase.ChangeProfile) *ChangeProfile {
	return &ChangeProfile{
		uc: uc,
	}
}

type changeProfileRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Email     *string `json:"email"`
}

func (r *changeProfileRequest) toProfileChange(userID uuid.UUID) usecase.ProfileChange {
	return usecase.ProfileChange{
		UserID:    userID,
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Email:     r.Email,
	}
}

// ServeHTTP godoc
// @Summary     Change profile
// @Description Partially changes the profile of the authenticated user; only provided fields are changed
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request  body      changeProfileRequest  true  "Fields to change"
// @Success     200      {object}  profileResponse
// @Failure     400      {object}  errorResponse  "Invalid request, empty or too long fields or malformed email"
// @Failure     401      {object}  errorResponse  "Missing or invalid token"
// @Failure     404      {object}  errorResponse  "User not found"
// @Failure     409      {object}  errorResponse  "User with this email already exists"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /users/profile [patch]
func (h *ChangeProfile) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	reqBody, err := readJSON[changeProfileRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	profile, err := h.uc.Execute(r.Context(), reqBody.toProfileChange(userID))
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

func (h *ChangeProfile) Route() Route {
	return Route{
		Pattern: "PATCH /users/profile",
		Handler: h,
		Auth:    AuthBearer,
	}
}

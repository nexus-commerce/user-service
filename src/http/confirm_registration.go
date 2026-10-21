package http

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type ConfirmRegistration struct {
	uc *usecase.ConfirmRegistration
}

func NewConfirmRegistration(uc *usecase.ConfirmRegistration) *ConfirmRegistration {
	return &ConfirmRegistration{
		uc: uc,
	}
}

type confirmRegistrationRequest struct {
	Token string `json:"token"`
}

type confirmRegistrationResponse struct {
	Token string `json:"token"`
}

// ServeHTTP godoc
// @Summary     Confirm registration
// @Description Confirms a pending registration with the token from the confirmation link, creates the user account, publishes a user.registered event and returns a JWT token
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      confirmRegistrationRequest   true  "Confirmation token"
// @Success     200      {object}  confirmRegistrationResponse
// @Failure     400      {object}  errorResponse  "Invalid request, invalid, already used or expired confirmation link"
// @Failure     409      {object}  errorResponse  "User with this email already exists"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /auth/register/confirm [post]
func (h *ConfirmRegistration) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	reqBody, err := readJSON[confirmRegistrationRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	token, err := h.uc.Execute(r.Context(), reqBody.Token)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidConfirmationToken):
			writeError(rw, r, http.StatusBadRequest, msgInvalidConfirmationLink)
			return
		case errors.Is(err, user.ErrConfirmationTokenExpired):
			writeError(rw, r, http.StatusBadRequest, msgExpiredConfirmationLink)
			return
		case errors.Is(err, user.ErrEmailTaken):
			writeError(rw, r, http.StatusConflict, msgEmailTaken)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, confirmRegistrationResponse{Token: token})
}

func (h *ConfirmRegistration) Route() Route {
	return Route{
		Pattern: "POST /auth/register/confirm",
		Handler: h,
	}
}

package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/internal/user"
)

type registrationConfirmer interface {
	Execute(ctx context.Context, token string) error
}

type ConfirmRegistration struct {
	uc registrationConfirmer
}

func NewConfirmRegistration(uc registrationConfirmer) *ConfirmRegistration {
	return &ConfirmRegistration{
		uc: uc,
	}
}

type confirmRegistrationRequest struct {
	Token string `json:"token"`
}

type confirmRegistrationResponse struct {
	Status string `json:"status"`
}

// @Summary     Confirm registration
// @Description Confirms a pending registration with the token from the confirmation link.
// @Description The account is created asynchronously: a user.registered event follows, or user.registration_rejected if the email was taken meanwhile.
// @Description Log in via /auth/login once the account exists
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      confirmRegistrationRequest   true  "Confirmation token"
// @Success     202      {object}  confirmRegistrationResponse  "Registration confirmed, account is being created"
// @Failure     400      {object}  errorResponse  "Invalid request, invalid, already used or expired confirmation link"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /auth/register/confirm [post]
func (h *ConfirmRegistration) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	reqBody, err := readJSON[confirmRegistrationRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	if err := h.uc.Execute(r.Context(), reqBody.Token); err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidConfirmationToken):
			writeError(rw, r, http.StatusBadRequest, msgInvalidConfirmationLink)
			return
		case errors.Is(err, user.ErrConfirmationTokenExpired):
			writeError(rw, r, http.StatusBadRequest, msgExpiredConfirmationLink)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusAccepted, confirmRegistrationResponse{Status: msgRegistrationConfirmed})
}

func (h *ConfirmRegistration) Route() Route {
	return Route{
		Pattern: "POST /auth/register/confirm",
		Handler: h,
	}
}

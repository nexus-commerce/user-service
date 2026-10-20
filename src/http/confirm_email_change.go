package http

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type ConfirmEmailChange struct {
	uc *usecase.ConfirmEmailChange
}

func NewConfirmEmailChange(uc *usecase.ConfirmEmailChange) *ConfirmEmailChange {
	return &ConfirmEmailChange{
		uc: uc,
	}
}

type confirmEmailChangeRequest struct {
	Token string `json:"token"`
}

// ServeHTTP godoc
// @Summary     Confirm email change
// @Description Confirms a pending email change with the token from the confirmation link and makes the new email current
// @Tags        Users
// @Accept      json
// @Produce     json
// @Param       request  body  confirmEmailChangeRequest  true  "Confirmation token"
// @Success     204
// @Failure     400  {object}  errorResponse  "Invalid request, invalid, already used or expired confirmation link"
// @Failure     409  {object}  errorResponse  "User with this email already exists"
// @Failure     413  {object}  errorResponse  "Request body too large"
// @Failure     500  {object}  errorResponse  "Internal server error"
// @Router      /users/email/confirm [post]
func (h *ConfirmEmailChange) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	reqBody, err := readJSON[confirmEmailChangeRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	err = h.uc.Execute(r.Context(), reqBody.Token)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrInvalidEmailChangeToken):
			writeError(rw, r, http.StatusBadRequest, msgInvalidConfirmationLink)
			return
		case errors.Is(err, user.ErrEmailChangeTokenExpired):
			writeError(rw, r, http.StatusBadRequest, msgExpiredConfirmationLink)
			return
		case errors.Is(err, user.ErrEmailTaken):
			writeError(rw, r, http.StatusConflict, msgEmailTaken)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (h *ConfirmEmailChange) Route() Route {
	return Route{
		Pattern: "POST /users/email/confirm",
		Handler: h,
	}
}

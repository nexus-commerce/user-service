package http

import (
	"context"
	"errors"
	"net/http"
	"uuid"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type emailChangeRequester interface {
	Execute(ctx context.Context, req usecase.EmailChangeRequest) error
}

type RequestEmailChange struct {
	uc emailChangeRequester
}

func NewRequestEmailChange(uc emailChangeRequester) *RequestEmailChange {
	return &RequestEmailChange{
		uc: uc,
	}
}

type emailChangeBody struct {
	NewEmail string `json:"newEmail"`
	Password string `json:"password"`
}

var emailChangeFields = wireFields{usecase.FieldEmail: "newEmail"}

func (r *emailChangeBody) toEmailChangeRequest(userID uuid.UUID) usecase.EmailChangeRequest {
	return usecase.EmailChangeRequest{
		UserID:   userID,
		NewEmail: r.NewEmail,
		Password: r.Password,
	}
}

// ServeHTTP starts an email change for the authenticated user.
//
// @Summary     Request email change
// @Description Starts an email change for the authenticated user; the new email takes effect
// @Description only after it is confirmed via the link sent to it.
// @Description If the new email already belongs to another account, the response is the same
// @Description and that address gets a notice instead of a link.
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request  body  emailChangeBody  true  "New email and current password"
// @Success     202
// @Failure     400  {object}  errorResponse  "Invalid request, malformed email or email equal to the current one"
// @Failure     401  {object}  errorResponse  "Missing or invalid token"
// @Failure     403  {object}  errorResponse  "Invalid password"
// @Failure     404  {object}  errorResponse  "User not found"
// @Failure     413  {object}  errorResponse  "Request body too large"
// @Failure     500  {object}  errorResponse  "Internal server error"
// @Router      /users/me/email [post]
func (re *RequestEmailChange) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	reqBody, err := readJSON[emailChangeBody](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	err = re.uc.Execute(r.Context(), reqBody.toEmailChangeRequest(userID))
	if err != nil {
		switch {
		case errors.Is(err, user.ErrValidation):
			writeError(rw, r, http.StatusBadRequest, validationMessage(err, emailChangeFields))
			return
		case errors.Is(err, user.ErrInvalidCredentials):
			writeError(rw, r, http.StatusForbidden, msgInvalidPassword)
			return
		case errors.Is(err, user.ErrNotFound):
			writeError(rw, r, http.StatusNotFound, msgUserNotFound)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	rw.WriteHeader(http.StatusAccepted)
}

func (re *RequestEmailChange) Route() Route {
	return Route{
		Pattern: "POST /users/me/email",
		Handler: re,
		Auth:    AuthBearer,
	}
}

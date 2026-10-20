package http

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type RequestEmailChange struct {
	uc *usecase.RequestEmailChange
}

func NewRequestEmailChange(uc *usecase.RequestEmailChange) *RequestEmailChange {
	return &RequestEmailChange{
		uc: uc,
	}
}

type requestEmailChangeRequest struct {
	NewEmail string `json:"newEmail"`
	Password string `json:"password"`
}

func (r *requestEmailChangeRequest) toEmailChangeRequest(userID uuid.UUID) usecase.EmailChangeRequest {
	return usecase.EmailChangeRequest{
		UserID:   userID,
		NewEmail: r.NewEmail,
		Password: r.Password,
	}
}

// ServeHTTP godoc
// @Summary     Request email change
// @Description Starts an email change for the authenticated user; the new email takes effect only after it is confirmed via the link sent to it
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request  body  requestEmailChangeRequest  true  "New email and current password"
// @Success     202
// @Failure     400  {object}  errorResponse  "Invalid request, malformed email or email equal to the current one"
// @Failure     401  {object}  errorResponse  "Missing or invalid token"
// @Failure     403  {object}  errorResponse  "Invalid password"
// @Failure     404  {object}  errorResponse  "User not found"
// @Failure     409  {object}  errorResponse  "User with this email already exists"
// @Failure     413  {object}  errorResponse  "Request body too large"
// @Failure     500  {object}  errorResponse  "Internal server error"
// @Router      /users/email [post]
func (h *RequestEmailChange) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	reqBody, err := readJSON[requestEmailChangeRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	err = h.uc.Execute(r.Context(), reqBody.toEmailChangeRequest(userID))
	if err != nil {
		switch {
		case errors.Is(err, user.ErrValidation):
			writeError(rw, r, http.StatusBadRequest, err.Error())
			return
		case errors.Is(err, user.ErrInvalidCredentials):
			writeError(rw, r, http.StatusForbidden, "Invalid password")
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

	rw.WriteHeader(http.StatusAccepted)
}

func (h *RequestEmailChange) Route() Route {
	return Route{
		Pattern: "POST /users/email",
		Handler: h,
		Auth:    AuthBearer,
	}
}

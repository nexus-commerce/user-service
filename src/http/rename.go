package http

import (
	"errors"
	"net/http"
	"uuid"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
)

type Rename struct {
	uc *usecase.Rename
}

func NewRename(uc *usecase.Rename) *Rename {
	return &Rename{
		uc: uc,
	}
}

type renameRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
}

var renameFields = wireFields{
	usecase.FieldFirstName: "firstName",
	usecase.FieldLastName:  "lastName",
}

func (r *renameRequest) toNameChange(userID uuid.UUID) usecase.NameChange {
	return usecase.NameChange{
		UserID:    userID,
		FirstName: r.FirstName,
		LastName:  r.LastName,
	}
}

// ServeHTTP godoc
// @Summary     Rename user
// @Description Partially changes the first and last name of the authenticated user; only provided fields are changed
// @Tags        Users
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request  body      renameRequest  true  "Fields to change"
// @Success     200      {object}  userEnvelope
// @Failure     400      {object}  errorResponse  "Invalid request, empty or too long fields"
// @Failure     401      {object}  errorResponse  "Missing or invalid token"
// @Failure     404      {object}  errorResponse  "User not found"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /users/me/name [patch]
func (h *Rename) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeInternalError(rw, r, err)
		return
	}

	reqBody, err := readJSON[renameRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	usr, err := h.uc.Execute(r.Context(), reqBody.toNameChange(userID))
	if err != nil {
		switch {
		case errors.Is(err, user.ErrValidation):
			writeError(rw, r, http.StatusBadRequest, validationMessage(err, renameFields))
			return
		case errors.Is(err, user.ErrNotFound):
			writeError(rw, r, http.StatusNotFound, msgUserNotFound)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, toUserEnvelope(usr))
}

func (h *Rename) Route() Route {
	return Route{
		Pattern: "PATCH /users/me/name",
		Handler: h,
		Auth:    AuthBearer,
	}
}

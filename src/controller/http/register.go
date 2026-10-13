package http

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/usecase"
)

type Register struct {
	uc *usecase.Register
}

func NewRegister(uc *usecase.Register) *Register {
	return &Register{
		uc: uc,
	}
}

type registerRequest struct {
	FirstName            string `json:"firstName"`
	LastName             string `json:"lastName"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"passwordConfirmation"`
}

func (r *registerRequest) toRegistration() usecase.Registration {
	return usecase.Registration{
		FirstName:            entity.Name(r.FirstName),
		LastName:             entity.Name(r.LastName),
		Email:                entity.Email(r.Email),
		Password:             entity.Password(r.Password),
		PasswordConfirmation: entity.Password(r.PasswordConfirmation),
	}
}

type registerResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// ServeHTTP godoc
// @Summary     Register
// @Description Creates a new user account and publishes a user.registered event
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      registerRequest   true  "Registration data"
// @Success     200      {object}  registerResponse
// @Failure     400      {object}  errorResponse  "Invalid request, empty or too long fields, malformed email or passwords do not match"
// @Failure     409      {object}  errorResponse  "User with this email already exists"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /auth/register [post]
func (h *Register) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	reqBody, err := readJSON[registerRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	user, err := h.uc.Execute(r.Context(), reqBody.toRegistration())
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrValidation):
			writeError(rw, r, http.StatusBadRequest, err.Error())
			return
		case errors.Is(err, usecase.ErrEmailTaken):
			writeError(rw, r, http.StatusConflict, err.Error())
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, registerResponse{
		ID:        user.ID,
		Email:     string(user.Email),
		FirstName: string(user.FirstName),
		LastName:  string(user.LastName),
	})
}

func (h *Register) Route() Route {
	return Route{
		Pattern: "POST /auth/register",
		Handler: h,
	}
}

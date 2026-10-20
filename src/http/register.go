package http

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/usecase"
	"github.com/nexus-commerce/user-service/src/user"
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

var registerFields = wireFields{
	usecase.FieldFirstName:            "firstName",
	usecase.FieldLastName:             "lastName",
	usecase.FieldEmail:                "email",
	usecase.FieldPassword:             "password",
	usecase.FieldPasswordConfirmation: "passwordConfirmation",
}

func (r *registerRequest) toRegistration() usecase.Registration {
	return usecase.Registration{
		FirstName:            r.FirstName,
		LastName:             r.LastName,
		Email:                r.Email,
		Password:             r.Password,
		PasswordConfirmation: r.PasswordConfirmation,
	}
}

type registerResponse struct {
	Token string `json:"token"`
}

// ServeHTTP godoc
// @Summary     Register
// @Description Creates a new user account, publishes a user.registered event and returns a JWT token
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      registerRequest   true  "Registration data"
// @Success     200      {object}  registerResponse
// @Failure     400      {object}  errorResponse  "Invalid request, empty or too long fields, too short password, malformed email or passwords do not match"
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

	token, err := h.uc.Execute(r.Context(), reqBody.toRegistration())
	if err != nil {
		switch {
		case errors.Is(err, user.ErrValidation):
			writeError(rw, r, http.StatusBadRequest, validationMessage(err, registerFields))
			return
		case errors.Is(err, user.ErrEmailTaken):
			writeError(rw, r, http.StatusConflict, msgEmailTaken)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, registerResponse{Token: token})
}

func (h *Register) Route() Route {
	return Route{
		Pattern: "POST /auth/register",
		Handler: h,
	}
}

package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type registrationRequester interface {
	Execute(ctx context.Context, req usecase.RegistrationRequest) error
}

type RequestRegistration struct {
	uc registrationRequester
}

func NewRequestRegistration(uc registrationRequester) *RequestRegistration {
	return &RequestRegistration{
		uc: uc,
	}
}

type registrationBody struct {
	FirstName            string `json:"firstName"`
	LastName             string `json:"lastName"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"passwordConfirmation"`
}

var registrationFields = wireFields{
	usecase.FieldFirstName:            "firstName",
	usecase.FieldLastName:             "lastName",
	usecase.FieldEmail:                "email",
	usecase.FieldPassword:             "password",
	usecase.FieldPasswordConfirmation: "passwordConfirmation",
}

func (r *registrationBody) toRegistrationRequest() usecase.RegistrationRequest {
	return usecase.RegistrationRequest{
		FirstName:            r.FirstName,
		LastName:             r.LastName,
		Email:                r.Email,
		Password:             r.Password,
		PasswordConfirmation: r.PasswordConfirmation,
	}
}

// ServeHTTP starts a registration.
//
// @Summary     Request registration
// @Description Starts a registration: sends a confirmation link to a new email
// @Description or an "account already exists" notice to a registered one.
// @Description The response does not reveal which.
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      registrationBody   true  "Registration data"
// @Success     202
// @Failure     400      {object}  errorResponse  "Invalid request, empty or too long fields, too short password, malformed email or passwords do not match"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /auth/register [post]
func (rr *RequestRegistration) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	reqBody, err := readJSON[registrationBody](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	err = rr.uc.Execute(r.Context(), reqBody.toRegistrationRequest())
	if err != nil {
		if errors.Is(err, user.ErrValidation) {
			writeError(rw, r, http.StatusBadRequest, validationMessage(err, registrationFields))
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	rw.WriteHeader(http.StatusAccepted)
}

func (rr *RequestRegistration) Route() Route {
	return Route{
		Pattern: "POST /auth/register",
		Handler: rr,
	}
}

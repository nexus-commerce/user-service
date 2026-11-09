package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
)

type authenticator interface {
	Execute(ctx context.Context, creds usecase.Credentials) (string, error)
}

type Login struct {
	uc authenticator
}

func NewLogin(uc authenticator) *Login {
	return &Login{
		uc: uc,
	}
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *loginBody) toCredentials() usecase.Credentials {
	return usecase.Credentials{
		Email:    r.Email,
		Password: r.Password,
	}
}

type loginResponse struct {
	Token string `json:"token"`
}

// ServeHTTP authenticates a user and returns a JWT token.
//
// @Summary     Login
// @Description Authenticates a user by email and password and returns a JWT token
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      loginBody   true  "User credentials"
// @Success     200      {object}  loginResponse
// @Failure     400      {object}  errorResponse  "Invalid request body"
// @Failure     401      {object}  errorResponse  "Invalid credentials"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /auth/login [post]
func (l *Login) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	reqBody, err := readJSON[loginBody](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	token, err := l.uc.Execute(r.Context(), reqBody.toCredentials())
	if err != nil {
		if errors.Is(err, user.ErrInvalidCredentials) {
			writeError(rw, r, http.StatusUnauthorized, msgInvalidCredentials)
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, loginResponse{Token: token})
}

func (l *Login) Route() Route {
	return Route{
		Pattern: "POST /auth/login",
		Handler: l,
	}
}

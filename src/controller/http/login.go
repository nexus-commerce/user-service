package http

import (
	"errors"
	"net/http"

	"github.com/nexus-commerce/user-service/src/entity"
	"github.com/nexus-commerce/user-service/src/usecase"
)

type Login struct {
	uc *usecase.Login
}

func NewLogin(uc *usecase.Login) *Login {
	return &Login{
		uc: uc,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// ServeHTTP godoc
// @Summary     Login
// @Description Authenticates a user by email and password and returns a JWT token
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request  body      loginRequest   true  "User credentials"
// @Success     200      {object}  loginResponse
// @Failure     400      {object}  errorResponse  "Invalid request body"
// @Failure     401      {object}  errorResponse  "Invalid credentials"
// @Failure     413      {object}  errorResponse  "Request body too large"
// @Failure     500      {object}  errorResponse  "Internal server error"
// @Router      /auth/login [post]
func (h *Login) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	reqBody, err := readJSON[loginRequest](rw, r)
	if err != nil {
		writeInvalidBody(rw, r, err)
		return
	}

	token, err := h.uc.Execute(r.Context(), reqBody.Email, reqBody.Password)
	if err != nil {
		if errors.Is(err, entity.ErrInvalidCredentials) {
			writeError(rw, r, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		writeInternalError(rw, r, err)
		return
	}

	writeJSON(rw, r, http.StatusOK, loginResponse{Token: token})
}

func (h *Login) Route() Route {
	return Route{
		Pattern: "POST /auth/login",
		Handler: h,
	}
}

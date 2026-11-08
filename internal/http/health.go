package http

import (
	"net/http"
)

type Health struct{}

func NewHealth() *Health {
	return &Health{}
}

// ServeHTTP godoc
// @Summary     Health Check
// @Description Returns 200 OK when the service is healthy
// @Tags        System
// @Success     200  {string}  string  "OK"
// @Router      /health [get]
func (*Health) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusOK)
}

func (h *Health) Route() Route {
	return Route{
		Pattern: "GET /health",
		Handler: h,
	}
}

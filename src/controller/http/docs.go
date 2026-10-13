package http

import (
	"log"
	"net/http"
)

const docsHTML = `<!DOCTYPE html>
<html>
  <head>
    <title>User Service API</title>
    <meta charset="utf-8" />
    <style>body { margin: 0; }</style>
  </head>
  <body>
    <script id="api-reference" data-url="/api/docs/swagger.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>
`

type Docs struct{}

func NewDocs() *Docs {
	return &Docs{}
}

// ServeHTTP godoc
// @Summary     API Documentation
// @Description Serves the interactive API reference UI
// @Tags        System
// @Produce     html
// @Security    BasicAuth
// @Success     200  {string}  string  "HTML page"
// @Failure     401  {object}  errorResponse  "Missing or invalid credentials"
// @Router      /docs [get]
func (h *Docs) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := rw.Write([]byte(docsHTML)); err != nil {
		log.Printf("%s %s: write response: %v", r.Method, r.URL.Path, err)
	}
}

func (h *Docs) Route() Route {
	return Route{
		Pattern: "GET /docs",
		Handler: h,
		Auth:    AuthBasic,
	}
}

type OpenAPI struct {
	spec []byte
}

// NewOpenAPI returns a handler that serves spec, a rendered Swagger document.
func NewOpenAPI(spec string) *OpenAPI {
	return &OpenAPI{spec: []byte(spec)}
}

// ServeHTTP godoc
// @Summary     API Specification
// @Description Serves the Swagger specification used by the API reference UI
// @Tags        System
// @Produce     json
// @Security    BasicAuth
// @Success     200  {object}  object  "Swagger 2.0 specification"
// @Failure     401  {object}  errorResponse  "Missing or invalid credentials"
// @Router      /docs/swagger.json [get]
func (h *OpenAPI) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	if _, err := rw.Write(h.spec); err != nil {
		log.Printf("%s %s: write response: %v", r.Method, r.URL.Path, err)
	}
}

func (h *OpenAPI) Route() Route {
	return Route{
		Pattern: "GET /docs/swagger.json",
		Handler: h,
		Auth:    AuthBasic,
	}
}

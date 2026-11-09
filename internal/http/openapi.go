package http

import "net/http"

const openAPIDocsHTML = `<!DOCTYPE html>
<html>
  <head>
    <title>User Service API</title>
    <meta charset="utf-8" />
    <style>body { margin: 0; }</style>
  </head>
  <body>
    <script id="api-reference" data-url="/api/docs/openapi.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>
`

type OpenAPIDocs struct{}

func NewOpenAPIDocs() *OpenAPIDocs {
	return &OpenAPIDocs{}
}

// @Summary     API Documentation
// @Description Serves the interactive API reference UI
// @Tags        System
// @Produce     html
// @Security    BasicAuth
// @Success     200  {string}  string  "HTML page"
// @Failure     401  {object}  errorResponse  "Missing or invalid credentials"
// @Router      /docs/openapi [get]
func (*OpenAPIDocs) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := rw.Write([]byte(openAPIDocsHTML)); err != nil {
		logRequestError(r, "write response", err)
	}
}

func (h *OpenAPIDocs) Route() Route {
	return Route{
		Pattern: "GET /docs/openapi",
		Handler: h,
		Auth:    AuthBasic,
	}
}

type OpenAPI struct {
	spec []byte
}

func NewOpenAPI(spec string) *OpenAPI {
	return &OpenAPI{spec: []byte(spec)}
}

// @Summary     API Specification
// @Description Serves the OpenAPI specification used by the API reference UI
// @Tags        System
// @Produce     json
// @Security    BasicAuth
// @Success     200  {object}  object  "OpenAPI 2.0 specification"
// @Failure     401  {object}  errorResponse  "Missing or invalid credentials"
// @Router      /docs/openapi.json [get]
func (h *OpenAPI) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	if _, err := rw.Write(h.spec); err != nil {
		logRequestError(r, "write response", err)
	}
}

func (h *OpenAPI) Route() Route {
	return Route{
		Pattern: "GET /docs/openapi.json",
		Handler: h,
		Auth:    AuthBasic,
	}
}

package http

import "net/http"

const asyncAPIDocsHTML = `<!DOCTYPE html>
<html>
  <head>
    <title>User Service Events</title>
    <meta charset="utf-8" />
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@asyncapi/react-component/styles/default.min.css" />
  </head>
  <body>
    <div id="asyncapi"></div>
    <script src="https://cdn.jsdelivr.net/npm/@asyncapi/react-component/browser/standalone/index.js"></script>
    <script>
      AsyncApiStandalone.render({
        schema: { url: "/api/docs/asyncapi.yaml", options: { credentials: "same-origin" } },
        config: { show: { sidebar: true } },
      }, document.getElementById("asyncapi"));
    </script>
  </body>
</html>
`

type AsyncAPIDocs struct{}

func NewAsyncAPIDocs() *AsyncAPIDocs {
	return &AsyncAPIDocs{}
}

// ServeHTTP serves the interactive AsyncAPI reference UI.
//
// @Summary     Events Documentation
// @Description Serves the interactive AsyncAPI reference UI for the events this service publishes
// @Tags        System
// @Produce     html
// @Security    BasicAuth
// @Success     200  {string}  string  "HTML page"
// @Failure     401  {object}  errorResponse  "Missing or invalid credentials"
// @Router      /docs/asyncapi [get]
func (*AsyncAPIDocs) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := rw.Write([]byte(asyncAPIDocsHTML)); err != nil {
		logRequestError(r, "write response", err)
	}
}

func (d *AsyncAPIDocs) Route() Route {
	return Route{
		Pattern: "GET /docs/asyncapi",
		Handler: d,
		Auth:    AuthBasic,
	}
}

type AsyncAPI struct {
	spec []byte
}

func NewAsyncAPI(spec []byte) *AsyncAPI {
	return &AsyncAPI{spec: spec}
}

// ServeHTTP serves the AsyncAPI specification of the published events.
//
// @Summary     Events Specification
// @Description Serves the AsyncAPI specification of the events this service publishes
// @Tags        System
// @Produce     application/yaml
// @Security    BasicAuth
// @Success     200  {string}  string  "AsyncAPI 3.0 specification"
// @Failure     401  {object}  errorResponse  "Missing or invalid credentials"
// @Router      /docs/asyncapi.yaml [get]
func (a *AsyncAPI) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/yaml")

	if _, err := rw.Write(a.spec); err != nil {
		logRequestError(r, "write response", err)
	}
}

func (a *AsyncAPI) Route() Route {
	return Route{
		Pattern: "GET /docs/asyncapi.yaml",
		Handler: a,
		Auth:    AuthBasic,
	}
}

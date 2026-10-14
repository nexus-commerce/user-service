package docs

import _ "embed"

// AsyncAPI is the AsyncAPI document describing the events this service publishes.
//
//go:embed asyncapi.yaml
var AsyncAPI []byte

package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/nexus-commerce/user-service/src/user"
)

const maxBodySize = 1 << 20 // 1 MB

func readJSON[T any](rw http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	r.Body = http.MaxBytesReader(rw, r.Body, maxBodySize)
	err := json.NewDecoder(r.Body).Decode(&v)
	return v, err
}

func writeJSON(rw http.ResponseWriter, r *http.Request, status int, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(v); err != nil {
		log.Printf("%s %s: encode response: %v", r.Method, r.URL.Path, err)
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(rw http.ResponseWriter, r *http.Request, status int, msg string) {
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	writeJSON(rw, r, status, errorResponse{Error: msg})
}

type wireFields map[user.Field]string

func validationMessage(err error, fields wireFields) string {
	if fe, ok := errors.AsType[*user.FieldError](err); ok {
		if name, ok := fields[fe.Field]; ok {
			return name + ": " + fe.Err.Error()
		}
	}
	return err.Error()
}

func writeInvalidBody(rw http.ResponseWriter, r *http.Request, err error) {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		writeError(rw, r, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	writeError(rw, r, http.StatusBadRequest, "invalid request body")
}

func writeInternalError(rw http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeError(rw, r, http.StatusInternalServerError, "internal server error")
}

// Package httpx holds small HTTP helpers shared by all handlers: JSON
// encoding/decoding, the error envelope, and common middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Error codes used in the "error" field of ErrorResponse. Keep in sync with docs/api.yaml.
const (
	CodeBadRequest       = "bad_request"
	CodeUnauthorized     = "unauthorized"
	CodeForbidden        = "forbidden"
	CodeNotFound         = "not_found"
	CodeMethodNotAllowed = "method_not_allowed"
	CodeInternal         = "internal"
	CodeUnavailable      = "unavailable"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// ErrorResponse is the body of every non-2xx API response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes the standard error envelope.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorResponse{Error: code, Message: message})
}

// Common shorthands.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, CodeBadRequest, message)
}

func Internal(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, CodeInternal, "internal server error")
}

func NotFound(w http.ResponseWriter, _ *http.Request) {
	Error(w, http.StatusNotFound, CodeNotFound, "resource not found")
}

func MethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	Error(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
}

// DecodeJSON reads a single JSON object from the request body into dst.
// Unknown fields are ignored; trailing data is rejected. The returned error is
// safe to show to the client.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return errors.New("request body is empty")
		case errors.As(err, &maxErr):
			return fmt.Errorf("request body must not exceed %d bytes", maxErr.Limit)
		default:
			return fmt.Errorf("invalid JSON body: %v", err)
		}
	}
	if dec.More() {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

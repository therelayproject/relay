package handler

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/relay-im/relay/shared/errors"
)

// writeJSON serialises v to JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers already sent; nothing we can do except log (caller's concern).
		return
	}
}

// errorResponse is the canonical JSON error body.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError maps an error to an HTTP status and writes a JSON error body.
func writeError(w http.ResponseWriter, err error) {
	status := apperrors.HTTPStatus(err)

	var body errorResponse
	if ae, ok := err.(*apperrors.AppError); ok {
		body = errorResponse{Code: string(ae.Code), Message: ae.Message}
	} else {
		body = errorResponse{Code: "INTERNAL", Message: "internal server error"}
	}

	writeJSON(w, status, body)
}

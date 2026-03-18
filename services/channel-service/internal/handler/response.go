// Package handler contains HTTP handlers for the channel service.
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
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a structured JSON error response derived from an AppError.
func writeError(w http.ResponseWriter, err error) {
	httpStatus := apperrors.HTTPStatus(err)
	code := "INTERNAL_ERROR"
	msg := "an unexpected error occurred"

	if ae, ok := err.(*apperrors.AppError); ok {
		code = string(ae.Code)
		msg = ae.Message
	}

	writeJSON(w, httpStatus, map[string]any{
		"error":   code,
		"message": msg,
	})
}

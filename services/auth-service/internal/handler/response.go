// Package handler contains HTTP handlers for the auth service.
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

// writeError writes a structured error response.
func writeError(w http.ResponseWriter, err error) {
	status := apperrors.HTTPStatus(err)
	code := "INTERNAL_ERROR"
	msg := "an unexpected error occurred"

	var ae *apperrors.AppError
	if appErr, ok := err.(*apperrors.AppError); ok {
		ae = appErr
		code = mapErrCode(ae.Code)
		msg = ae.Message
	}

	writeJSON(w, status, map[string]any{
		"error":   code,
		"message": msg,
	})
}

func mapErrCode(c apperrors.Code) string {
	switch c {
	case apperrors.CodeAlreadyExists:
		return "EMAIL_TAKEN"
	case apperrors.CodeUnauthenticated:
		return "INVALID_CREDENTIALS"
	case apperrors.CodePermissionDenied:
		return "FORBIDDEN"
	case apperrors.CodeNotFound:
		return "NOT_FOUND"
	case apperrors.CodeInvalidArgument:
		return "VALIDATION_ERROR"
	case apperrors.CodeResourceExhausted:
		return "RATE_LIMITED"
	default:
		return "INTERNAL_ERROR"
	}
}

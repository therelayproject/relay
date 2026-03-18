package handler

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/relay-im/relay/shared/errors"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	httpCode := apperrors.HTTPStatus(err)
	errCode := "INTERNAL_ERROR"
	msg := "an unexpected error occurred"
	if ae, ok := err.(*apperrors.AppError); ok {
		msg = ae.Message
		switch ae.Code {
		case apperrors.CodeNotFound:
			errCode = "NOT_FOUND"
		case apperrors.CodeAlreadyExists:
			errCode = "ALREADY_REACTED"
		case apperrors.CodePermissionDenied:
			errCode = "FORBIDDEN"
		case apperrors.CodeInvalidArgument:
			errCode = "VALIDATION_ERROR"
		default:
			errCode = string(ae.Code)
		}
	}
	writeJSON(w, httpCode, map[string]string{"error": errCode, "message": msg})
}

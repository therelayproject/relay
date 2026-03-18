// Package errors defines canonical error types and HTTP/gRPC status mappings
// used across all Relay services.
package errors

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Code is a machine-readable error classification.
type Code string

const (
	CodeNotFound          Code = "NOT_FOUND"
	CodeAlreadyExists     Code = "ALREADY_EXISTS"
	CodeUnauthenticated   Code = "UNAUTHENTICATED"
	CodePermissionDenied  Code = "PERMISSION_DENIED"
	CodeInvalidArgument   Code = "INVALID_ARGUMENT"
	CodeInternal          Code = "INTERNAL"
	CodeUnavailable       Code = "UNAVAILABLE"
	CodeResourceExhausted Code = "RESOURCE_EXHAUSTED"
)

// AppError is the standard error type returned by all service layer functions.
type AppError struct {
	Code    Code
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// New creates an AppError.
func New(code Code, msg string) *AppError {
	return &AppError{Code: code, Message: msg}
}

// Wrap wraps an existing error with AppError context.
func Wrap(code Code, msg string, err error) *AppError {
	return &AppError{Code: code, Message: msg, Err: err}
}

// HTTPStatus maps an AppError code to an HTTP status code.
func HTTPStatus(err error) int {
	var ae *AppError
	if !errors.As(err, &ae) {
		return http.StatusInternalServerError
	}
	switch ae.Code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists:
		return http.StatusConflict
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodePermissionDenied:
		return http.StatusForbidden
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeResourceExhausted:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// GRPCStatus converts an AppError to a gRPC status error.
func GRPCStatus(err error) error {
	var ae *AppError
	if !errors.As(err, &ae) {
		return status.Error(codes.Internal, err.Error())
	}
	var code codes.Code
	switch ae.Code {
	case CodeNotFound:
		code = codes.NotFound
	case CodeAlreadyExists:
		code = codes.AlreadyExists
	case CodeUnauthenticated:
		code = codes.Unauthenticated
	case CodePermissionDenied:
		code = codes.PermissionDenied
	case CodeInvalidArgument:
		code = codes.InvalidArgument
	case CodeResourceExhausted:
		code = codes.ResourceExhausted
	case CodeUnavailable:
		code = codes.Unavailable
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}

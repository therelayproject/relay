package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeNotFound, "not found")
	if err == nil {
		t.Fatal("New returned nil")
	}
	if err.Code != CodeNotFound {
		t.Errorf("Code: got %q, want %q", err.Code, CodeNotFound)
	}
	if err.Message != "not found" {
		t.Errorf("Message: got %q", err.Message)
	}
	if err.Err != nil {
		t.Error("expected nil wrapped error")
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("inner error")
	err := Wrap(CodeInternal, "wrapped", inner)
	if err.Code != CodeInternal {
		t.Errorf("Code: got %q", err.Code)
	}
	if err.Err != inner {
		t.Error("expected wrapped inner error")
	}
	if !errors.Is(err, inner) {
		t.Error("errors.Is should find inner error through Unwrap")
	}
}

func TestAppError_Error(t *testing.T) {
	e1 := New(CodeNotFound, "resource missing")
	if e1.Error() == "" {
		t.Error("Error() should not be empty")
	}

	inner := errors.New("db error")
	e2 := Wrap(CodeInternal, "something failed", inner)
	if e2.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		code       Code
		wantStatus int
	}{
		{CodeNotFound, http.StatusNotFound},
		{CodeAlreadyExists, http.StatusConflict},
		{CodeUnauthenticated, http.StatusUnauthorized},
		{CodePermissionDenied, http.StatusForbidden},
		{CodeInvalidArgument, http.StatusBadRequest},
		{CodeResourceExhausted, http.StatusTooManyRequests},
		{CodeUnavailable, http.StatusServiceUnavailable},
		{CodeInternal, http.StatusInternalServerError},
	}
	for _, c := range cases {
		got := HTTPStatus(New(c.code, "msg"))
		if got != c.wantStatus {
			t.Errorf("HTTPStatus(code=%s): got %d, want %d", c.code, got, c.wantStatus)
		}
	}
}

func TestHTTPStatus_PlainError(t *testing.T) {
	got := HTTPStatus(errors.New("plain error"))
	if got != http.StatusInternalServerError {
		t.Errorf("expected 500 for plain error, got %d", got)
	}
}

func TestErrorsAs(t *testing.T) {
	wrapped := Wrap(CodeNotFound, "not found", errors.New("inner"))
	var ae *AppError
	if !errors.As(wrapped, &ae) {
		t.Fatal("errors.As should find AppError")
	}
	if ae.Code != CodeNotFound {
		t.Errorf("wrong code: %q", ae.Code)
	}
}

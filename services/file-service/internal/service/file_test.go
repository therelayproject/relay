package service

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/relay-im/relay/shared/errors"
	"github.com/rs/zerolog"
)

func newTestFileService() *FileService {
	return &FileService{repo: nil, minio: nil, bucket: bucketName, log: zerolog.Nop()}
}

func assertFileCode(t *testing.T, err error, want apperrors.Code) {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Errorf("expected *apperrors.AppError, got %T: %v", err, err)
		return
	}
	if ae.Code != want {
		t.Errorf("error code: got %q, want %q", ae.Code, want)
	}
}

// ── Upload validation ─────────────────────────────────────────────────────────

func TestUpload_FileTooLarge(t *testing.T) {
	svc := newTestFileService()
	_, err := svc.Upload(
		context.Background(),
		"ws-1", "ch-1", "user-1",
		"file.png", "image/png",
		maxUploadBytes+1,
		nil,
	)
	if err == nil {
		t.Fatal("expected error for file too large, got nil")
	}
	assertFileCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUpload_EmptyFilename(t *testing.T) {
	svc := newTestFileService()
	_, err := svc.Upload(
		context.Background(),
		"ws-1", "ch-1", "user-1",
		"  ", "image/png",
		1024,
		nil,
	)
	if err == nil {
		t.Fatal("expected error for empty filename, got nil")
	}
	assertFileCode(t, err, apperrors.CodeInvalidArgument)
}

func TestUpload_ExactlyAtLimit_PassesValidation(t *testing.T) {
	// maxUploadBytes is the maximum allowed; it should NOT be rejected by validation.
	// The nil minio client causes a panic after validation — we recover from that.
	svc := newTestFileService()
	var validationErr bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				// nil minio client panic after validation passed — expected
			}
		}()
		_, err := svc.Upload(
			context.Background(),
			"ws-1", "ch-1", "user-1",
			"file.png", "image/png",
			maxUploadBytes,
			nil,
		)
		if err != nil {
			var ae *apperrors.AppError
			if errors.As(err, &ae) && ae.Code == apperrors.CodeInvalidArgument {
				validationErr = true
			}
		}
	}()
	if validationErr {
		t.Fatal("file at maxUploadBytes should NOT fail validation")
	}
}

// ── PresignThumbnail: no thumbnail ────────────────────────────────────────────

// PresignThumbnail requires a DB lookup; we can't easily unit test it without
// mocking the repo. Document that it requires integration test coverage.
func TestPresignThumbnail_RequiresIntegrationTest(t *testing.T) {
	t.Log("PresignThumbnail requires a DB (repo.GetByID) — covered by integration tests")
}

// ── maxUploadBytes constant ───────────────────────────────────────────────────

func TestMaxUploadBytes(t *testing.T) {
	if maxUploadBytes != 100<<20 {
		t.Errorf("expected maxUploadBytes=100MiB, got %d", maxUploadBytes)
	}
}

// ── bucketName constant ───────────────────────────────────────────────────────

func TestBucketName(t *testing.T) {
	if bucketName == "" {
		t.Error("bucketName must not be empty")
	}
}

// Package service contains the file service business logic.
package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	"github.com/relay-im/relay/services/file-service/internal/domain"
	"github.com/relay-im/relay/services/file-service/internal/repository"
	apperrors "github.com/relay-im/relay/shared/errors"
)

const (
	bucketName       = "relay-files"
	maxUploadBytes   = 100 << 20 // 100 MiB
	signedURLExpiry  = 1 * time.Hour
)

// FileService handles upload, download, and thumbnail generation.
type FileService struct {
	repo   *repository.FileRepo
	minio  *minio.Client
	bucket string
	log    zerolog.Logger
}

// New creates a FileService.
func New(repo *repository.FileRepo, mc *minio.Client, log zerolog.Logger) *FileService {
	return &FileService{repo: repo, minio: mc, bucket: bucketName, log: log}
}

// Upload stores a file in MinIO and records metadata in PostgreSQL.
// FILE-01: multipart upload, FILE-02: thumbnail generation hook.
func (s *FileService) Upload(
	ctx context.Context,
	workspaceID, channelID, uploaderID, filename, contentType string,
	sizeBytes int64,
	r io.Reader,
) (*domain.File, error) {
	if sizeBytes > maxUploadBytes {
		return nil, apperrors.New(apperrors.CodeInvalidArgument,
			fmt.Sprintf("file too large: max %d bytes", maxUploadBytes))
	}
	if strings.TrimSpace(filename) == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "filename is required")
	}

	ext := filepath.Ext(filename)
	storageKey := fmt.Sprintf("workspaces/%s/%s%s", workspaceID, uuid.NewString(), ext)

	// Stream directly into MinIO.
	_, err := s.minio.PutObject(ctx, s.bucket, storageKey, r, sizeBytes, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "minio put object", err)
	}

	isImage := domain.MIMEIsImage(contentType)
	f := &domain.File{
		WorkspaceID: workspaceID,
		ChannelID:   channelID,
		UploaderID:  uploaderID,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		StorageKey:  storageKey,
		IsImage:     isImage,
	}

	created, err := s.repo.Create(ctx, f)
	if err != nil {
		return nil, err
	}

	// Trigger async thumbnail generation for images (FILE-02).
	if isImage {
		go s.generateThumbnail(context.Background(), created, storageKey)
	}

	return created, nil
}

// PresignDownload returns a time-limited presigned URL for direct download (FILE-03).
func (s *FileService) PresignDownload(ctx context.Context, fileID string) (string, error) {
	f, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}

	params := make(url.Values)
	params.Set("response-content-disposition",
		fmt.Sprintf(`attachment; filename="%s"`, f.Filename))

	u, err := s.minio.PresignedGetObject(ctx, s.bucket, f.StorageKey, signedURLExpiry, params)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeInternal, "presign download", err)
	}
	return u.String(), nil
}

// PresignThumbnail returns a presigned URL for the thumbnail (if available).
func (s *FileService) PresignThumbnail(ctx context.Context, fileID string) (string, error) {
	f, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		return "", err
	}
	if f.ThumbnailKey == "" {
		return "", apperrors.New(apperrors.CodeNotFound, "thumbnail not yet generated")
	}
	u, err := s.minio.PresignedGetObject(ctx, s.bucket, f.ThumbnailKey, signedURLExpiry, nil)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeInternal, "presign thumbnail", err)
	}
	return u.String(), nil
}

// GetFile returns file metadata.
func (s *FileService) GetFile(ctx context.Context, fileID string) (*domain.File, error) {
	return s.repo.GetByID(ctx, fileID)
}

// ── thumbnail generation ──────────────────────────────────────────────────────

// generateThumbnail downloads the original image, resizes it using ImageMagick
// via a subprocess, and uploads the result.  Errors are logged only (best effort).
func (s *FileService) generateThumbnail(ctx context.Context, f *domain.File, storageKey string) {
	obj, err := s.minio.GetObject(ctx, s.bucket, storageKey, minio.GetObjectOptions{})
	if err != nil {
		s.log.Warn().Err(err).Str("key", storageKey).Msg("thumbnail: failed to get original")
		return
	}
	defer obj.Close()

	// Read original into memory (thumbnails only for reasonably-sized uploads).
	origData, err := io.ReadAll(io.LimitReader(obj, 20<<20)) // 20 MiB limit
	if err != nil {
		s.log.Warn().Err(err).Msg("thumbnail: failed to read original")
		return
	}

	thumb, err := resizeImage(origData, 320, 240, f.ContentType)
	if err != nil {
		s.log.Warn().Err(err).Str("file_id", f.ID).Msg("thumbnail: resize failed")
		return
	}

	thumbKey := strings.TrimSuffix(storageKey, filepath.Ext(storageKey)) + "_thumb.jpg"
	_, err = s.minio.PutObject(ctx, s.bucket, thumbKey,
		bytes.NewReader(thumb), int64(len(thumb)),
		minio.PutObjectOptions{ContentType: "image/jpeg"})
	if err != nil {
		s.log.Warn().Err(err).Msg("thumbnail: upload failed")
		return
	}

	if err := s.repo.UpdateThumbnailKey(ctx, f.ID, thumbKey); err != nil {
		s.log.Warn().Err(err).Msg("thumbnail: update db key failed")
	}
}

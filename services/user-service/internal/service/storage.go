package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig holds the connection settings for MinIO/S3.
type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
	PublicBaseURL   string // public-facing URL prefix for object keys
}

// MinIOStorage wraps minio.Client to implement StorageService.
type MinIOStorage struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

// NewMinIOStorage constructs a MinIOStorage and ensures the bucket exists.
func NewMinIOStorage(ctx context.Context, cfg MinIOConfig) (*MinIOStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio new client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket exists check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
	}

	return &MinIOStorage{
		client:    client,
		bucket:    cfg.Bucket,
		publicURL: cfg.PublicBaseURL,
	}, nil
}

// UploadAvatar stores the avatar bytes under "avatars/{userID}" and returns the public URL.
func (s *MinIOStorage) UploadAvatar(ctx context.Context, userID string, data []byte, contentType string) (string, error) {
	if contentType == "" {
		contentType = "image/jpeg"
	}
	objectKey := fmt.Sprintf("avatars/%s", userID)

	_, err := s.client.PutObject(ctx, s.bucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio put object: %w", err)
	}

	return fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucket, objectKey), nil
}

// NoopStorage is a no-op StorageService for testing or when MinIO is not configured.
type NoopStorage struct{}

func (n *NoopStorage) UploadAvatar(_ context.Context, userID string, _ []byte, _ string) (string, error) {
	return fmt.Sprintf("/avatars/%s", userID), nil
}

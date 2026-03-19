// Package repository contains PostgreSQL persistence for the file service.
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/relay-im/relay/services/file-service/internal/domain"
	apperrors "github.com/relay-im/relay/shared/errors"
)

// FileRepo persists file metadata in PostgreSQL.
type FileRepo struct {
	pool *pgxpool.Pool
}

// New creates a FileRepo.
func New(pool *pgxpool.Pool) *FileRepo {
	return &FileRepo{pool: pool}
}

// Create inserts a new file record.
func (r *FileRepo) Create(ctx context.Context, f *domain.File) (*domain.File, error) {
	const q = `
		INSERT INTO relay_files (workspace_id, channel_id, uploader_id, filename, content_type,
		                         size_bytes, storage_key, thumbnail_key, is_image)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`

	row := r.pool.QueryRow(ctx, q,
		f.WorkspaceID, nilUUID(f.ChannelID), f.UploaderID, f.Filename, f.ContentType,
		f.SizeBytes, f.StorageKey, nilStr(f.ThumbnailKey), f.IsImage,
	)
	if err := row.Scan(&f.ID, &f.CreatedAt); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "insert file", err)
	}
	return f, nil
}

// GetByID returns a file by its ID.
func (r *FileRepo) GetByID(ctx context.Context, id string) (*domain.File, error) {
	const q = `
		SELECT id, workspace_id, COALESCE(channel_id::text, ''), uploader_id, filename,
		       content_type, size_bytes, storage_key, COALESCE(thumbnail_key, ''), is_image, created_at
		FROM relay_files WHERE id = $1 AND NOT is_image = FALSE OR TRUE`

	// Simpler query:
	const sq = `
		SELECT id, workspace_id, COALESCE(channel_id::text, ''), uploader_id, filename,
		       content_type, size_bytes, storage_key, COALESCE(thumbnail_key, ''), is_image, created_at
		FROM relay_files WHERE id = $1`
	_ = q
	f := &domain.File{}
	row := r.pool.QueryRow(ctx, sq, id)
	if err := row.Scan(&f.ID, &f.WorkspaceID, &f.ChannelID, &f.UploaderID, &f.Filename,
		&f.ContentType, &f.SizeBytes, &f.StorageKey, &f.ThumbnailKey, &f.IsImage, &f.CreatedAt); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeNotFound, fmt.Sprintf("file %s not found", id), err)
	}
	return f, nil
}

// UpdateThumbnailKey sets the thumbnail storage key after generation.
func (r *FileRepo) UpdateThumbnailKey(ctx context.Context, fileID, thumbnailKey string) error {
	const q = `UPDATE relay_files SET thumbnail_key = $1 WHERE id = $2`
	if _, err := r.pool.Exec(ctx, q, thumbnailKey, fileID); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "update thumbnail key", err)
	}
	return nil
}

func nilUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

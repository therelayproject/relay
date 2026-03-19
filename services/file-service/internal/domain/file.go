// Package domain defines core types for the file service.
package domain

import "time"

// File represents a file uploaded to Relay.
type File struct {
	ID           string
	WorkspaceID  string
	ChannelID    string
	UploaderID   string
	Filename     string
	ContentType  string
	SizeBytes    int64
	StorageKey   string // MinIO object key
	ThumbnailKey string // MinIO object key for thumbnail (images only)
	IsImage      bool
	CreatedAt    time.Time
}

// MIMEIsImage returns true if the content type is an image the service can thumbnail.
func MIMEIsImage(mime string) bool {
	switch mime {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	}
	return false
}

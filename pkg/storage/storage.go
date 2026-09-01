package storage

import (
	"context"
	"io"
	"time"
)

// UploadInput contains the data needed to upload a file.
type UploadInput struct {
	Reader      io.Reader
	ObjectPath  string // e.g. "finance/attachments/uuid/file.pdf"
	ContentType string // MIME type
}

// FileInfo is returned after a successful upload.
type FileInfo struct {
	ObjectPath string
	PublicURL  string
	Size       int64
}

// Client is the abstraction over cloud object storage.
type Client interface {
	// Upload stores an object and returns its metadata.
	Upload(ctx context.Context, input *UploadInput) (*FileInfo, error)

	// Delete removes an object by path.
	Delete(ctx context.Context, objectPath string) error

	// SignedURL generates a time-limited read URL.
	SignedURL(ctx context.Context, objectPath string, expiry time.Duration) (string, error)

	// PublicURL returns the permanent public URL (assumes bucket is public-read or uniform ACL).
	PublicURL(objectPath string) string
}

package storage

import (
	"context"
	"time"
)

// ObjectStorage defines the interface for object storage operations.
// Implemented by R2Client for production; mocked in tests.
type ObjectStorage interface {
	// GeneratePresignedPUT creates a presigned PUT URL for direct upload.
	//
	// contentType and sizeBytes are not advisory: both are bound into the
	// signature, so the upload is rejected unless it declares exactly these.
	// sizeBytes is therefore the exact length of the upload, not a ceiling —
	// a caller holding a maximum rather than a real size has nothing to pass
	// here. Both must be non-zero; see the implementation for why an
	// unenforceable request is refused instead of signed.
	GeneratePresignedPUT(ctx context.Context, key, contentType string, sizeBytes int64, ttl time.Duration) (uploadURL, publicURL string, err error)
	// DeleteObject removes an object by key.
	DeleteObject(ctx context.Context, key string) error
	// KeyFromPublicURL extracts the object key from a public URL.
	// Returns ("", false) if the URL doesn't belong to this storage.
	KeyFromPublicURL(publicURL string) (string, bool)
}

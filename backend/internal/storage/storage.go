package storage

import (
	"context"
	"time"
)

// Object is one object this service uploads itself, body included.
//
// It exists because the two upload paths are not variations of each other. A
// presigned PUT hands the browser a URL and never sees the bytes; this one
// carries the bytes, because the producer is the server (the payments export
// workbook, built by a background job) and there is no browser in the
// exchange to hand a URL to.
//
// ContentDisposition is part of the object rather than of the request that
// later reads it: an object stored as an .xlsx download keeps that name for
// every reader, including one following a presigned URL that forgot to ask
// for it.
type Object struct {
	// Key is the object's full path in the bucket, e.g.
	// "exports/{complexID}/{exportID}.xlsx".
	Key string
	// ContentType is the object's media type. It is required: an object
	// stored without one is served as a generic stream and a browser saves it
	// under the wrong name.
	ContentType string
	// ContentDisposition is optional, e.g. `attachment; filename="x.xlsx"`.
	ContentDisposition string
	// Body is the whole object. Callers hold the bytes already — the export
	// is capped at a size that fits in memory — so there is no reader here to
	// keep open across a retry.
	Body []byte
}

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
	// PutObject uploads obj from this process. Unlike GeneratePresignedPUT it
	// is for bytes this service produced rather than bytes a browser holds.
	PutObject(ctx context.Context, obj Object) error
	// GeneratePresignedGET creates a short-lived signed URL that reads key.
	//
	// It is the read side of a PRIVATE bucket: an object in a bucket served
	// by a public domain needs no URL to be signed, so this is only
	// meaningful against a client built with no publicBaseURL.
	//
	// downloadName, when non-empty, is signed as ResponseContentDisposition,
	// so the file arrives named even when the stored object was not.
	//
	// An empty key or a non-positive ttl is refused rather than signed; see
	// the implementation for why.
	GeneratePresignedGET(ctx context.Context, key, downloadName string, ttl time.Duration) (string, error)
	// DeleteObject removes an object by key.
	DeleteObject(ctx context.Context, key string) error
	// KeyFromPublicURL extracts the object key from a public URL.
	// Returns ("", false) if the URL doesn't belong to this storage.
	KeyFromPublicURL(publicURL string) (string, bool)
}

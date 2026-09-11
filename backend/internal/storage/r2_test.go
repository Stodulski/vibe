package storage

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestKeyFromPublicURL(t *testing.T) {
	r := &R2Client{publicBaseURL: "https://cdn.example.com"}

	tests := []struct {
		name    string
		url     string
		wantKey string
		wantOK  bool
	}{
		{"valid key", "https://cdn.example.com/complexes/123/logo/abc.webp", "complexes/123/logo/abc.webp", true},
		{"nested path", "https://cdn.example.com/a/b/c.png", "a/b/c.png", true},
		{"different domain", "https://other.com/complexes/123/logo.webp", "", false},
		{"empty url", "", "", false},
		{"url equals base without trailing slash", "https://cdn.example.com/", "", false},
		{"url equals base exactly", "https://cdn.example.com", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, ok := r.KeyFromPublicURL(tt.url)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestKeyFromPublicURL_EmptyBase(t *testing.T) {
	r := &R2Client{publicBaseURL: ""}
	key, ok := r.KeyFromPublicURL("https://anything.com/key.png")
	if ok {
		t.Error("expected ok=false for empty publicBaseURL")
	}
	if key != "" {
		t.Errorf("expected empty key, got %q", key)
	}
}

func TestKeyFromPublicURL_TrailingSlashBase(t *testing.T) {
	// Constructor trims trailing slash, but test the struct directly
	r := &R2Client{publicBaseURL: "https://cdn.example.com"}
	key, ok := r.KeyFromPublicURL("https://cdn.example.com/test.webp")
	if !ok {
		t.Error("expected ok=true")
	}
	if key != "test.webp" {
		t.Errorf("key = %q, want %q", key, "test.webp")
	}
}

// --- Presign enforcement -----------------------------------------------------
//
// A presigned PUT is the only thing standing between an authenticated caller
// and the bucket: once the URL is handed out, nothing of ours sees the upload.
// Whatever the caller validated has to be inside the signature, or it was not
// enforced at all — it was merely checked and then thrown away.

// newPresignTestClient returns a client whose credentials are fixed, so a
// signature is a deterministic function of what is signed.
func newPresignTestClient(t *testing.T) *R2Client {
	t.Helper()
	c, err := NewR2Client("account", "AKIAEXAMPLEKEY", "examplesecret", "bucket", "https://cdn.example.com")
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}
	return c
}

// presignedQuery returns the query of a presigned upload URL.
func presignedQuery(t *testing.T, c *R2Client, contentType string, sizeBytes int64) url.Values {
	t.Helper()
	uploadURL, _, err := c.GeneratePresignedPUT(context.Background(), "complexes/x/logo/a.webp", contentType, sizeBytes, 10*time.Minute)
	if err != nil {
		t.Fatalf("presigning %q/%d: %v", contentType, sizeBytes, err)
	}
	u, err := url.Parse(uploadURL)
	if err != nil {
		t.Fatalf("the presigned URL does not parse: %v", err)
	}
	return u.Query()
}

// The content type must be signed, not merely sent. A type carried as an
// unsigned header is one the uploader can replace at will.
func TestPresignedPUTSignsTheContentTypeAndLength(t *testing.T) {
	c := newPresignTestClient(t)
	signed := presignedQuery(t, c, "image/webp", 4096).Get("X-Amz-SignedHeaders")

	for _, header := range []string{"content-type", "content-length"} {
		if !slices.Contains(strings.Split(signed, ";"), header) {
			t.Errorf("%s is not part of the signature (signed headers: %q), so the upload is free to declare its own", header, signed)
		}
	}
}

// If the content type were signed but ignored, two different types would yield
// the same signature — and the same URL would accept either.
func TestPresignedPUTBindsTheContentTypeToTheSignature(t *testing.T) {
	c := newPresignTestClient(t)

	webp := presignedQuery(t, c, "image/webp", 4096).Get("X-Amz-Signature")
	png := presignedQuery(t, c, "image/png", 4096).Get("X-Amz-Signature")

	if webp == "" {
		t.Fatal("no signature was produced")
	}
	if webp == png {
		t.Errorf("image/webp and image/png produced the same signature %q, so the type is not enforced", webp)
	}
}

// Likewise for the length: a URL signed for one size must not accept another.
func TestPresignedPUTBindsTheSizeToTheSignature(t *testing.T) {
	c := newPresignTestClient(t)

	small := presignedQuery(t, c, "image/webp", 4096).Get("X-Amz-Signature")
	large := presignedQuery(t, c, "image/webp", 5*1024*1024).Get("X-Amz-Signature")

	if small == large {
		t.Errorf("two sizes produced the same signature %q, so the length is not enforced", small)
	}
}

// A request the signer cannot enforce must be refused, never signed.
//
// The AWS signer omits a header it has no value for, and omitting
// content-length omits content-type along with it — so a zero size does not
// merely relax the size check, it produces a URL signed over the host alone,
// which accepts any bytes of any type. Handing that back looks identical to
// success at the call site.
func TestPresignedPUTRefusesWhatItCannotEnforce(t *testing.T) {
	c := newPresignTestClient(t)

	tests := []struct {
		name        string
		contentType string
		sizeBytes   int64
	}{
		{"no content type", "", 4096},
		{"zero size", "image/webp", 0},
		{"negative size", "image/webp", -1},
		{"neither", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploadURL, publicURL, err := c.GeneratePresignedPUT(
				context.Background(), "complexes/x/logo/a.webp", tt.contentType, tt.sizeBytes, 10*time.Minute)
			if err == nil {
				t.Fatalf("want a refusal; got upload URL %q", uploadURL)
			}
			if uploadURL != "" || publicURL != "" {
				t.Errorf("a refused presign must return no URLs; got %q / %q", uploadURL, publicURL)
			}
		})
	}
}

// The guard above exists because of what the signer does without it. This pins
// that behaviour, so the day the SDK starts signing a zero length the guard can
// be reconsidered on evidence rather than removed on a hunch.
func TestUnguardedZeroSizeWouldSignNothingButTheHost(t *testing.T) {
	c := newPresignTestClient(t)

	input := &s3.PutObjectInput{
		Bucket:        aws.String("bucket"),
		Key:           aws.String("complexes/x/logo/a.webp"),
		ContentType:   aws.String("image/webp"),
		ContentLength: aws.Int64(0),
	}
	presigned, err := c.presignClient.PresignPutObject(context.Background(), input, s3.WithPresignExpires(10*time.Minute))
	if err != nil {
		t.Fatalf("presigning: %v", err)
	}
	u, err := url.Parse(presigned.URL)
	if err != nil {
		t.Fatalf("the presigned URL does not parse: %v", err)
	}

	signed := u.Query().Get("X-Amz-SignedHeaders")
	if slices.Contains(strings.Split(signed, ";"), "content-type") {
		t.Fatalf("the SDK now signs the content type at a zero length (signed: %q); "+
			"GeneratePresignedPUT's non-positive-size guard can be revisited", signed)
	}
}

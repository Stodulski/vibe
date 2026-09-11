package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Client wraps the S3-compatible client used to upload and manage files in Cloudflare R2.
type R2Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBaseURL string
}

// NewR2Client builds an R2Client authenticated against the given Cloudflare R2 account and bucket.
func NewR2Client(accountID, accessKey, secretKey, bucketName, publicBaseURL string) (*R2Client, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg := aws.Config{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &R2Client{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        bucketName,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}, nil
}

// GeneratePresignedPUT returns a time-limited presigned upload URL for key and the public URL it will be served from once uploaded.
//
// contentType and sizeBytes become part of the SigV4 signature — they appear in
// X-Amz-SignedHeaders — so the upload fails unless it declares exactly these.
// sizeBytes is an exact length rather than a maximum, which is why it is not
// called one: a URL signed with a 5 MB ceiling accepts only a 5 MB upload.
//
// An empty contentType or a non-positive sizeBytes is refused rather than
// signed. The AWS signer drops a header it has no value for, and dropping
// content-length takes content-type with it: a zero size yields a URL signed
// over nothing but the host, which accepts any bytes of any type under a key
// the caller chose. That URL is indistinguishable from an enforcing one at the
// call site, so the caller would believe a restriction it does not have.
func (r *R2Client) GeneratePresignedPUT(ctx context.Context, key, contentType string, sizeBytes int64, ttl time.Duration) (string, string, error) {
	if contentType == "" {
		return "", "", fmt.Errorf("presign put %q: a content type is required, it is what the signature enforces", key)
	}
	if sizeBytes <= 0 {
		return "", "", fmt.Errorf("presign put %q: size must be positive, got %d; a non-positive size signs away both the length and the type restriction", key, sizeBytes)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(sizeBytes),
	}

	presigned, err := r.presignClient.PresignPutObject(ctx, input, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", "", fmt.Errorf("presign put: %w", err)
	}

	publicURL := r.publicBaseURL + "/" + key
	return presigned.URL, publicURL, nil
}

// DeleteObject removes the object at key from the bucket.
func (r *R2Client) DeleteObject(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}
	return nil
}

// KeyFromPublicURL extracts the object key from a public R2 URL, reporting false if the URL is not under this client's public base URL.
func (r *R2Client) KeyFromPublicURL(publicURL string) (string, bool) {
	if r.publicBaseURL == "" || publicURL == "" {
		return "", false
	}
	prefix := r.publicBaseURL + "/"
	if !strings.HasPrefix(publicURL, prefix) {
		return "", false
	}
	key := strings.TrimPrefix(publicURL, prefix)
	if key == "" {
		return "", false
	}
	return key, true
}

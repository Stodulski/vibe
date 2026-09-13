package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// r2RequestTimeout bounds one round trip to R2, transport and body included.
//
// It exists because the alternative is not "a longer timeout", it is none at
// all (S3-05). An aws.Config with no HTTPClient gets the SDK's default one,
// whose Timeout is zero: a request that gets a socket and then no bytes hangs
// until the caller's context expires, and the callers here are handlers
// deleting a court photo — several of which run under contexts measured in
// seconds and one of which, the cleanup that follows a court delete, runs
// detached. Every other outbound client in this repository fixes a timeout
// explicitly; this one was the exception.
//
// Fifteen seconds is generous for an object delete against an edge network and
// short enough that a stalled request fails inside a request budget rather
// than outliving it.
const r2RequestTimeout = 15 * time.Second

// r2MaxAttempts is how many times one operation is sent before the failure
// reaches the caller.
//
// Three, and named here rather than left to the SDK's default of three,
// because "the same as the default" and "nobody decided" are different states
// and only one of them survives an SDK upgrade. The retryer this configures
// retries transport failures, 5xx and throttling — never a 403 or a 404, which
// are answers about this object.
const r2MaxAttempts = 3

// R2Client wraps the S3-compatible client used to upload and manage files in Cloudflare R2.
type R2Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBaseURL string
}

// NewR2Client builds an R2Client authenticated against the given Cloudflare R2 account and bucket.
func NewR2Client(accountID, accessKey, secretKey, bucketName, publicBaseURL string) (*R2Client, error) {
	return newR2Client(accountID, accessKey, secretKey, bucketName, publicBaseURL,
		&http.Client{Timeout: r2RequestTimeout})
}

// newR2Client is NewR2Client with the HTTP client supplied, so a test can put
// a transport in front of the SDK and watch what it actually does — how many
// attempts one operation costs, and which failures it declines to retry.
// Those are the two properties this constructor exists to fix, and neither is
// observable from the outside.
func newR2Client(accountID, accessKey, secretKey, bucketName, publicBaseURL string, httpClient aws.HTTPClient) (*R2Client, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg := aws.Config{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
		HTTPClient:   httpClient,
		// A function, not a value: the SDK calls it per operation because a
		// standard retryer carries a rate-limiting token bucket, and sharing
		// one across every operation lets a burst of failures on one starve
		// the retries of another.
		Retryer: func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = r2MaxAttempts
			})
		},
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

// PutObject uploads obj into the bucket from this process.
//
// The body travels as a bytes.Reader rather than an io.Reader the caller
// keeps: the SDK's retryer re-sends a failed request, and a stream that has
// already been consumed once re-sends as an empty body — a zero-length object
// written over a good one, reported as success. A Reader over a slice can
// seek back to the start, so a retry sends the same bytes.
//
// An empty key or content type is refused rather than uploaded. R2 accepts an
// object with no declared type and serves it as an opaque stream, which is
// how a spreadsheet arrives in a browser as an unnamed download the operating
// system cannot open.
func (r *R2Client) PutObject(ctx context.Context, obj Object) error {
	if obj.Key == "" {
		return fmt.Errorf("put object: a key is required")
	}
	if obj.ContentType == "" {
		return fmt.Errorf("put object %q: a content type is required, it is what decides how the object is served", obj.Key)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(obj.Key),
		Body:          bytes.NewReader(obj.Body),
		ContentType:   aws.String(obj.ContentType),
		ContentLength: aws.Int64(int64(len(obj.Body))),
	}
	if obj.ContentDisposition != "" {
		input.ContentDisposition = aws.String(obj.ContentDisposition)
	}

	if _, err := r.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put object %q: %w", obj.Key, err)
	}
	return nil
}

// GeneratePresignedGET returns a signed URL that reads key for ttl.
//
// This is what makes a private bucket usable: the object is unreadable
// without a signature, the signature expires, and it is minted only after the
// caller has proved they own the complex the object belongs to. An
// unguessable key in a public bucket would be none of those things — it is a
// bearer token with no expiry that leaks through browser history, Referer
// headers and forwarded messages.
//
// downloadName is signed as ResponseContentDisposition rather than merely
// appended, because an unsigned query parameter is one R2 rejects: every
// response-override parameter is part of the SigV4 canonical query string.
//
// An empty key is refused because a signature over the bucket alone is not a
// URL anybody can use, and a non-positive ttl because the SDK reads it as
// "expire immediately" — a URL that is dead on arrival and reports itself as
// a 403 from R2 rather than as an error here.
func (r *R2Client) GeneratePresignedGET(ctx context.Context, key, downloadName string, ttl time.Duration) (string, error) {
	if key == "" {
		return "", fmt.Errorf("presign get: a key is required")
	}
	if ttl <= 0 {
		return "", fmt.Errorf("presign get %q: ttl must be positive, got %s; a URL signed to expire now is refused rather than handed out", key, ttl)
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	}
	if downloadName != "" {
		input.ResponseContentDisposition = aws.String(contentDisposition(downloadName))
	}

	presigned, err := r.presignClient.PresignGetObject(ctx, input, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get %q: %w", key, err)
	}
	return presigned.URL, nil
}

// contentDisposition is the attachment header a download is stored and served
// with. It is one function so the object's own disposition and the one signed
// into a presigned GET cannot spell the same file two ways.
//
// The name is quoted and any quote or backslash inside it removed: a filename
// carrying a `"` would otherwise close the quoted-string early and let the
// rest of the name be read as further header parameters.
func contentDisposition(downloadName string) string {
	safe := strings.NewReplacer(`"`, "", `\`, "", "\r", "", "\n", "").Replace(downloadName)
	return fmt.Sprintf(`attachment; filename="%s"`, safe)
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

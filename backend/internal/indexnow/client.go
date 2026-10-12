package indexnow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/jobs"
)

// endpoint is the IndexNow submission endpoint the participating engines share.
const endpoint = "https://api.indexnow.org/indexnow"

// requestTimeout bounds one submission, so a slow endpoint fails the attempt
// instead of holding a queue worker.
const requestTimeout = 10 * time.Second

// Client posts submissions to IndexNow through a circuit breaker.
type Client struct {
	endpoint    string
	host        string
	keyLocation string
	http        *http.Client
	cb          *circuitbreaker.CircuitBreaker
}

// NewClient returns a Client for the site at origin. cb may be nil.
func NewClient(origin string, cb *circuitbreaker.CircuitBreaker) *Client {
	base := strings.TrimRight(origin, "/")
	host := ""
	if u, err := url.Parse(base); err == nil {
		host = u.Host
	}
	return &Client{
		endpoint:    endpoint,
		host:        host,
		keyLocation: base + "/" + Key + ".txt",
		http:        &http.Client{Timeout: requestTimeout},
		cb:          cb,
	}
}

// submitBody is the request body the IndexNow protocol defines.
type submitBody struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation"`
	URLList     []string `json:"urlList"`
}

// Submit posts urls. A 200 or 202 is success. A 429, a 5xx and a connection that
// never answers are transient. Any other 4xx is permanent (see statusError).
func (c *Client) Submit(ctx context.Context, urls []string) error {
	raw, err := json.Marshal(submitBody{Host: c.host, Key: Key, KeyLocation: c.keyLocation, URLList: urls})
	if err != nil {
		return fmt.Errorf("indexnow: encode submission: %w", jobs.ErrPermanent)
	}
	if err := c.cb.AllowRequest(); err != nil {
		return fmt.Errorf("indexnow: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("indexnow: build request: %w", jobs.ErrPermanent)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		c.cb.RecordFailure()
		return fmt.Errorf("indexnow: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		c.cb.RecordSuccess()
		return nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		c.cb.RecordFailure()
	case resp.StatusCode >= 400:
		// The endpoint answered and refused this request, so the endpoint itself is healthy.
		c.cb.RecordSuccess()
	default:
		c.cb.RecordFailure()
	}
	return &statusError{code: resp.StatusCode}
}

// statusError is an answer from the endpoint that was not a success.
type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("indexnow: endpoint answered %d", e.code) }

// Permanent reports whether sending the same request again would get the same
// answer. A 4xx other than 429 is the endpoint refusing this request, such as a
// key it cannot verify or a malformed body.
func (e *statusError) Permanent() bool {
	return e.code >= 400 && e.code < 500 && e.code != http.StatusTooManyRequests
}

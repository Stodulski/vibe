package publicsite

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxTemplateBytes caps the index.html read, so a misconfigured frontend URL
// pointing at something large cannot exhaust memory.
const maxTemplateBytes = 512 * 1024

// templateCache holds the frontend's index.html, which prerendering rewrites.
//
// A stale copy is always preferred to an error: if the frontend is briefly
// unreachable, serving last-known markup to a crawler is far better than
// serving it a 500, which can cost the page its index entry.
type templateCache struct {
	mu        sync.RWMutex
	template  string
	fetchedAt time.Time
	ttl       time.Duration
	client    *http.Client
}

func newTemplateCache(ttl, timeout time.Duration) *templateCache {
	return &templateCache{ttl: ttl, client: &http.Client{Timeout: timeout}}
}

// fresh reports whether the cached copy is still within its TTL. Callers must
// hold at least the read lock.
func (tc *templateCache) fresh() bool {
	return tc.template != "" && time.Since(tc.fetchedAt) < tc.ttl
}

// get returns the cached template, fetching it from frontendURL when stale.
func (tc *templateCache) get(ctx context.Context, frontendURL string) (string, error) {
	tc.mu.RLock()
	if tc.fresh() {
		t := tc.template
		tc.mu.RUnlock()
		return t, nil
	}
	tc.mu.RUnlock()

	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Another goroutine may have refreshed it while this one waited.
	if tc.fresh() {
		return tc.template, nil
	}

	body, err := tc.fetch(ctx, frontendURL)
	if err != nil {
		if tc.template != "" {
			return tc.template, nil
		}
		return "", err
	}

	tc.template = body
	tc.fetchedAt = time.Now()
	return tc.template, nil
}

// fetch retrieves index.html from the frontend origin. frontendURL is operator
// configuration, never request input.
func (tc *templateCache) fetch(ctx context.Context, frontendURL string) (string, error) {
	url := strings.TrimRight(frontendURL, "/") + "/index.html"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building the index.html request: %w", err)
	}

	resp, err := tc.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching index.html from the frontend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A non-2xx body is a page, not the template. Without this check the
	// frontend's 404 or its CDN's 503 page arrives as a successful fetch: it
	// replaces the good cached copy, is served to crawlers under every
	// complex's canonical URL with a 200 and a public cache header, and is
	// kept for the full TTL. It also defeats the stale-is-better-than-an-error
	// fallback below, which only runs when the fetch reports an error — so the
	// one case that rule was written for is the one case it never sees.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("fetching index.html from the frontend: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTemplateBytes))
	if err != nil {
		return "", fmt.Errorf("reading index.html: %w", err)
	}

	return string(body), nil
}

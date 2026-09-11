package httpx

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnErrorLogNeverCarriesAQueryValue is the test for what the error log is
// allowed to write down about a request.
//
// A booking link carries its credential in the query string — internal/booklink
// mints an opaque token and the three public routes read it from ?token=. The
// error log used to write r.URL.RequestURI() whole, so every error on one of
// those routes — a 404 for a token that had already expired, a 500 from any
// store behind it — put a live bearer token into the log in plaintext. A log is
// the wrong place for a credential twice over: it outlives the request by
// however long logs are kept, and it is readable by everyone who can read logs,
// which is a wider set than everyone who may hold the booking.
//
// The three assertions are one requirement each, and dropping any of them makes
// the test pass against a wrong fix:
//
//   - the secret must be absent. That is the defect.
//   - the KEY must be present. Otherwise a change that logs nothing at all
//     passes, and the debugging value of the line goes with it.
//   - the path must be present, for the same reason.
func TestAnErrorLogNeverCarriesAQueryValue(t *testing.T) {
	const secret = "tok_live_9f3c1aa4de2b47e8b0c5"

	var buf bytes.Buffer
	rs := NewResponder(slog.New(slog.NewTextHandler(&buf, nil)))

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/v1/public/bookings/detail?token="+secret+"&debug=1", nil)
	rs.ServerError(httptest.NewRecorder(), r, errors.New("the store could not answer"))

	logged := buf.String()
	if logged == "" {
		t.Fatal("ServerError wrote no log line at all")
	}

	if strings.Contains(logged, secret) {
		t.Errorf("the booking link's token reached the log in plaintext:\n%s", logged)
	}
	if !strings.Contains(logged, "token=REDACTED") {
		t.Errorf("the query key is gone as well as its value; a redacted line still has to say "+
			"which parameters were sent:\n%s", logged)
	}
	if !strings.Contains(logged, "/api/v1/public/bookings/detail") {
		t.Errorf("the path is not in the line, so nothing says which handler failed:\n%s", logged)
	}
}

// TestRedactedQueryKeepsEveryKeyAndNoValue covers redactedQuery itself on the
// shapes a request can actually arrive in — including the two that a
// value-dropping implementation is most likely to get wrong.
func TestRedactedQueryKeepsEveryKeyAndNoValue(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"no query", "/x", ""},
		{"one parameter", "/x?token=secret", "token=REDACTED"},
		// Sorted, so one request shape reads the same way in every line.
		{"several, out of order", "/x?z=3&a=1&m=2", "a=REDACTED&m=REDACTED&z=REDACTED"},
		// A repeated key is one key, not one entry per value — otherwise the
		// number of values leaks through the line's length.
		{"a repeated key", "/x?id=1&id=2&id=3", "id=REDACTED"},
		// A valueless key is still a key.
		{"a bare key", "/x?verbose", "verbose=REDACTED"},
		// A query that will not parse must not fall back to printing itself.
		{"unparseable", "/x?%zz", "(unparseable)"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, c.raw, nil)
			if got := redactedQuery(r.URL); got != c.want {
				t.Errorf("redactedQuery(%q) = %q; want %q", c.raw, got, c.want)
			}
		})
	}
}

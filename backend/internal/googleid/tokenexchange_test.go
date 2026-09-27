package googleid

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newExchanger(t *testing.T, tokenURL string) *CodeExchanger {
	t.Helper()
	// A nil breaker: every method on *circuitbreaker.CircuitBreaker is safe on
	// a nil receiver (see internal/circuitbreaker), so these tests stay
	// independent of its own failure-counting behaviour.
	return NewCodeExchanger(ExchangeConfig{
		ClientID:     testClientID,
		ClientSecret: "test-client-secret",
		TokenURL:     tokenURL,
	})
}

// tokenServer answers the same body+status to every request, and records the
// form values the last request carried — so a test can assert exactly what
// Exchange sent Google, in one place, without a bespoke handler per case.
type tokenServer struct {
	*httptest.Server
	status int
	body   string
	form   url.Values
}

func newTokenServer(t *testing.T, status int, body string) *tokenServer {
	t.Helper()
	s := &tokenServer{status: status, body: body}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			s.form = r.PostForm
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.status)
		_, _ = w.Write([]byte(s.body))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestExchangeSuccess(t *testing.T) {
	srv := newTokenServer(t, http.StatusOK, `{"id_token":"the-id-token","access_token":"unused"}`)
	c := newExchanger(t, srv.URL)

	idToken, err := c.Exchange(context.Background(), "the-code", "the-verifier", "https://vibe.test/auth/google/callback")
	if err != nil {
		t.Fatalf("Exchange() = %v, want nil", err)
	}
	if idToken != "the-id-token" {
		t.Errorf("idToken = %q, want %q", idToken, "the-id-token")
	}

	want := map[string]string{
		"grant_type":    "authorization_code",
		"code":          "the-code",
		"client_id":     testClientID,
		"client_secret": "test-client-secret",
		"redirect_uri":  "https://vibe.test/auth/google/callback",
		"code_verifier": "the-verifier",
	}
	for key, val := range want {
		if got := srv.form.Get(key); got != val {
			t.Errorf("form[%q] = %q, want %q", key, got, val)
		}
	}
}

func TestExchangeInvalidGrant(t *testing.T) {
	srv := newTokenServer(t, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Malformed auth code."}`)
	c := newExchanger(t, srv.URL)

	_, err := c.Exchange(context.Background(), "spent-code", "verifier", "https://vibe.test/auth/google/callback")
	if !errors.Is(err, ErrCodeRejected) {
		t.Fatalf("Exchange() = %v, want an error wrapping ErrCodeRejected", err)
	}
}

func TestExchangeServerError(t *testing.T) {
	srv := newTokenServer(t, http.StatusInternalServerError, `{}`)
	c := newExchanger(t, srv.URL)

	_, err := c.Exchange(context.Background(), "code", "verifier", "https://vibe.test/auth/google/callback")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Exchange() = %v, want an error wrapping ErrUnavailable", err)
	}
}

func TestExchangeMissingIDToken(t *testing.T) {
	srv := newTokenServer(t, http.StatusOK, `{"access_token":"no-id-token-here"}`)
	c := newExchanger(t, srv.URL)

	_, err := c.Exchange(context.Background(), "code", "verifier", "https://vibe.test/auth/google/callback")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Exchange() = %v, want an error wrapping ErrUnavailable for a response with no id_token", err)
	}
}

func TestExchangeOversizedResponse(t *testing.T) {
	// A valid id_token field followed by more padding than the reader will
	// take: the decode stops at the cap and fails instead of buffering it all.
	padding := strings.Repeat("x", tokenResponseMaxBytes)
	srv := newTokenServer(t, http.StatusOK, `{"id_token":"the-id-token","padding":"`+padding+`"}`)
	c := newExchanger(t, srv.URL)

	_, err := c.Exchange(context.Background(), "the-code", "the-verifier", "https://vibe.test/auth/google/callback")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Exchange() = %v, want an error wrapping ErrUnavailable for an oversized response", err)
	}
}

func TestExchangeUnreachable(t *testing.T) {
	c := newExchanger(t, "http://127.0.0.1:0")

	_, err := c.Exchange(context.Background(), "code", "verifier", "https://vibe.test/auth/google/callback")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Exchange() = %v, want an error wrapping ErrUnavailable", err)
	}
}

func TestExchangeContextCanceled(t *testing.T) {
	srv := newTokenServer(t, http.StatusOK, `{"id_token":"unused"}`)
	c := newExchanger(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Exchange(ctx, "code", "verifier", "https://vibe.test/auth/google/callback")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Exchange() = %v, want an error wrapping ErrUnavailable", err)
	}
}

func TestCodeExchangerEnabled(t *testing.T) {
	cases := []struct {
		name         string
		clientID     string
		clientSecret string
		want         bool
	}{
		{"both set", testClientID, "secret", true},
		{"no secret", testClientID, "", false},
		{"no client id", "", "secret", false},
		{"neither", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCodeExchanger(ExchangeConfig{ClientID: tc.clientID, ClientSecret: tc.clientSecret})
			if got := c.Enabled(); got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// request builds a request from a peer address and a set of headers.
func request(t *testing.T, remoteAddr string, headers map[string]string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	return r
}

// mustParse is the trusted-proxy set a test declares inline.
func mustParse(t *testing.T, spec string) TrustedProxies {
	t.Helper()
	tp, err := ParseTrustedProxies(spec)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", spec, err)
	}
	return tp
}

// ---------------------------------------------------------------------------
// The two live attacks
// ---------------------------------------------------------------------------

// Attack one: rotate X-Forwarded-For and mint a fresh rate-limit bucket per
// request. The header used to be read leftmost-first, so whatever the caller
// typed became the limiter key and every limiter was a formality.
func TestARotatedForwardedHeaderCannotMintFreshRateLimitBuckets(t *testing.T) {
	trusted := mustParse(t, "true")

	first := ClientIPFrom(request(t, "10.0.0.1:443", map[string]string{
		"X-Forwarded-For": "1.1.1.1, 203.0.113.7",
	}), trusted)

	second := ClientIPFrom(request(t, "10.0.0.1:443", map[string]string{
		"X-Forwarded-For": "2.2.2.2, 203.0.113.7",
	}), trusted)

	if first != "203.0.113.7" || second != "203.0.113.7" {
		t.Errorf("the entry the trusted proxy appended is the client; got %q and %q", first, second)
	}
}

// Attack two: set X-Forwarded-For to somebody else's address and fill their
// bucket. Aimed at an office's egress address it locks every employee out of
// login, and nothing in the request identifies who did it.
func TestAForgedForwardedHeaderCannotBeAimedAtAVictim(t *testing.T) {
	got := ClientIPFrom(request(t, "10.0.0.1:443", map[string]string{
		"X-Forwarded-For": "198.51.100.99, 203.0.113.7",
	}), mustParse(t, "true"))

	if got == "198.51.100.99" {
		t.Error("an address the client typed must never become the rate-limit key: " +
			"that is how one caller locks out another")
	}
	if got != "203.0.113.7" {
		t.Errorf("want the real peer of the proxy, 203.0.113.7; got %q", got)
	}
}

// Unparseable values used to be stored verbatim and reached audit_log.ip_address
// as NULL: a garbage header erased attribution without erroring anywhere.
// Everything that comes back now parses as an address.
func TestAnUnparseableForwardedValueFallsBackToThePeer(t *testing.T) {
	tests := []string{
		"not-an-address",
		"<script>alert(1)</script>",
		"; DROP TABLE audit_log",
		strings.Repeat("9", 400),
		"",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			got := ClientIPFrom(request(t, "10.0.0.1:443", map[string]string{
				"X-Forwarded-For": value,
			}), mustParse(t, "true"))

			if got != "10.0.0.1" {
				t.Errorf("an unusable header must fall back to the peer address; got %q", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The walk
// ---------------------------------------------------------------------------

func TestClientIPFrom(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		trusted    string
		want       string
	}{
		{
			name:       "direct connection, nothing trusted",
			remoteAddr: "203.0.113.7:54321",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8"},
			trusted:    "false",
			want:       "203.0.113.7",
		},
		{
			name:       "a chain of trusted proxies is walked back to the client",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.2, 10.0.0.1"},
			trusted:    "true",
			want:       "203.0.113.7",
		},
		{
			name:       "one forwarded value from a trusted peer",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": " 203.0.113.7 "},
			trusted:    "true",
			want:       "203.0.113.7",
		},
		{
			name:       "X-Real-IP when there is no chain",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Real-IP": "203.0.113.9"},
			trusted:    "true",
			want:       "203.0.113.9",
		},
		{
			// The proxy sits on a public address that the set names.
			name:       "an explicitly named public proxy",
			remoteAddr: "198.51.100.10:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.7"},
			trusted:    "198.51.100.10",
			want:       "203.0.113.7",
		},
		{
			// The same proxy, not named. Falling back to the balancer is
			// wrong, but it is not attacker-chosen — the whole reason for
			// preferring a set over a hop count.
			name:       "a public proxy the set does not name",
			remoteAddr: "198.51.100.10:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.7"},
			trusted:    "true",
			want:       "198.51.100.10",
		},
		{
			name:       "every entry is a trusted proxy, so the client is not in the header",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "10.0.0.3, 10.0.0.2"},
			trusted:    "true",
			want:       "10.0.0.1",
		},
		{
			name:       "a proxy that appends a port",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.7:51234"},
			trusted:    "true",
			want:       "203.0.113.7",
		},
		{
			name:       "an IPv4-mapped IPv6 entry is canonicalised",
			remoteAddr: "10.0.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "::ffff:203.0.113.7"},
			trusted:    "true",
			want:       "203.0.113.7",
		},
		{
			name:       "an IPv6 peer behind an IPv6 proxy",
			remoteAddr: "[fd00::1]:443",
			headers:    map[string]string{"X-Forwarded-For": "2001:db8::5, fd00::2"},
			trusted:    "true",
			want:       "2001:db8::5",
		},
		{
			name:       "X-Real-IP is ignored when the peer is untrusted",
			remoteAddr: "203.0.113.7:54321",
			headers:    map[string]string{"X-Real-IP": "198.51.100.99"},
			trusted:    "true",
			want:       "203.0.113.7",
		},
		{
			name:       "unparseable RemoteAddr falls through",
			remoteAddr: "/tmp/app.sock",
			trusted:    "false",
			want:       "/tmp/app.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClientIPFrom(request(t, tt.remoteAddr, tt.headers), mustParse(t, tt.trusted))
			if got != tt.want {
				t.Errorf("ClientIPFrom() = %q; want %q", got, tt.want)
			}
		})
	}
}

// The header may legitimately arrive more than once; the entries concatenate,
// and a walk that only read the last header would take the wrong entry.
func TestARepeatedForwardedHeaderIsOneChain(t *testing.T) {
	r := request(t, "10.0.0.1:443", nil)
	r.Header.Add("X-Forwarded-For", "203.0.113.7")
	r.Header.Add("X-Forwarded-For", "10.0.0.2")

	if got := ClientIPFrom(r, mustParse(t, "true")); got != "203.0.113.7" {
		t.Errorf("want the client from the concatenated chain; got %q", got)
	}
}

// A padded header is work an unauthenticated caller chooses the size of, so
// the walk is bounded. Past the bound the answer is the peer, never an entry
// the caller supplied.
func TestAPaddedForwardedHeaderIsBounded(t *testing.T) {
	padding := strings.Repeat("10.0.0.9, ", maxForwardedHops+50)
	got := ClientIPFrom(request(t, "10.0.0.1:443", map[string]string{
		"X-Forwarded-For": "203.0.113.7, " + padding + "10.0.0.2",
	}), mustParse(t, "true"))

	if got != "10.0.0.1" {
		t.Errorf("a header longer than the bound must fall back to the peer; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// The setting
// ---------------------------------------------------------------------------

func TestParseTrustedProxies(t *testing.T) {
	t.Run("the default trusts nobody", func(t *testing.T) {
		tp, err := ParseTrustedProxies("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tp.Any() {
			t.Error("an unset trusted-proxy setting must trust nothing: with no proxy in front, " +
				"the forwarded headers are attacker-supplied")
		}
	})

	t.Run("true resolves to the private ranges", func(t *testing.T) {
		tp := mustParse(t, "true")
		if !tp.TrustsPeer(request(t, "10.0.0.1:443", nil)) {
			t.Error("10.0.0.1 is an RFC 1918 address and must be trusted by the default set")
		}
		if tp.TrustsPeer(request(t, "203.0.113.7:443", nil)) {
			t.Error("a public peer must not be trusted by the default set")
		}
	})

	t.Run("an explicit set", func(t *testing.T) {
		tp := mustParse(t, "198.51.100.0/24, 2001:db8::1")
		if !tp.TrustsPeer(request(t, "198.51.100.10:443", nil)) {
			t.Error("an address inside a named prefix must be trusted")
		}
		if !tp.TrustsPeer(request(t, "[2001:db8::1]:443", nil)) {
			t.Error("a bare address is a single-host prefix")
		}
		if tp.TrustsPeer(request(t, "10.0.0.1:443", nil)) {
			t.Error("an explicit set replaces the private ranges rather than extending them")
		}
	})

	t.Run("a set that trusts every peer is refused at boot", func(t *testing.T) {
		for _, spec := range []string{"0.0.0.0/0", "::/0", "10.0.0.0/8, 0.0.0.0/0"} {
			if _, err := ParseTrustedProxies(spec); !errors.Is(err, ErrTrustsEveryPeer) {
				t.Errorf("%q must be refused: it is indistinguishable from having no check at all; got %v", spec, err)
			}
		}
	})

	t.Run("nonsense is refused rather than ignored", func(t *testing.T) {
		for _, spec := range []string{"not-an-address", "10.0.0.0/64", "10.0.0.0/8, oops"} {
			if _, err := ParseTrustedProxies(spec); err == nil {
				t.Errorf("%q must fail at boot, not silently trust nothing at runtime", spec)
			}
		}
	})
}

// ClientIP is the boolean entry point the audit-log call sites still take. It
// must resolve to the same set, or the rate limiter and the audit trail
// disagree about who made the request.
func TestClientIPBooleanFormMatchesTheDefaultSet(t *testing.T) {
	r := request(t, "10.0.0.1:443", map[string]string{"X-Forwarded-For": "198.51.100.99, 203.0.113.7"})

	if got, want := ClientIP(r, true), ClientIPFrom(r, DefaultTrustedProxies()); got != want {
		t.Errorf("ClientIP(r, true) = %q; DefaultTrustedProxies gives %q", got, want)
	}
	if got := ClientIP(r, false); got != "10.0.0.1" {
		t.Errorf("ClientIP(r, false) must ignore the headers entirely; got %q", got)
	}
}

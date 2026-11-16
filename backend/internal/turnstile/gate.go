package turnstile

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Verifier is what a Gate needs from a Client. It is declared here, by the
// consumer, and Client satisfies it.
type Verifier interface {
	// Enabled reports whether verification is configured at all. When it is
	// not, Verify must not be called.
	Enabled() bool
	// Verify checks token and reports a rejection through the package's
	// sentinels (ErrMissingToken, ErrInvalidToken, ErrUnavailable).
	Verify(ctx context.Context, token, remoteIP string) error
}

// Gate is the storefront's bot check for the public routes that take a slot or
// a spot: the booking hold, the open-play sign-up and the tournament pair
// sign-up all ask the same Gate under the same rule, so they cannot drift.
type Gate struct {
	verifier Verifier
	enforce  bool
	respond  *httpx.Responder
	logger   *slog.Logger
	proxies  httpx.TrustedProxies
}

// NewGate returns a Gate. verifier may be nil, and enforce false (the operator
// has not set TURNSTILE_BOOKING_ENFORCE) leaves every check a no-op, as does a
// verifier that is not Enabled (no TURNSTILE_SECRET_KEY).
func NewGate(verifier Verifier, enforce bool, respond *httpx.Responder, logger *slog.Logger, proxies httpx.TrustedProxies) *Gate {
	return &Gate{verifier: verifier, enforce: enforce, respond: respond, logger: logger, proxies: proxies}
}

// Check verifies token when the operator opted in and a secret is configured.
// It reports whether the request may go on; when it returns false it has
// already written the 422, `turnstile_token` as `required` or `invalid`, in the
// shape and codes auth uses. A nil Gate lets every request through.
//
// Cloudflare being unreachable fails open, as login does: a storefront that
// cannot take bookings while Cloudflare is down costs the venue real money, and
// the rate limiter still bounds what an unchecked request can do.
func (g *Gate) Check(w http.ResponseWriter, r *http.Request, token string) bool {
	if g == nil || !g.enforce || g.verifier == nil || !g.verifier.Enabled() {
		return true
	}

	v := validator.New()
	if token == "" {
		v.AddError("turnstile_token", "required")
		g.respond.FailedValidation(w, r, v.Errors)
		return false
	}

	err := g.verifier.Verify(r.Context(), token, httpx.ClientIPFrom(r, g.proxies))
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrUnavailable):
		g.logger.Warn("turnstile: verification unavailable, request allowed", "path", r.URL.Path, "error", err)
		return true
	case errors.Is(err, ErrMissingToken):
		v.AddError("turnstile_token", "required")
	default:
		g.logger.Warn("turnstile: token rejected", "path", r.URL.Path, "error", err)
		v.AddError("turnstile_token", "invalid")
	}
	g.respond.FailedValidation(w, r, v.Errors)
	return false
}

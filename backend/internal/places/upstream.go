package places

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// Places API (New) statuses this proxy treats specially. Every other status
// (INVALID_ARGUMENT, UNAVAILABLE, INTERNAL, an unrecognized value, or none at
// all) falls through to the generic "upstream is unavailable" 502 — this
// proxy has no useful way to tell those apart for the browser.
const (
	apiStatusResourceExhausted = "RESOURCE_EXHAUSTED"
	apiStatusPermissionDenied  = "PERMISSION_DENIED"
	apiStatusNotFound          = "NOT_FOUND"
)

// maxResponseBytes caps what is read from an upstream response. Both endpoints
// return a handful of addresses; anything approaching this is a broken or
// hostile upstream, and reading it unbounded would let one reply exhaust the
// process.
const maxResponseBytes = 1 << 20 // 1 MiB

// maxSnippetBytes caps how much of a failed upstream body — or its "message"
// field — is carried into the log line, which only needs enough to identify
// the failure.
const maxSnippetBytes = 512

// redactedKey replaces the API key wherever it would otherwise be written out.
const redactedKey = "[REDACTED_PLACES_KEY]"

// googleErrorEnvelope is the error body Places API (New) sends alongside a
// non-2xx HTTP status: {"error": {"code": ..., "message": ..., "status": ...}}.
type googleErrorEnvelope struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// upstreamError is a Places call that did not produce a usable answer,
// carrying both the HTTP status and Google's own "status" string so the log
// line names the real cause (PERMISSION_DENIED — the API is not enabled for
// this key — reads very differently from an outage) and the caller gets a
// status that distinguishes the two sides.
type upstreamError struct {
	// httpStatus is what the upstream answered with.
	httpStatus int
	// apiStatus is Places API (New)'s "error.status" field, e.g.
	// PERMISSION_DENIED, RESOURCE_EXHAUSTED, NOT_FOUND, INVALID_ARGUMENT.
	// Empty when the body carried no such envelope (an outage, a proxy
	// error, an unreadable body).
	apiStatus string
	// detail is the upstream's own explanation, truncated.
	detail string
}

func (e *upstreamError) Error() string {
	msg := fmt.Sprintf("places upstream: http %d", e.httpStatus)
	if e.apiStatus != "" {
		msg += ", status " + e.apiStatus
	}
	if e.detail != "" {
		msg += ": " + e.detail
	}
	return msg
}

// refusal is the status and message the browser receives for this failure.
//
// Collapsing every upstream outcome onto one status hides which side broke: a
// 200 with an empty list says the address simply does not exist, and a 500 says
// this service has a bug. Neither is true when Google is rate limiting us or is
// down, and a caller told the wrong thing retries the wrong way — or gives up
// on an address that is fine.
//
// The message describes the dependency rather than naming Google, because which
// geocoder is behind this proxy is not the browser's business. It travels with
// the status because the two were derived from the same three cases and could
// not be allowed to disagree about which case this is.
func (e *upstreamError) refusal() httpx.Refusal {
	switch {
	case e.apiStatus == apiStatusResourceExhausted, e.httpStatus == upstreamRateLimited:
		// The platform's own quota, not the caller's — but backing off is
		// still the only useful response, and 429 is the only status that
		// says so.
		return httpx.TooManyRequests("the address lookup service is rate limited, please retry shortly")
	case e.apiStatus == apiStatusNotFound:
		// Only Details can produce this, and only for a place_id the caller
		// supplied, so it is the caller's input that is wrong.
		return httpx.NotFound("the requested place could not be found")
	case e.apiStatus == apiStatusPermissionDenied:
		// The key's Cloud project does not have Places API (New) enabled (or
		// the key is otherwise restricted against this call) — still this
		// service failing to get an answer out of a dependency, so the
		// browser gets the same 502 as any other upstream failure. Kept as
		// its own case, rather than falling into default, so
		// e.apiStatus's logged value and this branch both name the cause an
		// operator actually needs: "the key isn't enabled for this API",
		// distinguishable in logs from a plain outage.
		return httpx.BadGateway(upstreamUnavailableMessage)
	default:
		// Everything else — 5xx, an unreadable body, an unrecognized status —
		// is this service failing to get an answer out of a dependency.
		return httpx.BadGateway(upstreamUnavailableMessage)
	}
}

// upstreamUnavailableMessage is what the browser is told for every failure that
// is this service's problem to solve rather than the caller's.
const upstreamUnavailableMessage = "the address lookup service is unavailable"

// upstreamRateLimited is the status Google answers with when it is throttling
// us. It is the upstream's status, not one this API chooses, which is why it is
// read here rather than built from internal/httpx.
const upstreamRateLimited = http.StatusTooManyRequests

// readBounded reads body up to maxResponseBytes. A hostile or broken upstream
// must not be able to make this process read an unbounded body.
func readBounded(body io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(body, maxResponseBytes))
}

// parseUpstreamError turns a non-2xx Places API (New) response into an
// *upstreamError, reading Google's {"error": {...}} envelope when the body
// carries one and falling back to a bounded snippet of the raw body when it
// does not (a proxy or load balancer error page, for instance).
func parseUpstreamError(httpStatus int, body []byte) error {
	var env googleErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil || env.Error.Status == "" {
		return &upstreamError{httpStatus: httpStatus, detail: snippet(body)}
	}
	return &upstreamError{
		httpStatus: httpStatus,
		apiStatus:  env.Error.Status,
		detail:     snippet([]byte(env.Error.Message)),
	}
}

// snippet trims an upstream body down to what a log line needs.
func snippet(body []byte) string {
	if len(body) > maxSnippetBytes {
		body = body[:maxSnippetBytes]
	}
	return strings.TrimSpace(string(body))
}

// sanitize returns err with the API key removed from its message.
//
// Places API (New) takes the key only as the X-Goog-Api-Key header, which
// net/http does not fold into a transport error's message the way it folded a
// legacy query-string key into a *url.Error's URL — so there should be
// nothing left to redact. This stays as a defense-in-depth pass regardless:
// a future error path (a redirect, a debug dump) could still carry the key,
// and every error leaving this package goes through here so that path cannot
// quietly reintroduce a leak.
func (h *Handler) sanitize(err error) error {
	if err == nil || h.cfg.APIKey == "" {
		return err
	}

	msg := err.Error()
	clean := strings.ReplaceAll(msg, h.cfg.APIKey, redactedKey)
	clean = strings.ReplaceAll(clean, url.QueryEscape(h.cfg.APIKey), redactedKey)
	if clean == msg {
		return err
	}

	// The wrapping is dropped deliberately: an error chain can only be
	// inspected by unwrapping to a concrete type, and every concrete type that
	// reached this branch is one that holds the key.
	return errors.New(clean)
}

// failUpstream answers a failed Places call, logging the cause with the key
// removed (see sanitize) and giving the caller the status that names which
// side failed.
func (h *Handler) failUpstream(w http.ResponseWriter, r *http.Request, err error) {
	h.respond.LogError(r, h.sanitize(err))

	var ue *upstreamError
	if !errors.As(err, &ue) {
		// A transport failure or an unreadable body: no answer came back, so
		// this service is a failing gateway, not a broken one.
		ue = &upstreamError{}
	}

	h.respond.Refuse(w, r, ue.refusal())
}

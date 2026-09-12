package httpx

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Responder writes the API's error responses. Every error the client sees goes
// through it, so the response shape and the log line stay in one place instead
// of being reconstructed at each call site.
type Responder struct {
	logger *slog.Logger
}

// NewResponder returns a Responder logging to the given logger.
func NewResponder(logger *slog.Logger) *Responder {
	return &Responder{logger: logger}
}

// JSON writes a successful response, turning a failed write into a logged 500.
//
// Callers otherwise repeat the same three lines at every success path — encode,
// check the error, hand it to ServerError — which is both noise and an easy
// place to forget the check. It also means a handler reaches for the Responder
// for every response it writes, rather than the error paths only.
func (rs *Responder) JSON(w http.ResponseWriter, r *http.Request, status int, data Envelope) {
	if err := WriteJSON(w, status, data, nil); err != nil {
		rs.ServerError(w, r, err)
	}
}

// LogError records err along with the request that produced it.
//
// It logs the path and the query's KEYS, never a query value. RequestURI()
// used to go in whole, and a booking link carries its credential in the query
// string (booklink.QueryParam): every error on one of those routes — a 404 for
// a token that had already expired, a 500 from any store behind it — wrote a
// live bearer token into the log in plaintext, where it outlives the request by
// however long logs are kept and is readable by anyone who can read them.
//
// Keys are kept because a key cannot be a secret and knowing which parameters
// were present is most of the debugging value. Values are dropped wholesale
// rather than by a list of the ones known to be sensitive: a list is a snapshot
// of what somebody remembered, and the next credential to travel in a query
// string would be logged until they remembered again. This way a new parameter
// is safe on the day it is added.
//
// The access log already logs r.URL.Path alone (internal/middleware/logging.go),
// so nothing here loses information that log still had.
func (rs *Responder) LogError(r *http.Request, err error) {
	rs.logger.Error("error",
		"error", err.Error(),
		"method", r.Method,
		"path", r.URL.Path,
		"query", redactedQuery(r.URL),
		"request_id", ContextGetRequestID(r),
	)
}

// redactedQuery renders a URL's query as its keys with every value replaced.
//
// Keys are sorted so the same request shape reads the same way in every log
// line, which is what makes them greppable. A query that will not parse still
// yields whatever keys url.ParseQuery recovered before it gave up — the point
// is that no value escapes, and a malformed query is not a reason to fall back
// to printing the raw string.
func redactedQuery(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}

	values, _ := url.ParseQuery(u.RawQuery) //nolint:errcheck // whatever keys it recovered before giving up is exactly what this wants; see above
	if len(values) == 0 {
		return "(unparseable)"
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"=REDACTED")
	}
	return strings.Join(parts, "&")
}

// ServerError logs err and reports a generic 500, never leaking the underlying
// message to the client.
func (rs *Responder) ServerError(w http.ResponseWriter, r *http.Request, err error) {
	rs.LogError(r, err)
	rs.writeProblem(w, r, http.StatusInternalServerError, KindInternal,
		"the server encountered a problem and could not process your request", nil)
}

// NotFound reports 404 for a domain resource that is not there, or is not
// this caller's.
func (rs *Responder) NotFound(w http.ResponseWriter, r *http.Request) {
	rs.writeProblem(w, r, http.StatusNotFound, KindNotFound, "the requested resource could not be found", nil)
}

// RouteNotFound reports 404 for a path this API does not serve at all — the
// mux's own miss, before any handler or domain ever saw the request. It is a
// distinct kind from NotFound: a caller who mistyped the path and a caller
// who asked for a since-deleted booking get different `type` values, even
// though both answer the same status.
func (rs *Responder) RouteNotFound(w http.ResponseWriter, r *http.Request) {
	rs.writeProblem(w, r, http.StatusNotFound, KindRouteNotFound, "the requested resource could not be found", nil)
}

// MethodNotAllowed reports 405.
func (rs *Responder) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	rs.writeProblem(w, r, http.StatusMethodNotAllowed, KindMethodNotAllowed,
		"the method is not supported for this resource", nil)
}

// BadRequest reports 400 with err's message, which callers construct to be
// safe for the client to read. An oversized request body (BodyTooLargeError,
// from ReadJSON) is the one exception: it reports 413 Request Entity Too
// Large instead, with the same message.
func (rs *Responder) BadRequest(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *BodyTooLargeError
	if errors.As(err, &tooLarge) {
		rs.writeProblem(w, r, http.StatusRequestEntityTooLarge, KindTooLarge, err.Error(), nil)
		return
	}
	rs.writeProblem(w, r, http.StatusBadRequest, KindInvalidJSON, err.Error(), nil)
}

// FailedValidation reports 422 with the per-field validation errors, sorted
// by field name so the same set of failures always serializes in the same
// order.
func (rs *Responder) FailedValidation(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	rs.writeProblem(w, r, http.StatusUnprocessableEntity, KindValidation,
		"the request failed validation", sortedFieldErrors(errors))
}

// EditConflict reports 409 when an optimistic-concurrency update lost its race.
func (rs *Responder) EditConflict(w http.ResponseWriter, r *http.Request) {
	rs.writeProblem(w, r, http.StatusConflict, KindConflict,
		"unable to update the record due to an edit conflict, please try again", nil)
}

// RateLimitExceeded reports 429.
func (rs *Responder) RateLimitExceeded(w http.ResponseWriter, r *http.Request) {
	rs.writeProblem(w, r, http.StatusTooManyRequests, KindRateLimited, "rate limit exceeded", nil)
}

// RateLimitExceededAfter reports 429 and tells the client, in whole seconds,
// when it may try again (RFC 9110 §10.2.3 Retry-After).
func (rs *Responder) RateLimitExceededAfter(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	rs.RateLimitExceeded(w, r)
}

// InvalidCredentials reports 401 for a failed login.
func (rs *Responder) InvalidCredentials(w http.ResponseWriter, r *http.Request) {
	rs.writeProblem(w, r, http.StatusUnauthorized, KindUnauthorized, "invalid authentication credentials", nil)
}

// InvalidAuthenticationToken reports 401 for a missing or unusable token and
// advertises the expected scheme.
func (rs *Responder) InvalidAuthenticationToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	rs.writeProblem(w, r, http.StatusUnauthorized, KindUnauthorized, "invalid or missing authentication token", nil)
}

// NotPermitted reports 403 for an authenticated user lacking the required role
// or ownership.
func (rs *Responder) NotPermitted(w http.ResponseWriter, r *http.Request) {
	rs.writeProblem(w, r, http.StatusForbidden, KindForbidden,
		"your user account doesn't have the necessary permissions to access this resource", nil)
}

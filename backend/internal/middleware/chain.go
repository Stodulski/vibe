package middleware

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// identityQueryTimeout bounds the two database reads the chain performs before
// a handler runs: the account behind a session, and the complex an
// ownership-scoped route names.
//
// Both used to run on the bare request context, which has no deadline of its
// own — http.Server's WriteTimeout closes the connection but does not cancel
// the handler, so a stalled read held its pool connection until the query
// itself gave up. With a pool of 25, a database that has stopped answering
// takes the whole instance down through a queue of requests that are each
// waiting on a read nobody is going to answer.
//
// Two seconds is deliberately tighter than the store's own 3s query budget:
// this is identification, it runs before every request, and a database that
// cannot answer it in two seconds is not going to serve the handler either.
const identityQueryTimeout = 2 * time.Second

// RequestID mints a short correlation id for each request, puts it in the
// context for error logs and echoes it in X-Request-ID so a client can quote
// it in a bug report.
//
// The id is minted here rather than read from the request: an id the caller
// chooses is an id the caller can reuse, collide with, or fill with newlines
// aimed at whoever greps the logs.
func (m *Middleware) RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()[:8]
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, httpx.ContextSetRequestID(r, id))
	})
}

// responseTracker records whether anything has reached the client yet.
//
// RecoverPanic needs the answer: a 500 body appended to a response that has
// already begun is not an error page, it is corruption of whatever was
// half-written.
type responseTracker struct {
	http.ResponseWriter
	// wrote is written and read from the serving goroutine only — the deferred
	// recover runs on it too — so it needs no synchronisation.
	wrote bool
}

func (t *responseTracker) WriteHeader(status int) {
	t.wrote = true
	t.ResponseWriter.WriteHeader(status)
}

func (t *responseTracker) Write(b []byte) (int, error) {
	t.wrote = true
	return t.ResponseWriter.Write(b)
}

// Flush keeps the SSE stream working: it type-asserts the writer to
// http.Flusher, and a wrapper that does not implement it turns a live event
// stream into a 500.
func (t *responseTracker) Flush() {
	t.wrote = true
	if flusher, ok := t.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer, which is how the
// stream handler clears its write deadline.
func (t *responseTracker) Unwrap() http.ResponseWriter { return t.ResponseWriter }

// RecoverPanic turns a panic below it into a 500 rather than a dropped
// connection, and reports it to Sentry.
//
// Two panics are not errors and are not treated as one.
//
// http.ErrAbortHandler is the documented way for a handler to abandon a
// response on purpose; net/http suppresses its own log line for it and closes
// the connection. Converting it into a 500 both hides a deliberate abort and
// answers a request the handler decided not to answer.
//
// A panic after the response has started is the second. The status line and
// part of the body are already on the wire, so no error body can be sent —
// appending one produces a response that parses as a success with garbage
// stapled to the end. Re-panicking with ErrAbortHandler is what forces net/http
// to close the connection, which is the only signal left that tells the client
// the response it received is incomplete.
func (m *Middleware) RecoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &responseTracker{ResponseWriter: w}

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(recovered)
			}

			reportPanic(r, recovered)

			if tracked.wrote {
				m.respond.LogError(r, fmt.Errorf("panic after the response had started: %s", recovered))
				panic(http.ErrAbortHandler)
			}

			w.Header().Set("Connection", "close")
			m.respond.ServerError(w, r, fmt.Errorf("%s", recovered))
		}()

		next.ServeHTTP(tracked, r)
	})
}

// reportPanic sends the recovered value to Sentry, tagged with the request id
// the client was given.
//
// The log line carries that id because RecoverPanic sits inside RequestID —
// see Wrap. The Sentry event is the other half of the same problem and was
// still missing it: an operator is paged on the event, not on the log, and a
// client quoting the id from their X-Request-ID header had nothing to quote it
// at. The tag is what makes a panic report searchable by the one string the
// person reporting it actually holds.
//
// The hub is cloned rather than tagged in place. sentry.CurrentHub is
// process-global and shared by every request in flight, so configuring its
// scope would stamp one request's id onto another request's event — a
// correlation id that points at the wrong request is worse than none, because
// it is believed. A clone carries the same client and a copy of the scope, so
// the tag lives exactly as long as this event.
func reportPanic(r *http.Request, recovered any) {
	hub := sentry.CurrentHub().Clone()
	if scope := hub.Scope(); scope != nil {
		scope.SetTag("request_id", httpx.ContextGetRequestID(r))
	}
	hub.Recover(recovered)
}

// SecurityHeaders sets the response headers that constrain what a browser will
// do with our responses.
func (m *Middleware) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		next.ServeHTTP(w, r)
	})
}

// Authenticate identifies the caller when it can, and lets anonymous requests
// through — some routes are public. RequireAuth is what makes a route private.
//
// It checks the blacklist as well as the signature, because an access token is
// a self-contained JWT: signing out cannot invalidate one by deleting a row.
//
// it, check the blacklist, load and cache the user, set the context.
//
//nolint:funlen // one linear identification sequence: read the token, verify
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")
		w.Header().Add("Vary", "Cookie")

		tokenString := credential(r)

		if tokenString == "" {
			next.ServeHTTP(w, r)
			return
		}

		// From here the response depends on who is asking. Vary alone is not
		// enough for that: a shared cache that ignores Vary, or a browser
		// restoring a page from the back-forward cache after a logout, will
		// hand one account's response to whoever is at the keyboard next.
		// no-store is set for any credentialed request, including the ones
		// whose credential turns out to be invalid — those responses are
		// per-request too, and a cached 401 outlives the token that caused it.
		w.Header().Set("Cache-Control", "no-store")

		claims, err := m.tokens.ValidateAccessToken(tokenString)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		if m.blacklist.IsBlacklisted(r.Context(), tokenString, userID, claims.IssuedAt.Time) {
			next.ServeHTTP(w, r)
			return
		}

		// Try the Redis user cache before hitting the database.
		user := m.getCachedUser(r.Context(), userID)
		if user == nil {
			user, err = m.loadUser(r.Context(), userID)
			if err != nil {
				if errors.Is(err, data.ErrRecordNotFound) {
					next.ServeHTTP(w, r)
					return
				}
				m.respond.ServerError(w, r, err)
				return
			}
			m.cacheUser(r.Context(), user)
		}

		if !user.IsActive {
			next.ServeHTTP(w, r)
			return
		}

		r = httpx.ContextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

// credential returns the raw access token on the request: the cookie first,
// then a Bearer header.
func credential(r *http.Request) string {
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return ""
}

// loadUser reads the account behind a session under its own deadline. See
// identityQueryTimeout for why the request's context is not enough.
func (m *Middleware) loadUser(ctx context.Context, id uuid.UUID) (*data.User, error) {
	ctx, cancel := context.WithTimeout(ctx, identityQueryTimeout)
	defer cancel()
	return m.users.GetByID(ctx, id)
}

// loadComplex reads a complex under its own deadline, for the same reason.
func (m *Middleware) loadComplex(ctx context.Context, id uuid.UUID) (*data.Complex, error) {
	ctx, cancel := context.WithTimeout(ctx, identityQueryTimeout)
	defer cancel()
	return m.complexes.GetByID(ctx, id)
}

// RequireAuth rejects a request that carries no authenticated user.
//
// Authenticate runs earlier and is permissive — it identifies the caller when
// it can and lets anonymous requests through, because some routes are public.
// This is the guard that makes a route private.
func (m *Middleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := httpx.ContextGetAuthenticatedUser(r)
		if !ok {
			m.respond.InvalidAuthenticationToken(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// RequireRole rejects an authenticated user whose role is not among those
// listed.
func (m *Middleware) RequireRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, ok := httpx.ContextGetAuthenticatedUser(r)
			if !ok {
				m.respond.InvalidAuthenticationToken(w, r)
				return
			}

			if slices.Contains(roles, user.Role) {
				next.ServeHTTP(w, r)
				return
			}
			m.respond.NotPermitted(w, r)
		}
	}
}

// RequireComplexOwner rejects a caller who does not own the complex named in
// the route, and puts that complex in the context so the handler does not
// load it again.
//
// A complex the caller does not own reads as missing rather than forbidden: a
// 403 would confirm the id exists.
func (m *Middleware) RequireComplexOwner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := httpx.ContextGetAuthenticatedUser(r)
		if !ok {
			m.respond.InvalidAuthenticationToken(w, r)
			return
		}

		complexID, err := httpx.ReadUUIDParam(r, "id")
		if err != nil {
			m.respond.NotFound(w, r)
			return
		}

		// Resolving the tenant cannot itself be scoped to that tenant: this
		// read is what decides which complex the request is for, and under the
		// tenant policies an unscoped session sees no complexes at
		// all. The bypass is therefore on for exactly this one statement, and
		// the ownership comparison below is what it buys — ContextSetComplex
		// then narrows the session to the verified complex and drops the
		// bypass, so nothing downstream inherits it.
		complex, err := m.loadComplex(data.ContextWithTenantBypass(r.Context()), complexID)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				m.respond.NotFound(w, r)
			default:
				m.respond.ServerError(w, r, err)
			}
			return
		}

		if complex.OwnerID != user.ID {
			m.respond.NotPermitted(w, r)
			return
		}

		r = httpx.ContextSetComplex(r, complex)
		next.ServeHTTP(w, r)
	}
}

// csrfExemptRoutes names the exact routes that carry no CSRF token, each with
// the reason it carries none. The key is "METHOD /path", written exactly as the
// route is registered.
//
// One route, one entry, no subtrees. A subtree exemption is a grant made to
// routes that do not exist yet: whoever adds the next endpoint under it inherits
// the exemption without ever seeing this file, and the symptom is a CSRF hole
// rather than a failing test. This table was three subtrees — "/api/v1/webhooks",
// "/api/v1/public" and "/api/v1/book" — and the last of those also exempted, as a
// bare string prefix, every path merely beginning with those characters, so a
// future "/api/v1/bookmarks" would have been exempt too.
//
// The exemptions are all the same claim in the end: this route has no session
// cookie, so there is no ambient credential for another site to make a browser
// spend. The auth routes are the ones that mint or clear the cookie; the public
// booking and webhook routes never use one.
//
// TestEveryPublicWriteDeclaresItsCSRFPosition in cmd/api keeps this honest in
// both directions: nothing here may name an unregistered or session-authenticated
// route, and no public state-changing route may be missing from it.
var csrfExemptRoutes = map[string]string{
	"POST /api/v1/auth/register": "creates the account; there is no session to derive a token from",
	"POST /api/v1/auth/login":    "mints the cookie the token would be derived from",
	"POST /api/v1/auth/google": "verifies a Google Identity Services ID token and mints the " +
		"cookie the token would be derived from, or answers with no session at all",
	"POST /api/v1/auth/google/complete": "creates the account and mints the cookie; there is no " +
		"session yet to derive a token from",
	"POST /api/v1/auth/logout":              "must succeed even when the session is already invalid",
	"POST /api/v1/auth/refresh":             "runs on an expired access token by design",
	"POST /api/v1/auth/verify-email":        "reached from an emailed link, before first login",
	"POST /api/v1/auth/resend-verification": "reached before the account can log in",
	"POST /api/v1/auth/forgot-password":     "the user cannot log in, that is the point",
	"POST /api/v1/auth/reset-password":      "authenticated by the emailed token, not a session",

	"POST /api/v1/book":        "public booking: clients book without an account, so no cookie is in play",
	"POST /api/v1/book/cancel": "same flow, authenticated by the booking's own link token",

	"POST /api/v1/public/leads/abandoned-registration": "reached before an account exists, so no cookie is in play",

	"POST /api/v1/webhooks/mercadopago": "authenticated by MercadoPago's signature; no cookie auth in this flow",
	"POST /api/v1/webhooks/whatsapp":    "authenticated by Meta's signature; no cookie auth in this flow",
}

// CSRFExemptRoutes returns the routes that carry no CSRF token, keyed by
// "METHOD /path" with the reason as the value.
//
// It is exported for the route audit in cmd/api, which is the only place that
// can see the registered route table and this list at the same time.
func CSRFExemptRoutes() map[string]string { return maps.Clone(csrfExemptRoutes) }

// csrfExempt reports whether a route is outside CSRF protection.
func csrfExempt(method, path string) bool {
	_, exempt := csrfExemptRoutes[method+" "+path]
	return exempt
}

// CSRFProtect requires state-changing requests to echo the CSRF token in a
// header. The token is derived from the session's own access token, so
// echoing it proves the request came from our frontend rather than from
// another site holding the user's cookies.
//
// It runs before Authenticate so a forged request is rejected without
// costing a database read.
func (m *Middleware) CSRFProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate CSRF on state-changing methods.
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if csrfExempt(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Read access_token from cookie to derive expected CSRF token.
		cookie, err := r.Cookie("access_token")
		if err != nil || cookie.Value == "" {
			// No access_token cookie — let the auth middleware handle the 401.
			next.ServeHTTP(w, r)
			return
		}

		csrfHeader := r.Header.Get("X-CSRF-Token")
		if csrfHeader == "" || !m.tokens.ValidateCSRFToken(cookie.Value, csrfHeader) {
			m.respond.Error(w, r, http.StatusForbidden, "invalid or missing CSRF token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// keepAliveInterval is how often a comment frame is sent on an idle stream.
// Proxies and load balancers close connections that go quiet, so an idle
// dashboard would otherwise be disconnected every few minutes.
const keepAliveInterval = 30 * time.Second

// defaultRecheckInterval is how often an open stream re-asks whether the caller
// is still allowed to receive this complex's events.
//
// Authorization is decided once, by the guards, before the handler runs — and
// then the connection outlives that decision by however long the client keeps
// it open. An owner whose account is deactivated, whose session is signed out,
// or whose venue changes hands keeps receiving live booking events until they
// close the tab. One minute is the window that leaves open. It is chosen
// against the cost: each re-check is one cached user read and one complex read
// per stream, which at the per-complex cap is bounded and small, while a value
// in the minutes puts a demoted account back inside the venue's live booking
// feed for that whole time.
const defaultRecheckInterval = time.Minute

// defaultMaxLifetime is the longest a single stream may stay open.
//
// It is the access token's own lifetime (auth.accessTokenExpiry, 15 minutes):
// no stream should outlive the credential that opened it, and a connection
// established at minute zero has no more right to minute twenty than a new
// request with that same expired token would. Reaching it is not an error —
// the client is told to reconnect, and the reconnect is an ordinary request
// that goes through the full guard chain and can be answered with a status
// code.
//
// It is also the backstop for every resource a stream holds: whatever happens
// to the re-check, nothing here is held for longer than this.
const defaultMaxLifetime = 15 * time.Minute

// maxRecheckFailures is how many consecutive re-checks may fail to reach a
// verdict before the stream is closed.
//
// A failure here is not a denial: the database or cache is unreachable, so we
// know neither that the caller is still authorized nor that they are not.
// Dropping every stream on the first blip would turn a brief outage into a
// reconnect storm against the same sick dependency. Tolerating them forever
// would let an outage be the way to keep a revoked stream alive. Two failures
// bound that to a couple of minutes.
const maxRecheckFailures = 2

// recheckTimeout bounds one authorization re-check, so that a wedged
// dependency cannot hold the stream's loop — and every event queued behind
// this iteration — for as long as it likes.
const recheckTimeout = 3 * time.Second

// ErrStreamUnauthorized is what an Authorizer returns when the caller is no
// longer allowed to receive a complex's events. Any other error means the check
// could not be completed, which is treated differently — see maxRecheckFailures.
var ErrStreamUnauthorized = errors.New("realtime: stream no longer authorized")

// Authorizer re-decides, while a stream is open, whether the caller may still
// receive a complex's events.
//
// It takes the original request because the caller's credentials live on it,
// and the complex id separately because that is the stream's subject: it is
// fixed at connect time and must not be re-read from anything the client can
// influence afterwards.
//
// Implementations must not consult the request's context values for identity —
// the user and complex already in there are the connect-time decision this
// interface exists to distrust.
type Authorizer interface {
	Authorize(ctx context.Context, r *http.Request, complexID uuid.UUID) error
}

// Config tunes the stream's lifecycle. A zero value means the defaults; tests
// set them to something they can wait for.
type Config struct {
	// RecheckInterval is how often authorization is re-decided.
	RecheckInterval time.Duration
	// MaxLifetime is the longest a stream stays open before the client is
	// asked to reconnect.
	MaxLifetime time.Duration
}

func (c Config) recheckInterval() time.Duration {
	if c.RecheckInterval > 0 {
		return c.RecheckInterval
	}
	return defaultRecheckInterval
}

func (c Config) maxLifetime() time.Duration {
	if c.MaxLifetime > 0 {
		return c.MaxLifetime
	}
	return defaultMaxLifetime
}

// Handler serves the event stream.
type Handler struct {
	hub      *Hub
	auth     Authorizer
	respond  *httpx.Responder
	logger   *slog.Logger
	shutdown <-chan struct{}
	cfg      Config
}

// NewHandler returns a Handler. shutdown is the application's shutdown signal,
// which closes open streams so a graceful stop is not held up by connections
// that would otherwise stay open indefinitely.
//
// A nil auth is not a way to opt out of re-checking: it fails every re-check
// closed, so an unwired authorizer shows up as streams that end after one
// interval rather than as streams nobody is re-authorizing.
func NewHandler(hub *Hub, auth Authorizer, respond *httpx.Responder, logger *slog.Logger,
	shutdown <-chan struct{}, cfg Config,
) *Handler {
	return &Handler{hub: hub, auth: auth, respond: respond, logger: logger, shutdown: shutdown, cfg: cfg}
}

// Routes registers the stream endpoint. It is scoped to a complex and readable
// only by its owner, like the dashboard it feeds.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/events",
		guards.RequireAuth(guards.RequireComplexOwner(h.Stream)))
}

// Stream handles GET /api/v1/complexes/:id/events, holding the connection open
// and writing events as they arrive.
//
// the single select loop that owns every way it can end. Splitting the loop out
// would hide which exits write a terminal frame and which cannot.
//
//nolint:funlen // one connection lifecycle: admit, commit the response, then
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("streaming unsupported"))
		return
	}

	// Admission happens before a single header is written, so a refusal is a
	// status code the client can act on rather than a stream that opens and
	// immediately dies.
	client, admitted := h.hub.Subscribe(complex.ID)
	if !admitted {
		h.respond.RateLimitExceeded(w, r)
		return
	}
	defer h.hub.Unsubscribe(complex.ID, client)

	// This connection is meant to stay open, so the server's write deadline
	// has to come off before anything is written.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		h.logger.Error("realtime: failed to disable write deadline", "error", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Tells nginx not to buffer, which would hold events until the buffer fills.
	w.Header().Set("X-Accel-Buffering", "no")

	// From here the response is committed, so a write failure cannot be
	// reported to the client. The loop notices instead when the request
	// context is cancelled.
	_, _ = fmt.Fprint(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	recheck := time.NewTicker(h.cfg.recheckInterval())
	defer recheck.Stop()

	lifetime := time.NewTimer(h.cfg.maxLifetime())
	defer lifetime.Stop()

	failures := 0
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.shutdown:
			return
		case <-lifetime.C:
			// Not a failure: the client is expected to come straight back,
			// and that reconnect is a fresh authorization decision.
			h.end(w, flusher, "expired", "the stream reached its maximum lifetime")
			return
		case <-recheck.C:
			err := h.authorized(ctx, r, complex.ID)
			switch {
			case err == nil:
				failures = 0
			case errors.Is(err, ErrStreamUnauthorized):
				h.logger.Info("realtime: closing stream, caller is no longer authorized",
					"complex_id", complex.ID, "error", err)
				h.end(w, flusher, "unauthorized", "access to this complex has ended")
				return
			default:
				failures++
				h.logger.Error("realtime: could not re-check stream authorization",
					"complex_id", complex.ID, "error", err, "consecutive_failures", failures)
				if failures >= maxRecheckFailures {
					// We cannot prove the caller is still allowed and we
					// cannot answer with a status code on a committed
					// response. Ending the stream hands the question back to
					// a fresh request, which can be answered properly.
					h.end(w, flusher, "expired", "authorization could not be re-checked")
					return
				}
			}
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case event, open := <-client.events:
			if !open {
				return
			}
			data, _ := json.Marshal(event.Data)
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}

// authorized re-decides whether this stream may continue.
//
// The re-check is given its own deadline: it must not be able to hold the
// stream's loop — and with it every event queued behind this iteration — for
// as long as a wedged dependency feels like taking.
func (h *Handler) authorized(ctx context.Context, r *http.Request, complexID uuid.UUID) error {
	if h.auth == nil {
		return fmt.Errorf("%w: no authorizer wired", ErrStreamUnauthorized)
	}

	ctx, cancel := context.WithTimeout(ctx, recheckTimeout)
	defer cancel()

	return h.auth.Authorize(ctx, r, complexID)
}

// end writes the frame that says why the stream is closing.
//
// A stream that simply stops is indistinguishable from a dropped connection,
// and a browser's EventSource answers that by reconnecting — forever, for a
// caller who is no longer allowed in. The named event is what lets the
// frontend tell "your access ended, re-authenticate" from "reconnect, this was
// routine" from an actual network failure, which sends no frame at all.
func (h *Handler) end(w http.ResponseWriter, flusher http.Flusher, event, reason string) {
	payload, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		payload = []byte("{}")
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
}

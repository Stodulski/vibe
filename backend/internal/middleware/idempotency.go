package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// Idempotency is the Idempotency-Key handling for the requests that create a
// booking or move money.
//
// Those endpoints are protected against a double submit today by the database:
// the EXCLUDE constraint on the booking's hours and the slot lock refute the
// duplicate. But refuting it answers 409 — "somebody has that slot" — to the
// client that already had it, because a retried request is indistinguishable
// from a second person's. The client, which usually retried because it never
// saw the first answer, is told its own booking failed. What it needs is the
// first answer again (API-05).
//
// So a request carrying the header is recorded with its answer, and a repeat
// of it is replayed rather than re-executed. A repeat of the KEY with a
// different request is a mistake on the caller's side and is refused.
//
// A request with no Idempotency-Key behaves exactly as it did before this
// existed, which is what makes the header adoptable one caller at a time.
type Idempotency struct {
	rdb     *goredis.Client
	env     string
	respond *httpx.Responder
}

// idempotencyTTL is how long an answer stays replayable. A day covers a client
// retrying through an outage, a user reopening a tab, and a mobile app resumed
// the next morning; past that, the booking itself is the record.
const idempotencyTTL = 24 * time.Hour

// idempotencyLockTTL bounds how long a key stays claimed by a request that has
// not finished. It has to outlast the slowest of these handlers — a booking
// that waits on MercadoPago — and has to expire on its own, or a process killed
// mid-request would wedge that key until the full day was up.
const idempotencyLockTTL = 2 * time.Minute

// maxIdempotencyKeyLength bounds what a caller may send. A UUID is 36; the
// extra room is for a caller that prefers its own order id, and the bound is
// what stops the header being used to write arbitrary-length keys into Redis.
const maxIdempotencyKeyLength = 64

// idempotencyRecord is what is stored under the key: what the request was, and
// what it was answered with.
type idempotencyRecord struct {
	// Fingerprint identifies the request this key was first used for. A second
	// request under the same key with a different fingerprint is the caller
	// reusing a key, not retrying.
	Fingerprint string `json:"fingerprint"`
	// Done distinguishes a claimed key whose request is still running from one
	// whose answer is recorded.
	Done bool `json:"done"`
	// Status, ContentType and Body are the answer, replayed verbatim. Only the
	// content type is kept of the headers: the rest are either per-request
	// (the request id, which a replay must earn for itself) or written by the
	// chain outside this (CORS, security headers, cache control).
	Status      int    `json:"status,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Body        []byte `json:"body,omitempty"`
}

// NewIdempotency returns the middleware. A nil client disables it: the same
// degradation every other Redis-backed feature here makes, and the same
// behaviour as a request that sends no key.
func NewIdempotency(rdb *goredis.Client, env string, respond *httpx.Responder) *Idempotency {
	return &Idempotency{rdb: rdb, env: env, respond: respond}
}

// Guard returns a per-route wrapper. scope separates the keys of two endpoints
// so that one caller's key for a booking cannot collide with its key for a
// refund.
func (i *Idempotency) Guard(scope string) httpx.Guard {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			i.serve(scope, w, r, next)
		}
	}
}

// would scatter the states of a single protocol across helpers.
//
//nolint:funlen // one linear decision — no key, bad key, claimed key, replay, run — and splitting it
func (i *Idempotency) serve(scope string, w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || i.rdb == nil {
		next(w, r)
		return
	}

	if err := validIdempotencyKey(key); err != nil {
		i.respond.BadRequest(w, r, err)
		return
	}

	body, err := rewindBody(w, r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			// Redis is never touched: an oversized body is refused the same
			// way ReadJSON would refuse it, before there is anything to claim.
			i.respond.Refuse(w, r, httpx.TooLarge(
				fmt.Sprintf("body must not be larger than %d bytes", tooLarge.Limit)))
			return
		}
		i.respond.ServerError(w, r, fmt.Errorf("idempotency: reading the request body: %w", err))
		return
	}

	redisKey := i.key(scope, key)
	fingerprint := requestFingerprint(r, body)

	claimed, existing, err := i.claim(r.Context(), redisKey, fingerprint)
	if err != nil {
		// Redis is the record, not the rule: without it this endpoint behaves
		// the way it did before the header existed, which is the same thing a
		// caller that sends no key gets. Refusing the booking instead would
		// turn a cache outage into an outage.
		i.respond.LogError(r, fmt.Errorf("idempotency: claiming %q, continuing without it: %w", scope, err))
		next(w, r)
		return
	}

	if !claimed {
		if existing.Fingerprint != fingerprint {
			i.respond.Refuse(w, r, httpx.Conflict(
				"this Idempotency-Key was already used for a different request"))
			return
		}
		if !existing.Done {
			i.respond.Refuse(w, r, httpx.Conflict(
				"a request with this Idempotency-Key is still in progress, retry shortly"))
			return
		}
		i.replay(w, existing)
		return
	}

	rec := &answerRecorder{ResponseWriter: w, status: http.StatusOK}
	next(rec, r)
	i.record(r, redisKey, fingerprint, rec)
}

// key is the Redis key one request's record lives under. The environment is in
// it because a staging deployment sharing a Redis with production must not
// replay production's answers.
func (i *Idempotency) key(scope, key string) string {
	return fmt.Sprintf("vibe:%s:idem:%s:%s", i.env, scope, key)
}

// claim takes the key for this request, or reports what already holds it.
//
// SET NX is what makes two simultaneous requests one winner and one loser
// rather than two executions: the loser reads back the record the winner wrote
// and is told the request is in progress.
func (i *Idempotency) claim(ctx context.Context, redisKey, fingerprint string) (bool, idempotencyRecord, error) {
	pending, err := json.Marshal(idempotencyRecord{Fingerprint: fingerprint})
	if err != nil {
		return false, idempotencyRecord{}, err
	}

	ok, err := i.rdb.SetNX(ctx, redisKey, pending, idempotencyLockTTL).Result()
	if err != nil {
		return false, idempotencyRecord{}, err
	}
	if ok {
		return true, idempotencyRecord{}, nil
	}

	raw, err := i.rdb.Get(ctx, redisKey).Bytes()
	if errors.Is(err, goredis.Nil) {
		// The holder's lock expired between the SET NX and the GET. Treating
		// it as in progress is the safe half of the race: the alternative is
		// running a second booking while the first may still be running.
		return false, idempotencyRecord{Fingerprint: fingerprint}, nil
	}
	if err != nil {
		return false, idempotencyRecord{}, err
	}

	var existing idempotencyRecord
	if err := json.Unmarshal(raw, &existing); err != nil {
		return false, idempotencyRecord{}, fmt.Errorf("idempotency: stored record is unreadable: %w", err)
	}
	return false, existing, nil
}

// record stores the answer, or releases the key.
//
// A 5xx is not stored and the key is dropped: the failure is this service's,
// the caller is expected to retry, and replaying a 500 for a day would make one
// bad minute permanent for that key. Everything else — the booking that was
// created, the request that was refused for a reason the caller has to fix — is
// the answer, and is replayed.
func (i *Idempotency) record(r *http.Request, redisKey, fingerprint string, rec *answerRecorder) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), idempotencyWriteTimeout)
	defer cancel()

	if rec.status >= http.StatusInternalServerError {
		if err := i.rdb.Del(ctx, redisKey).Err(); err != nil {
			i.respond.LogError(r, fmt.Errorf("idempotency: releasing %s after a %d: %w", redisKey, rec.status, err))
		}
		return
	}

	stored, err := json.Marshal(idempotencyRecord{
		Fingerprint: fingerprint,
		Done:        true,
		Status:      rec.status,
		ContentType: rec.Header().Get("Content-Type"),
		Body:        rec.body.Bytes(),
	})
	if err != nil {
		i.respond.LogError(r, fmt.Errorf("idempotency: encoding the record for %s: %w", redisKey, err))
		return
	}

	if err := i.rdb.Set(ctx, redisKey, stored, idempotencyTTL).Err(); err != nil {
		// The answer already went out; all that is lost is the ability to
		// replay it, which is the state this endpoint was in before.
		i.respond.LogError(r, fmt.Errorf("idempotency: storing the record for %s: %w", redisKey, err))
	}
}

// idempotencyWriteTimeout bounds the store that happens after the response has
// been written. It runs on a context detached from the request's, because the
// request's is cancelled the moment the client hangs up — which is exactly when
// the record matters most.
const idempotencyWriteTimeout = 3 * time.Second

// replay writes the recorded answer again, marked so a client can tell it apart
// from a fresh one.
func (i *Idempotency) replay(w http.ResponseWriter, rec idempotencyRecord) {
	if rec.ContentType != "" {
		w.Header().Set("Content-Type", rec.ContentType)
	}
	w.Header().Set("Idempotent-Replay", "true")
	w.WriteHeader(rec.Status)
	_, _ = w.Write(rec.Body) //nolint:errcheck // the response is committed; a write failure can no longer be reported
}

// validIdempotencyKey checks what a caller may send.
func validIdempotencyKey(key string) error {
	if len(key) > maxIdempotencyKeyLength {
		return fmt.Errorf("Idempotency-Key must be at most %d characters", maxIdempotencyKeyLength)
	}
	for _, c := range key {
		// Printable ASCII without space: a key ends up in a Redis key and in
		// log lines, and neither is a place for control characters.
		if c < '!' || c > '~' {
			return errors.New("Idempotency-Key must be printable ASCII with no spaces")
		}
	}
	return nil
}

// requestFingerprint identifies the request a key was used for: the operation,
// the caller, and what they asked for.
//
// The actor is in it because two different people must not be able to collide
// on a key one of them chose — "1" is a plausible key, and without the actor
// the second person would be replayed the first's booking.
func requestFingerprint(r *http.Request, body []byte) string {
	sum := sha256.New()
	sum.Write([]byte(r.Method))
	sum.Write([]byte{'\n'})
	sum.Write([]byte(r.URL.Path))
	sum.Write([]byte{'\n'})
	sum.Write([]byte(fingerprintActor(r)))
	sum.Write([]byte{'\n'})
	sum.Write(body)
	return hex.EncodeToString(sum.Sum(nil))
}

// fingerprintActor is who is making the request: the authenticated user where
// there is one, and otherwise the empty string. A public booking has no
// account, so its key is scoped by its body alone — which carries the slot, the
// court and the contact details, and is what distinguishes two bookings.
func fingerprintActor(r *http.Request) string {
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok && user != nil {
		return user.ID.String()
	}
	return ""
}

// rewindBody reads the body out and puts it back, so the handler behind this
// still sees it. The read is capped at httpx.MaxJSONBody, the same limit
// ReadJSON enforces, so this guard never reads more than the handler would.
func rewindBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, httpx.MaxJSONBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close() //nolint:errcheck // the bytes are already in hand; nothing is left to fail on
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// answerRecorder keeps the answer while it is being written, so it can be
// stored as well as sent. It writes through rather than buffering the response
// away from the client: the client gets its answer at the usual moment, and the
// copy is a side effect. It is separate from logging.go's recorder, which keeps
// a byte count rather than the bytes.
type answerRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func (rec *answerRecorder) WriteHeader(status int) {
	if !rec.wroteHeader {
		rec.status = status
		rec.wroteHeader = true
	}
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *answerRecorder) Write(b []byte) (int, error) {
	if !rec.wroteHeader {
		rec.wroteHeader = true
	}
	rec.body.Write(b)
	return rec.ResponseWriter.Write(b)
}

// Flush and Unwrap keep the wrapper transparent to everything that looks past
// a ResponseWriter — the SSE stream flushes, and net/http's own
// ResponseController unwraps.
func (rec *answerRecorder) Flush() {
	if f, ok := rec.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rec *answerRecorder) Unwrap() http.ResponseWriter { return rec.ResponseWriter }

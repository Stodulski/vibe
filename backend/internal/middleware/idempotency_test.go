package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// These run against a live miniredis rather than a fake client: the SET NX, the
// TTLs and the key names are most of what this middleware is, and a stub of the
// client would prove only that the stub agrees with the code.

func newIdempotencyFixture(t *testing.T) (*Idempotency, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return NewIdempotency(rdb, "test", respond), mr
}

// countingBooking is the handler under the guard: it answers 201 with a body
// that changes every time, so a replay is distinguishable from a second run.
func countingBooking() (http.HandlerFunc, *atomic.Int64) {
	var runs atomic.Int64
	return func(w http.ResponseWriter, _ *http.Request) {
		n := runs.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"booking":"` + strings.Repeat("x", int(n)) + `"}`))
	}, &runs
}

func book(t *testing.T, key, body string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/book", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	return r
}

// The point of the whole thing: a retried request is answered with the booking
// it already made, not refused as somebody else's.
func TestARepeatedRequestIsReplayedNotRerun(t *testing.T) {
	idem, _ := newIdempotencyFixture(t)
	handler, runs := countingBooking()
	guarded := idem.Guard("public-book")(handler)

	first := httptest.NewRecorder()
	guarded(first, book(t, "9d1d1f4e-0a0a-4a4a-8b8b-000000000001", `{"slot":"10:00"}`))

	second := httptest.NewRecorder()
	guarded(second, book(t, "9d1d1f4e-0a0a-4a4a-8b8b-000000000001", `{"slot":"10:00"}`))

	if runs.Load() != 1 {
		t.Errorf("the handler ran %d times, want 1", runs.Load())
	}
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Errorf("statuses = %d and %d, want 201 twice", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Errorf("replay returned %q, want the first answer %q", second.Body.String(), first.Body.String())
	}
	if first.Header().Get("Content-Type") != second.Header().Get("Content-Type") {
		t.Errorf("replay content type = %q, want %q",
			second.Header().Get("Content-Type"), first.Header().Get("Content-Type"))
	}
	if second.Header().Get("Idempotent-Replay") != "true" {
		t.Error("a replay is not marked as one")
	}
	if first.Header().Get("Idempotent-Replay") != "" {
		t.Error("the first answer was marked as a replay")
	}
}

// A key reused for a different request is the caller's mistake, and replaying
// the first answer for it would hand them somebody else's booking.
func TestTheSameKeyWithADifferentRequestIsRefused(t *testing.T) {
	idem, _ := newIdempotencyFixture(t)
	handler, runs := countingBooking()
	guarded := idem.Guard("public-book")(handler)

	guarded(httptest.NewRecorder(), book(t, "reused", `{"slot":"10:00"}`))

	w := httptest.NewRecorder()
	guarded(w, book(t, "reused", `{"slot":"11:00"}`))

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "different request") {
		t.Errorf("the refusal does not say why: %s", w.Body.String())
	}
	if runs.Load() != 1 {
		t.Errorf("the handler ran %d times, want 1", runs.Load())
	}
}

// While the first request is still executing there is no answer to replay, and
// running the second would be the double booking this exists to prevent.
func TestARequestStillInProgressIsRefused(t *testing.T) {
	idem, _ := newIdempotencyFixture(t)

	release := make(chan struct{})
	inner := make(chan struct{})
	var runs atomic.Int64
	guarded := idem.Guard("public-book")(func(w http.ResponseWriter, _ *http.Request) {
		runs.Add(1)
		close(inner)
		<-release
		w.WriteHeader(http.StatusCreated)
	})

	go guarded(httptest.NewRecorder(), book(t, "in-flight", `{"slot":"10:00"}`))
	<-inner

	w := httptest.NewRecorder()
	guarded(w, book(t, "in-flight", `{"slot":"10:00"}`))
	close(release)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "in progress") {
		t.Errorf("the refusal does not say the first request is still running: %s", w.Body.String())
	}
	if runs.Load() != 1 {
		t.Errorf("the handler ran %d times, want 1", runs.Load())
	}
}

// A day later the record is gone and the same key is a new request. The clock
// is miniredis's, so nothing sleeps.
func TestARecordExpiresAfterADay(t *testing.T) {
	idem, mr := newIdempotencyFixture(t)
	handler, runs := countingBooking()
	guarded := idem.Guard("public-book")(handler)

	guarded(httptest.NewRecorder(), book(t, "stale", `{"slot":"10:00"}`))
	mr.FastForward(idempotencyTTL + 1)

	w := httptest.NewRecorder()
	guarded(w, book(t, "stale", `{"slot":"10:00"}`))

	if runs.Load() != 2 {
		t.Errorf("the handler ran %d times, want 2: the record should have expired", runs.Load())
	}
	if w.Header().Get("Idempotent-Replay") != "" {
		t.Error("an expired record was replayed")
	}
}

// Absent header is today's behaviour, which is what makes the feature
// adoptable one caller at a time.
func TestWithoutTheHeaderNothingChanges(t *testing.T) {
	idem, mr := newIdempotencyFixture(t)
	handler, runs := countingBooking()
	guarded := idem.Guard("public-book")(handler)

	guarded(httptest.NewRecorder(), book(t, "", `{"slot":"10:00"}`))
	guarded(httptest.NewRecorder(), book(t, "", `{"slot":"10:00"}`))

	if runs.Load() != 2 {
		t.Errorf("the handler ran %d times, want 2", runs.Load())
	}
	if keys := mr.Keys(); len(keys) != 0 {
		t.Errorf("a request with no key still wrote to Redis: %v", keys)
	}
}

func TestAnUnusableKeyIsRefused(t *testing.T) {
	idem, _ := newIdempotencyFixture(t)
	handler, runs := countingBooking()
	guarded := idem.Guard("public-book")(handler)

	for name, key := range map[string]string{
		"too long":         strings.Repeat("k", maxIdempotencyKeyLength+1),
		"a space":          "two words",
		"a control byte":   "line\nbreak",
		"not ASCII at all": "clé",
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			guarded(w, book(t, key, `{"slot":"10:00"}`))

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", w.Code, w.Body.String())
			}
		})
	}
	if runs.Load() != 0 {
		t.Errorf("the handler ran %d times on an unusable key, want 0", runs.Load())
	}
}

// A 5xx is this service's failure, and the caller is expected to retry. Storing
// it would make one bad minute permanent for that key.
func TestAServerErrorIsNotRecorded(t *testing.T) {
	idem, mr := newIdempotencyFixture(t)

	var runs atomic.Int64
	guarded := idem.Guard("public-book")(func(w http.ResponseWriter, _ *http.Request) {
		if runs.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	guarded(httptest.NewRecorder(), book(t, "retry-me", `{"slot":"10:00"}`))
	if keys := mr.Keys(); len(keys) != 0 {
		t.Errorf("a 500 was recorded: %v", keys)
	}

	w := httptest.NewRecorder()
	guarded(w, book(t, "retry-me", `{"slot":"10:00"}`))

	if w.Code != http.StatusCreated {
		t.Errorf("the retry after a 500 was not run: status %d", w.Code)
	}
}

// Redis is the record, not the rule. Without it these endpoints behave the way
// they did before the header existed, rather than refusing every booking.
func TestWithoutRedisTheRequestStillRuns(t *testing.T) {
	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler, runs := countingBooking()
	guarded := NewIdempotency(nil, "test", respond).Guard("public-book")(handler)

	w := httptest.NewRecorder()
	guarded(w, book(t, "9d1d1f4e-0a0a-4a4a-8b8b-000000000001", `{"slot":"10:00"}`))

	if w.Code != http.StatusCreated || runs.Load() != 1 {
		t.Errorf("status %d after %d runs, want 201 after 1", w.Code, runs.Load())
	}
}

// The key carries the environment and the scope, so a staging deployment
// sharing a Redis cannot replay production's answers and one endpoint's key
// cannot collide with another's.
func TestTheRedisKeyNamesTheEnvironmentAndTheScope(t *testing.T) {
	idem, mr := newIdempotencyFixture(t)
	handler, _ := countingBooking()

	idem.Guard("public-book")(handler)(httptest.NewRecorder(), book(t, "abc", `{"slot":"10:00"}`))

	keys := mr.Keys()
	if len(keys) != 1 || keys[0] != "vibe:test:idem:public-book:abc" {
		t.Errorf("keys = %v, want [vibe:test:idem:public-book:abc]", keys)
	}
}

// The handler behind the guard must still see its body: the middleware reads it
// to fingerprint the request.
func TestTheHandlerStillSeesTheRequestBody(t *testing.T) {
	idem, _ := newIdempotencyFixture(t)

	seen := ""
	guarded := idem.Guard("public-book")(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading the body: %v", err)
		}
		seen = string(body)
		w.WriteHeader(http.StatusCreated)
	})

	guarded(httptest.NewRecorder(), book(t, "body-check", `{"slot":"10:00"}`))

	if seen != `{"slot":"10:00"}` {
		t.Errorf("the handler saw %q, want the original body", seen)
	}
}

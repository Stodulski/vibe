package health

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stodulski/vibe-server/internal/httpx"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

func newResponder() *httpx.Responder {
	return httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
}

func newHandler(database, cache Pinger) *Handler {
	return NewHandler(Dependencies{
		Database: database, Cache: cache,
		Respond: newResponder(),
	}, Config{Version: "1.0.0"})
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return body
}

func TestCheckReportsAvailableWhenEverythingResponds(t *testing.T) {
	h := newHandler(stubPinger{}, stubPinger{})

	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d", w.Code)
	}
	if got := decode(t, w)["status"]; got != statusAvailable {
		t.Errorf("want %q; got %v", statusAvailable, got)
	}
}

// An unreachable database means the instance cannot serve, so it must answer
// 503 and be pulled from the balancer.
func TestUnreachableDatabaseTakesTheInstanceOutOfRotation(t *testing.T) {
	h := newHandler(stubPinger{err: errors.New("connection refused")}, stubPinger{})

	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503; got %d", w.Code)
	}
	if got := decode(t, w)["status"]; got != statusDegraded {
		t.Errorf("want %q; got %v", statusDegraded, got)
	}
}

// Redis is optional — the application falls back to in-memory behaviour — so
// losing it must not answer 503 and pull every instance at once.
func TestUnreachableCacheDegradesButKeepsServing(t *testing.T) {
	h := newHandler(stubPinger{}, stubPinger{err: errors.New("i/o timeout")})

	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("an unreachable cache must not take the instance out of rotation; got %d", w.Code)
	}
	if got := decode(t, w)["status"]; got != statusDegraded {
		t.Errorf("want %q; got %v", statusDegraded, got)
	}
}

// Running without Redis at all is a supported deployment, not a fault.
func TestAbsentCacheIsNotAFailure(t *testing.T) {
	h := newHandler(stubPinger{}, nil)

	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d", w.Code)
	}

	body := decode(t, w)
	if body["status"] != statusAvailable {
		t.Errorf("want %q; got %v", statusAvailable, body["status"])
	}
	if _, present := body["impaired"]; present {
		t.Errorf("an absent cache must not be reported as impaired; got %v", body["impaired"])
	}
}

// The public probe must not disclose dependency states or the environment name.
func TestPublicCheckExposesOnlyStatusAndVersion(t *testing.T) {
	h := newHandler(stubPinger{}, stubPinger{})

	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	body := decode(t, w)
	for _, leaked := range []string{"dependencies", "environment"} {
		if _, present := body[leaked]; present {
			t.Errorf("the public probe must not expose %q", leaked)
		}
	}
	if body["version"] != "1.0.0" {
		t.Errorf("want the version; got %v", body["version"])
	}
}

// TestLiveAnswersWhileEveryDependencyIsDown is the separation this endpoint
// exists for. With one endpoint doing both jobs, a database outage marked the
// process dead and the platform restarted every replica — turning an outage
// that would have ended when the database came back into a restart loop that
// could not.
func TestLiveAnswersWhileEveryDependencyIsDown(t *testing.T) {
	down := stubPinger{err: errors.New("connection refused")}
	h := newHandler(down, down)

	w := httptest.NewRecorder()
	h.Live(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/livez", nil))

	if w.Code != http.StatusOK {
		t.Errorf("want 200 while the dependencies are down; got %d", w.Code)
	}
	if got := decode(t, w)["status"]; got != statusAvailable {
		t.Errorf("want %q; got %v", statusAvailable, got)
	}
}

// TestLiveProbesNothing pins the other half: an endpoint a restart policy polls
// every few seconds must not open a database connection to answer.
func TestLiveProbesNothing(t *testing.T) {
	probe := &countingPinger{}
	h := newHandler(probe, probe)

	w := httptest.NewRecorder()
	h.Live(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/livez", nil))

	if probe.calls != 0 {
		t.Errorf("Live pinged a dependency %d times; want none", probe.calls)
	}
	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d", w.Code)
	}
}

// countingPinger counts the probes made against it.
type countingPinger struct{ calls int }

func (p *countingPinger) Ping(context.Context) error {
	p.calls++
	return nil
}

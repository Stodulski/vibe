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

type stubPool struct{ stats map[string]int64 }

func (s stubPool) PoolStats() map[string]int64 { return s.stats }

func newResponder() *httpx.Responder {
	return httpx.NewResponder(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
}

func newHandler(database, cache Pinger, dbPool, cachePool PoolReporter) *Handler {
	return NewHandler(Dependencies{
		Database: database, Cache: cache, DBPool: dbPool, CachePool: cachePool,
		Respond: newResponder(),
	}, Config{Environment: "test", Version: "1.0.0"})
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
	h := newHandler(stubPinger{}, stubPinger{}, nil, nil)

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
	h := newHandler(stubPinger{err: errors.New("connection refused")}, stubPinger{}, nil, nil)

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
	h := newHandler(stubPinger{}, stubPinger{err: errors.New("i/o timeout")}, nil, nil)

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
	h := newHandler(stubPinger{}, nil, nil, nil)

	w := httptest.NewRecorder()
	h.Detailed(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d", w.Code)
	}

	body := decode(t, w)
	if body["status"] != statusAvailable {
		t.Errorf("want %q; got %v", statusAvailable, body["status"])
	}
	deps, _ := body["dependencies"].(map[string]any)
	if deps["redis"] != stateNotConfigured {
		t.Errorf("want %q; got %v", stateNotConfigured, deps["redis"])
	}
}

// The public probe must not disclose pool figures or the environment name.
func TestPublicCheckExposesOnlyStatusAndVersion(t *testing.T) {
	h := newHandler(stubPinger{}, stubPinger{}, stubPool{map[string]int64{"total": 25}}, nil)

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

func TestDetailedIncludesPoolFigures(t *testing.T) {
	h := newHandler(
		stubPinger{}, stubPinger{},
		stubPool{map[string]int64{"total": 25, "idle": 10}},
		stubPool{map[string]int64{"hits": 100}},
	)

	w := httptest.NewRecorder()
	h.Detailed(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	deps, _ := decode(t, w)["dependencies"].(map[string]any)
	for key, want := range map[string]string{
		"db_pool_total":   "25",
		"db_pool_idle":    "10",
		"redis_pool_hits": "100",
	} {
		if deps[key] != want {
			t.Errorf("want %s = %s; got %v", key, want, deps[key])
		}
	}
}

func TestDetailedOmitsPoolFiguresWhenUnavailable(t *testing.T) {
	h := newHandler(stubPinger{}, stubPinger{}, nil, nil)

	w := httptest.NewRecorder()
	h.Detailed(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	deps, _ := decode(t, w)["dependencies"].(map[string]any)
	for key := range deps {
		if len(key) > 5 && key[len(key)-5:] == "_pool" {
			t.Errorf("unexpected pool key %q with no pooler wired", key)
		}
	}
	if deps["database"] != stateOK {
		t.Errorf("the dependency states must still be reported; got %v", deps["database"])
	}
}

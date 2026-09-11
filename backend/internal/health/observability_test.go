package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubBreaker struct {
	name    string
	state   string
	revenue bool
}

func (s stubBreaker) Name() string        { return s.name }
func (s stubBreaker) State() string       { return s.state }
func (s stubBreaker) BlocksRevenue() bool { return s.revenue }

// payments returns a breaker standing in for MercadoPago in the given state.
func payments(state string) stubBreaker {
	return stubBreaker{name: "mercadopago", state: state, revenue: true}
}

// whatsapp returns a breaker for a dependency whose loss costs notifications
// rather than money.
func whatsapp(state string) stubBreaker {
	return stubBreaker{name: "whatsapp", state: state, revenue: false}
}

type stubQueues struct {
	stats []QueueStats
	err   error
}

func (s stubQueues) QueueStats(context.Context) ([]QueueStats, error) {
	return s.stats, s.err
}

type stubMetrics struct{ values map[string]any }

func (s stubMetrics) Metrics() map[string]any { return s.values }

func newObservedHandler(t *testing.T, d Dependencies) *Handler {
	t.Helper()

	if d.Database == nil {
		d.Database = stubPinger{}
	}
	if d.Cache == nil {
		d.Cache = stubPinger{}
	}
	d.Respond = newResponder()
	return NewHandler(d, Config{Environment: "test", Version: "1.0.0"})
}

func get(t *testing.T, handler http.HandlerFunc) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	return w, decode(t, w)
}

// impairedList reads the "impaired" array as strings.
func impairedList(t *testing.T, body map[string]any) []string {
	t.Helper()

	raw, present := body["impaired"]
	if !present {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("want impaired to be a list; got %T", raw)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.(string))
	}
	return out
}

// ---------------------------------------------------------------------------
// The payment stack
// ---------------------------------------------------------------------------

// The finding: with MercadoPago down, no client on any tenant can pay and this
// endpoint answered 200 available. Railway and the Docker healthcheck stayed
// green while revenue was zero.
func TestAnOpenPaymentBreakerIsReportedAsDegraded(t *testing.T) {
	h := newObservedHandler(t, Dependencies{Breakers: []Breaker{payments("open"), whatsapp("closed")}})

	_, body := get(t, h.Check)

	if body["status"] != statusDegraded {
		t.Errorf("want %q with the payment breaker open; got %v", statusDegraded, body["status"])
	}
	if got := impairedList(t, body); !slices.Contains(got, impairedPayments) {
		t.Errorf("want %q named in impaired; got %v", impairedPayments, got)
	}
}

// The other half of the same decision. Restarting the container does not
// restore the payment provider: it removes the instance still serving the
// schedule, the dashboard and every booking taken at the counter.
func TestAnOpenPaymentBreakerDoesNotTakeTheInstanceOutOfRotation(t *testing.T) {
	h := newObservedHandler(t, Dependencies{Breakers: []Breaker{payments("open")}})

	w, _ := get(t, h.Check)

	if w.Code != http.StatusOK {
		t.Errorf("a payment outage must stay in rotation; got %d", w.Code)
	}
}

// Half-open means the breaker is probing a dependency that was failing a
// moment ago and is admitting one or two calls. The payment path is not
// working; it is being retried.
func TestAHalfOpenPaymentBreakerIsStillDegraded(t *testing.T) {
	h := newObservedHandler(t, Dependencies{Breakers: []Breaker{payments("half-open")}})

	_, body := get(t, h.Check)

	if body["status"] != statusDegraded {
		t.Errorf("want %q while the payment breaker is probing; got %v", statusDegraded, body["status"])
	}
}

// Losing WhatsApp costs notifications, not money. Reporting it as degraded
// would train whoever reads this endpoint to ignore it.
func TestABreakerThatDoesNotBlockRevenueDoesNotDegradeTheService(t *testing.T) {
	h := newObservedHandler(t, Dependencies{Breakers: []Breaker{payments("closed"), whatsapp("open")}})

	w, body := get(t, h.Check)

	if w.Code != http.StatusOK || body["status"] != statusAvailable {
		t.Errorf("want an available 200 with only the WhatsApp breaker open; got %d / %v",
			w.Code, body["status"])
	}
	if got := impairedList(t, body); len(got) != 0 {
		t.Errorf("want nothing named impaired; got %v", got)
	}
}

// The public probe answers an uptime monitor, which needs to know that this is
// a payment outage and not a database one.
func TestThePublicProbeNamesTheImpairedSubsystem(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Cache:    stubPinger{err: errors.New("i/o timeout")},
		Breakers: []Breaker{payments("open")},
	})

	_, body := get(t, h.Check)

	got := impairedList(t, body)
	for _, want := range []string{impairedCache, impairedPayments} {
		if !slices.Contains(got, want) {
			t.Errorf("want %q named in impaired; got %v", want, got)
		}
	}
	// It still says only that much: the state of each individual breaker stays
	// behind the superadmin guard.
	if _, present := body["breakers"]; present {
		t.Error("the public probe must not expose per-breaker state")
	}
}

// The detailed endpoint is where an operator finds out which breaker.
func TestTheDetailedProbeReportsEveryBreakerState(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Breakers: []Breaker{payments("open"), whatsapp("half-open")},
	})

	_, body := get(t, h.Detailed)

	breakers, ok := body["breakers"].(map[string]any)
	if !ok {
		t.Fatalf("want a breakers object; got %T", body["breakers"])
	}
	for name, want := range map[string]string{"mercadopago": "open", "whatsapp": "half-open"} {
		if breakers[name] != want {
			t.Errorf("want %s = %q; got %v", name, want, breakers[name])
		}
	}
}

// A deployment with no breakers configured must not grow an empty object that
// reads as "all breakers are fine".
func TestNoBreakersMeansNoBreakerSection(t *testing.T) {
	h := newObservedHandler(t, Dependencies{})

	_, body := get(t, h.Detailed)

	if _, present := body["breakers"]; present {
		t.Errorf("want no breakers section when none are wired; got %v", body["breakers"])
	}
}

// ---------------------------------------------------------------------------
// Queue depth
// ---------------------------------------------------------------------------

// Both sweepers log a completion count bounded by their own batch limit, so a
// backlog of fifty and a backlog of fifty thousand log the same line. These
// are the figures that tell them apart.
func TestTheDetailedProbeReportsQueueDepthAndOldestDueAge(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Queues: stubQueues{stats: []QueueStats{
			{Name: "webhook_events", Pending: 51234, Processing: 4, Exhausted: 9, OldestDueSeconds: 7200},
			{Name: "failed_refunds", Pending: 0, Processing: 0, Exhausted: 2, OldestDueSeconds: 0},
		}},
	})

	_, body := get(t, h.Detailed)

	queues, ok := body["queues"].([]any)
	if !ok {
		t.Fatalf("want a queues list; got %T (%v)", body["queues"], body["queues"])
	}
	if len(queues) != 2 {
		t.Fatalf("want both queues reported; got %d", len(queues))
	}

	first, _ := queues[0].(map[string]any)
	for key, want := range map[string]float64{
		"pending":            51234,
		"processing":         4,
		"exhausted":          9,
		"oldest_due_seconds": 7200,
	} {
		if got := first[key]; got != want {
			t.Errorf("want webhook_events %s = %v; got %v", key, want, got)
		}
	}
	if first["name"] != "webhook_events" {
		t.Errorf("want the queue named; got %v", first["name"])
	}
}

// The health endpoint's first job is to say whether the process is serving,
// and a backlog query that failed does not change that answer.
func TestAFailedQueueQueryIsReportedWithoutFailingTheCheck(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Queues: stubQueues{err: errors.New("statement timeout")},
	})

	w, body := get(t, h.Detailed)

	if w.Code != http.StatusOK {
		t.Errorf("want the check to still answer 200; got %d", w.Code)
	}
	if body["status"] != statusAvailable {
		t.Errorf("want %q; got %v", statusAvailable, body["status"])
	}
	if got, _ := body["queues_error"].(string); got != "statement timeout" {
		t.Errorf("want the failure reported rather than swallowed; got %v", body["queues_error"])
	}
	if _, present := body["queues"]; present {
		t.Error("a failed query must not also report a backlog")
	}
}

// ---------------------------------------------------------------------------
// Process metrics
// ---------------------------------------------------------------------------

// The finding: expvar publishes the goroutine count that would diagnose pool
// starvation, and /debug/vars is registered only in development — so in
// production the number is computed and unreachable. This is where it becomes
// reachable.
func TestTheDetailedProbeCarriesTheProcessMetrics(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Metrics: stubMetrics{values: map[string]any{
			"requests_in_flight": int64(37),
			"latency_ms_p99":     412.5,
			"goroutines":         int64(2100),
		}},
	})

	_, body := get(t, h.Detailed)

	metrics, ok := body["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("want a metrics object; got %T (%v)", body["metrics"], body["metrics"])
	}
	for key, want := range map[string]float64{
		"requests_in_flight": 37,
		"latency_ms_p99":     412.5,
		"goroutines":         2100,
	} {
		if got := metrics[key]; got != want {
			t.Errorf("want %s = %v; got %v", key, want, got)
		}
	}
}

// The counters describe traffic volume and process internals, which is not
// public information.
func TestThePublicProbeCarriesNoMetrics(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Metrics: stubMetrics{values: map[string]any{"requests_in_flight": int64(37)}},
		Queues:  stubQueues{stats: []QueueStats{{Name: "webhook_events", Pending: 5}}},
	})

	_, body := get(t, h.Check)

	for _, leaked := range []string{"metrics", "queues", "breakers", "dependencies", "environment"} {
		if _, present := body[leaked]; present {
			t.Errorf("the public probe must not expose %q", leaked)
		}
	}
}

// An unreachable database is still the one thing that takes the instance out
// of rotation, whatever else is impaired.
func TestTheDatabaseStillOutranksEverythingElse(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Database: stubPinger{err: errors.New("connection refused")},
		Breakers: []Breaker{payments("open")},
	})

	w, body := get(t, h.Check)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 with the database unreachable; got %d", w.Code)
	}
	got := impairedList(t, body)
	for _, want := range []string{impairedDatabase, impairedPayments} {
		if !slices.Contains(got, want) {
			t.Errorf("want %q named in impaired; got %v", want, got)
		}
	}
}

package health

import (
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
	state   string
	revenue bool
}

func (s stubBreaker) State() string       { return s.state }
func (s stubBreaker) BlocksRevenue() bool { return s.revenue }

// payments returns a breaker standing in for MercadoPago in the given state.
func payments(state string) stubBreaker {
	return stubBreaker{state: state, revenue: true}
}

// whatsapp returns a breaker for a dependency whose loss costs notifications
// rather than money.
func whatsapp(state string) stubBreaker {
	return stubBreaker{state: state, revenue: false}
}

func newObservedHandler(t *testing.T, d Dependencies) *Handler {
	t.Helper()

	if d.Database == nil {
		d.Database = stubPinger{}
	}
	if d.Cache == nil {
		d.Cache = stubPinger{}
	}
	d.Respond = newResponder()
	return NewHandler(d, Config{Version: "1.0.0"})
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

// Pool figures, queue backlogs and process counters are not public information,
// so the public probe carries none of them, whatever is impaired.
func TestThePublicProbeCarriesNoMetrics(t *testing.T) {
	h := newObservedHandler(t, Dependencies{
		Breakers: []Breaker{payments("open")},
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

package circuitbreaker

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestClosedAllowsRequests(t *testing.T) {
	cb := New(Config{Name: "test", MaxFailures: 3, ResetTimeout: 100 * time.Millisecond})

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestOpensAfterMaxFailures(t *testing.T) {
	cb := New(Config{Name: "test", MaxFailures: 3, ResetTimeout: 100 * time.Millisecond})

	for range 3 {
		if err := cb.AllowRequest(); err != nil {
			t.Fatalf("unexpected AllowRequest error during setup: %v", err)
		}
		cb.RecordFailure()
	}

	if err := cb.AllowRequest(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
}

func TestSuccessResetsFailureCount(t *testing.T) {
	cb := New(Config{Name: "test", MaxFailures: 3, ResetTimeout: 100 * time.Millisecond})

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordSuccess() // Reset consecutive failures.

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()

	// Should still be closed (only 2 consecutive failures after reset).
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestTransitionsToHalfOpenAfterTimeout(t *testing.T) {
	cb := New(Config{Name: "test", MaxFailures: 2, ResetTimeout: 50 * time.Millisecond, HalfOpenMaxReqs: 1})

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()

	// Should be open.
	if err := cb.AllowRequest(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open and allow one request.
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("expected nil after timeout, got %v", err)
	}
}

func TestHalfOpenClosesOnSuccess(t *testing.T) {
	cb := New(Config{Name: "test", MaxFailures: 1, ResetTimeout: 10 * time.Millisecond, HalfOpenMaxReqs: 1})

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()

	time.Sleep(15 * time.Millisecond)

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordSuccess()

	// Should be closed now.
	for range 5 {
		if err := cb.AllowRequest(); err != nil {
			t.Fatalf("expected nil (closed), got %v", err)
		}
		cb.RecordSuccess()
	}
}

func TestHalfOpenReopensOnFailure(t *testing.T) {
	cb := New(Config{Name: "test", MaxFailures: 1, ResetTimeout: 10 * time.Millisecond, HalfOpenMaxReqs: 1})

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure()

	time.Sleep(15 * time.Millisecond)

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure() // Fail in half-open → back to open.

	if err := cb.AllowRequest(); !errors.Is(err, ErrOpen) {
		t.Fatalf("expected ErrOpen, got %v", err)
	}
}

func TestOnStateChangeCallback(t *testing.T) {
	var transitions []string
	cb := New(Config{
		Name:            "test",
		MaxFailures:     1,
		ResetTimeout:    10 * time.Millisecond,
		HalfOpenMaxReqs: 1,
		OnStateChange: func(name string, from, to State) {
			transitions = append(transitions, from.String()+"->"+to.String())
		},
	})

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordFailure() // closed -> open

	time.Sleep(15 * time.Millisecond)
	if err := cb.AllowRequest(); err != nil { // open -> half-open
		t.Fatalf("unexpected AllowRequest error during setup: %v", err)
	}
	cb.RecordSuccess() // half-open -> closed

	expected := []string{"closed->open", "open->half-open", "half-open->closed"}
	if len(transitions) != len(expected) {
		t.Fatalf("expected %d transitions, got %d: %v", len(expected), len(transitions), transitions)
	}
	for i, e := range expected {
		if transitions[i] != e {
			t.Errorf("transition %d: expected %q, got %q", i, e, transitions[i])
		}
	}
}

func TestNilCircuitBreakerIsNoOp(t *testing.T) {
	var cb *CircuitBreaker

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	// Should not panic.
	cb.RecordSuccess()
	cb.RecordFailure()

	if name := cb.Name(); name != "" {
		t.Fatalf("expected empty name, got %q", name)
	}
}

// ---------------------------------------------------------------------------
// State and Status — the accessors the health endpoint reads
// ---------------------------------------------------------------------------

// trip drives a breaker to open through the normal path.
func trip(t *testing.T, cb *CircuitBreaker, failures int) {
	t.Helper()
	for range failures {
		if err := cb.AllowRequest(); err != nil {
			t.Fatalf("unexpected AllowRequest error during setup: %v", err)
		}
		cb.RecordFailure()
	}
}

func TestStateReportsClosedBeforeAnythingFails(t *testing.T) {
	cb := New(Config{Name: "mercadopago", MaxFailures: 3, ResetTimeout: time.Hour})

	if got := cb.State(); got != StateClosed {
		t.Errorf("want closed; got %v", got)
	}
}

// The whole point of the accessor: with the payment provider's breaker open,
// no client on any tenant can pay, and the health endpoint has to be able to
// see it.
func TestStateReportsOpenOnceTheBreakerHasTripped(t *testing.T) {
	cb := New(Config{Name: "mercadopago", MaxFailures: 3, ResetTimeout: time.Hour})
	trip(t, cb, 3)

	if got := cb.State(); got != StateOpen {
		t.Errorf("want open after the failure budget is spent; got %v", got)
	}
	if got := cb.State().String(); got != "open" {
		t.Errorf("want the string %q; got %q", "open", got)
	}
}

// A breaker whose reset timeout has elapsed is stored as open but will admit
// the next probe. Reporting "open" there tells an operator the payment path is
// refusing every call when it is in fact already testing for recovery.
func TestStateReportsHalfOpenOnceTheResetTimeoutHasElapsed(t *testing.T) {
	cb := New(Config{Name: "mercadopago", MaxFailures: 1, ResetTimeout: 10 * time.Millisecond})
	trip(t, cb, 1)

	if got := cb.State(); got != StateOpen {
		t.Fatalf("setup: want open; got %v", got)
	}

	time.Sleep(20 * time.Millisecond)

	if got := cb.State(); got != StateHalfOpen {
		t.Errorf("want half-open once the reset timeout has elapsed; got %v", got)
	}
}

// Reading the state must not transition the breaker. A health endpoint polled
// every ten seconds would otherwise perform the open -> half-open transition
// itself and spend the probe a real payment should have had, so the breaker
// would report recovery nobody tested and the state-change log line would fire
// from a health check rather than from traffic.
func TestReadingTheStateDoesNotTransitionTheBreaker(t *testing.T) {
	var transitions []string
	cb := New(Config{
		Name: "mercadopago", MaxFailures: 1, ResetTimeout: 10 * time.Millisecond, HalfOpenMaxReqs: 1,
		OnStateChange: func(_ string, from, to State) {
			transitions = append(transitions, from.String()+"->"+to.String())
		},
	})
	trip(t, cb, 1)

	if want := []string{"closed->open"}; len(transitions) != 1 || transitions[0] != want[0] {
		t.Fatalf("setup: want %v; got %v", want, transitions)
	}
	time.Sleep(20 * time.Millisecond)

	for range 50 {
		if got := cb.State(); got != StateHalfOpen {
			t.Fatalf("want half-open reported; got %v", got)
		}
	}

	if len(transitions) != 1 {
		t.Errorf("reading the state must not transition the breaker; transitions after 50 reads: %v", transitions)
	}

	// The first real caller is still the one that performs the transition.
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("the first real request after the reset timeout must be admitted; got %v", err)
	}
	if len(transitions) != 2 || transitions[1] != "open->half-open" {
		t.Errorf("want the real request to perform the transition; got %v", transitions)
	}
}

func TestStatusCarriesTheNameAndFailureCount(t *testing.T) {
	cb := New(Config{Name: "mercadopago", MaxFailures: 5, ResetTimeout: time.Hour})
	trip(t, cb, 2)

	got := cb.Status()
	if got.Name != "mercadopago" {
		t.Errorf("want the breaker named; got %q", got.Name)
	}
	if got.State != StateClosed {
		t.Errorf("two failures out of five must still read closed; got %v", got.State)
	}
	if got.ConsecutiveFailures != 2 {
		t.Errorf("want 2 consecutive failures; got %d", got.ConsecutiveFailures)
	}
	if got.LastFailure.IsZero() {
		t.Error("want the time of the last failure, so an operator can tell an actively-failing " +
			"dependency from one nobody has retried")
	}
}

// Every other method on this type is a no-op on nil so the breaker can be left
// unconfigured. A health endpoint that panics because one is absent is worse
// than no health endpoint.
func TestStateAndStatusAreSafeOnANilBreaker(t *testing.T) {
	var cb *CircuitBreaker

	if got := cb.State(); got != StateClosed {
		t.Errorf("an unconfigured breaker rejects nothing, so it must read closed; got %v", got)
	}
	got := cb.Status()
	if got.Name != "" || got.ConsecutiveFailures != 0 || !got.LastFailure.IsZero() {
		t.Errorf("want the zero Status; got %+v", got)
	}
}

// TestHalfOpenAdmitsExactlyItsBudget pins how many real calls a breaker gambles
// on a provider it believes is down.
//
// The request that finds the reset timeout elapsed is itself the first probe.
// Admitting it without charging the budget makes the effective allowance
// HalfOpenMaxReqs+1 — on an allowance of one, twice the intended number of live
// payments sent at a provider that has been failing.
func TestHalfOpenAdmitsExactlyItsBudget(t *testing.T) {
	for _, budget := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("budget_%d", budget), func(t *testing.T) {
			cb := New(Config{
				Name: "test", MaxFailures: 1,
				ResetTimeout:    time.Millisecond,
				HalfOpenMaxReqs: budget,
			})

			if err := cb.AllowRequest(); err != nil {
				t.Fatalf("setup: a fresh breaker must allow: %v", err)
			}
			cb.RecordFailure()
			time.Sleep(5 * time.Millisecond)

			// Count admissions without answering any of them, so nothing closes
			// the breaker or reopens it part-way through the count.
			admitted := 0
			for range budget + 5 {
				if cb.AllowRequest() == nil {
					admitted++
				}
			}

			if admitted != budget {
				t.Errorf("half-open admitted %d live calls on a budget of %d; "+
					"the probe that opened the window must be charged for", admitted, budget)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Stale in-flight reports
// ---------------------------------------------------------------------------

// recorder collects the breaker's state transitions.
//
// The assertions below are on this sequence rather than on a final State
// reading, because the two answers differ in exactly the case under test: a
// breaker that closed on a stale success and one that never left half-open both
// end up admitting the next call, and only the transition log says which
// happened and on whose evidence.
type recorder struct{ seen []string }

func (r *recorder) hook(_ string, from, to State) {
	r.seen = append(r.seen, from.String()+"->"+to.String())
}

func (r *recorder) equal(want ...string) bool {
	if len(r.seen) != len(want) {
		return false
	}
	for i := range want {
		if r.seen[i] != want[i] {
			return false
		}
	}
	return true
}

// openWithOneCallStillInFlight drives a breaker to open while leaving exactly
// one admitted call unanswered, and returns it once the reset timeout has
// elapsed so the next AllowRequest opens the half-open window.
//
// The unanswered call is the whole point: it was admitted while the breaker was
// closed, so whatever it eventually reports is evidence about the provider
// before the outage was declared.
func openWithOneCallStillInFlight(t *testing.T, cb *CircuitBreaker) {
	t.Helper()

	// The call that will report late. Admitted, never answered here.
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("setup: a fresh breaker must admit the long-running call: %v", err)
	}

	// A different call fails and trips the breaker.
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("setup: a closed breaker must admit: %v", err)
	}
	cb.RecordFailure()

	if err := cb.AllowRequest(); !errors.Is(err, ErrOpen) {
		t.Fatalf("setup: want the breaker open; got %v", err)
	}

	time.Sleep(5 * time.Millisecond) // past ResetTimeout, which callers set to 1ms
}

// A success from a call issued before the outage must not close the breaker.
//
// On the mercadopago breaker this is the payment path: an eight-second HTTP
// timeout means a request sent before MercadoPago started failing can land its
// RecordSuccess inside the half-open window that opened afterwards. Crediting it
// closes the breaker on evidence gathered before anybody thought the provider
// was down — nothing re-probed it, so the next real payment is the probe, and it
// fails.
func TestAStaleSuccessDoesNotCloseTheBreaker(t *testing.T) {
	var log recorder
	cb := New(Config{
		Name: "mercadopago", MaxFailures: 1,
		ResetTimeout: time.Millisecond, HalfOpenMaxReqs: 1,
		OnStateChange: log.hook,
	})

	openWithOneCallStillInFlight(t, cb)

	// The half-open window opens and admits its probe. The probe has not
	// answered yet.
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("setup: the first call after the reset timeout must be admitted; got %v", err)
	}
	if !log.equal("closed->open", "open->half-open") {
		t.Fatalf("setup: want the breaker in a half-open window; transitions were %v", log.seen)
	}

	// Now the pre-outage call finally answers, successfully.
	cb.RecordSuccess()

	if !log.equal("closed->open", "open->half-open") {
		t.Errorf("a success from a call issued before the breaker opened closed it: transitions %v; "+
			"nothing has re-probed the provider, so the next live payment is the probe", log.seen)
	}
	if got := cb.State(); got != StateHalfOpen {
		t.Errorf("want the breaker still testing for recovery; got %v", got)
	}
}

// The probe's own success still closes the breaker. Without this the fix would
// be a breaker that never recovers, which is the same outage by another route.
func TestTheProbesOwnSuccessStillClosesTheBreaker(t *testing.T) {
	var log recorder
	cb := New(Config{
		Name: "mercadopago", MaxFailures: 1,
		ResetTimeout: time.Millisecond, HalfOpenMaxReqs: 1,
		OnStateChange: log.hook,
	})

	openWithOneCallStillInFlight(t, cb)

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("setup: the probe must be admitted; got %v", err)
	}
	cb.RecordSuccess() // the stale call, discounted
	cb.RecordSuccess() // the probe

	if !log.equal("closed->open", "open->half-open", "half-open->closed") {
		t.Errorf("want the probe's success to close the breaker; transitions %v", log.seen)
	}
}

// A discounted stale success hands its probe slot back.
//
// Spending the slot on a report that tested nothing would leave the window with
// no budget and no verdict: AllowRequest refuses every later caller, and since
// only a report can move the breaker out of half-open, a breaker in that
// position never leaves it. That is a permanent outage on the payment path,
// which is worse than the defect being fixed.
func TestADiscountedStaleSuccessDoesNotSpendTheProbeBudget(t *testing.T) {
	cb := New(Config{
		Name: "mercadopago", MaxFailures: 1,
		ResetTimeout: time.Millisecond, HalfOpenMaxReqs: 1,
	})

	openWithOneCallStillInFlight(t, cb)

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("setup: the probe must be admitted; got %v", err)
	}
	cb.RecordSuccess() // the stale call

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("the window must still admit a probe after a stale report was discounted; got %v", err)
	}
}

// A stale success arriving before the window opens is spent while the breaker
// is still open, so it does not follow the probe into the window and get
// discounted in its place. Getting this wrong costs a recovery: the probe
// succeeds, its success is taken for the stale one, and the breaker stays open
// for another reset timeout with a provider that is already back.
func TestAStaleSuccessSpentWhileOpenDoesNotDelayRecovery(t *testing.T) {
	var log recorder
	cb := New(Config{
		Name: "mercadopago", MaxFailures: 1,
		ResetTimeout: time.Millisecond, HalfOpenMaxReqs: 1,
		OnStateChange: log.hook,
	})

	openWithOneCallStillInFlight(t, cb)

	cb.RecordSuccess() // the stale call, answering while the breaker is open

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("setup: the probe must be admitted; got %v", err)
	}
	cb.RecordSuccess() // the probe

	if !log.equal("closed->open", "open->half-open", "half-open->closed") {
		t.Errorf("want the probe's success to close the breaker on the first window; transitions %v", log.seen)
	}
}

// A failure counts wherever it came from.
//
// The stale accounting must not swallow failures: discarding a real one because
// a pre-outage call happened to report first would leave the breaker closed over
// a provider that is still down, which is the failure this whole type exists to
// prevent.
func TestAFailureIsNeverDiscountedAsStale(t *testing.T) {
	var log recorder
	cb := New(Config{
		Name: "mercadopago", MaxFailures: 1,
		ResetTimeout: time.Millisecond, HalfOpenMaxReqs: 1,
		OnStateChange: log.hook,
	})

	openWithOneCallStillInFlight(t, cb)

	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("setup: the probe must be admitted; got %v", err)
	}
	cb.RecordFailure() // the probe: the provider is still down

	if !log.equal("closed->open", "open->half-open", "half-open->open") {
		t.Errorf("want the probe's failure to reopen the breaker; transitions %v", log.seen)
	}
	if err := cb.AllowRequest(); !errors.Is(err, ErrOpen) {
		t.Errorf("want the breaker open again; got %v", err)
	}
}

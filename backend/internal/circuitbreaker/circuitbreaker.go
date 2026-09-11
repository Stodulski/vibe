package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned when the circuit breaker is open and requests are rejected.
var ErrOpen = errors.New("circuit breaker is open")

// State represents the circuit breaker state.
type State int

// State constants for the circuit breaker's three-state machine.
const (
	StateClosed   State = iota // Normal operation, requests flow through.
	StateOpen                  // Requests are rejected immediately.
	StateHalfOpen              // Limited requests allowed to test recovery.
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures a CircuitBreaker.
type Config struct {
	Name            string
	MaxFailures     int           // Consecutive failures before opening. Default: 5.
	ResetTimeout    time.Duration // Time before transitioning open -> half-open. Default: 30s.
	HalfOpenMaxReqs int           // Successful requests in half-open to close. Default: 2.
	OnStateChange   func(name string, from, to State)
}

// CircuitBreaker implements the circuit breaker pattern for external service calls.
// All methods are safe to call on a nil receiver (no-op), making it optional.
type CircuitBreaker struct {
	name            string
	maxFailures     int
	resetTimeout    time.Duration
	halfOpenMaxReqs int
	onStateChange   func(string, State, State)

	mu                  sync.Mutex
	state               State
	consecutiveFailures int
	lastFailureTime     time.Time
	halfOpenSuccesses   int
	halfOpenRequests    int

	// inFlight is how many admitted calls have not reported yet.
	//
	// It relies on the contract every caller of this package already follows
	// and CLAUDE.md already mandates: an AllowRequest that returns nil is
	// followed by exactly one RecordSuccess or RecordFailure. A rejected
	// request admits nothing and so counts for nothing.
	inFlight int
	// staleReports is how many of the calls that were still in flight when the
	// breaker last opened have yet to report.
	//
	// It is the answer to a success that proves nothing. A request issued while
	// the breaker was closed can land its RecordSuccess long after the outage
	// was declared — an eight-second HTTP timeout against a provider that
	// started failing seven seconds ago — and if it lands during the half-open
	// window it is credited as a probe. On the mercadopago breaker, whose
	// HalfOpenMaxReqs is 2, two of those close the payment path on the strength
	// of two calls that were made before anybody thought MercadoPago was down.
	// Nothing re-probed it; the next real payment is the probe, and it fails.
	//
	// Counting them is what makes them distinguishable: exactly this many
	// reports can arrive that were not admitted by the current window, so the
	// first staleReports of them are spent against the snapshot and only what
	// is left over is a probe answering.
	staleReports int
}

// New creates a CircuitBreaker with the given configuration.
func New(cfg Config) *CircuitBreaker {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = 30 * time.Second
	}
	if cfg.HalfOpenMaxReqs <= 0 {
		cfg.HalfOpenMaxReqs = 2
	}
	return &CircuitBreaker{
		name:            cfg.Name,
		maxFailures:     cfg.MaxFailures,
		resetTimeout:    cfg.ResetTimeout,
		halfOpenMaxReqs: cfg.HalfOpenMaxReqs,
		onStateChange:   cfg.OnStateChange,
		state:           StateClosed,
	}
}

// AllowRequest checks if a request is allowed. Returns ErrOpen if the circuit is open.
func (cb *CircuitBreaker) AllowRequest() error {
	if cb == nil {
		return nil
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.inFlight++
		return nil
	case StateOpen:
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.transition(StateHalfOpen)
			cb.halfOpenSuccesses = 0
			// This request is the first probe, not a free one alongside the
			// budget. Starting the count at zero admits halfOpenMaxReqs more
			// on top of it, so a breaker set to gamble one live payment on a
			// provider it believes is down gambles two.
			cb.halfOpenRequests = 1
			cb.inFlight++
			return nil
		}
		return ErrOpen
	case StateHalfOpen:
		if cb.halfOpenRequests >= cb.halfOpenMaxReqs {
			return ErrOpen
		}
		cb.halfOpenRequests++
		cb.inFlight++
		return nil
	}
	return nil
}

// settle books one report against the admission it answers, and reports
// whether that admission predates the breaker's current open period.
//
// Caller must hold cb.mu.
func (cb *CircuitBreaker) settle() (stale bool) {
	if cb.inFlight > 0 {
		cb.inFlight--
	}
	if cb.staleReports > 0 {
		cb.staleReports--
		return true
	}
	return false
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	stale := cb.settle()

	switch cb.state {
	case StateClosed:
		cb.consecutiveFailures = 0
	case StateHalfOpen:
		if stale {
			// Issued before the breaker opened, so it says nothing about
			// whether the provider has recovered — crediting it would close the
			// payment path on evidence gathered before the outage.
			//
			// The probe slot is handed back rather than left spent. Without
			// that, a window whose only report was a stale one is a window with
			// its budget consumed and nothing to show for it: AllowRequest
			// refuses every later caller, and since only a real report can
			// reopen the breaker, a breaker in that position never leaves
			// half-open again. Refunding keeps the window testable, and the
			// number of refunds is bounded by the number of calls that were in
			// flight when it opened.
			if cb.halfOpenRequests > 0 {
				cb.halfOpenRequests--
			}
			return
		}
		cb.halfOpenSuccesses++
		if cb.halfOpenSuccesses >= cb.halfOpenMaxReqs {
			cb.transition(StateClosed)
			cb.consecutiveFailures = 0
		}
	case StateOpen:
		// AllowRequest rejects every call while open, so the only successes
		// that reach this state are the stale ones settle has just booked. No
		// state change, kept explicit for exhaustiveness.
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// A failure counts wherever it came from. A stale one reopening a half-open
	// breaker costs one reset timeout; discarding a real one because a stale
	// call happened to report first would leave the breaker closed over a
	// provider that is still down.
	cb.settle()

	switch cb.state {
	case StateClosed:
		cb.consecutiveFailures++
		cb.lastFailureTime = time.Now()
		if cb.consecutiveFailures >= cb.maxFailures {
			cb.transition(StateOpen)
		}
	case StateHalfOpen:
		cb.lastFailureTime = time.Now()
		cb.transition(StateOpen)
	case StateOpen:
		// Already open (only reachable via a race between AllowRequest and
		// RecordFailure, since every caller checks AllowRequest first and
		// returns early on ErrOpen). No-op, kept explicit for exhaustiveness
		// and to preserve the pre-existing behavior of this unreachable path.
	}
}

// Name returns the circuit breaker name.
func (cb *CircuitBreaker) Name() string {
	if cb == nil {
		return ""
	}
	return cb.name
}

func (cb *CircuitBreaker) transition(to State) {
	from := cb.state
	cb.state = to

	switch to {
	case StateOpen:
		// Everything still unanswered at this instant was issued while the
		// provider was believed healthy. Whatever those calls eventually report
		// is evidence about the world before the outage was declared, so it is
		// booked against this snapshot rather than against the half-open window
		// they may happen to land in. See staleReports.
		cb.staleReports = cb.inFlight
	case StateClosed:
		// The window did its job: whatever is outstanding now was admitted by a
		// breaker that has already decided the provider is back.
		cb.staleReports = 0
	case StateHalfOpen:
		// The snapshot taken when the breaker opened is exactly what this window
		// needs to discount, so it is carried through untouched.
	}

	if cb.onStateChange != nil && from != to {
		cb.onStateChange(cb.name, from, to)
	}
}

// Status is a point-in-time reading of a breaker, for the health endpoints and
// for logs. It is a copy: nothing here holds the lock after Status returns.
type Status struct {
	Name string
	// State is the state the NEXT request would see. See State.
	State State
	// ConsecutiveFailures is how far the breaker is along its way to opening.
	// It is the figure that distinguishes "one flaky call" from "about to trip"
	// while the state is still closed.
	ConsecutiveFailures int
	// LastFailure is when the last failure was recorded, zero if there has
	// never been one. An open breaker whose LastFailure is minutes old is a
	// dependency nobody has retried, not one that is actively failing.
	LastFailure time.Time
}

// State reports the state the next request would see.
//
// It deliberately does not report the stored state verbatim. A breaker that
// opened more than ResetTimeout ago is stored as open, but the next
// AllowRequest will move it to half-open and let a probe through — so
// reporting "open" there tells an operator the payment path is refusing every
// call when in fact it is already probing for recovery. This is a read: it
// resolves that without transitioning, so polling a health endpoint can never
// consume the half-open probe budget a real request should have had.
//
// Safe on a nil receiver, like every other method here: a breaker that was
// never configured never rejects anything, which is exactly closed.
func (cb *CircuitBreaker) State() State {
	return cb.Status().State
}

// Status returns the full reading. Nil-safe; the zero Status names nothing and
// reads closed.
func (cb *CircuitBreaker) Status() Status {
	if cb == nil {
		return Status{State: StateClosed}
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.state
	if state == StateOpen && time.Since(cb.lastFailureTime) > cb.resetTimeout {
		state = StateHalfOpen
	}

	return Status{
		Name:                cb.name,
		State:               state,
		ConsecutiveFailures: cb.consecutiveFailures,
		LastFailure:         cb.lastFailureTime,
	}
}

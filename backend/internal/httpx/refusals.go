package httpx

import (
	"errors"
	"net/http"

	"github.com/stodulski/vibe-server/internal/data"
)

// This file is the one place in the tree that decides which status a refused
// request gets.
//
// It used to be thirty-five places. Every domain answered its own errors with
// respond.Error(w, r, http.StatusConflict, "…") inside an errors.Is switch, so
// "which errors are a 409" was a question you answered by reading thirteen
// files, and two handlers refusing the same sentinel could — and did — disagree
// about the status. The status now travels with the refusal instead of being
// re-decided at each call site: a domain declares its table once (Refusals), a
// handler hands the error over (DomainError), and net/http's status constants
// are named here and nowhere else outside this package.
//
// What stays at the call site is the message, because the message is the only
// part that is genuinely local: the same complexes.ErrActiveBookings is
// "cannot delete complex while it has active bookings" on one route and
// "cannot disconnect MercadoPago while you have active bookings" on another.

// Refusal is a refused request's answer: the status, and the message the client
// reads. A nil Message writes the status alone with no body, which is what a
// webhook's provider reads and all it reads.
type Refusal struct {
	Status  int
	Message any
}

// The refusal constructors. They exist so that a domain package names the
// meaning of a refusal rather than a number, and so that grepping the tree for
// http.Status finds this file and the third-party statuses it does not own.

// BadRequest refuses a request the caller can fix: 400.
func BadRequest(message any) Refusal { return Refusal{http.StatusBadRequest, message} }

// Unauthorized refuses a request with no usable credential: 401.
func Unauthorized(message any) Refusal { return Refusal{http.StatusUnauthorized, message} }

// Forbidden refuses a caller who is known and still not allowed: 403.
func Forbidden(message any) Refusal { return Refusal{http.StatusForbidden, message} }

// NotFound refuses a resource that is not there, or is not this caller's: 404.
func NotFound(message any) Refusal { return Refusal{http.StatusNotFound, message} }

// Conflict refuses a write that collides with the state it found: 409.
func Conflict(message any) Refusal { return Refusal{http.StatusConflict, message} }

// Gone refuses a resource that existed and deliberately does not any more: 410.
func Gone(message any) Refusal { return Refusal{http.StatusGone, message} }

// Unprocessable refuses a well-formed request the rules reject: 422.
func Unprocessable(message any) Refusal { return Refusal{http.StatusUnprocessableEntity, message} }

// TooManyRequests refuses a caller who is over a limit: 429.
func TooManyRequests(message any) Refusal { return Refusal{http.StatusTooManyRequests, message} }

// NotImplemented refuses a feature this deployment is not configured for: 501.
func NotImplemented(message any) Refusal { return Refusal{http.StatusNotImplemented, message} }

// BadGateway reports that a dependency did not give us an answer: 502.
func BadGateway(message any) Refusal { return Refusal{http.StatusBadGateway, message} }

// Unavailable reports that this service cannot do the work right now: 503.
func Unavailable(message any) Refusal { return Refusal{http.StatusServiceUnavailable, message} }

// IsErrorStatus reports whether a status code is a failure, for the middleware
// that decides how loudly to log a response. It is here for the same reason the
// constructors are: nothing outside this package should have to name 400 to ask
// the question.
func IsErrorStatus(status int) bool { return status >= http.StatusBadRequest }

// Refusals is a domain's whole error-to-refusal table, declared once beside its
// handler instead of being spread across that domain's error switches.
//
// Lookup is by errors.Is, so a wrapped sentinel matches its entry. Order is
// therefore significant only in that a domain must not register two sentinels
// where one wraps the other; no domain does, and the shared table below is
// consulted only after this one, so a domain can override a shared answer.
type Refusals map[error]Refusal

// Refuser is a Responder that also knows one domain's table. It embeds the
// Responder, so a handler holding a *Refuser keeps every response helper it had
// and gains DomainError over its own errors.
type Refuser struct {
	*Responder
	table Refusals
}

// WithRefusals returns a Refuser answering the given domain table before
// falling back to the shared sentinels.
//
// The table is captured at construction and never written to afterwards, which
// is what keeps this free of the locking a package-level registry would need —
// and of the ordering trap where a handler built before its own init() ran
// answers 500 to an error it has a status for.
func (rs *Responder) WithRefusals(table Refusals) *Refuser {
	return &Refuser{Responder: rs, table: table}
}

// Refuse writes a refusal. A refusal with no message writes the status alone.
func (rs *Responder) Refuse(w http.ResponseWriter, r *http.Request, ref Refusal) {
	if ref.Message == nil {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(ref.Status)
		return
	}
	rs.Error(w, r, ref.Status, ref.Message)
}

// DomainError answers err with the status its kind of failure earns.
//
// It knows the sentinels every domain shares (internal/data): a row that is not
// there, a cursor that will not parse, a row that moved under a read, a resend
// inside its cooldown. Anything else is a fault rather than a refusal, and gets
// the generic 500 with err logged — which is the point of routing through here
// rather than through a default branch that each handler writes for itself.
func (rs *Responder) DomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		rs.NotFound(w, r)
	case errors.Is(err, data.ErrInvalidCursor):
		rs.BadRequest(w, r, errors.New("invalid cursor value"))
	case errors.Is(err, data.ErrEditConflict):
		rs.EditConflict(w, r)
	case errors.Is(err, data.ErrCooldownActive):
		rs.RateLimitExceeded(w, r)
	default:
		rs.ServerError(w, r, err)
	}
}

// DomainError answers err from this domain's table first, then from the shared
// sentinels.
func (f *Refuser) DomainError(w http.ResponseWriter, r *http.Request, err error) {
	if ref, ok := f.Lookup(err); ok {
		f.Refuse(w, r, ref)
		return
	}
	f.Responder.DomainError(w, r, err)
}

// DomainErrorWith answers err with message instead of whatever the table holds,
// keeping the status the table chose.
//
// It is for the sentinel that means two different things to a person depending
// on the route it was raised from — the same "you still have active bookings"
// refusing a delete on one endpoint and a MercadoPago disconnect on another.
// The status is still the table's; only the sentence is the handler's.
func (f *Refuser) DomainErrorWith(w http.ResponseWriter, r *http.Request, err error, message any) {
	if ref, ok := f.Lookup(err); ok {
		ref.Message = message
		f.Refuse(w, r, ref)
		return
	}
	f.Responder.DomainError(w, r, err)
}

// Lookup reports the refusal this domain declared for err, if it declared one.
// Handlers that still need a switch — because some of their branches are not
// keyed on a sentinel at all — use it to keep the sentinel branches in the
// table.
func (f *Refuser) Lookup(err error) (Refusal, bool) {
	for sentinel, ref := range f.table {
		if errors.Is(err, sentinel) {
			return ref, true
		}
	}
	return Refusal{}, false
}

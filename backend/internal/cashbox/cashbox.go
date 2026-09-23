// Package cashbox owns a complex's counter cash: opening and closing a cash
// shift ("session"), recording income and expenses against it
// ("movements"), and reconciling the till against what the system expects.
//
// A session is the unit everything else hangs off: at most one is open per
// complex at a time (idx_cash_sessions_one_open, db/migrations/003_cashbox.sql),
// every movement belongs to one, and a movement write always requires the
// session it targets to still be open. Movements are append-only — a
// correction is a new movement (a "void") pointing back at the one it
// corrects, never an edit.
//
// Expected cash also subtracts cash handed back by hand through
// internal/payments.RecordManualRefund, confirmed within the session's own
// window: payments.manual_refund_amount and manual_refunded_at
// (db/migrations/006_payment_manual_refunds.sql) are what let this package
// tell a manual refund apart from an unrelated payment update — see
// buildSummary (service.go).
package cashbox

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// SessionStore is the cash-session half of the domain's persistence.
type SessionStore interface {
	OpenSession(ctx context.Context, s *cashboxstore.CashSession) error
	GetOpenByComplex(ctx context.Context, complexID uuid.UUID) (*cashboxstore.CashSession, error)
	GetByID(ctx context.Context, complexID, sessionID uuid.UUID) (*cashboxstore.CashSession, error)
	ListByComplex(ctx context.Context, complexID uuid.UUID, filters data.Filters) ([]*cashboxstore.CashSession, data.Metadata, error)
	Close(ctx context.Context, complexID, sessionID, closedBy uuid.UUID, countedCash, cashBookingPaymentsInWindow, cashManualRefundsInWindow int64, closedAt time.Time, note *string) (*cashboxstore.CashSession, error)
}

// MovementStore is the cash-movement half.
type MovementStore interface {
	InsertMovement(ctx context.Context, m *cashboxstore.CashMovement) error
	GetMovementByID(ctx context.Context, complexID, movementID uuid.UUID) (*cashboxstore.CashMovement, error)
	ListMovementsBySession(ctx context.Context, complexID, sessionID uuid.UUID) ([]*cashboxstore.CashMovement, error)
	SumBySession(ctx context.Context, complexID, sessionID uuid.UUID) ([]cashboxstore.MovementTotal, error)
}

// Store is both together: one concrete store implements them, and the
// composition is what NewService takes, so a caller still passes one value.
type Store interface {
	SessionStore
	MovementStore
}

// PaymentWindowReader is the payments side of a session's reconciliation:
// what came in through the booking flow, and what went back out by hand,
// per method, during the session's own window (opened_at to
// closed_at-or-now). Satisfied by reportstore.Store, which already owns the
// "what counts as collected money" rule the monthly report uses — see
// PaymentSummaryByMethodWindow and ManualRefundSummaryByMethodWindow.
type PaymentWindowReader interface {
	PaymentSummaryByMethodWindow(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error)
	ManualRefundSummaryByMethodWindow(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.ManualRefundMethodSummary, error)
}

// Recorder writes the audit trail for a session's lifecycle and a session's
// movements.
type Recorder interface {
	Record(e audit.Entry)
}

// Actor is who a change is attributed to, as the handler read it off the
// request. The service needs it for the audit trail and for nothing else.
type Actor struct {
	// UserID is the authenticated owner. Every cashbox route requires one —
	// there is no system actor here — but the field stays a pointer for the
	// same reason courts.Actor's does: audit.Entry.UserID is itself optional.
	UserID *uuid.UUID
	IP     string
}

// Handler serves the cash-session and cash-movement routes. It decodes,
// validates, and maps the service's domain errors onto HTTP; every rule
// lives in the Service.
type Handler struct {
	svc          *Service
	respond      *httpx.Refuser
	trustProxies bool
}

// refusals is this module's whole error-to-status table. Everything absent
// from it — data.ErrRecordNotFound in particular — is answered by
// internal/httpx's shared sentinels.
var refusals = httpx.Refusals{
	cashboxstore.ErrSessionAlreadyOpen: httpx.Conflict("this complex already has an open cash session"),
	cashboxstore.ErrSessionNotOpen:     httpx.Conflict("no cash session is open, or it was just closed"),
	cashboxstore.ErrAlreadyVoided:      httpx.Conflict("this movement has already been voided"),
	cashboxstore.ErrVoidOfVoid:         httpx.Conflict("cannot void a movement that is itself a void"),
	cashboxstore.ErrCannotVoidSaleManually: httpx.Conflict(
		"a sale's income can only be voided by voiding the sale"),
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder, trustProxies bool) *Handler {
	return &Handler{
		svc:          svc,
		respond:      respond.WithRefusals(refusals),
		trustProxies: trustProxies,
	}
}

// actor reads who is making the change, and from where, off the request —
// the only thing the audit trail needs that lives on the HTTP side.
func (h *Handler) actor(r *http.Request) Actor {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}
	return Actor{UserID: userID, IP: httpx.ClientIP(r, h.trustProxies)}
}

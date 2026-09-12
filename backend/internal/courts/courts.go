// Package courts owns a complex's playing surfaces: the courts themselves,
// their per-day price bands, the slots an owner blocks off, and the
// availability grid a client books from.
//
// Availability is the read that ties them together — it is the intersection of
// what the complex is open for, what each court is priced for, and what is not
// already taken or blocked.
package courts

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// The court domain reads three different things through its store, and a
// caller almost never wants all three: the surfaces themselves, the price
// bands on them, and the hours their owner has taken off sale. They are three
// ports for that reason — a reader of this package can see which of the three
// a rule touches from its call site, and a double in a test only has to answer
// for the one it is standing in for.

// CourtStore is the surfaces themselves.
type CourtStore interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error)
	GetByID(ctx context.Context, id uuid.UUID) (*courtstore.Court, error)
	Insert(ctx context.Context, c *courtstore.Court) error
	Update(ctx context.Context, c *courtstore.Court, expectedVersion *int) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// PriceStore is the per-day price bands a court is sold at.
type PriceStore interface {
	GetPrices(ctx context.Context, courtID uuid.UUID) ([]*courtstore.CourtPrice, error)
	GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*courtstore.CourtPrice, error)
	// ReplacePrices atomically replaces a court's whole price table in one
	// transaction (H-07) — see its comment in internal/courts/store/courts.go. It
	// replaces the old DeletePricesByCourtID-then-InsertPrice-loop shape
	// UpdatePrices used to call directly.
	ReplacePrices(ctx context.Context, courtID uuid.UUID, prices []*courtstore.CourtPrice,
		expectedVersion *int) (failedIndex int, err error)
}

// BlockedSlotStore is the hours an owner has withdrawn from sale.
type BlockedSlotStore interface {
	InsertBlockedSlot(ctx context.Context, s *courtstore.BlockedSlot) error
	GetBlockedSlotByID(ctx context.Context, id uuid.UUID) (*courtstore.BlockedSlot, error)
	// GetBlockedSlots serves Service.GetBlockedSlots, which the booking domain
	// reads this module through.
	GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error)
	GetBlockedSlotsByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time) ([]*courtstore.BlockedSlot, error)
	GetBlockedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error)
	DeleteBlockedSlot(ctx context.Context, id uuid.UUID) error
}

// Store is all three together: one concrete store implements them, and the
// composition is what NewService takes, so a caller still passes one value.
type Store interface {
	CourtStore
	PriceStore
	BlockedSlotStore
}

// BookingReader is the booking side of availability and of the check that
// stops a court with live bookings from being deleted.
type BookingReader interface {
	GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]bookingstore.BookedSpan, error)
	HasActiveBookingsByCourt(ctx context.Context, courtID uuid.UUID) (bool, error)
}

// ComplexReader supplies the complex a public availability request names, and
// its opening hours.
type ComplexReader interface {
	GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error)
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
}

// Recorder writes the audit trail for the owner's changes.
type Recorder interface {
	Record(e audit.Entry)
}

// Handler serves the court routes. It decodes, validates, and maps the
// service's domain errors onto HTTP; every rule lives in the Service.
type Handler struct {
	svc          *Service
	respond      *httpx.Refuser
	trustProxies bool
}

// refusals is this module's whole error-to-status table: every domain error of
// its own that is a refusal rather than a fault, and the status and message it
// earns. Everything absent from it — data.ErrRecordNotFound, the edit conflict
// ErrEditConflict wraps — is answered by internal/httpx.
var refusals = httpx.Refusals{
	courtstore.ErrCourtHasActiveBookings: httpx.Conflict(
		"cannot delete court while it has active bookings, cancel them first"),
	courtstore.ErrSlotAlreadyBlocked: httpx.Conflict("this time range already has a blocked slot"),
	// One sentence for one collision. A client cannot be told two different
	// things about it depending on which of the service's two checks happened
	// to see it — the only difference between them is that one ran inside the
	// transaction, which is not something the owner can act on.
	courtstore.ErrSlotHasBooking: httpx.Conflict(blockedSlotHasBookingMessage),
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder, trustProxies bool) *Handler {
	return &Handler{
		svc:          svc,
		respond:      respond.WithRefusals(refusals),
		trustProxies: trustProxies,
	}
}

// Routes registers this module's endpoints.
//
// Availability is the one public route: a client picking a slot has no account.
// Everything else changes the complex's configuration and needs its owner.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	owner := func(next http.HandlerFunc) http.HandlerFunc {
		return guards.RequireAuth(guards.RequireComplexOwner(next))
	}

	router.HandlerFunc(http.MethodGet, "/api/v1/public/complexes/{slug}/availability", h.Availability)

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/{id}/courts", owner(h.List))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/{id}/courts", owner(h.Create))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/{id}/courts/{courtID}", owner(h.Update))
	router.HandlerFunc(http.MethodDelete, "/api/v1/complexes/{id}/courts/{courtID}", owner(h.Delete))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/{id}/courts/{courtID}/prices", owner(h.UpdatePrices))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/{id}/courts/{courtID}/block", owner(h.BlockSlot))

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/{id}/blocked-slots", owner(h.ListBlockedSlots))
	router.HandlerFunc(http.MethodDelete, "/api/v1/complexes/{id}/blocked-slots/{slotID}", owner(h.DeleteBlockedSlot))
}

// actor reads who is making the change, and from where, off the request. It is
// the only thing the audit trail needs that lives on the HTTP side.
func (h *Handler) actor(r *http.Request) Actor {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}
	return Actor{UserID: userID, IP: httpx.ClientIP(r, h.trustProxies)}
}

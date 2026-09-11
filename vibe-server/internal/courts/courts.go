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
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// Store is the court, price and blocked-slot persistence this module uses.
type Store interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*data.Court, error)
	GetByID(ctx context.Context, id uuid.UUID) (*data.Court, error)
	Insert(ctx context.Context, c *data.Court) error
	Update(ctx context.Context, c *data.Court) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	GetPrices(ctx context.Context, courtID uuid.UUID) ([]*data.CourtPrice, error)
	GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*data.CourtPrice, error)
	// ReplacePrices atomically replaces a court's whole price table in one
	// transaction (H-07) — see its comment in internal/data/courts.go. It
	// replaces the old DeletePricesByCourtID-then-InsertPrice-loop shape
	// UpdatePrices used to call directly.
	ReplacePrices(ctx context.Context, courtID uuid.UUID, prices []*data.CourtPrice) (failedIndex int, err error)

	InsertBlockedSlot(ctx context.Context, s *data.BlockedSlot) error
	GetBlockedSlotByID(ctx context.Context, id uuid.UUID) (*data.BlockedSlot, error)
	GetBlockedSlotsByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time) ([]*data.BlockedSlot, error)
	GetBlockedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]*data.BlockedSlot, error)
	DeleteBlockedSlot(ctx context.Context, id uuid.UUID) error
}

// BookingReader is the booking side of availability and of the check that
// stops a court with live bookings from being deleted.
type BookingReader interface {
	GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]data.BookedSpan, error)
	HasActiveBookingsByCourt(ctx context.Context, courtID uuid.UUID) (bool, error)
}

// ComplexReader supplies the complex a public availability request names, and
// its opening hours.
type ComplexReader interface {
	GetBySlug(ctx context.Context, slug string) (*data.Complex, error)
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*data.Schedule, error)
}

// Recorder writes the audit trail for the owner's changes.
type Recorder interface {
	Record(e audit.Entry)
}

// Handler serves the court routes.
type Handler struct {
	store        Store
	bookings     BookingReader
	complexes    ComplexReader
	audit        Recorder
	respond      *httpx.Responder
	trustProxies bool
}

// NewHandler returns a Handler backed by the given stores.
func NewHandler(store Store, bookings BookingReader, complexes ComplexReader, recorder Recorder, respond *httpx.Responder, trustProxies bool) *Handler {
	return &Handler{
		store:        store,
		bookings:     bookings,
		complexes:    complexes,
		audit:        recorder,
		respond:      respond,
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

	router.HandlerFunc(http.MethodGet, "/api/v1/public/complexes/:slug/availability", h.Availability)

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/courts", owner(h.List))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/courts", owner(h.Create))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/:id/courts/:courtID", owner(h.Update))
	router.HandlerFunc(http.MethodDelete, "/api/v1/complexes/:id/courts/:courtID", owner(h.Delete))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/:id/courts/:courtID/prices", owner(h.UpdatePrices))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/courts/:courtID/block", owner(h.BlockSlot))

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/blocked-slots", owner(h.ListBlockedSlots))
	router.HandlerFunc(http.MethodDelete, "/api/v1/complexes/:id/blocked-slots/:slotID", owner(h.DeleteBlockedSlot))
}

// record writes an audit entry for a change to this complex's configuration.
func (h *Handler) record(r *http.Request, complexID uuid.UUID, action, entityType string, entityID *uuid.UUID, oldVal, newVal any) {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}

	h.audit.Record(audit.Entry{
		UserID:     userID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   oldVal,
		NewValue:   newVal,
		IPAddress:  httpx.ClientIP(r, h.trustProxies),
	})
}

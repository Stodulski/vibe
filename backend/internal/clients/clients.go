// Package clients serves the complex owner's view of the people who book at
// their courts: listing them, reading one with their recent bookings, and
// editing the owner's own notes and block flag.
//
// A client record belongs to exactly one complex. Every handler here checks
// that ownership before returning anything, and reports a mismatch as 404
// rather than 403 so the endpoint cannot be used to probe which client ids
// exist under another complex.
package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// defaultPageLimit is the page size when the caller does not ask for one.
const defaultPageLimit = 50

// recentBookingLimit is how many of a client's bookings the detail view returns.
const recentBookingLimit = 20

// Store is the client persistence this module uses. It is declared here rather
// than reused from data.ClientStore so that this package depends on the three
// methods it calls, and no more.
type Store interface {
	GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error)
	GetByComplex(ctx context.Context, complexID uuid.UUID, search string, filters data.Filters) ([]*clientstore.Client, data.Metadata, error)
	Update(ctx context.Context, c *clientstore.Client) error
}

// BookingReader is the one booking query this module needs, for the recent
// bookings shown on a client's detail view. Declaring it here keeps clients
// from depending on the booking domain.
type BookingReader interface {
	GetByClient(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*bookingstore.Booking, error)
}

// Handler serves the client routes.
type Handler struct {
	store    Store
	bookings BookingReader
	respond  *httpx.Responder
}

// NewHandler returns a Handler backed by the given stores.
func NewHandler(store Store, bookings BookingReader, respond *httpx.Responder) *Handler {
	return &Handler{store: store, bookings: bookings, respond: respond}
}

// Routes registers this module's endpoints. All three are scoped to a complex
// and readable only by its owner.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	protected := func(next http.HandlerFunc) http.HandlerFunc {
		return guards.RequireAuth(guards.RequireComplexOwner(next))
	}

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/clients", protected(h.List))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/clients/:clientID", protected(h.Get))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/:id/clients/:clientID", protected(h.Update))
}

// load resolves the client named in the route and confirms it belongs to the
// complex the caller owns.
//
// The three handlers opened with the same twenty lines of context lookup,
// parameter parsing, fetch and ownership check. It returns ok rather than an
// error because it has already written the response on every failure path.
func (h *Handler) load(w http.ResponseWriter, r *http.Request) (*complexstore.Complex, *clientstore.Client, bool) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return nil, nil, false
	}

	clientID, err := httpx.ReadUUIDParam(r, "clientID")
	if err != nil {
		h.respond.NotFound(w, r)
		return nil, nil, false
	}

	client, err := h.store.GetByID(r.Context(), clientID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.NotFound(w, r)
		} else {
			h.respond.ServerError(w, r, err)
		}
		return nil, nil, false
	}

	// A client under another complex is reported as missing, not forbidden:
	// a 403 would confirm the id exists.
	if client.ComplexID != complex.ID {
		h.respond.NotFound(w, r)
		return nil, nil, false
	}

	return complex, client, true
}

// Get handles GET /api/v1/complexes/:id/clients/:clientID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, client, ok := h.load(w, r)
	if !ok {
		return
	}

	recentBookings, err := h.bookings.GetByClient(r.Context(), complex.ID, client.ID, recentBookingLimit)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"client":          client,
		"recent_bookings": recentBookings,
	})
}

// Update handles PUT /api/v1/complexes/:id/clients/:clientID.
//
// Only the owner's own annotations are editable. The client's identity fields
// come from their bookings and are not writable here.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	_, client, ok := h.load(w, r)
	if !ok {
		return
	}

	var input struct {
		Notes     *string `json:"notes"`
		IsBlocked *bool   `json:"is_blocked"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	// Both fields are pointers so that omitting one leaves it untouched,
	// rather than clearing it.
	if input.Notes != nil {
		client.Notes = input.Notes
	}
	if input.IsBlocked != nil {
		client.IsBlocked = *input.IsBlocked
	}

	if err := h.store.Update(r.Context(), client); err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.EditConflict(w, r)
		} else {
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"client": client})
}

// List handles GET /api/v1/complexes/:id/clients.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	qs := r.URL.Query()
	search := httpx.ReadString(qs, "search", "")
	filters := data.Filters{
		Cursor: httpx.ReadString(qs, "cursor", ""),
		Limit:  httpx.ReadInt(qs, "limit", defaultPageLimit),
	}

	v := validator.New()
	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	clients, metadata, err := h.store.GetByComplex(r.Context(), complex.ID, search, filters)
	if err != nil {
		if errors.Is(err, data.ErrInvalidCursor) {
			h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		} else {
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"clients":  clients,
		"metadata": metadata,
	})
}

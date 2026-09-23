// Package sales owns selling products from a complex's catalog: a sale
// snapshots each line's product name and price, moves stock for every
// tracked item, and records exactly one cash income for its total — all in
// one transaction (internal/sales/store.Store.Create) — and a void reverses
// all three together (Store.Void).
//
// A sale is never deleted, and the only mutation an existing row ever gets is
// the one-time void. Selling past zero stock is allowed (the counter never
// refuses a sale for lack of stock): the response instead carries
// stock_warnings for every tracked product that ended at zero or below, so
// the owner notices and counts the drawer.
package sales

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// Store is the sales domain's whole persistence port. One concrete store
// (internal/sales/store) implements it.
type Store interface {
	Create(ctx context.Context, complexID, actorID uuid.UUID, items []salestore.ItemInput, method string, note *string) (*salestore.Sale, []*salestore.SaleItem, []salestore.StockWarning, error)
	GetByID(ctx context.Context, complexID, saleID uuid.UUID) (*salestore.Sale, error)
	ListItemsBySale(ctx context.Context, complexID, saleID uuid.UUID) ([]*salestore.SaleItem, error)
	ListItemsBySaleIDs(ctx context.Context, complexID uuid.UUID, saleIDs []uuid.UUID) ([]*salestore.SaleItem, error)
	ListByComplex(ctx context.Context, complexID uuid.UUID, sessionID *uuid.UUID, filters data.Filters) ([]*salestore.Sale, data.Metadata, error)
	Void(ctx context.Context, complexID, saleID, actorID uuid.UUID, note *string) (*salestore.Sale, error)
}

// SaleWithItems is a sale together with its line items — what every read and
// write endpoint (create, get, list, void) answers with.
type SaleWithItems struct {
	*salestore.Sale
	Items []*salestore.SaleItem
}

// Recorder writes the audit trail for a sale's lifecycle.
type Recorder interface {
	Record(e audit.Entry)
}

// Actor is who a change is attributed to, as the handler read it off the
// request. The service needs it for the audit trail and for nothing else.
type Actor struct {
	// UserID is the authenticated owner. Every sales route requires one —
	// there is no system actor here — but the field stays a pointer for the
	// same reason products.Actor's does: audit.Entry.UserID is itself
	// optional.
	UserID *uuid.UUID
	IP     string
}

// Handler serves the sale routes. It decodes, validates, and maps the
// service's domain errors onto HTTP; every rule lives in the Service.
type Handler struct {
	svc          *Service
	respond      *httpx.Refuser
	trustProxies bool
}

// refusals is this module's whole error-to-status table. *salestore.ErrInvalidItems
// gets its own field-scoped 422 in Create rather than a table entry, the same
// way products.ErrDuplicateProductName does, because the table can only carry
// a flat message and this refusal names one or more fields.
var refusals = httpx.Refusals{
	salestore.ErrNoOpenCashSession:   httpx.Conflict("no cash session is open"),
	salestore.ErrZeroTotal:           httpx.Unprocessable("sale total must be greater than zero"),
	salestore.ErrTotalExceedsCap:     httpx.Unprocessable("sale total must not exceed 2000000000"),
	salestore.ErrAlreadyVoided:       httpx.Conflict("this sale has already been voided"),
	salestore.ErrIncomeAlreadyVoided: httpx.Conflict("this sale's income movement was already voided and cannot be voided again"),
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

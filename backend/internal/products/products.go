// Package products owns a complex's counter catalog and its stock ledger:
// creating and editing products, restocking them (which also records the
// cash expense it cost), and adjusting stock by hand for a reason (breakage,
// an expired item, a count correction) with no money involved.
//
// A product is deactivated, never deleted: stock_movements rows (and, from
// T4b, sale_items) reference it, so removing the row would either cascade
// away real history or leave it dangling. active is the only "this product
// is gone" a client ever sees.
//
// Selling a product (T4b's sales/sale_items, and the 'sale'/'sale_void'
// stock movements they drive) is a later delivery of pos-cashbox — this
// package only ever writes 'restock' and 'adjustment' rows.
package products

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
)

// Catalog is the product-catalog half of the domain's persistence.
type Catalog interface {
	Insert(ctx context.Context, p *productstore.Product) error
	GetByID(ctx context.Context, complexID, productID uuid.UUID) (*productstore.Product, error)
	ListByComplex(ctx context.Context, complexID uuid.UUID, activeFilter *bool) ([]*productstore.Product, error)
	Update(ctx context.Context, p *productstore.Product, expectedVersion *int) error
}

// StockLedger is the stock-movement half.
type StockLedger interface {
	Restock(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity, totalCost int, method string, note *string) (*productstore.Product, *productstore.StockMovement, error)
	Adjust(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity int, reason string, note *string) (*productstore.Product, *productstore.StockMovement, error)
	ListStockMovements(ctx context.Context, complexID, productID uuid.UUID, filters data.Filters) ([]*productstore.StockMovement, data.Metadata, error)
}

// Store is both together: one concrete store implements them, and the
// composition is what NewService takes, so a caller still passes one value.
type Store interface {
	Catalog
	StockLedger
}

// Recorder writes the audit trail for a product's lifecycle and its stock
// movements.
type Recorder interface {
	Record(e audit.Entry)
}

// Actor is who a change is attributed to, as the handler read it off the
// request. The service needs it for the audit trail and for nothing else.
type Actor struct {
	// UserID is the authenticated owner. Every products route requires one —
	// there is no system actor here — but the field stays a pointer for the
	// same reason cashbox.Actor's does: audit.Entry.UserID is itself optional.
	UserID *uuid.UUID
	IP     string
}

// Handler serves the product and stock-movement routes. It decodes,
// validates, and maps the service's domain errors onto HTTP; every rule
// lives in the Service.
type Handler struct {
	svc          *Service
	respond      *httpx.Refuser
	trustProxies bool
}

// refusals is this module's whole error-to-status table. Everything absent
// from it — data.ErrRecordNotFound and productstore.ErrDuplicateProductName
// in particular — is answered elsewhere: ErrDuplicateProductName gets its own
// field-scoped 422 (httpx.CodeProductNameTaken) in Create/Update, the same
// way courts.ErrDuplicateCourtName does, rather than a table entry, because
// the table can only carry a flat message and this refusal names a field.
var refusals = httpx.Refusals{
	productstore.ErrNoOpenCashSession:       httpx.Conflict("no cash session is open"),
	productstore.ErrProductNotTrackingStock: httpx.Conflict("this product does not track stock"),
	productstore.ErrProductInactive:         httpx.Conflict("this product is not active"),
	ErrProductHasStock:                      httpx.Conflict("cannot stop tracking stock while stock_on_hand is not zero"),
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

package products

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	paymentmethod "github.com/stodulski/vibe-server/internal/paymentmethod"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
	"github.com/stodulski/vibe-server/internal/validator"
)

// defaultPageLimit is the page size when the caller does not ask for one.
const defaultPageLimit = 50

// noteMaxLen bounds every free-text note this module accepts — the same cap
// internal/cashbox's own noteMaxLen uses.
const noteMaxLen = 500

// nameMaxLen and categoryMaxLen mirror products_name_length /
// products_category_length (db/migrations/004_pos_catalog_stock.sql).
const nameMaxLen = 120
const categoryMaxLen = 60

// maxMoney caps price and a restock's total_cost, which land in INTEGER
// columns (products.price directly; a restock's cost becomes
// cash_movements.amount) — the same maxMovementAmount cashbox's own handler
// caps cash_movements.amount at.
const maxMoney = 2_000_000_000

const maxMoneyMessage = "must not exceed 2000000000"

// maxStockQuantity bounds a single restock or adjustment: sensible ceilings
// for one counter transaction, well under int32 range.
const maxStockQuantity = 100_000

// Create handles POST /api/v1/complexes/{id}/products.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var body gen.ProductsCreateJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	category := trimmedOrNil(body.Category)

	v := validator.New()
	v.Check(name != "", "name", "must be provided")
	v.Check(len(name) <= nameMaxLen, "name", "must not be more than 120 characters")
	if category != nil {
		v.Check(len(*category) <= categoryMaxLen, "category", "must not be more than 60 characters")
	}
	// Price is a pointer (openapi.yaml's productsCreate requestBody makes it
	// nullable) for the same reason cashbox's opening_cash is: a request that
	// omits it must decode to nil, not a valid-looking 0, because 0 is itself
	// a legitimate price (a free sample) and the OpenAPI request validator
	// that would otherwise catch a missing required field never runs in
	// production (internal/middleware/openapi.go).
	v.Check(body.Price != nil, "price", "must be provided")
	if body.Price != nil {
		v.Check(*body.Price >= 0, "price", "must not be negative")
		v.Check(*body.Price <= maxMoney, "price", maxMoneyMessage)
	}
	if body.LowStockThreshold != nil {
		v.Check(*body.LowStockThreshold >= 0, "low_stock_threshold", "must not be negative")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	tracksStock := true
	if body.TracksStock != nil {
		tracksStock = *body.TracksStock
	}

	product, err := h.svc.Create(r.Context(), complex.ID, h.actor(r), CreateInput{
		Name:              name,
		Category:          category,
		Price:             *body.Price,
		TracksStock:       tracksStock,
		LowStockThreshold: body.LowStockThreshold,
	})
	if err != nil {
		if errors.Is(err, productstore.ErrDuplicateProductName) {
			// idx_products_active_name_unique is partial over active products,
			// so this is a name collision with another LIVE product — reusing
			// a deactivated product's name is an ordinary insert. Same shape
			// as courts.ErrDuplicateCourtName.
			h.respond.FailedValidation(w, r, map[string]string{"name": httpx.CodeProductNameTaken})
			return
		}
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"product": toGenProduct(product)})
}

// List handles GET /api/v1/complexes/{id}/products.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	// Parsed with strconv.ParseBool, the same rule the generated wrapper
	// validated the parameter with, so active=1 or active=f filter instead of
	// silently returning every product.
	var activeFilter *bool
	if raw := r.URL.Query().Get("active"); raw != "" {
		active, err := strconv.ParseBool(raw)
		if err != nil {
			h.respond.BadRequest(w, r, fmt.Errorf("active must be a boolean"))
			return
		}
		activeFilter = &active
	}

	products, err := h.svc.List(r.Context(), complex.ID, activeFilter)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	genProducts := make([]gen.Product, len(products))
	for i, p := range products {
		genProducts[i] = toGenProduct(p)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"products": genProducts})
}

// Get handles GET /api/v1/complexes/{id}/products/{productID}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	productID, err := httpx.ReadUUIDParam(r, "productID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	product, err := h.svc.Get(r.Context(), complex.ID, productID)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"product": toGenProduct(product)})
}

// Update handles PATCH /api/v1/complexes/{id}/products/{productID}. Every
// field is optional; an omitted one keeps its current value.
//
//nolint:funlen // one cohesive request lifecycle for a single resource operation, per this codebase's handler conventions (CLAUDE.md) — see courts.Handler.Update's identical justification.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	productID, err := httpx.ReadUUIDParam(r, "productID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var body gen.ProductsUpdateJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	var name *string
	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		v.Check(trimmed != "", "name", "must not be empty")
		v.Check(len(trimmed) <= nameMaxLen, "name", "must not be more than 120 characters")
		name = &trimmed
	}
	// Trimmed like Create's category, so a whitespace-only value clears the
	// field instead of being stored.
	var category *string
	if body.Category != nil {
		trimmed := strings.TrimSpace(*body.Category)
		v.Check(len(trimmed) <= categoryMaxLen, "category", "must not be more than 60 characters")
		category = &trimmed
	}
	if body.Price != nil {
		v.Check(*body.Price >= 0, "price", "must not be negative")
		v.Check(*body.Price <= maxMoney, "price", maxMoneyMessage)
	}
	if body.LowStockThreshold != nil {
		v.Check(*body.LowStockThreshold >= 0, "low_stock_threshold", "must not be negative")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	expectedVersion, err := httpx.ExpectedVersion(r, body.Version)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	product, err := h.svc.Update(r.Context(), complex.ID, h.actor(r), productID, UpdateInput{
		Name:              name,
		Category:          category,
		Price:             body.Price,
		LowStockThreshold: body.LowStockThreshold,
		Active:            body.Active,
		TracksStock:       body.TracksStock,
		ExpectedVersion:   expectedVersion,
	})
	if err != nil {
		switch {
		case errors.Is(err, productstore.ErrDuplicateProductName):
			h.respond.FailedValidation(w, r, map[string]string{"name": httpx.CodeProductNameTaken})
		case errors.Is(err, ErrEditConflict):
			h.respond.StaleVersion(w, r)
		default:
			h.respond.DomainError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"product": toGenProduct(product)})
}

// Restock handles POST /api/v1/complexes/{id}/products/{productID}/restock.
func (h *Handler) Restock(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	productID, err := httpx.ReadUUIDParam(r, "productID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var body gen.ProductsRestockJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	// Quantity and TotalCost are pointers for the same missing-vs-zero reason
	// Create's Price is.
	v.Check(body.Quantity != nil, "quantity", "must be provided")
	if body.Quantity != nil {
		v.Check(*body.Quantity >= 1, "quantity", "must be at least 1")
		v.Check(*body.Quantity <= maxStockQuantity, "quantity", "must not exceed 100000")
	}
	v.Check(body.TotalCost != nil, "total_cost", "must be provided")
	if body.TotalCost != nil {
		// A free restock (total_cost 0) is an adjustment, not a restock — see
		// the feature document's own decision.
		v.Check(*body.TotalCost >= 1, "total_cost", "must be at least 1")
		v.Check(*body.TotalCost <= maxMoney, "total_cost", maxMoneyMessage)
	}
	v.Check(paymentmethod.IsCounter(string(body.Method)), "method", paymentmethod.Message)
	if body.Note != nil {
		v.Check(len(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	product, movement, err := h.svc.Restock(r.Context(), complex.ID, productID, h.actor(r), user.ID, RestockInput{
		Quantity:  *body.Quantity,
		TotalCost: *body.TotalCost,
		Method:    string(body.Method),
		Note:      body.Note,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"product":        toGenProduct(product),
		"stock_movement": toGenStockMovement(movement),
	})
}

// noteMaxTooLong is the message every note field's length check answers with.
const noteMaxTooLong = "must not be more than 500 characters"

// Adjust handles POST /api/v1/complexes/{id}/products/{productID}/adjustments.
func (h *Handler) Adjust(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	productID, err := httpx.ReadUUIDParam(r, "productID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var body gen.ProductsAdjustJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	// Quantity is a pointer for the same missing-vs-zero reason Create's Price
	// is; 0 itself is refused below as not a real adjustment.
	v.Check(body.Quantity != nil, "quantity", "must be provided")
	if body.Quantity != nil {
		v.Check(*body.Quantity != 0, "quantity", "must not be zero")
		v.Check(*body.Quantity >= -maxStockQuantity && *body.Quantity <= maxStockQuantity, "quantity", "must be within +/-100000")
	}
	v.Check(validator.PermittedValue(string(body.Reason), AdjustmentReasons...), "reason",
		"must be one of: breakage, expired, own_consumption, count_correction, other")
	if body.Note != nil {
		v.Check(len(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	product, movement, err := h.svc.Adjust(r.Context(), complex.ID, productID, h.actor(r), user.ID, AdjustInput{
		Quantity: *body.Quantity,
		Reason:   string(body.Reason),
		Note:     body.Note,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"product":        toGenProduct(product),
		"stock_movement": toGenStockMovement(movement),
	})
}

// ListStockMovements handles
// GET /api/v1/complexes/{id}/products/{productID}/stock-movements.
func (h *Handler) ListStockMovements(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	productID, err := httpx.ReadUUIDParam(r, "productID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	qs := r.URL.Query()
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

	movements, metadata, err := h.svc.StockMovements(r.Context(), complex.ID, productID, filters)
	if err != nil {
		if errors.Is(err, data.ErrInvalidCursor) {
			h.respond.BadRequest(w, r, errors.New("invalid cursor value"))
			return
		}
		h.respond.DomainError(w, r, err)
		return
	}

	genMovements := make([]gen.StockMovement, len(movements))
	for i, m := range movements {
		genMovements[i] = toGenStockMovement(m)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"stock_movements": genMovements,
		"metadata":        metadata,
	})
}

// trimmedOrNil trims s and returns nil for an empty (or now-empty) result,
// the same "empty means absent" convention emptyToNil (service.go) applies
// to a PATCH; Create has no existing value to preserve, so there is nothing
// to distinguish "omitted" from "sent empty" here.
func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

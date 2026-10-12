package sales

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	paymentmethod "github.com/stodulski/vibe-server/internal/paymentmethod"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
	"github.com/stodulski/vibe-server/internal/validator"
)

// defaultPageLimit is the page size when the caller does not ask for one.
const defaultPageLimit = 50

// noteMaxLen bounds every free-text note this module accepts — the same cap
// internal/cashbox's and internal/products' own noteMaxLen use.
const noteMaxLen = 500

// noteMaxTooLong is the message every note field's length check answers with.
const noteMaxTooLong = "must not be more than 500 characters"

// maxItemsPerSale bounds how many distinct product lines one checkout may name
// in a single request. A checkout needs at least one product line or booking
// payment; Create checks that.
const maxItemsPerSale = 50

// maxQuantityPerLine mirrors sale_items_quantity_check
// (db/migrations/005_pos_sales.sql).
const maxQuantityPerLine = 1000

// maxBookingPaymentsPerCheckout bounds the balances one checkout collects, and
// maxBookingPaymentAmount is the per-balance cap the booking payment path
// applies (bookings.Service.ConfirmPayment).
const (
	maxBookingPaymentsPerCheckout = 20
	maxBookingPaymentAmount       = 99_999_999
)

// itemNotSellableMessage is the field message for every salestore.ItemProblem
// — deliberately the same wording whether the product does not exist or is
// merely inactive: from the client's side, both mean "you cannot sell this
// right now", and a sale of up to 50 lines should not have to leak which
// tenant-visible distinction applied to fix its request.
const itemNotSellableMessage = "product not found or not active"

// Create handles POST /api/v1/complexes/{id}/sales.
//
//nolint:funlen // one cohesive request lifecycle for a single resource operation, per this codebase's handler conventions (CLAUDE.md) — see products.Handler.Restock's identical justification.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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

	var body gen.SalesCreateJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(len(body.Items) <= maxItemsPerSale, "items", "must not contain more than 50 items")

	seen := make(map[uuid.UUID]bool, len(body.Items))
	items := make([]salestore.ItemInput, 0, len(body.Items))
	for i, it := range body.Items {
		if seen[it.ProductId] {
			v.Check(false, fmt.Sprintf("items[%d].product_id", i), "duplicate product in the same sale")
			continue
		}
		seen[it.ProductId] = true

		// Quantity is a pointer for the same missing-vs-zero reason products'
		// Create's Price is: a request that omits it must decode to nil, not a
		// valid-looking 0, because the OpenAPI request validator that would
		// otherwise catch a missing required field never runs in production
		// (internal/middleware/openapi.go).
		field := fmt.Sprintf("items[%d].quantity", i)
		v.Check(it.Quantity != nil, field, "must be provided")
		quantity := 0
		if it.Quantity != nil {
			quantity = *it.Quantity
			v.Check(quantity >= 1, field, "must be at least 1")
			v.Check(quantity <= maxQuantityPerLine, field, "must not exceed 1000")
		}
		items = append(items, salestore.ItemInput{ProductID: it.ProductId, Quantity: quantity})
	}

	balances := make([]salestore.BookingPaymentInput, 0)
	if body.BookingPayments != nil {
		entries := *body.BookingPayments
		v.Check(len(entries) <= maxBookingPaymentsPerCheckout, "booking_payments", "must not contain more than 20 booking payments")
		seenBooking := make(map[uuid.UUID]bool, len(entries))
		for i, bp := range entries {
			field := fmt.Sprintf("booking_payments[%d]", i)
			if seenBooking[bp.BookingId] {
				v.Check(false, field+".booking_id", "duplicate booking in the same checkout")
				continue
			}
			seenBooking[bp.BookingId] = true

			// Nullable for the same reason as a line's quantity: a missing amount
			// is a 422, never a zero that could pass for a valid balance.
			v.Check(bp.Amount != nil, field+".amount", "must be provided")
			amount := 0
			if bp.Amount != nil {
				amount = *bp.Amount
				v.Check(amount >= 1, field+".amount", "must be at least 1")
				v.Check(amount <= maxBookingPaymentAmount, field+".amount", "amount too large")
			}
			balances = append(balances, salestore.BookingPaymentInput{BookingID: bp.BookingId, Amount: amount})
		}
	}

	v.Check(len(items)+len(balances) > 0, "items", "must contain at least 1 item or 1 booking payment")
	v.Check(paymentmethod.IsCounter(string(body.Method)), "method", paymentmethod.Message)
	if body.Note != nil {
		v.Check(utf8.RuneCountInString(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	res, err := h.svc.Checkout(r.Context(), complex.ID, h.actor(r), user.ID, CheckoutInput{
		Items:           items,
		BookingPayments: balances,
		Method:          string(body.Method),
		Note:            body.Note,
	})
	if err != nil {
		var invalid *salestore.ErrInvalidItems
		if errors.As(err, &invalid) {
			fieldErrors := make(map[string]string, len(invalid.Problems))
			for i, it := range body.Items {
				if _, bad := invalid.Problems[it.ProductId]; bad {
					fieldErrors[fmt.Sprintf("items[%d].product_id", i)] = itemNotSellableMessage
				}
			}
			h.respond.FailedValidation(w, r, fieldErrors)
			return
		}
		var unknown *salestore.ErrUnknownBooking
		if errors.As(err, &unknown) {
			fieldErrors := map[string]string{}
			if body.BookingPayments != nil {
				for i, bp := range *body.BookingPayments {
					if bp.BookingId == unknown.BookingID {
						fieldErrors[fmt.Sprintf("booking_payments[%d].booking_id", i)] = "booking not found in this complex"
					}
				}
			}
			h.respond.FailedValidation(w, r, fieldErrors)
			return
		}
		h.respond.DomainError(w, r, err)
		return
	}

	// A booking-only checkout has no sale: the field is null, not absent.
	var genSale any
	if res.Sale != nil {
		genSale = toGenSale(&SaleWithItems{Sale: res.Sale, Items: res.Items, BookingPayments: res.BookingLines})
	}
	genWarnings := make([]gen.SaleStockWarning, len(res.Warnings))
	for i, warning := range res.Warnings {
		genWarnings[i] = toGenStockWarning(warning)
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"sale":             genSale,
		"stock_warnings":   genWarnings,
		"booking_payments": toGenSaleBookingPayments(res.BookingLines),
	})
}

// BookingsDue handles GET /api/v1/complexes/{id}/cash/bookings-due: today's
// bookings of the complex that still owe money, for the sell page.
func (h *Handler) BookingsDue(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	day, rows, err := h.svc.BookingsDue(r.Context(), complex.ID)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	bookings := make([]gen.BalanceDue, len(rows))
	for i, row := range rows {
		bookings[i] = toGenBalanceDue(row)
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"date":     day.Format("2006-01-02"),
		"bookings": bookings,
	})
}

// Get handles GET /api/v1/complexes/{id}/sales/{saleID}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	saleID, err := httpx.ReadUUIDParam(r, "saleID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	sale, err := h.svc.Get(r.Context(), complex.ID, saleID)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"sale": toGenSale(sale)})
}

// List handles GET /api/v1/complexes/{id}/sales — the paginated history,
// newest first, optionally restricted to one session.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	qs := r.URL.Query()
	filters := data.Filters{
		Cursor: httpx.ReadString(qs, "cursor", ""),
		Limit:  httpx.ReadInt(qs, "limit", defaultPageLimit),
	}

	var sessionID *uuid.UUID
	if raw := qs.Get("session_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			h.respond.BadRequest(w, r, fmt.Errorf("session_id must be a valid uuid"))
			return
		}
		sessionID = &parsed
	}

	v := validator.New()
	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	sales, metadata, err := h.svc.List(r.Context(), complex.ID, sessionID, filters)
	if err != nil {
		if errors.Is(err, data.ErrInvalidCursor) {
			h.respond.BadRequest(w, r, errors.New("invalid cursor value"))
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	genSales := make([]gen.Sale, len(sales))
	for i, s := range sales {
		genSales[i] = toGenSale(s)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"sales":    genSales,
		"metadata": metadata,
	})
}

// Void handles POST /api/v1/complexes/{id}/sales/{saleID}/void.
func (h *Handler) Void(w http.ResponseWriter, r *http.Request) {
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

	saleID, err := httpx.ReadUUIDParam(r, "saleID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	// The request body is optional (salesVoid carries no `required: true`),
	// the same reasoning cashbox's own VoidMovement gives — see that
	// handler's bodyIsEmpty for why the check cannot stop at
	// r.ContentLength == 0 alone.
	var body gen.SalesVoidJSONBody
	empty, err := bodyIsEmpty(r)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	if !empty {
		if err := httpx.ReadJSON(w, r, &body); err != nil {
			h.respond.BadRequest(w, r, err)
			return
		}
	}

	v := validator.New()
	if body.Note != nil {
		v.Check(utf8.RuneCountInString(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	voided, err := h.svc.Void(r.Context(), complex.ID, saleID, h.actor(r), user.ID, body.Note)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"sale": toGenSale(voided)})
}

// bodyIsEmpty reports whether r carries no body worth decoding — the same
// shape internal/cashbox's own copy is (unexported to its own package, hence
// duplicated rather than shared) and internal/reporting's export_handlers.go
// copy before it.
func bodyIsEmpty(r *http.Request) (bool, error) {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return true, nil
	}
	if r.ContentLength > 0 {
		return false, nil
	}

	var first [1]byte
	n, err := r.Body.Read(first[:])
	if n == 0 {
		if err != nil && !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("sales: read request body: %w", err)
		}
		return true, nil
	}

	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(first[:n]), r.Body), r.Body}
	return false, nil
}

package cashbox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	paymentmethod "github.com/stodulski/vibe-server/internal/paymentmethod"
	"github.com/stodulski/vibe-server/internal/validator"
)

// defaultPageLimit is the page size when the caller does not ask for one.
const defaultPageLimit = 50

// noteMaxLen bounds every free-text note this module accepts — the same cap
// courts' BlockSlot reason uses.
const noteMaxLen = 500

// maxSessionCash caps opening_cash and counted_cash, which land in BIGINT
// columns (003_cashbox.sql). A till at the end of a busy day in pesos is
// routinely past a million, so the ceiling only has to stop an absurd
// keystroke, not a real count: about 1,000 million ARS.
const maxSessionCash = 99_999_999_999

const maxSessionCashMessage = "must not exceed 99999999999"

// maxMovementAmount caps cash_movements.amount, which stays INTEGER like
// payments.amount: 20 million ARS covers a payroll or a large supplier
// payment and stays under the int32 range the column allows.
const maxMovementAmount = 2_000_000_000

const maxMovementAmountMessage = "must not exceed 2000000000"

// Open handles POST /api/v1/complexes/{id}/cash-sessions.
func (h *Handler) Open(w http.ResponseWriter, r *http.Request) {
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

	var body gen.CashSessionsOpenJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	// OpeningCash is a pointer (internal/openapi/openapi.yaml's cashSessionsOpen
	// requestBody makes it nullable) specifically so a request that omits it
	// decodes to nil rather than a valid-looking 0: the OpenAPI request
	// validator that would otherwise catch a missing required field never runs
	// in production (internal/middleware/openapi.go), and 0 is itself a
	// legitimate opening float, so the handler has to check presence itself.
	v.Check(body.OpeningCash != nil, "opening_cash", "must be provided")
	if body.OpeningCash != nil {
		v.Check(*body.OpeningCash >= 0, "opening_cash", "must not be negative")
		v.Check(*body.OpeningCash <= maxSessionCash, "opening_cash", maxSessionCashMessage)
	}
	if body.Note != nil {
		v.Check(utf8.RuneCountInString(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	session, err := h.svc.Open(r.Context(), complex.ID, h.actor(r), user.ID, OpenInput{
		OpeningCash: int64(*body.OpeningCash),
		OpeningNote: body.Note,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"cash_session": toGenCashSession(session)})
}

// noteMaxTooLong is the message every note field's length check answers with.
const noteMaxTooLong = "must not be more than 500 characters"

// Current handles GET /api/v1/complexes/{id}/cash-session.
//
// No session open answers 404, the same as any other single-resource read
// this API has none of: the feature document leaves the exact shape to
// "whatever matches existing API conventions", and every other singular GET
// in this document (a booking, a court, a complex) answers 404 rather than a
// bespoke empty envelope when the thing it names does not exist.
func (h *Handler) Current(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	current, err := h.svc.Current(r.Context(), complex.ID)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"cash_session": toGenCashSession(current.CashSession),
		"summary":      toGenCashSessionSummary(current.Summary),
	})
}

// List handles GET /api/v1/complexes/{id}/cash-sessions — the paginated
// history, newest (most recently opened) first.
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

	v := validator.New()
	data.ValidateFilters(v, filters)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	sessions, metadata, err := h.svc.List(r.Context(), complex.ID, filters)
	if err != nil {
		if errors.Is(err, data.ErrInvalidCursor) {
			h.respond.BadRequest(w, r, errors.New("invalid cursor value"))
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	genSessions := make([]gen.CashSession, len(sessions))
	for i, s := range sessions {
		genSessions[i] = toGenCashSession(s)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"cash_sessions": genSessions,
		"metadata":      metadata,
	})
}

// Get handles GET /api/v1/complexes/{id}/cash-sessions/{sessionID}: the
// session, its summary and its full movement ledger.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	sessionID, err := httpx.ReadUUIDParam(r, "sessionID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	detail, err := h.svc.Get(r.Context(), complex.ID, sessionID)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	movements := make([]gen.CashMovement, len(detail.Movements))
	for i, m := range detail.Movements {
		movements[i] = toGenCashMovement(m)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"cash_session": toGenCashSession(detail.CashSession),
		"summary":      toGenCashSessionSummary(detail.Summary),
		"movements":    movements,
	})
}

// Close handles POST /api/v1/complexes/{id}/cash-sessions/{sessionID}/close.
func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
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

	sessionID, err := httpx.ReadUUIDParam(r, "sessionID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var body gen.CashSessionsCloseJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	// CountedCash is a pointer for the same reason CashSessionsOpen's
	// OpeningCash is (see Open above): 0 is a legitimate count, so a missing
	// field has to decode to nil rather than a valid-looking 0.
	v.Check(body.CountedCash != nil, "counted_cash", "must be provided")
	if body.CountedCash != nil {
		v.Check(*body.CountedCash >= 0, "counted_cash", "must not be negative")
		v.Check(*body.CountedCash <= maxSessionCash, "counted_cash", maxSessionCashMessage)
	}
	if body.Note != nil {
		v.Check(utf8.RuneCountInString(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	closed, err := h.svc.Close(r.Context(), complex.ID, sessionID, h.actor(r), user.ID, CloseInput{
		CountedCash: int64(*body.CountedCash),
		ClosingNote: body.Note,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"cash_session": toGenCashSession(closed)})
}

// CreateMovement handles
// POST /api/v1/complexes/{id}/cash-sessions/{sessionID}/movements.
func (h *Handler) CreateMovement(w http.ResponseWriter, r *http.Request) {
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

	sessionID, err := httpx.ReadUUIDParam(r, "sessionID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var body gen.CashMovementsCreateJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	category := string(body.Category)

	v := validator.New()
	v.Check(validator.PermittedValue(string(body.Kind), "income", "expense"), "kind", "must be 'income' or 'expense'")
	switch body.Kind {
	case gen.CashMovementsCreateJSONBodyKindIncome:
		v.Check(validator.PermittedValue(category, IncomeCategories...), "category",
			"must be one of: other_income, classes, tournaments, events, memberships, sponsorship, cash_contribution")
	case gen.CashMovementsCreateJSONBodyKindExpense:
		v.Check(validator.PermittedValue(category, ExpenseCategories...), "category",
			"must be one of: supplies, salaries, services, maintenance, cleaning, withdrawal, other_expense, "+
				"rent, taxes, professional_fees, marketing, bank_fees")
	}
	v.Check(paymentmethod.IsCounter(string(body.Method)), "method", paymentmethod.Message)
	v.Check(body.Amount > 0, "amount", "must be greater than 0")
	v.Check(body.Amount <= maxMovementAmount, "amount", maxMovementAmountMessage)
	if body.Note != nil {
		v.Check(utf8.RuneCountInString(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	movement, err := h.svc.CreateMovement(r.Context(), complex.ID, sessionID, h.actor(r), user.ID, MovementInput{
		Kind:     string(body.Kind),
		Category: category,
		Method:   string(body.Method),
		Amount:   body.Amount,
		Note:     body.Note,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"cash_movement": toGenCashMovement(movement)})
}

// VoidMovement handles
// POST /api/v1/complexes/{id}/cash-sessions/{sessionID}/movements/{movementID}/void.
//
// sessionID addresses the movement being corrected; the void it creates
// always lands in whichever session is open right now, which may not be
// sessionID — see Service.VoidMovement's own comment.
func (h *Handler) VoidMovement(w http.ResponseWriter, r *http.Request) {
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

	sessionID, err := httpx.ReadUUIDParam(r, "sessionID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}
	movementID, err := httpx.ReadUUIDParam(r, "movementID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	// The request body is optional in the document (cashMovementsVoid carries
	// no `required: true`, and its schema requires no property): an owner
	// voiding a movement usually sends no note at all. ReadJSON refuses an
	// empty body outright — rightly, for every route whose body IS required —
	// so emptiness has to be checked first rather than treating that refusal
	// as this route's own 400.
	var body gen.CashMovementsVoidJSONBody
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

	void, err := h.svc.VoidMovement(r.Context(), complex.ID, sessionID, movementID, h.actor(r), user.ID, body.Note)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"cash_movement": toGenCashMovement(void)})
}

// bodyIsEmpty reports whether r carries no body worth decoding: an absent
// body (r.ContentLength == 0), the http.NoBody sentinel, or a chunked body
// (r.ContentLength == -1, whose length is unknown until read) that turns out
// to hold zero bytes. A positive ContentLength is trusted without reading
// ahead. A chunked body found to be non-empty has its first byte read back
// onto r.Body, unconsumed, so a later httpx.ReadJSON(r) still decodes the
// whole thing.
//
// Same shape as internal/reporting/export_handlers.go's own bodyIsEmpty,
// which VoidMovement cannot reuse directly (that one is unexported to its own
// package) — see that copy's comment for why the check cannot stop at
// r.ContentLength == 0 alone.
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
			return false, fmt.Errorf("cashbox: read request body: %w", err)
		}
		return true, nil
	}

	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(first[:n]), r.Body), r.Body}
	return false, nil
}

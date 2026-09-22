package cashbox

import (
	"errors"
	"fmt"
	"net/http"

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
	v.Check(body.OpeningCash >= 0, "opening_cash", "must not be negative")
	if body.Note != nil {
		v.Check(len(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	session, err := h.svc.Open(r.Context(), complex.ID, h.actor(r), user.ID, OpenInput{
		OpeningCash: body.OpeningCash,
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

// Current handles GET /api/v1/complexes/{id}/cash-sessions/current.
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
	v.Check(body.CountedCash >= 0, "counted_cash", "must not be negative")
	if body.Note != nil {
		v.Check(len(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	closed, err := h.svc.Close(r.Context(), complex.ID, sessionID, h.actor(r), user.ID, CloseInput{
		CountedCash: body.CountedCash,
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
			"must be 'other_income' for an income movement")
	case gen.CashMovementsCreateJSONBodyKindExpense:
		v.Check(validator.PermittedValue(category, ExpenseCategories...), "category",
			"must be one of: supplies, salaries, services, maintenance, cleaning, withdrawal, other_expense")
	}
	v.Check(paymentmethod.IsCounter(string(body.Method)), "method", paymentmethod.Message)
	v.Check(body.Amount > 0, "amount", "must be greater than 0")
	if body.Note != nil {
		v.Check(len(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
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

	var body gen.CashMovementsVoidJSONBody
	if err := httpx.ReadJSON(w, r, &body); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	if body.Note != nil {
		v.Check(len(*body.Note) <= noteMaxLen, "note", noteMaxTooLong)
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

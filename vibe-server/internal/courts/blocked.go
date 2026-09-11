package courts

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// ListBlockedSlots handles GET /api/v1/complexes/:id/blocked-slots over a date
// range.
//
// H-09: with 500 blocked slots seeded, this used to answer 366 rows in one
// response — every other list endpoint on this API takes limit/cursor, and
// this one had been left out. Pagination is applied here, over the full
// range GetBlockedSlotsByComplex still fetches in one query, rather than by
// pushing limit/cursor into that store method's own SQL: that method's
// signature is also what internal/data.CourtBlockedSlotManager declares, and
// that interface is what cmd/api's own store wiring is built against
// (courts.NewHandler(d.models.Courts, ...) in cmd/api/app.go) — widening it
// would need a matching stub added to cmd/api's test mocks, which is outside
// this fix's tree.
//
// R3-blocked-slots-silent-truncation: a first version of this fix defaulted
// the limit to 50 whenever the caller sent none, which silently truncated
// every existing consumer that reads only "blocked_slots" and has no reason
// to read the new "metadata" key — a caller asking for a month's calendar got
// back 50 rows and no error, and the rows that fell off the page are exactly
// the ones a calendar renders as bookable. Pagination is now opt-in: a caller
// that sends neither "limit" nor "cursor" gets every slot in the range, the
// same contract this endpoint had before H-09, bounded only by the 366-day
// range cap below (which bounds the query's own cost, not the response
// size). A caller that sends either one gets the limit/cursor/metadata
// envelope every other list endpoint already has.
func (h *Handler) ListBlockedSlots(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	qs := r.URL.Query()

	dateFrom, dateTo, err := parseBlockedSlotsDateRange(qs)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	// Pagination is opt-in (R3-blocked-slots-silent-truncation): a caller that
	// names neither query parameter gets the old, unbounded contract back
	// rather than a limit it never asked for.
	limitParam := httpx.ReadString(qs, "limit", "")
	cursorParam := httpx.ReadString(qs, "cursor", "")
	paginate := limitParam != "" || cursorParam != ""

	filters := data.Filters{
		Cursor: cursorParam,
		Limit:  httpx.ReadInt(qs, "limit", 50),
	}
	if paginate {
		v := validator.New()
		data.ValidateFilters(v, filters)
		if !v.Valid() {
			h.respond.FailedValidation(w, r, v.Errors)
			return
		}
	}

	slots, err := h.store.GetBlockedSlotsByComplex(r.Context(), complex.ID, dateFrom, dateTo)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if slots == nil {
		slots = []*data.BlockedSlot{}
	}

	if !paginate {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
			"blocked_slots": slots,
			"metadata":      data.Metadata{},
		})
		return
	}

	page, metadata, err := trimBlockedSlotsPage(slots, filters)
	if err != nil {
		h.respond.BadRequest(w, r, fmt.Errorf("invalid cursor value"))
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"blocked_slots": page,
		"metadata":      metadata,
	})
}

// parseBlockedSlotsDateRange reads and validates the required date_from/
// date_to query parameters, capping the range at 366 days to bound
// GetBlockedSlotsByComplex's own query cost. Split out of ListBlockedSlots
// for the same reason trimBlockedSlotsPage below is: this handler grew a
// second, independent concern (pagination) alongside its original one, and
// keeping both inline pushed the function past funlen's statement budget.
func parseBlockedSlotsDateRange(qs url.Values) (dateFrom, dateTo time.Time, err error) {
	dateFromStr := httpx.ReadString(qs, "date_from", "")
	dateToStr := httpx.ReadString(qs, "date_to", "")
	if dateFromStr == "" || dateToStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("date_from and date_to query parameters are required")
	}

	dateFrom, err = time.Parse("2006-01-02", dateFromStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("date_from must be in YYYY-MM-DD format")
	}

	dateTo, err = time.Parse("2006-01-02", dateToStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("date_to must be in YYYY-MM-DD format")
	}

	if dateTo.Before(dateFrom) {
		return time.Time{}, time.Time{}, fmt.Errorf("date_to must be on or after date_from")
	}

	// Cap range to 366 days to prevent abuse.
	if dateTo.Sub(dateFrom).Hours() > 366*24 {
		return time.Time{}, time.Time{}, fmt.Errorf("date range must not exceed 366 days")
	}

	return dateFrom, dateTo, nil
}

// trimBlockedSlotsPage applies the cursor skip and the limit trim to an
// already-fetched, already-ordered slice of slots. Split out of
// ListBlockedSlots so the opt-in-pagination branch above it stays short
// enough to read as one decision.
func trimBlockedSlotsPage(slots []*data.BlockedSlot, filters data.Filters) ([]*data.BlockedSlot, data.Metadata, error) {
	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, data.Metadata{}, err
	}
	if !cursorTime.IsZero() {
		// slots is already ordered (date, start_time, id) — the same order
		// CursorKey reports — so the first element strictly after the cursor
		// is where the next page starts.
		start := len(slots)
		for i, s := range slots {
			key, id := s.CursorKey()
			if key.After(cursorTime) || (key.Equal(cursorTime) && id.String() > cursorID.String()) {
				start = i
				break
			}
		}
		slots = slots[start:]
	}

	page, metadata := data.TrimPage(slots, filters.Limit, data.BuildTimestampCursor)
	if page == nil {
		page = []*data.BlockedSlot{}
	}
	return page, metadata, nil
}

// DeleteBlockedSlot handles DELETE /api/v1/complexes/:id/blocked-slots/:slotID,
// putting the slot back on sale.
func (h *Handler) DeleteBlockedSlot(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	slotID, err := httpx.ReadUUIDParam(r, "slotID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	slot, err := h.store.GetBlockedSlotByID(r.Context(), slotID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// Verify the blocked slot belongs to a court of this complex.
	court, err := h.store.GetByID(r.Context(), slot.CourtID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	if court.ComplexID != complex.ID {
		h.respond.NotFound(w, r)
		return
	}

	// H-10: DeleteBlockedSlot now reports ErrRecordNotFound when it removed no
	// row — a concurrent delete of the same slot, in particular, since the
	// existence check above already ran. Answering 404 rather than the same
	// 200 the winner gets is what lets a caller tell whether their own request
	// was the one that actually deleted something.
	err = h.store.DeleteBlockedSlot(r.Context(), slotID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.record(r, complex.ID, "delete", "blocked_slot", &slotID, slot, nil)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "blocked slot deleted"})
}

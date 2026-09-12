package courts

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// List handles GET /api/v1/complexes/{id}/courts, returning every court the
// complex has, active or not — the owner manages both.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	result, err := h.svc.List(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"courts": result})
}

// A description is free text a player reads on a storefront card, so it is
// capped where the column is capped (courts_description_length). Two lines is the whole
// point: past that it stops describing the court and starts being a page.
const descriptionMaxLen = 200

const descriptionMessage = "must not be more than 200 characters"

// maxPriceValue is a price band's real ceiling, not an invented one:
// court_prices.price is INTEGER (db/migrations/001_init.sql), a signed
// 32-bit column, and math.MaxInt32 is exactly what it can hold. See H-17's
// comment where this is checked, in UpdatePrices below.
const maxPriceValue = math.MaxInt32

const maxPriceMessage = "must not exceed the maximum price this column can store (2147483647)"

// blockedSlotHasBookingMessage is what a blocked-slot write answers with when
// the hours are already sold. Two checks can produce it — the service's
// pre-check, which names the collision while the request is still in hand, and
// InsertBlockedSlot's own check under the court-day lock — and they answer with
// one sentence because they are answering one question.
const blockedSlotHasBookingMessage = "this time range overlaps with an existing booking"

// emptyToNil folds a description of "" into no description at all.
//
// A client that clears the field sends an empty string, and a court that was
// never described sends nothing; both mean the same thing to a reader, so they
// are stored the same way instead of leaving the storefront to decide whether
// "" counts.
func emptyToNil(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// Create handles POST /api/v1/complexes/{id}/courts.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input struct {
		Name        string  `json:"name"`
		Sport       string  `json:"sport"`
		CourtType   string  `json:"court_type"`
		Description *string `json:"description"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Name != "", "name", "must be provided")
	v.Check(len(input.Name) <= 100, "name", "must not be more than 100 characters")
	v.Check(validator.PermittedValue(input.Sport, "padel", "tennis", "soccer", "basketball"), "sport", "must be one of: padel, tennis, soccer, basketball")
	v.Check(validator.PermittedValue(input.CourtType, "indoor", "outdoor", "semi_covered"), "court_type", "must be one of: indoor, outdoor, semi_covered")
	v.Check(input.Description == nil || len(*input.Description) <= descriptionMaxLen, "description", descriptionMessage)
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	court, err := h.svc.Create(r.Context(), complex.ID, h.actor(r), CreateInput{
		Name:        input.Name,
		Sport:       input.Sport,
		CourtType:   input.CourtType,
		Description: input.Description,
	})
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"court": court})
}

// Update handles PUT /api/v1/complexes/{id}/courts/{courtID}. Every field is
// optional; an omitted one keeps its current value.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	courtID, err := httpx.ReadUUIDParam(r, "courtID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		Name        *string `json:"name"`
		Sport       *string `json:"sport"`
		CourtType   *string `json:"court_type"`
		IsActive    *bool   `json:"is_active"`
		Description *string `json:"description"`
		// Version is the optimistic-concurrency precondition in the body, for a
		// client that finds that easier than If-Match. Either spelling works
		// and neither is required — see httpx.ExpectedVersion (API-08).
		Version *int `json:"version"`
	}

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	if input.Name != nil {
		v.Check(*input.Name != "", "name", "must not be empty")
		v.Check(len(*input.Name) <= 100, "name", "must not be more than 100 characters")
	}
	if input.Sport != nil {
		v.Check(validator.PermittedValue(*input.Sport, "padel", "tennis", "soccer", "basketball"), "sport", "must be one of: padel, tennis, soccer, basketball")
	}
	if input.CourtType != nil {
		v.Check(validator.PermittedValue(*input.CourtType, "indoor", "outdoor", "semi_covered"), "court_type", "must be one of: indoor, outdoor, semi_covered")
	}
	if input.Description != nil {
		v.Check(len(*input.Description) <= descriptionMaxLen, "description", descriptionMessage)
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	expectedVersion, err := httpx.ExpectedVersion(r, input.Version)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	court, err := h.svc.Update(r.Context(), complex.ID, h.actor(r), courtID, UpdateInput{
		Name:            input.Name,
		Sport:           input.Sport,
		CourtType:       input.CourtType,
		IsActive:        input.IsActive,
		Description:     input.Description,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"court": court})
}

// Delete handles DELETE /api/v1/complexes/{id}/courts/{courtID}. The court is
// soft-deleted, and refused outright while it still has live bookings.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	courtID, err := httpx.ReadUUIDParam(r, "courtID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	err = h.svc.Delete(r.Context(), complex.ID, h.actor(r), courtID)
	if err != nil {
		// SoftDelete's WHERE clause can also match nothing because the row
		// disappeared between the service's own lookup and the delete, or
		// because a concurrent delete already soft-deleted it — the ordinary
		// lookup race, not the has-bookings conflict, so it answers 404 through
		// the shared sentinel. See SoftDelete's own comment
		// (internal/courts/store/courts.go) for why the three zero-row causes
		// are no longer conflated into one sentinel.
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "court deleted"})
}

// UpdatePrices handles PUT /api/v1/complexes/{id}/courts/{courtID}/prices.
//
// The band is replaced wholesale rather than merged, so the request body is the
// court's complete price list — a partial update would leave the old bands in
// place alongside the new ones.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) UpdatePrices(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	courtID, err := httpx.ReadUUIDParam(r, "courtID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		Prices []struct {
			Price    int    `json:"price"`
			DayType  string `json:"day_type"`
			TimeFrom string `json:"time_from"`
			TimeTo   string `json:"time_to"`
		} `json:"prices"`
		// Version is the COURT's version, not a band's: the bands are replaced
		// wholesale, so the court is the only thing a client can have read and
		// still hold. If-Match carries the same value (API-08).
		Version *int `json:"version"`
	}

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(len(input.Prices) > 0, "prices", "must contain at least one price")

	// Bands are grouped by day_type and checked for overlap in-memory before
	// anything is written, using the same wrap rule the DB's generated
	// span_min column applies (court_prices in db/migrations/001_init.sql): time_to <= time_from means
	// the band runs past midnight rather than that it ends before it starts.
	// A complex may close at 01:30, and Thursday's price band for that window
	// is legitimately "08:00" to "01:30" — time_from < time_to would reject
	// exactly the schedules the product is meant to support.
	bandsByDay := make(map[string][]priceBand)

	prices := make([]PriceInput, len(input.Prices))
	for i, p := range input.Prices {
		prices[i] = PriceInput{Price: p.Price, DayType: p.DayType, TimeFrom: p.TimeFrom, TimeTo: p.TimeTo}

		v.Check(p.Price > 0, keyIdx("prices", i, "price"), "must be greater than 0")
		// H-17: court_prices.price is INTEGER (db/migrations/001_init.sql), and
		// the sign check above is the only bound this validator applied before
		// this — an out-of-range value was caught by the database instead, and
		// on this path that meant it was caught *after* DeletePricesByCourtID
		// had already committed (H-07). Bounding it here, in the same block
		// that already checks it is positive, is what makes a too-large price
		// answer an ordinary 422 naming the field rather than reaching the
		// database at all. The ReplacePrices transaction behind the service is
		// still needed regardless — it closes the same window for everything
		// else the database can refuse that this one bound does not cover.
		v.Check(p.Price <= maxPriceValue, keyIdx("prices", i, "price"), maxPriceMessage)
		v.Check(validator.PermittedValue(p.DayType, "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"), keyIdx("prices", i, "day_type"), "must be a valid day")

		fromMin, fromOK := parseClock(p.TimeFrom)
		v.Check(fromOK, keyIdx("prices", i, "time_from"), "must be a valid HH:MM time")
		toMin, toOK := parseClock(p.TimeTo)
		v.Check(toOK, keyIdx("prices", i, "time_to"), "must be a valid HH:MM time")
		if !fromOK || !toOK {
			continue
		}

		v.Check(fromMin != toMin, keyIdx("prices", i, "time_to"), "must not equal time_from")
		if fromMin == toMin {
			continue
		}

		spanTo := toMin
		if toMin <= fromMin {
			spanTo += minutesPerDay
		}
		bandsByDay[p.DayType] = append(bandsByDay[p.DayType], priceBand{index: i, fromMin: fromMin, toMin: spanTo})
	}

	for _, bands := range bandsByDay {
		checkOverlaps(bands, v)
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	expectedVersion, err := httpx.ExpectedVersion(r, input.Version)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	written, failedIndex, err := h.svc.UpdatePrices(r.Context(), complex.ID, h.actor(r), courtID, prices, expectedVersion)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		// The in-memory overlap check above should already have caught this,
		// but the DB's exclusion constraint is the real source of truth
		// (concurrent writers, or a rule this handler's grouping missed) —
		// translate it to the same 422 shape rather than falling through to a
		// 500.
		case errors.Is(err, courtstore.ErrOverlappingPriceRule) && failedIndex >= 0:
			h.respond.FailedValidation(w, r, map[string]string{
				keyIdx("prices", failedIndex, "time_from"): "overlaps another price rule for this day",
			})
		default:
			// Covers courts.ErrEditConflict on a stale If-Match/version, which
			// wraps data.ErrEditConflict (409), falling through to
			// ServerError only for anything DomainError does not know.
			h.respond.DomainError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"prices": written})
}

// BlockSlot handles POST /api/v1/complexes/{id}/courts/{courtID}/block, taking a
// time range off sale for maintenance or a private event.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) BlockSlot(w http.ResponseWriter, r *http.Request) {
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

	courtID, err := httpx.ReadUUIDParam(r, "courtID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}

	var input struct {
		Date      string  `json:"date"`
		StartTime string  `json:"start_time"`
		EndTime   string  `json:"end_time"`
		Reason    *string `json:"reason"`
	}

	err = httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Date != "", "date", "must be provided")
	v.Check(input.StartTime != "", "start_time", "must be provided")
	v.Check(input.EndTime != "", "end_time", "must be provided")
	if input.StartTime != "" {
		v.Check(slots.ValidFormat(input.StartTime), "start_time", "must be in HH:MM format")
	}
	if input.EndTime != "" {
		v.Check(slots.ValidFormat(input.EndTime), "end_time", "must be in HH:MM format")
	}
	v.Check(input.StartTime < input.EndTime, "end_time", "must be after start_time")
	if input.Reason != nil {
		v.Check(len(*input.Reason) <= 500, "reason", "must not be more than 500 characters")
	}

	date, dateErr := timezone.ParseDay(input.Date)
	v.Check(dateErr == nil, "date", "must be a valid date (YYYY-MM-DD)")

	// Date must not be in the past — on the product's calendar, not the
	// server's. See onOrAfterToday.
	if dateErr == nil {
		v.Check(onOrAfterToday(date, time.Now()), "date", "must not be in the past")
		// H-08: the upper end of the date range was never bounded at all — a
		// block dated "9999-12-31" was accepted and stored. It sits in a future
		// nobody queries, so it is merely inert rather than harmful the way an
		// unbounded public booking is (see the horizon's own comment,
		// bookingstore.MaxBookingHorizonDays), but the two write paths share one
		// horizon so an owner and a client hit the same wall for the same
		// reason.
		v.Check(!date.After(maxBookableDate(time.Now())), "date",
			fmt.Sprintf("must not be more than %d days in the future", bookingstore.MaxBookingHorizonDays))
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	slot, err := h.svc.BlockSlot(r.Context(), complex.ID, h.actor(r), courtID, BlockSlotInput{
		Date:      date,
		StartTime: input.StartTime,
		EndTime:   input.EndTime,
		Reason:    input.Reason,
		CreatedBy: user.ID,
	})
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"blocked_slot": slot})
}

// onOrAfterToday reports whether a requested calendar date is today or later.
//
// "Today" is a calendar question and this product's calendar is Argentina's.
// The check here used to be time.Now().Truncate(24*time.Hour), which truncates
// against the UTC epoch rather than any local day: between 21:00 and midnight in
// Buenos Aires the server has already rolled over to tomorrow in UTC, so an
// owner trying to block off the rest of tonight was told the date was in the
// past. Every other date decision in the product — the booking path's own
// past-date guard, the cancellation window, the reminder queries — is taken in
// Argentina time, and this was the one that was not.
//
// date is a UTC-midnight value, which is what time.Parse produces for a
// "YYYY-MM-DD" string, so today is projected into the same frame before the
// comparison.
func onOrAfterToday(date, now time.Time) bool {
	today := now.In(timezone.Argentina)
	midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return !date.Before(midnight)
}

// maxBookableDate is the latest calendar date — on the product's calendar,
// same as onOrAfterToday above — a blocked slot may be dated. See
// bookingstore.MaxBookingHorizonDays (H-08) for why this bound exists and why a
// year is the value.
func maxBookableDate(now time.Time) time.Time {
	today := now.In(timezone.Argentina)
	midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.AddDate(0, 0, bookingstore.MaxBookingHorizonDays)
}

// minutesPerDay is used to project a time-of-day past midnight, mirroring
// the span_min generated column on court_prices (db/migrations/001_init.sql).
const minutesPerDay = 24 * 60

// parseClock parses a strict "HH:MM" 24-hour clock string into minutes past
// that day's own midnight. It rejects anything time.Parse would silently fold
// into a zero value (convert.go's timeStrToPg does exactly that for a
// malformed string), so a typo in the request body surfaces as a 422 instead
// of being stored as "00:00".
func parseClock(s string) (int, bool) {
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return 0, false
	}
	// Sscanf accepts trailing garbage after the two ints unless the consumed
	// length matches the input exactly.
	if len(fmt.Sprintf("%02d:%02d", h, m)) != len(s) {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// priceBand is one price[i] entry reduced to the span it covers, in minutes
// from its day's own midnight, after applying the crossing-midnight wrap rule
// (toMin > minutesPerDay when the band runs past midnight).
type priceBand struct {
	index          int
	fromMin, toMin int
}

// checkOverlaps flags every band in a single day_type group that overlaps an
// earlier one, naming the offending index. Bands are half-open, matching the
// DB's exclusion constraint (court_prices_no_overlapping_rule), so two bands
// that share only an endpoint (08:00-12:00 then 12:00-23:00) do not overlap.
//
// This is the classic sweep-line interval-overlap check: sort by start, and
// while walking left to right compare against the furthest end seen so far
// rather than only the immediately preceding band — a band nested inside an
// earlier, wider one would otherwise go undetected.
func checkOverlaps(bands []priceBand, v *validator.Validator) {
	sort.Slice(bands, func(a, b int) bool { return bands[a].fromMin < bands[b].fromMin })

	maxEnd := bands[0].toMin
	maxEndIndex := bands[0].index
	for i := 1; i < len(bands); i++ {
		b := bands[i]
		if b.fromMin < maxEnd {
			v.Check(false, keyIdx("prices", b.index, "time_from"),
				fmt.Sprintf("overlaps prices[%d] for the same day", maxEndIndex))
		}
		if b.toMin > maxEnd {
			maxEnd = b.toMin
			maxEndIndex = b.index
		}
	}
}

// keyIdx names one field of one element of a validation-error array, e.g.
// "prices[2].time_to".
//
// Today's only call site passes "prices", but prefix is part of the helper's
// general-purpose contract: it stays correct for any other validation array.
//
//nolint:unparam // see the note above
func keyIdx(prefix string, index int, field string) string {
	return prefix + "[" + itoa(index) + "]." + field
}

func itoa(i int) string {
	if i < 10 {
		//nolint:gosec // G115: i is a single decimal digit (0-9) here, always in-range for rune conversion.
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

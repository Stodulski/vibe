package courts

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// List handles GET /api/v1/complexes/:id/courts, returning every court the
// complex has, active or not — the owner manages both.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	courts, err := h.store.GetByComplex(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	type courtWithPrices struct {
		*data.Court
		Prices []*data.CourtPrice `json:"prices"`
	}

	result := make([]courtWithPrices, len(courts))
	for i, c := range courts {
		prices, err := h.store.GetPrices(r.Context(), c.ID)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}
		result[i] = courtWithPrices{Court: c, Prices: prices}
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
// the hours are already sold. Two checks can produce it — the pre-check below,
// which names the collision while the request is still in hand, and
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

// Create handles POST /api/v1/complexes/:id/courts.
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

	court := &data.Court{
		ComplexID:   complex.ID,
		Name:        input.Name,
		Sport:       input.Sport,
		CourtType:   input.CourtType,
		Description: emptyToNil(input.Description),
	}

	err = h.store.Insert(r.Context(), court)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "create", "court", &court.ID, nil, court)

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"court": court})
}

// Update handles PUT /api/v1/complexes/:id/courts/:courtID. Every field is
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

	court, err := h.store.GetByID(r.Context(), courtID)
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

	var input struct {
		Name        *string `json:"name"`
		Sport       *string `json:"sport"`
		CourtType   *string `json:"court_type"`
		IsActive    *bool   `json:"is_active"`
		Description *string `json:"description"`
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
		court.Name = *input.Name
	}
	if input.Sport != nil {
		v.Check(validator.PermittedValue(*input.Sport, "padel", "tennis", "soccer", "basketball"), "sport", "must be one of: padel, tennis, soccer, basketball")
		court.Sport = *input.Sport
	}
	if input.CourtType != nil {
		v.Check(validator.PermittedValue(*input.CourtType, "indoor", "outdoor", "semi_covered"), "court_type", "must be one of: indoor, outdoor, semi_covered")
		court.CourtType = *input.CourtType
	}
	if input.IsActive != nil {
		court.IsActive = *input.IsActive
	}
	if input.Description != nil {
		v.Check(len(*input.Description) <= descriptionMaxLen, "description", descriptionMessage)
		// An empty string is how a client clears a description, so it lands as
		// NULL rather than as a row holding "". Same fact, one representation.
		court.Description = emptyToNil(input.Description)
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	err = h.store.Update(r.Context(), court)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.EditConflict(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.record(r, complex.ID, "update", "court", &court.ID, nil, court)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"court": court})
}

// Delete handles DELETE /api/v1/complexes/:id/courts/:courtID. The court is
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

	court, err := h.store.GetByID(r.Context(), courtID)
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

	// H-02: the existence test and the delete used to be two calls — this
	// handler's own HasActiveBookingsByCourt check, then SoftDelete — with
	// nothing serializing them. A booking committing in the gap between the
	// two survived on a court the owner had just watched disappear from their
	// own dashboard. SoftDelete now asks and acts in one statement, so there
	// is no gap left for that booking to land in; ErrCourtHasActiveBookings is
	// the database's answer to the same question this used to ask separately.
	err = h.store.SoftDelete(r.Context(), courtID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrCourtHasActiveBookings):
			h.respond.Error(w, r, http.StatusConflict, "cannot delete court while it has active bookings, cancel them first")
		// SoftDelete's WHERE clause can also match nothing because the row
		// disappeared between the GetByID above and this call, or because a
		// concurrent delete already soft-deleted it — the ordinary GetByID
		// race, not the has-bookings conflict. See SoftDelete's own comment
		// (internal/data/courts.go) for why the three zero-row causes are no
		// longer conflated into one sentinel.
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.record(r, complex.ID, "delete", "court", &courtID, nil, nil)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"message": "court deleted"})
}

// UpdatePrices handles PUT /api/v1/complexes/:id/courts/:courtID/prices.
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

	court, err := h.store.GetByID(r.Context(), courtID)
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

	var input struct {
		Prices []struct {
			Price    int    `json:"price"`
			DayType  string `json:"day_type"`
			TimeFrom string `json:"time_from"`
			TimeTo   string `json:"time_to"`
		} `json:"prices"`
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

	for i, p := range input.Prices {
		v.Check(p.Price > 0, keyIdx("prices", i, "price"), "must be greater than 0")
		// H-17: court_prices.price is INTEGER (db/migrations/001_init.sql), and
		// the sign check above is the only bound this validator applied before
		// this — an out-of-range value was caught by the database instead, and
		// on this path that meant it was caught *after* DeletePricesByCourtID
		// had already committed (H-07). Bounding it here, in the same block
		// that already checks it is positive, is what makes a too-large price
		// answer an ordinary 422 naming the field rather than reaching the
		// database at all. The ReplacePrices transaction below is still needed
		// regardless — it closes the same window for everything else the
		// database can refuse that this one bound does not cover.
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

	// Delete existing prices and insert new ones, atomically.
	//
	// H-07: these used to be a DeletePricesByCourtID call followed by one
	// InsertPrice per row, with no transaction around either — the delete
	// committed on its own, and any insert failure after it (a price out of
	// the column's range, an overlap the in-memory check above missed, a
	// concurrent writer) answered a 4xx to the owner with the court's whole
	// price table already gone. ReplacePrices wraps both halves in one
	// transaction, so a refused write costs nothing: see its comment in
	// internal/data/courts.go.
	prices := make([]*data.CourtPrice, len(input.Prices))
	for i, p := range input.Prices {
		prices[i] = &data.CourtPrice{
			CourtID:  courtID,
			Price:    p.Price,
			DayType:  p.DayType,
			TimeFrom: p.TimeFrom,
			TimeTo:   p.TimeTo,
		}
	}

	failedIndex, err := h.store.ReplacePrices(r.Context(), courtID, prices)
	if err != nil {
		// The in-memory overlap check above should already have caught this,
		// but the DB's exclusion constraint is the real source of truth
		// (concurrent writers, or a rule this handler's grouping missed) —
		// translate it to the same 422 shape rather than falling through to a
		// 500.
		if errors.Is(err, data.ErrOverlappingPriceRule) && failedIndex >= 0 {
			h.respond.FailedValidation(w, r, map[string]string{
				keyIdx("prices", failedIndex, "time_from"): "overlaps another price rule for this day",
			})
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "update_prices", "court", &courtID, nil, prices)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"prices": prices})
}

// BlockSlot handles POST /api/v1/complexes/:id/courts/:courtID/block, taking a
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

	court, err := h.store.GetByID(r.Context(), courtID)
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
		// data.MaxBookingHorizonDays), but the two write paths share one
		// horizon so an owner and a client hit the same wall for the same
		// reason.
		v.Check(!date.After(maxBookableDate(time.Now())), "date",
			fmt.Sprintf("must not be more than %d days in the future", data.MaxBookingHorizonDays))
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// Check for overlapping bookings on this court/date/time range.
	//
	// This is the pre-check, and it is here for the message rather than for the
	// invariant: it names the collision while the request still knows what the
	// owner asked for. It runs outside any transaction, so a booking committing
	// after it passes is not its business — InsertBlockedSlot asks the same
	// question again under the court-day lock and answers ErrSlotHasBooking,
	// which reaches the same 409 below.
	bookedSlots, err := h.bookings.GetBookedSlotsByCourtIDs(r.Context(), []uuid.UUID{courtID}, date)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	// Compared as instants, not as times of day. A booking may end after
	// midnight, and "01:00" sorts below "23:00" — so the string comparison this
	// replaced reported no overlap for every candidate against such a booking,
	// which would let an owner block hours a client had already paid for.
	blockStart := slots.At(date, input.StartTime)
	blockEnd := slots.At(date, input.EndTime)
	for _, s := range bookedSlots {
		if slots.OverlapAt(blockStart, blockEnd, s.StartsAt, s.EndsAt) {
			h.respond.Error(w, r, http.StatusConflict, blockedSlotHasBookingMessage)
			return
		}
	}

	slot := &data.BlockedSlot{
		CourtID:   courtID,
		Date:      date,
		StartTime: input.StartTime,
		EndTime:   input.EndTime,
		Reason:    input.Reason,
		CreatedBy: &user.ID,
	}

	err = h.store.InsertBlockedSlot(r.Context(), slot)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrSlotAlreadyBlocked):
			h.respond.Error(w, r, http.StatusConflict, "this time range already has a blocked slot")
		case errors.Is(err, data.ErrSlotHasBooking):
			// The same sentence the pre-check answers with. A client cannot be
			// told two different things about one collision depending on which
			// of the two checks happened to see it — the only difference
			// between them is that this one ran inside the transaction, which
			// is not something the owner can act on.
			h.respond.Error(w, r, http.StatusConflict, blockedSlotHasBookingMessage)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// The court name is filled in before the entry is recorded, not after. It used
	// to be set on the next line, which left every audit row for a block naming a
	// court only by an id the row does not carry either — the reader of the trail
	// could not tell which court had been taken off sale without a second query.
	slot.CourtName = court.Name

	h.record(r, complex.ID, "create", "blocked_slot", &slot.ID, nil, slot)

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
// data.MaxBookingHorizonDays (H-08) for why this bound exists and why a
// year is the value.
func maxBookableDate(now time.Time) time.Time {
	today := now.In(timezone.Argentina)
	midnight := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.AddDate(0, 0, data.MaxBookingHorizonDays)
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

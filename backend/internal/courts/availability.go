package courts

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// defaultAvailabilityDuration is the booking length the availability grid
// generates slots for when the caller does not name one.
//
// It matches slots.DefaultDuration, kept as its own constant here because this
// is a query-parameter default rather than the grid arithmetic itself.
const defaultAvailabilityDuration = slots.DefaultDuration

// durationParamMessage is what a caller reading a bad `duration` query param
// is told, derived from slots.PermittedDurations the same way the courts and
// bookings handlers derive their own duration refusals.
var durationParamMessage = func() string {
	d := slots.PermittedDurations()
	parts := make([]string, len(d))
	for i, m := range d {
		parts[i] = strconv.Itoa(m)
	}
	return "duration must be " + strings.Join(parts[:len(parts)-1], ", ") + ", or " + parts[len(parts)-1]
}()

type availabilitySlot struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	// Where this slot sits in its trading window, counted in minutes from the
	// window's own midnight and never wrapped: the 00:30 slot of a Thursday
	// 20:00-02:00 window is 1470, not 30.
	//
	// StartTime cannot answer that question. `slots.FromMinutes` takes the
	// value modulo a day to render a clock face, so a venue trading past
	// midnight publishes closing hours that read as the earliest of the day
	// and sort to the top of any list ordered by the string. This is the same
	// trap the booking lookup fell into — see the note on the overnight
	// comparison below, where '01:00' read as earlier than '23:00' and the
	// hours a client had paid for went back on sale. A consumer that needs
	// chronological order sorts on this and never re-derives it from a clock.
	StartMin        int  `json:"start_min"`
	DurationMinutes int  `json:"duration_minutes"`
	Price           int  `json:"price"`
	Available       bool `json:"available"`
}

type courtAvailability struct {
	CourtID   string `json:"court_id"`
	CourtName string `json:"court_name"`
	Sport     string `json:"sport"`
	CourtType string `json:"court_type"`
	// What the court is like, when the owner has said. Omitted rather than
	// sent empty so the storefront renders nothing at all for a court that
	// has never been described, instead of an empty line where text goes.
	Description *string            `json:"description,omitempty"`
	Slots       []availabilitySlot `json:"slots"`
}

type availabilityResponse struct {
	Date   string              `json:"date"`
	Day    string              `json:"day"`
	IsOpen bool                `json:"is_open"`
	Courts []courtAvailability `json:"courts"`
}

// Availability handles GET /api/v1/public/complexes/:slug/availability,
// returning the bookable grid for one day: the complex's open hours, minus
// what is unpriced, booked, blocked, or already in the past.
//
// It is one cohesive request lifecycle — parse the query params, load the
// schedule, courts and prices, compute the slots per court, respond — and
// splitting it would relocate sequential steps into helpers without reducing
// what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Availability(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	qs := r.URL.Query()
	dateStr := httpx.ReadString(qs, "date", "")
	if dateStr == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("date query parameter is required"))
		return
	}

	date, err := timezone.ParseDay(dateStr)
	if err != nil {
		h.respond.BadRequest(w, r, fmt.Errorf("date must be in YYYY-MM-DD format"))
		return
	}

	// A day that has already ended has nothing left to book. This mirrors the
	// past-date refusal POST /api/v1/book gives, anchored to the same
	// product wall-clock, so the storefront never advertises a grid for a
	// date the booking write would immediately reject.
	if date.Before(timezone.Today()) {
		v := validator.New()
		v.AddError("date", "date must not be in the past")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// duration is the booking length the grid is rendered for. It defaults to
	// slots.DefaultDuration, the same default a booking gets when nothing
	// chooses a length for it, and is otherwise restricted to the same set the
	// write paths accept — so the storefront can never publish a length
	// neither booking handler would take back.
	duration := defaultAvailabilityDuration
	if durationStr := httpx.ReadString(qs, "duration", ""); durationStr != "" {
		parsed, convErr := strconv.Atoi(durationStr)
		if convErr != nil || !validator.PermittedValue(parsed, slots.PermittedDurations()...) {
			v := validator.New()
			v.AddError("duration", durationParamMessage)
			h.respond.FailedValidation(w, r, v.Errors)
			return
		}
		duration = parsed
	}

	complex, err := h.complexes.GetBySlug(r.Context(), slug)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// A deactivated venue is not open for business. The booking write already
	// refuses it, so publishing a live grid for it can only end in a client
	// picking a slot and being answered with a bare 404.
	if !complex.IsActive {
		h.respond.NotFound(w, r)
		return
	}

	dayName := slots.DayName(date.Weekday())

	schedules, err := h.complexes.GetSchedules(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	var schedule *complexstore.Schedule
	for _, s := range schedules {
		if s.Day == dayName {
			schedule = s
			break
		}
	}

	if schedule == nil || schedule.IsClosed {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
			"availability": availabilityResponse{
				Date:   dateStr,
				Day:    dayName,
				IsOpen: false,
				Courts: []courtAvailability{},
			},
		})
		return
	}

	courts, err := h.store.GetByComplex(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// Filter only active courts.
	activeCourts := make([]*courtstore.Court, 0, len(courts))
	for _, c := range courts {
		if c.IsActive {
			activeCourts = append(activeCourts, c)
		}
	}

	// No active courts → return open day with empty courts list.
	if len(activeCourts) == 0 {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
			"availability": availabilityResponse{
				Date:   dateStr,
				Day:    dayName,
				IsOpen: true,
				Courts: []courtAvailability{},
			},
		})
		return
	}

	now := time.Now().In(timezone.Argentina)
	isToday := date.Year() == now.Year() && date.YearDay() == now.YearDay()
	currentTime := now.Format("15:04")

	// Batch-fetch all prices, bookings, and blocked slots (3 queries total).
	courtIDs := make([]uuid.UUID, len(activeCourts))
	for i, c := range activeCourts {
		courtIDs[i] = c.ID
	}

	allPrices, err := h.store.GetPricesByCourtIDs(r.Context(), courtIDs)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	allBookedSlots, err := h.bookings.GetBookedSlotsByCourtIDs(r.Context(), courtIDs, date)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	allBlockedSlots, err := h.store.GetBlockedSlotsByCourtIDs(r.Context(), courtIDs, date)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// Index data by court ID for O(1) lookups.
	pricesByCourtID := make(map[uuid.UUID][]*courtstore.CourtPrice, len(courtIDs))
	for _, p := range allPrices {
		pricesByCourtID[p.CourtID] = append(pricesByCourtID[p.CourtID], p)
	}
	bookedByCourtID := make(map[uuid.UUID][]data.BookedSpan, len(courtIDs))
	for _, s := range allBookedSlots {
		bookedByCourtID[s.CourtID] = append(bookedByCourtID[s.CourtID], s)
	}
	blockedByCourtID := make(map[uuid.UUID][]*courtstore.BlockedSlot, len(courtIDs))
	for _, s := range allBlockedSlots {
		blockedByCourtID[s.CourtID] = append(blockedByCourtID[s.CourtID], s)
	}

	courtResults := make([]courtAvailability, 0, len(activeCourts))

	for _, court := range activeCourts {
		bookedSlots := bookedByCourtID[court.ID]
		blockedSlots := blockedByCourtID[court.ID]

		// The grid is derived, not generated here: slots.NewGrid is the same
		// value the booking handlers validate a write against, so this loop
		// cannot offer a position that the write path would refuse. Every
		// start it yields ends before midnight, which is also why the
		// lexicographic comparisons below are safe — no time string wraps.
		grid := slots.NewGrid(schedule.OpenTime, schedule.CloseTime)
		gridSlots := grid.Slots(duration)

		courtSlots := make([]availabilitySlot, 0, len(gridSlots))
		for _, gs := range gridSlots {
			// pricing.BookingPrice is the same function the booking handlers
			// charge from. A booking no rule fully covers is not on sale, so
			// it is omitted rather than published at 0 — the storefront would
			// otherwise advertise a price the write path never charges.
			//
			// `dayName` and `gs.StartMin` name the window this slot came out
			// of and where in it the slot sits. For a venue trading past
			// midnight that is not the calendar day: the 00:30 slot of a
			// Thursday 08:00-01:30 window is Thursday minute 1470, and pays
			// Thursday's rate.
			price, priceErr := pricing.BookingPrice(pricesByCourtID[court.ID], dayName, gs.StartMin, duration)
			if priceErr != nil {
				continue
			}

			// Both kinds of obstacle are compared as instants. The grid emits
			// times of day, but a booking already on the books may run past
			// midnight, and "01:00" reads as earlier than "23:00" under the
			// string comparison this replaced — so such a booking matched
			// nothing and the hours a client had paid for went back on sale.
			//
			// The end is derived by adding the duration rather than reading
			// gs.End, so it stays right if the grid is ever allowed to emit a
			// slot whose end lands on the following day.
			slotStart := slots.At(date, gs.Start)
			slotEnd := slotStart.Add(time.Duration(duration) * time.Minute)

			available := true

			for _, booked := range bookedSlots {
				if slots.OverlapAt(slotStart, slotEnd, booked.StartsAt, booked.EndsAt) {
					available = false
					break
				}
			}

			if available {
				for _, blocked := range blockedSlots {
					// Placed on `date` rather than on blocked.Date: the query
					// that fetched these was scoped to this day, so they are
					// the same calendar date, and `date` is the one already
					// anchored in the product's wall-clock. Anchoring the two
					// sides of a comparison differently is a three-hour error
					// that looks like nothing.
					//
					// A blocked slot cannot itself cross midnight —
					// blocked_slots has carried CHECK (start_time < end_time)
					// since the initial schema — so its two times of day belong to
					// that date and placing them there is exact.
					blockedStart := slots.At(date, blocked.StartTime)
					blockedEnd := slots.At(date, blocked.EndTime)
					if slots.OverlapAt(slotStart, slotEnd, blockedStart, blockedEnd) {
						available = false
						break
					}
				}
			}

			// Check if slot is in the past (if today).
			if available && isToday && gs.Start <= currentTime {
				available = false
			}

			courtSlots = append(courtSlots, availabilitySlot{
				StartTime:       gs.Start,
				EndTime:         gs.End,
				StartMin:        gs.StartMin,
				DurationMinutes: duration,
				Price:           price,
				Available:       available,
			})
		}

		courtResults = append(courtResults, courtAvailability{
			CourtID:     court.ID.String(),
			CourtName:   court.Name,
			Sport:       court.Sport,
			CourtType:   court.CourtType,
			Description: court.Description,
			Slots:       courtSlots,
		})
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"availability": availabilityResponse{
			Date:   dateStr,
			Day:    dayName,
			IsOpen: true,
			Courts: courtResults,
		},
	})
}

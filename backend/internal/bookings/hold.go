// hold.go — the storefront's slot hold. Picking a court on the public page
// holds the slot for the payment window (owner decision 2026-11-10); leaving
// releases it; POST /book with the hold's token converts it into the booking
// the visitor pays for.
//
// The hold IS the pending storefront booking, created when the court is
// picked instead of when the visitor pays, and before anybody is named
// (db/migrations/025_booking_slot_holds.sql). Its window runs from its
// created_at for BOOKING_PAYMENT_EXPIRY, the same window that already decides
// when an unpaid storefront booking stops holding its slot (SlotTaken,
// GetBookedSlots, the expiry sweep), so the countdown, the grid and the sweep
// can never disagree about when a hold ends.
package bookings

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/validator"
)

// maxActiveHoldsPerIP is how many live holds one address may have at once.
// One browser has one (each new hold replaces the last), so this is a few
// visitors behind one NAT — a household, a club's wifi — and not a way for one
// script to hold a venue's evening.
const maxActiveHoldsPerIP = 3

// minCheckoutWindow is the shortest a converted hold's checkout link is made
// payable for. The link expires with the hold, never a fresh window; but a
// visitor who submits with a few seconds left must still get a link MercadoPago
// can open. A payment that completes in that overshoot, after the sweep has
// released the booking, is refused and refunded by the confirmation path's
// guardSlotStillFree, as any late payment is.
const minCheckoutWindow = time.Minute

// ErrHoldExpired reports a booking request whose hold is gone: unknown,
// released, already converted, or past its window. The storefront answers it
// by sending the visitor back to pick a time.
var ErrHoldExpired = errors.New("bookings: the slot hold has expired")

// ErrHoldMismatch reports a booking request for other hours than its hold's.
var ErrHoldMismatch = errors.New("bookings: the request does not match its slot hold")

// ErrTooManyHolds reports an address already holding maxActiveHoldsPerIP slots.
var ErrTooManyHolds = errors.New("bookings: too many slots held from this address")

// holdExpiredMessage and holdMismatchMessage are the 409 details.
const (
	holdExpiredMessage  = "the slot hold has expired, please choose a time again"
	holdMismatchMessage = "the booking does not match the held slot"
	tooManyHoldsMessage = "too many slots held from this address, release one or wait for it to expire"
)

// HoldInput is a validated request to hold a slot from the public page.
type HoldInput struct {
	ComplexID       uuid.UUID
	CourtID         uuid.UUID
	Date            time.Time
	StartTime       string
	DurationMinutes int
	// Replaces is the visitor's previous hold token, released first: one
	// browser holds one slot.
	Replaces string
}

// Hold is a slot held for a visitor: the token that names it, and when it
// ends by itself.
type Hold struct {
	Token     string
	ExpiresAt time.Time
}

// TakeHold holds a slot for a visitor: the same refusals POST /book makes for
// the slot, then the pending booking with nobody named on it.
//
// No slot lock is taken. slot_locks keys on the exact start only, and the
// booking row itself is the hold: InsertSafe's court-day lock and the
// exclusion constraint already make exactly one of two overlapping holds win,
// and a lock left behind would keep the start blocked after a release.
func (s *Service) TakeHold(ctx context.Context, actor Actor, in HoldInput) (Hold, error) {
	if in.Replaces != "" {
		s.ReleaseHold(ctx, in.Replaces)
	}

	req := publicSlotRequest{
		ComplexID: in.ComplexID, CourtID: in.CourtID, Date: in.Date,
		StartTime: in.StartTime, DurationMinutes: in.DurationMinutes,
	}
	complex, court, err := s.publicVenue(ctx, req)
	if err != nil {
		return Hold{}, err
	}
	offer, err := s.publicOffer(ctx, complex, court, req)
	if err != nil {
		return Hold{}, err
	}

	token, hash, err := bookingstore.NewHoldToken()
	if err != nil {
		return Hold{}, err
	}
	booking := &bookingstore.Booking{
		ComplexID:        complex.ID,
		CourtID:          court.ID,
		Date:             offer.startDate,
		StartTime:        in.StartTime,
		DurationMinutes:  in.DurationMinutes,
		Price:            offer.price,
		DepositAmount:    offer.deposit,
		Status:           "pending",
		CollectionStatus: bookingstore.CollectionStatusUnpaid,
		RefundStatus:     bookingstore.RefundStatusNone,
	}
	if err := s.store.InsertHold(ctx, booking, hash, actor.IP, maxActiveHoldsPerIP); err != nil {
		switch {
		case errors.Is(err, bookingstore.ErrHoldLimit):
			return Hold{}, ErrTooManyHolds
		case errors.Is(err, bookingstore.ErrDuplicateBooking), errors.Is(err, bookingstore.ErrSlotUnavailable):
			return Hold{}, ErrSlotTaken
		}
		return Hold{}, err
	}

	s.realtime.PublishBookingChanged(complex.ID)
	return Hold{Token: token, ExpiresAt: booking.CreatedAt.Add(s.cfg.PaymentExpiry)}, nil
}

// ReleaseHold ends the hold a token names, if it is still an untouched hold. It
// never fails the caller: an unknown, converted or already released token is
// a no-op, and a failure to delete leaves the hold to run out on its own.
func (s *Service) ReleaseHold(ctx context.Context, token string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), checkoutCleanupBudget)
	defer cancel()

	complexID, released, err := s.store.ReleaseHold(ctx, bookingstore.HashHoldToken(token))
	if err != nil {
		s.logger.Error("slot hold: release failed; it ends with its window", "error", err)
		return
	}
	if released {
		s.realtime.PublishBookingChanged(complexID)
	}
}

// bookFromHold is POST /book with a hold token: the held booking is converted
// into the storefront booking the visitor pays for. The hours come from the
// hold, so the slot checks it passed are not run again; the request must name
// the same hours, or it is a stale draft and refused.
func (s *Service) bookFromHold(ctx context.Context, actor Actor, in PublicBookInput) (PublicBookResult, error) {
	hash := bookingstore.HashHoldToken(in.HoldToken)
	held, err := s.store.GetHold(ctx, hash)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return PublicBookResult{}, ErrHoldExpired
		}
		return PublicBookResult{}, err
	}
	remaining := time.Until(held.CreatedAt.Add(s.cfg.PaymentExpiry))
	if held.Status != "pending" || remaining <= 0 {
		return PublicBookResult{}, ErrHoldExpired
	}
	if !holdMatches(held, in) {
		return PublicBookResult{}, ErrHoldMismatch
	}
	if !held.StartsAt.After(time.Now()) {
		return PublicBookResult{}, &ConflictError{Message: passedSlotMessage}
	}

	complex, court, err := s.publicVenue(ctx, in.slot())
	if err != nil {
		return PublicBookResult{}, err
	}
	if err := s.refuseBlockedPhone(ctx, complex.ID, in.ClientPhone); err != nil {
		return PublicBookResult{}, err
	}
	seller, err := s.publicSeller(complex)
	if err != nil {
		return PublicBookResult{}, err
	}

	held.ClientSnapshot = &bookingstore.ClientSnapshot{
		FirstName: in.ClientFirstName, LastName: in.ClientLastName,
		Phone: in.ClientPhone, Email: in.ClientEmail,
	}
	if in.ClientNotes != "" {
		held.Notes = &in.ClientNotes
	}
	if err := s.store.ConvertHold(ctx, held, hash); err != nil {
		if errors.Is(err, bookingstore.ErrHoldNotFound) {
			return PublicBookResult{}, ErrHoldExpired
		}
		return PublicBookResult{}, err
	}
	s.recordPublicBook(actor, held)

	// The checkout is payable for what is left of the hold, never a fresh
	// window: the 15 minutes run from the pick and cover both the Datos step
	// and MercadoPago, and the sweep releases the booking when they end.
	expiresIn := max(time.Until(held.CreatedAt.Add(s.cfg.PaymentExpiry)), minCheckoutWindow)
	return s.openCheckout(ctx, checkoutRequest{
		booking: held, complex: complex, court: court, seller: seller,
		rawDate: in.RawDate, expiresIn: expiresIn, payer: in,
	})
}

// holdMatches reports whether a booking request names the hours its hold
// holds. The hold is filed under the calendar day its start falls on, which
// for an after-midnight start is the day after the storefront's grid day.
func holdMatches(held *bookingstore.Booking, in PublicBookInput) bool {
	sameDay := held.Date.Equal(in.Date) || held.Date.Equal(in.Date.AddDate(0, 0, 1))
	return held.ComplexID == in.ComplexID && held.CourtID == in.CourtID && sameDay &&
		held.StartTime == in.StartTime && held.DurationMinutes == in.DurationMinutes
}

// TakeHold handles POST /api/v1/book/holds: the visitor picked a court. It is
// checked by Turnstile under the same rule as POST /book, because this is now
// the step that takes the slot; POST /book with the hold's token is not
// checked again.
func (h *Handler) TakeHold(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ComplexID       string `json:"complex_id"`
		CourtID         string `json:"court_id"`
		Date            string `json:"date"`
		StartTime       string `json:"start_time"`
		DurationMinutes int    `json:"duration_minutes"`
		TurnstileToken  string `json:"turnstile_token"`
		Replaces        string `json:"replaces"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	complexID, complexErr := uuid.Parse(input.ComplexID)
	v.Check(complexErr == nil, "complex_id", "must be a valid UUID")
	courtID, courtErr := uuid.Parse(input.CourtID)
	v.Check(courtErr == nil, "court_id", "must be a valid UUID")
	date, dateErr := time.Parse("2006-01-02", input.Date)
	v.Check(dateErr == nil, "date", "must be in YYYY-MM-DD format")
	v.Check(slots.ValidFormat(input.StartTime), "start_time", "must be in HH:MM format (00:00-23:59)")
	v.Check(validator.PermittedValue(input.DurationMinutes, slots.PermittedDurations()...), "duration_minutes", durationMessage)
	v.Check(len(input.Replaces) <= 256, "replaces", "must not be more than 256 characters")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// The bot check runs before anything is held: a refused token must cost
	// this server nothing.
	if !h.checkTurnstile(w, r, input.TurnstileToken) {
		return
	}

	hold, err := h.svc.TakeHold(r.Context(), h.actor(r), HoldInput{
		ComplexID: complexID, CourtID: courtID, Date: date,
		StartTime: input.StartTime, DurationMinutes: input.DurationMinutes,
		Replaces: input.Replaces,
	})
	if err != nil {
		h.refuse(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"hold": httpx.Envelope{
			"token":      hold.Token,
			"expires_at": hold.ExpiresAt.UTC().Format(time.RFC3339),
		},
	})
}

// ReleaseHold handles POST /api/v1/book/holds/{token}/release. A POST rather
// than a DELETE so navigator.sendBeacon can call it when the page goes away.
// It answers 204 whatever happened, so the route says nothing about which
// tokens exist.
func (h *Handler) ReleaseHold(w http.ResponseWriter, r *http.Request, token string) {
	h.svc.ReleaseHold(r.Context(), token)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

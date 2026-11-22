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
//
// Three rules sit on top of that window (slot-hold-gaps, 2026-11-21), all kept
// in created_at so every reader still agrees:
//   - An address may hold the same hours for one window in all, counting the
//     time each of its holds and unpaid bookings actually held them: letting
//     a hold go and picking the hours again gets only what is left, and once
//     it is spent the address waits bookingstore.HoldCooldown
//     (bookingstore.Store.InsertHold).
//   - Submitting the details leaves at least bookingstore.MinPaymentWindow to
//     pay (bookingstore.Store.ConvertHold).
//   - A visitor whose payment failed can pay again on the same checkout, or let
//     the slot go, through the booking's link token (RetryCheckout,
//     ReleaseCheckout).
package bookings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
	"github.com/stodulski/vibe-server/internal/waitlist"
)

// maxActiveHoldsPerIP is how many live holds one address may have at once.
// One browser has one (each new hold replaces the last), so this is a few
// visitors behind one NAT — a household, a club's wifi — and not a way for one
// script to hold a venue's evening.
const maxActiveHoldsPerIP = 3

// holdsPerIP is the cap InsertHold enforces; 0 means none (Config.UncappedHoldsPerIP).
func (s *Service) holdsPerIP() int {
	if s.cfg.UncappedHoldsPerIP {
		return 0
	}
	return maxActiveHoldsPerIP
}

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
	// Counted per network, the key the rate limiter uses: an IPv6 client owns a
	// whole /64, and keyed on the full address each of its addresses would get
	// a cap of its own.
	if err := s.store.InsertHold(ctx, booking, hash, httpx.RateLimitKey(actor.IP), s.holdsPerIP()); err != nil {
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

	freed, released, err := s.store.ReleaseHold(ctx, bookingstore.HashHoldToken(token))
	if err != nil {
		s.logger.Error("slot hold: release failed; it ends with its window", "error", err)
		return
	}
	if released {
		s.realtime.PublishBookingChanged(freed.ComplexID)
		s.freeHeldSlot(ctx, freed)
	}
}

// freeHeldSlot hands the day a released hold freed to the waitlist. A hold
// takes a slot from the court pick, so a visitor who found the day full may
// have joined the waitlist behind it, exactly as behind any unpaid booking
// (cancelUnpaidHold does the same when one of those is released).
func (s *Service) freeHeldSlot(ctx context.Context, freed bookingstore.ReleasedHold) {
	s.freeWaitlistSlot(ctx, waitlist.Freed{
		ComplexID: freed.ComplexID, ComplexName: freed.ComplexName, ComplexSlug: freed.ComplexSlug, Day: freed.Day,
	})
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

	// The checkout is payable for what is left of the hold, which ConvertHold
	// has just made at least bookingstore.MinPaymentWindow (it moved
	// held.CreatedAt), and the sweep releases the booking when it ends.
	expiresIn := time.Until(held.CreatedAt.Add(s.cfg.PaymentExpiry))
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

// KindHoldCooldown is the Problem kind of a hold refused by the cooldown, so
// the storefront can tell it from somebody else holding the slot.
const KindHoldCooldown = httpx.KindHoldCooldown

// holdCooldownRefusal is the 409 for a hold, or a booking without one, refused
// by the address's spent budget. Its detail is written for the visitor, who
// reads it as it is: when the hours can be picked again, on the venue's clock.
// It says "from this connection" and blames nobody: behind a shared address
// (mobile carriers, a club's wifi) the time may have been spent by someone
// else.
func holdCooldownRefusal(until time.Time) httpx.Refusal {
	refusal := httpx.Conflict(fmt.Sprintf(
		"Este horario se reservó hace poco desde esta conexión sin completar el pago. Vas a poder elegirlo de nuevo desde las %s, o podés elegir otro horario.",
		until.In(timezone.Argentina).Format("15:04")))
	refusal.Kind = KindHoldCooldown
	return refusal
}

// checkoutNotRetryableMessage is the 409 detail when the booking a failed
// payment left behind can no longer be paid: its window ended, it was released,
// or it was paid after all.
const checkoutNotRetryableMessage = "the booking can no longer be paid, please choose a time again"

// checkoutPaidMessage is the 409 detail for a release of a booking that was
// paid after all: there is nothing to let go.
const checkoutPaidMessage = "La reserva ya está pagada, no hace falta liberarla."

// preferenceReader reads back a checkout link. *mp.MPClient implements it; the
// Checkout interface stays the three calls every other path needs, and a
// Checkout without it answers RetryCheckout with ErrCheckoutUnavailable.
type preferenceReader interface {
	GetPreference(ctx context.Context, preferenceID string, caller mp.Caller) (*mp.Preference, error)
}

var _ preferenceReader = (*mp.MPClient)(nil)

// unpaidStorefrontBooking reports whether b is a visitor's booking still
// waiting for its payment: made from the storefront, nothing collected, nobody
// attached by a payment.
func unpaidStorefrontBooking(b *bookingstore.Booking) bool {
	return b.CreatedBy == nil && b.CollectionStatus == bookingstore.CollectionStatusUnpaid && !b.HasClient()
}

// CheckoutRetry is the checkout a visitor pays again on, and when it closes.
type CheckoutRetry struct {
	InitPoint string
	ExpiresAt time.Time
}

// RetryCheckout hands a visitor whose payment failed the same MercadoPago
// checkout again, while the booking's payment window lasts. The booking is
// still pending and still holds its slot; nothing is written.
func (s *Service) RetryCheckout(ctx context.Context, token string) (CheckoutRetry, error) {
	booking, complex, err := s.ResolveLink(ctx, token)
	if err != nil {
		return CheckoutRetry{}, err
	}
	expiresAt := booking.CreatedAt.Add(s.cfg.PaymentExpiry)
	if booking.Status != "pending" || !unpaidStorefrontBooking(booking) || !time.Now().Before(expiresAt) {
		return CheckoutRetry{}, &ConflictError{Message: checkoutNotRetryableMessage}
	}

	payment, err := s.payments.GetByBookingID(ctx, booking.ID)
	if err != nil || payment.MPPreferenceID == nil || *payment.MPPreferenceID == "" {
		if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
			return CheckoutRetry{}, err
		}
		return CheckoutRetry{}, ErrCheckoutUnavailable
	}
	reader, ok := s.checkout.(preferenceReader)
	if !ok {
		return CheckoutRetry{}, ErrCheckoutUnavailable
	}
	seller, err := s.publicSeller(complex)
	if err != nil {
		return CheckoutRetry{}, err
	}
	pref, err := reader.GetPreference(ctx, *payment.MPPreferenceID, seller)
	if err != nil || pref.InitPoint == "" {
		s.logger.Error("checkout retry: the checkout link could not be read", "error", err, "booking_id", booking.ID)
		return CheckoutRetry{}, ErrCheckoutUnavailable
	}
	return CheckoutRetry{InitPoint: pref.InitPoint, ExpiresAt: expiresAt}, nil
}

// publicRelease is the audit entry of a visitor letting go of a booking whose
// payment failed.
type publicRelease struct {
	Actor   string                `json:"actor"`
	Booking *bookingstore.Booking `json:"booking"`
}

// ReleaseCheckout lets a visitor whose payment failed give the slot back, so
// they can pick another: the checkout is closed, the booking cancelled and the
// slot handed to the grid and the waitlist. A booking already released or run
// out is a no-op; one that was paid after all is refused. No email is sent:
// the visitor is on the page that asked for it, and the owner never had this
// booking (slotguard.UnpaidPublicBooking).
//
// A released booking no longer counts against the visitor's per-address cap
// (only pending ones do). Picking the same hours again gets only what is left
// of the address's budget on them (bookingstore.Store.InsertHold).
func (s *Service) ReleaseCheckout(ctx context.Context, actor Actor, token string) error {
	booking, complex, err := s.ResolveLink(ctx, token)
	if err != nil {
		return err
	}
	if booking.Status == "cancelled" {
		return nil
	}
	if booking.Status != "pending" || !unpaidStorefrontBooking(booking) {
		return &ConflictError{Message: checkoutPaidMessage}
	}

	// Closed first, so the visitor cannot pay while the booking is cancelled.
	// A link that would not close is alerted inside; a payment that lands on
	// it anyway is the late payment the confirmation path already handles.
	s.expireCheckoutPreference(ctx, booking, complex)

	booking.Status = "cancelled"
	note := "Liberado por el cliente tras un pago rechazado"
	if booking.Notes != nil {
		note = *booking.Notes + " | " + note
	}
	booking.Notes = &note
	if err := s.store.UpdateIfUnchanged(ctx, booking); err != nil {
		if errors.Is(err, data.ErrEditConflict) {
			return &ConflictError{Message: publicCancelRaceMessage}
		}
		return err
	}

	s.realtime.PublishBookingChanged(booking.ComplexID)
	s.record(actor, booking.ComplexID, "public_release", &booking.ID, publicRelease{Actor: publicActor, Booking: booking})
	s.releaseHeldSlots(ctx, booking)
	s.freeWaitlistSlot(ctx, waitlist.Freed{
		ComplexID: complex.ID, ComplexName: complex.Name, ComplexSlug: complex.Slug, Day: booking.Date,
	})
	return nil
}

// readCheckoutToken decodes the {"token": "..."} body the two checkout routes
// take, answering the request itself when it is unusable.
func (h *Handler) readCheckoutToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	var input struct {
		Token string `json:"token"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return "", false
	}
	v := validator.New()
	v.Check(input.Token != "", "token", "must be provided")
	v.Check(len(input.Token) <= 256, "token", "must not be more than 256 characters")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return "", false
	}
	return input.Token, true
}

// RetryCheckout handles POST /api/v1/book/checkout/retry: the failure page's
// "Reintentar el pago". Authenticated by the booking's link token, which the
// failure redirect carries (booklink.Failure).
func (h *Handler) RetryCheckout(w http.ResponseWriter, r *http.Request) {
	token, ok := h.readCheckoutToken(w, r)
	if !ok {
		return
	}
	retry, err := h.svc.RetryCheckout(r.Context(), token)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"checkout": httpx.Envelope{
			"mp_init_point": retry.InitPoint,
			"expires_at":    retry.ExpiresAt.UTC().Format(time.RFC3339),
		},
	})
}

// ReleaseCheckout handles POST /api/v1/book/checkout/release: the failure
// page's "Elegir otro horario". 204 when the slot is free (released now, or
// already), 409 when the booking was paid after all.
func (h *Handler) ReleaseCheckout(w http.ResponseWriter, r *http.Request) {
	token, ok := h.readCheckoutToken(w, r)
	if !ok {
		return
	}
	if err := h.svc.ReleaseCheckout(r.Context(), h.actor(r), token); err != nil {
		h.refuse(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

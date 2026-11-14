// public_offer.go — the steps the storefront's write paths share: deciding
// whether some hours are for sale to a visitor (POST /book and POST
// /book/holds ask the same question), and opening the MercadoPago checkout for
// a pending storefront booking (POST /book, with or without a hold).
package bookings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/checkout"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// publicSlotRequest names the hours a visitor asks for: a court of a complex,
// the storefront's grid day, a start on that grid and a length.
type publicSlotRequest struct {
	ComplexID       uuid.UUID
	CourtID         uuid.UUID
	Date            time.Time
	StartTime       string
	DurationMinutes int
}

// publicOffer is a slot the storefront may sell: where it is, the calendar day
// it is filed under, what it costs, and the seller the checkout is created as.
type publicOffer struct {
	startDate time.Time
	price     int
	deposit   int
	seller    mp.Caller
}

// argentinaToday is today's calendar day on the product's wall-clock, at
// midnight UTC like a parsed request date.
func argentinaToday() time.Time {
	now := time.Now().In(timezone.Argentina)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// publicVenue applies the cheap date bounds and loads the complex and court a
// storefront request names, refusing an inactive or mismatched one as not
// found.
func (s *Service) publicVenue(ctx context.Context, in publicSlotRequest) (*complexstore.Complex, *courtstore.Court, error) {
	// Cheap early bound on the grid day, before any lookup. It stops one day
	// short of today rather than at today: yesterday's grid carries
	// after-midnight slots that start on today's calendar day, and a 00:30
	// start is still bookable at 00:10. Whether a start has already passed is
	// the instant check's call below; a booking whose calendar day is over is
	// refused on startDate, once the grid has placed the start.
	todayDate := argentinaToday()
	if in.Date.Before(todayDate.AddDate(0, 0, -1)) {
		return nil, nil, &ConflictError{Message: "cannot book a past date"}
	}

	// H-08: the upper end of the date was never bounded at all — "9999-12-31"
	// was accepted with a real MercadoPago preference created. Unlike a block
	// dated that far out (internal/courts.BlockSlot has the same bound, for
	// the schedules to agree), a booking left standing holds a slot forever
	// and is never reaped: the completion sweep only completes a booking
	// whose end time has already passed. It is a standing way for an
	// anonymous visitor — this endpoint needs no account — to leave permanent
	// rows behind, one request at a time. This early bound allows one day past
	// the horizon, because an after-midnight start of the last bookable grid day
	// falls on the day after it; the exact horizon is applied to startDate below.
	if in.Date.After(todayDate.AddDate(0, 0, bookingstore.MaxBookingHorizonDays+1)) {
		return nil, nil, &ConflictError{Message: fmt.Sprintf(
			"cannot book more than %d days in advance", bookingstore.MaxBookingHorizonDays)}
	}

	// Verify complex exists and is active.
	complex, err := s.complexes.GetByID(ctx, in.ComplexID)
	if err != nil {
		return nil, nil, err
	}
	if !complex.IsActive {
		return nil, nil, data.ErrRecordNotFound
	}

	// Verify court belongs to complex and is active.
	court, err := s.courts.GetByID(ctx, in.CourtID)
	if err != nil {
		return nil, nil, err
	}
	if court.ComplexID != complex.ID || !court.IsActive {
		return nil, nil, data.ErrRecordNotFound
	}
	return complex, court, nil
}

// refuseBlockedPhone answers ErrClientBlocked for a phone the venue blocked,
// reading it without writing anything; an unknown phone carries on.
func (s *Service) refuseBlockedPhone(ctx context.Context, complexID uuid.UUID, phone string) error {
	known, err := s.clients.GetByPhone(ctx, complexID, phone)
	switch {
	case err == nil && known.IsBlocked:
		return ErrClientBlocked
	case err != nil && !errors.Is(err, data.ErrRecordNotFound):
		return err
	}
	return nil
}

// publicOffer decides whether the requested hours are for sale on this court
// and at what price: on the grid, inside the horizon, not blocked, not begun,
// priced, and sold by a venue with a usable MercadoPago credential.
//
// It is one cohesive sequence of refusals, kept in the order the storefront
// has always been told them; splitting it would scatter that order.
//
//nolint:funlen // see the cohesion note above
func (s *Service) publicOffer(ctx context.Context, complex *complexstore.Complex, court *courtstore.Court, in publicSlotRequest) (publicOffer, error) {
	todayDate := argentinaToday()

	// One derivation of "a bookable position on this court today", shared with
	// the storefront that offered it. This replaced three separate rules that
	// lived here: a schedule lookup, a midnight guard with the day's length
	// written out as a literal, and a containment check that carried an
	// after-midnight closing past 1440 a second time. What none of them asked is
	// whether the start is a position on the 30-minute grid at all — see grid.go
	// for what an off-grid booking costs.
	//
	// The order of the refusals is preserved: a venue that is shut is told so
	// before anything about the hours, and a request that reaches midnight is
	// told that rather than that it is outside the schedule.
	grid, gridErr := s.courtGrid(ctx, complex.ID, in.Date)
	if gridErr != nil && !errors.Is(gridErr, errComplexClosed) {
		return publicOffer{}, gridErr
	}
	// The storefront lists a start past midnight under the day its window was
	// sold from, so in.Date is that grid day. Place says how far into it the
	// start falls, and the booking is then filed, timed, priced and locked on the
	// calendar day the start actually falls on, startDate. A booking is stored
	// under its calendar day everywhere else (see the owner path and grid.go),
	// and this path must agree with them.
	var minute int
	if gridErr == nil {
		minute, gridErr = grid.Place(in.StartTime, in.DurationMinutes)
	}
	if gridErr != nil {
		return publicOffer{}, &ConflictError{Message: gridRefusal(gridErr)}
	}
	startDate := in.Date.AddDate(0, 0, slots.CalendarDays(minute))
	// The two calendar-day rules are checked on startDate, the day the booking
	// is stored under. Checked on in.Date, an after-midnight slot of the last
	// bookable day would pass the horizon one day late.
	if startDate.Before(todayDate) {
		return publicOffer{}, &ConflictError{Message: "cannot book a past date"}
	}
	if startDate.After(todayDate.AddDate(0, 0, bookingstore.MaxBookingHorizonDays)) {
		return publicOffer{}, &ConflictError{Message: fmt.Sprintf(
			"cannot book more than %d days in advance", bookingstore.MaxBookingHorizonDays)}
	}

	// A slot the owner blocked is not for sale, and this path had no way of
	// knowing it was blocked: the storefront omits it, and nothing between the
	// storefront and the insert asked again.
	// The end comes from adding the duration to the start instant, never from
	// parsing the end string: slots.Add wraps at midnight, so a 23:00 booking of
	// sixty minutes would read as ending before it began.
	// The candidate's instants are built on the product's wall-clock, the same
	// one blockedDays anchors the block side to. date is parsed as midnight UTC
	// here, and slots.At carries a date's location through, so leaving it
	// unanchored put both sides three hours out — which happened to cancel out
	// on this path and did not on the confirmation path (see grid.go).
	startAt := slots.At(timezone.Day(startDate), in.StartTime)
	endAt := startAt.Add(time.Duration(in.DurationMinutes) * time.Minute)

	blocked, err := s.slotIsBlocked(ctx, court.ID, startDate, startAt, endAt)
	if err != nil {
		return publicOffer{}, err
	}
	if blocked {
		return publicOffer{}, &ConflictError{Message: blockedSlotMessage}
	}
	// The storefront's grid already hides the hours a fixed weekly series holds;
	// this is the same answer for a request that names them anyway. The
	// insert-time guard (store.SlotTaken) is what holds against a race.
	held, err := s.heldByFixedBooking(ctx, court.ID, startDate, startAt, endAt, nil)
	if err != nil {
		return publicOffer{}, err
	}
	if held {
		return publicOffer{}, &ConflictError{Message: blockedSlotMessage}
	}
	// An open-play event takes its courts off sale for its window the same way.
	heldByEvent, err := s.heldByEvent(ctx, court.ID, startDate, startAt, endAt)
	if err != nil {
		return publicOffer{}, err
	}
	if heldByEvent {
		return publicOffer{}, &ConflictError{Message: blockedSlotMessage}
	}

	// Reject slots that have already begun. Compared as instants: an after-
	// midnight slot of today's grid is tomorrow's, and is not in the past.
	now := time.Now().In(timezone.Argentina)
	if !startAt.After(now.Truncate(time.Minute)) {
		return publicOffer{}, &ConflictError{Message: passedSlotMessage}
	}

	totalPrice, priceErr := s.findPrice(ctx, court.ComplexID, court.ID, startDate, in.StartTime, in.DurationMinutes)
	if priceErr != nil {
		// A booking no rule fully covers is not for sale — the same sentence
		// the availability grid says by omitting it. This used to fall back to
		// another band and charge a real amount for it.
		if errors.Is(priceErr, pricing.ErrNoPriceRule) {
			return publicOffer{}, &ConflictError{Message: unpricedSlotMessage}
		}
		return publicOffer{}, priceErr
	}

	depositAmount := totalPrice * complex.DepositPercentage / 100

	// Public bookings always require MercadoPago payment.
	// If no deposit configured, charge full price.
	if depositAmount == 0 {
		depositAmount = totalPrice
	}

	seller, err := s.publicSeller(complex)
	if err != nil {
		return publicOffer{}, err
	}
	return publicOffer{startDate: startDate, price: totalPrice, deposit: depositAmount, seller: seller}, nil
}

// passedSlotMessage refuses hours that have already begun.
const passedSlotMessage = "cannot book a time slot that has already passed"

// publicSeller is the party a storefront checkout is created as. The checkout
// is created as this seller and nobody else — mp.AsSeller has no arm that
// reaches MercadoPago as the platform, which is where the money would
// otherwise land.
func (s *Service) publicSeller(complex *complexstore.Complex) (mp.Caller, error) {
	seller, err := checkout.Seller(complex)
	if err != nil {
		return seller, ErrMercadoPagoNotConnected
	}
	return seller, nil
}

// recordPublicBook writes the audit entry for a storefront booking that now
// names who it is for.
//
// The slot is now held by a booking a stranger created, which until now was
// the one mutation on this system that left no trail at all: everything an
// owner does is recorded and everything an unauthenticated caller does was
// not.
//
// It is recorded at the commit, and not at the end. The checkout failure
// cancels this booking again a few hundred milliseconds later, but that is
// this request undoing its own work rather than an actor doing something, and
// a second nil-actor row for it would say somebody cancelled a booking when
// nobody did. Anyone following the entity id reads the booking's real current
// status.
//
// The entity id is the booking's primary key. specs/booking-link-credential
// forbids that id reaching Sentry from the three public token routes, and
// forbids it appearing in these routes' response bodies — both because the id
// was doing duty as a bearer credential and Sentry has no scrubber rule for a
// bare UUID. Neither applies here. The trail is a first-party table scoped by
// complex_id and readable only by the complex that owns the booking
// (internal/audit/handler.go) or by a superadmin, so holding the id there
// authorizes nothing; the staff cancel path already writes the same id
// (Service.Cancel); and an entry with no entity id would say that somebody
// booked something, with no way to tell what — which is not a trail. What the
// spec does reach is the token, and the encoding keeps it out.
func (s *Service) recordPublicBook(actor Actor, booking *bookingstore.Booking) {
	s.record(actor, booking.ComplexID, "public_book", &booking.ID, publicBooking{
		Actor:    publicActor,
		ClientID: booking.ClientID,
		Booking:  booking,
	})
}

// checkoutRequest is what openCheckout needs: the pending booking, where it
// is, who sells it, how long the link may stay payable, and who pays.
type checkoutRequest struct {
	booking   *bookingstore.Booking
	complex   *complexstore.Complex
	court     *courtstore.Court
	seller    mp.Caller
	rawDate   string
	expiresIn time.Duration
	payer     PublicBookInput
}

// openCheckout creates the MercadoPago checkout for a pending storefront
// booking and records its payment row. When MercadoPago will not create it,
// the booking is cancelled so its slot frees at once, and the answer is
// ErrCheckoutUnavailable.
//
// One external call and the two writes that depend on its answer, kept
// together so the failure path is read beside the call that causes it.
//
//nolint:funlen // see the cohesion note above
func (s *Service) openCheckout(ctx context.Context, req checkoutRequest) (PublicBookResult, error) {
	booking, complex := req.booking, req.complex
	mpAmount := booking.DepositAmount

	// Calculate the flat service fee, paid by client.
	serviceFee := pricing.ServiceFee(mpAmount)
	totalClientPays := mpAmount + serviceFee

	result := PublicBookResult{
		Booking:         booking,
		Complex:         complex,
		Court:           req.court,
		ServiceFee:      serviceFee,
		TotalClientPays: totalClientPays,
	}

	// Always create MercadoPago preference for public bookings. back_urls are
	// built through booklink, keyed on the booking's access token — the
	// credential the three public routes now authorize on, not the primary
	// key. mp.CreatePreferenceInput no longer builds any client-facing URL
	// itself.
	prefInput := mp.CreatePreferenceInput{
		BookingID:      booking.ID,
		ComplexName:    complex.Name,
		CourtName:      req.court.Name,
		Date:           req.rawDate,
		StartTime:      booking.StartTime,
		Amount:         totalClientPays,
		MarketplaceFee: serviceFee,
		Caller:         req.seller,
		BackURLs: mp.BackURLs{
			Success: booklink.Success(s.cfg.PublicURL, complex.Slug, booking.LinkToken),
			Failure: booklink.Failure(s.cfg.PublicURL, complex.Slug),
			Pending: booklink.SuccessPending(s.cfg.PublicURL, complex.Slug, booking.LinkToken),
		},
		BackendURL:     s.cfg.BackendURL,
		ExpiresIn:      req.expiresIn,
		PayerEmail:     req.payer.ClientEmail,
		PayerFirstName: req.payer.ClientFirstName,
		PayerLastName:  req.payer.ClientLastName,
		PayerPhone:     req.payer.ClientPhone,
	}

	pref, err := s.createMPPreferenceWithRetry(ctx, prefInput, complex)
	if err != nil {
		s.logger.Error("public booking: failed to create MP preference",
			"error", err,
			"booking_id", booking.ID,
		)
		// Cancel the booking so the slot is freed immediately instead of
		// blocking it for 15 minutes until the expiration sweep runs.
		//
		// The client may have left while MercadoPago was refusing the checkout,
		// and that cancels ctx. A cancelled context fails the write before it
		// reaches PostgreSQL, so the booking would stay pending and the slot
		// taken until the sweep. Detach from that cancellation, and keep a
		// deadline so a wedged database cannot hold this request's goroutine.
		booking.Status = "cancelled"
		cancelCtx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), checkoutCleanupBudget)
		cancelErr := s.store.Update(cancelCtx, booking)
		cancelWrite()
		if cancelErr != nil {
			s.logger.Error("public booking: failed to cancel booking after MP preference failure",
				"error", cancelErr,
				"booking_id", booking.ID,
			)
		}
		return PublicBookResult{}, ErrCheckoutUnavailable
	}

	payment := &paymentstore.Payment{
		BookingID:      booking.ID,
		ComplexID:      booking.ComplexID,
		Amount:         mpAmount,
		ServiceFee:     serviceFee,
		Method:         "mercadopago",
		Status:         "unpaid",
		MPPreferenceID: &pref.ID,
	}

	if err := s.payments.Insert(ctx, payment); err != nil {
		s.logger.Error("public booking: failed to save payment record",
			"error", err,
			"booking_id", booking.ID,
		)
	}

	s.logger.Info("mp preference created",
		"init_point", pref.InitPoint,
		"env", s.cfg.Environment,
	)

	result.Preference = pref
	return result, nil
}

package bookings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/validator"
)

// publicActor is what the audit trail records as the actor for the two
// mutations an unauthenticated caller can make.
//
// audit_log.user_id is a foreign key into users, and a client has no account:
// there is no id to put there, so it is nil — the same nil a system action
// writes. The two must stay distinguishable, so the actor is named in the
// value instead. The IP address on the entry is the only other handle anyone
// has on this caller.
const publicActor = "client"

// publicBooking is the audit value for a booking a client created without an
// account.
//
// It carries the booking struct itself, exactly as the staff paths do, rather
// than a hand-picked subset: the point of an audit value is that a reader can
// see what the record looked like, and a subset drifts from the struct the
// moment a field is added. The booking's own tags decide what is safe to
// encode — LinkToken is `json:"-"` (internal/data/bookings.go), which is what
// keeps the access token that authorizes the three public routes out of the
// trail, the same way data.Complex's tags keep MercadoPago credentials out of
// it. TestPublicBookAuditEntryCarriesNoLinkToken pins that.
type publicBooking struct {
	Actor    string        `json:"actor"`
	ClientID uuid.UUID     `json:"client_id"`
	Booking  *data.Booking `json:"booking"`
}

// publicCancellation is the audit value for a cancellation a client made
// without an account.
//
// It records the refund window decision alongside the booking because that
// decision is this handler's alone. The staff path deliberately ignores the
// complex's window (see actions.go), so on that path there is nothing to
// record; here, whether the client's deposit comes back is decided in this
// request, from a policy value read off the complex, and nothing else in the
// system writes down that it was applied. What then became of the money is
// internal/payments' entry to write, not this one's.
type publicCancellation struct {
	Actor              string        `json:"actor"`
	ClientID           uuid.UUID     `json:"client_id"`
	Booking            *data.Booking `json:"booking"`
	WithinRefundWindow bool          `json:"within_refund_window"`
	OwesRefund         bool          `json:"owes_refund"`
}

// PublicBook handles POST /api/v1/book, the flow a client uses with no account.
//
// It holds every slot it is about to take, creates the booking as pending,
// then creates the MercadoPago checkout. The booking is only confirmed when
// the payment webhook lands. If anything fails after the slots are held they
// are released, or they stay locked until the TTL expires and nobody can
// book them.
//
// It is one cohesive request lifecycle — validate, price, hold the slots,
// insert, create the checkout — and splitting it would spread a single
// transaction-like sequence across helpers.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) PublicBook(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ComplexID       string `json:"complex_id"`
		CourtID         string `json:"court_id"`
		Date            string `json:"date"`
		StartTime       string `json:"start_time"`
		DurationMinutes int    `json:"duration_minutes"`
		ClientFirstName string `json:"client_first_name"`
		ClientLastName  string `json:"client_last_name"`
		ClientPhone     string `json:"client_phone"`
		ClientEmail     string `json:"client_email"`
		ClientNotes     string `json:"client_notes"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.ComplexID != "", "complex_id", "must be provided")
	v.Check(input.CourtID != "", "court_id", "must be provided")
	v.Check(input.Date != "", "date", "must be provided")
	v.Check(input.StartTime != "", "start_time", "must be provided")
	v.Check(input.ClientFirstName != "", "client_first_name", "must be provided")
	// H-16: a 10,000-character client_first_name used to be accepted outright
	// — nothing checked how long it was, only that it was non-empty. The value
	// goes into clients, into the booking, into every notification payload,
	// and out to MercadoPago as the payer name, so an unbounded free-text
	// field here reaches a third-party API and the owner's dashboard from an
	// endpoint that needs no account. 100 matches the bound this codebase
	// already puts on other short name fields (e.g. courts.Create's own
	// "name").
	v.Check(len(input.ClientFirstName) <= 100, "client_first_name", "must not be more than 100 characters")
	v.Check(input.ClientLastName != "", "client_last_name", "must be provided")
	v.Check(len(input.ClientLastName) <= 100, "client_last_name", "must not be more than 100 characters")
	v.Check(input.ClientPhone != "", "client_phone", "must be provided")
	if input.ClientPhone != "" {
		normalized, nerr := validator.NormalizePhone(input.ClientPhone)
		if nerr != nil {
			v.AddError("client_phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			input.ClientPhone = normalized
		}
	}
	// Optional: every notification below already skips an empty address, and
	// the payer email on the MercadoPago preference is sent only when present.
	if input.ClientEmail != "" {
		// 254 is RFC 5321's own practical ceiling on a full email address
		// (local-part@domain, both bounded), and EmailRX's character classes
		// alone do not bound the total length.
		v.Check(len(input.ClientEmail) <= 254, "client_email", "must not be more than 254 characters")
		v.Check(validator.Matches(input.ClientEmail, validator.EmailRX), "client_email", "must be a valid email address")
	}
	if input.StartTime != "" {
		v.Check(slots.ValidFormat(input.StartTime), "start_time", "must be in HH:MM format (00:00-23:59)")
	}

	v.Check(validator.PermittedValue(input.DurationMinutes, slots.PermittedDurations()...), "duration_minutes", durationMessage)
	v.Check(len(input.ClientNotes) <= 2000, "client_notes", "must not exceed 2000 characters")

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	complexID, err := uuid.Parse(input.ComplexID)
	if err != nil {
		v.AddError("complex_id", "must be a valid UUID")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	courtID, err := uuid.Parse(input.CourtID)
	if err != nil {
		v.AddError("court_id", "must be a valid UUID")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		v.AddError("date", "must be in YYYY-MM-DD format")
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// Reject past dates.
	todayArg := time.Now().In(timezone.Argentina)
	todayDate := time.Date(todayArg.Year(), todayArg.Month(), todayArg.Day(), 0, 0, 0, 0, time.UTC)
	if date.Before(todayDate) {
		h.respond.Error(w, r, http.StatusConflict, "cannot book a past date")
		return
	}

	// H-08: the upper end of the date was never bounded at all — "9999-12-31"
	// was accepted with a real MercadoPago preference created. Unlike a block
	// dated that far out (internal/courts.BlockSlot has the same bound, for
	// the schedules to agree), a booking left standing holds a slot forever
	// and is never reaped: cron's completeBookings only completes a booking
	// whose end time has already passed. It is a standing way for an
	// anonymous visitor — this endpoint needs no account — to leave permanent
	// rows behind, one request at a time.
	if date.After(todayDate.AddDate(0, 0, data.MaxBookingHorizonDays)) {
		h.respond.Error(w, r, http.StatusConflict,
			fmt.Sprintf("cannot book more than %d days in advance", data.MaxBookingHorizonDays))
		return
	}

	// Verify complex exists and is active.
	complex, err := h.complexes.GetByID(r.Context(), complexID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}
	if !complex.IsActive {
		h.respond.NotFound(w, r)
		return
	}

	// Verify court belongs to complex and is active.
	court, err := h.courts.GetByID(r.Context(), courtID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}
	if court.ComplexID != complex.ID || !court.IsActive {
		h.respond.NotFound(w, r)
		return
	}

	// Get or create client. This endpoint needs no account (R1-client-name-
	// overwrite), so a phone match here must never overwrite an existing
	// client's stored name — only the authenticated owner path (create.go)
	// is trusted with that — see ClientModel.GetOrCreate's comment on
	// allowNameUpdate.
	client, err := h.clients.GetOrCreate(r.Context(), complex.ID, input.ClientFirstName, input.ClientLastName, input.ClientPhone, input.ClientEmail, false)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	// Verify client is not blocked.
	if client.IsBlocked {
		h.respond.Error(w, r, http.StatusForbidden, "your account is blocked, contact the complex for more information")
		return
	}

	// The slot lock is keyed on a date and two times of day, so it still needs
	// the clock reading the booking's hours end on. slots.Add wraps at
	// midnight, which is right for slot_locks — a lock is asked for and
	// released by the same pair of strings and never compared as an interval —
	// and is exactly what was wrong about storing it on the booking, which
	// no longer stores.
	lockEndTime := slots.Add(input.StartTime, input.DurationMinutes)

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
	grid, gridErr := h.courtGrid(r.Context(), complex.ID, date)
	if gridErr != nil && !errors.Is(gridErr, errComplexClosed) {
		h.respond.ServerError(w, r, gridErr)
		return
	}
	if gridErr == nil {
		gridErr = grid.Validate(input.StartTime, input.DurationMinutes)
	}
	if gridErr != nil {
		h.respond.Error(w, r, http.StatusConflict, gridRefusal(gridErr))
		return
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
	startAt := slots.At(timezone.Day(date), input.StartTime)
	endAt := startAt.Add(time.Duration(input.DurationMinutes) * time.Minute)

	blocked, err := h.slotIsBlocked(r.Context(), court.ID, date, startAt, endAt)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if blocked {
		h.respond.Error(w, r, http.StatusConflict, blockedSlotMessage)
		return
	}

	// Reject past slots if booking for today.
	now := time.Now().In(timezone.Argentina)
	isToday := date.Year() == now.Year() && date.YearDay() == now.YearDay()
	if isToday && input.StartTime <= now.Format("15:04") {
		h.respond.Error(w, r, http.StatusConflict, "cannot book a time slot that has already passed")
		return
	}

	totalPrice, priceErr := h.findPrice(r, court.ComplexID, court.ID, date, input.StartTime, input.DurationMinutes)
	if priceErr != nil {
		// A booking no rule fully covers is not for sale — the same sentence
		// the availability grid says by omitting it. This used to fall back to
		// another band and charge a real amount for it.
		if errors.Is(priceErr, pricing.ErrNoPriceRule) {
			h.respond.Error(w, r, http.StatusConflict, unpricedSlotMessage)
			return
		}
		h.respond.ServerError(w, r, priceErr)
		return
	}

	depositAmount := totalPrice * complex.DepositPercentage / 100

	// Public bookings always require MercadoPago payment.
	// If no deposit configured, charge full price.
	if depositAmount == 0 {
		depositAmount = totalPrice
	}
	mpAmount := depositAmount

	// Verify that the complex has MercadoPago connected (marketplace). The
	// checkout is created as this seller and nobody else — mp.AsSeller has no
	// arm that reaches MercadoPago as the platform, which is where the money
	// would otherwise land.
	var seller mp.Caller
	sellerToken, sellerErr := complex.SellerAccessToken()
	if sellerErr == nil {
		seller, sellerErr = mp.AsSeller(sellerToken)
	}
	if sellerErr != nil {
		if errors.Is(sellerErr, mpcred.ErrMPCredentialUnreadable) {
			sentry.CaptureMessage(fmt.Sprintf("SELLER TOKEN UNREADABLE (refusing checkout): complex_id=%s error=%v", complex.ID, sellerErr))
		}
		h.respond.Error(w, r, http.StatusBadRequest, "the complex does not have MercadoPago connected, contact the complex")
		return
	}

	// Calculate service fee: 7% (min 1000 ARS), paid by client.
	serviceFee := pricing.ServiceFee(mpAmount)
	totalClientPays := mpAmount + serviceFee

	booking := &data.Booking{
		ComplexID:        complex.ID,
		CourtID:          courtID,
		ClientID:         client.ID,
		Date:             date,
		StartTime:        input.StartTime,
		DurationMinutes:  input.DurationMinutes,
		Price:            totalPrice,
		DepositAmount:    depositAmount,
		Status:           "pending",
		CollectionStatus: data.CollectionStatusUnpaid,
		RefundStatus:     data.RefundStatusNone,
	}

	if input.ClientNotes != "" {
		booking.Notes = &input.ClientNotes
	}

	// Acquire a slot lock over the whole booking span to prevent the race
	// condition where another user books the same slot while this user is
	// completing the MP payment (~15 min window).
	//
	// This used to be one lock per fixed-size chunk, one per consecutive slot
	// the booking spanned. Now that a booking's length is chosen per request
	// rather than assembled from repeats of the court's own slot length, there
	// is one span and one lock for it.
	slotLockTTL := h.cfg.SlotLockTTL
	lockErr := h.locks.AcquireLock(r.Context(), courtID, date, input.StartTime, lockEndTime, nil, slotLockTTL)
	if lockErr != nil {
		if errors.Is(lockErr, data.ErrSlotLocked) {
			h.respond.Error(w, r, http.StatusConflict, "the selected time slot is no longer available, please choose another")
			return
		}
		h.respond.ServerError(w, r, lockErr)
		return
	}

	err = h.store.InsertSafe(r.Context(), booking)
	if err != nil {
		// Release the slot lock on insert failure.
		h.releaseSlotLock(r.Context(), courtID, date, input.StartTime)
		if errors.Is(err, data.ErrDuplicateBooking) || errors.Is(err, data.ErrSlotUnavailable) {
			h.respond.Error(w, r, http.StatusConflict, "the selected time slot is no longer available, please choose another")
			return
		}
		// H-22: the venue deleted the court between the GetByID above and this
		// commit. Not a slot conflict — the slot message would tell the visitor
		// to pick another hour on a court that no longer exists — and not a
		// 500 either. The same NotFound the check above answers.
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.NotFound(w, r)
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	h.realtime.PublishBookingChanged(complex.ID)

	// The slot is now held by a booking a stranger created, which until now was
	// the one mutation on this system that left no trail at all: everything an
	// owner does is recorded and everything an unauthenticated caller does was
	// not.
	//
	// It is recorded here, at the commit, and not at the end of the handler.
	// The checkout failure below cancels this booking again a few hundred
	// milliseconds later, but that is this request undoing its own work rather
	// than an actor doing something, and a second nil-actor row for it would
	// say somebody cancelled a booking when nobody did. Anyone following the
	// entity id reads the booking's real current status.
	//
	// The entity id is the booking's primary key. specs/booking-link-credential
	// forbids that id reaching Sentry from the three public token routes, and
	// forbids it appearing in these routes' response bodies — both because the
	// id was doing duty as a bearer credential and Sentry has no scrubber rule
	// for a bare UUID. Neither applies here. The trail is a first-party table
	// scoped by complex_id and readable only by the complex that owns the
	// booking (internal/audit/handler.go) or by a superadmin, so holding the id
	// there authorizes nothing; the staff cancel path already writes the same
	// id (actions.go); and an entry with no entity id would say that somebody
	// booked something, with no way to tell what — which is not a trail. What
	// the spec does reach is the token, and the encoding above keeps it out.
	h.record(r, complex.ID, "public_book", &booking.ID, publicBooking{
		Actor:    publicActor,
		ClientID: booking.ClientID,
		Booking:  booking,
	})

	// The response never carries the booking's primary key
	// (specs/booking-link-credential): only the fields the frontend needs to
	// render a confirmation, plus the token that now authorizes the three
	// public routes.
	response := httpx.Envelope{
		"booking": httpx.Envelope{
			"status":            booking.Status,
			"collection_status": booking.CollectionStatus,
			"refund_status":     booking.RefundStatus,
			"date":              booking.Date.Format("2006-01-02"),
			"start_time":        booking.StartTime,
			"starts_at":         booking.StartsAt.Format(time.RFC3339),
			"ends_at":           booking.EndsAt.Format(time.RFC3339),
			"court_name":        court.Name,
			"complex_name":      complex.Name,
			"price":             booking.Price,
			"deposit_amount":    booking.DepositAmount,
		},
		"token": booking.LinkToken,
	}

	// Always create MercadoPago preference for public bookings. back_urls are
	// built through booklink, keyed on the booking's access token — the
	// credential the three public routes now authorize on, not the primary
	// key. mp.CreatePreferenceInput no longer builds any client-facing URL
	// itself.
	prefInput := mp.CreatePreferenceInput{
		BookingID:      booking.ID,
		ComplexName:    complex.Name,
		CourtName:      court.Name,
		Date:           input.Date,
		StartTime:      input.StartTime,
		Amount:         totalClientPays,
		MarketplaceFee: serviceFee,
		Caller:         seller,
		BackURLs: mp.BackURLs{
			Success: booklink.Success(h.cfg.FrontendURL, complex.Slug, booking.LinkToken),
			Failure: booklink.Failure(h.cfg.FrontendURL, complex.Slug),
			Pending: booklink.SuccessPending(h.cfg.FrontendURL, complex.Slug, booking.LinkToken),
		},
		BackendURL:     h.cfg.BackendURL,
		ExpiresIn:      h.cfg.PaymentExpiry,
		PayerEmail:     input.ClientEmail,
		PayerFirstName: input.ClientFirstName,
		PayerLastName:  input.ClientLastName,
		PayerPhone:     input.ClientPhone,
	}

	pref, err := h.createMPPreferenceWithRetry(r.Context(), prefInput, complex)
	if err != nil {
		h.logger.Error("public booking: failed to create MP preference",
			"error", err,
			"booking_id", booking.ID,
		)
		// Cancel the booking so the slot is freed immediately instead of
		// blocking it for 15 minutes until the expiration cron runs.
		booking.Status = "cancelled"
		if cancelErr := h.store.Update(r.Context(), booking); cancelErr != nil {
			h.logger.Error("public booking: failed to cancel booking after MP preference failure",
				"error", cancelErr,
				"booking_id", booking.ID,
			)
		}
		h.releaseSlotLock(r.Context(), courtID, date, input.StartTime)
		h.respond.Error(w, r, http.StatusServiceUnavailable, "no se pudo crear el enlace de pago, intente nuevamente")
		return
	} else {
		payment := &data.Payment{
			BookingID:      booking.ID,
			ComplexID:      booking.ComplexID,
			Amount:         mpAmount,
			ServiceFee:     serviceFee,
			Method:         "mercadopago",
			Status:         "unpaid",
			MPPreferenceID: &pref.ID,
		}

		if err := h.payments.Insert(r.Context(), payment); err != nil {
			h.logger.Error("public booking: failed to save payment record",
				"error", err,
				"booking_id", booking.ID,
			)
		}

		h.logger.Info("mp preference created",
			"init_point", pref.InitPoint,
			"env", h.cfg.Environment,
		)
		// MP retired the sandbox environment: whether a payment runs in test
		// or production mode is decided by the credential (test-user
		// APP_USR- tokens), not by the redirect URL. sandbox_init_point is
		// deprecated by MP; always use init_point.
		response["mp_init_point"] = pref.InitPoint
		response["mp_preference_id"] = pref.ID
		response["service_fee"] = serviceFee
		response["total_client_pays"] = totalClientPays
	}

	h.respond.JSON(w, r, http.StatusCreated, response)
}

// linkExpiredMessage is the 410 body's recourse copy for all three public
// routes (specs/booking-link-credential's "distinguishable, actionable
// response" scenario): non-empty, distinguishable from the 404 an unknown
// token gets, and it names a next step. No self-service reissue path exists
// (proposal's Out of Scope), so the recourse is contacting the venue.
const linkExpiredMessage = "this link has expired; contact the complex directly to check on your booking"

// venueGoneMessage is H-18's fix direction applied: a booking link can be
// perfectly live and still point at a venue (or, more narrowly, a single
// court of one still-open venue) that has since been soft-deleted. That is
// not the same fact as the token itself being unknown or expired, and the
// client should not be told to keep retrying a request that will never
// succeed. It follows linkExpiredMessage's lead — non-empty, distinguishable
// from a bare 404/500, and naming a next step — because that is the only
// hand-written, deliberately actionable body this package already has for
// exactly this class of dead end; a second copy of the same shape would be
// the one that drifts.
const venueGoneMessage = "the venue for this booking is no longer available; contact them directly if you need to follow up"

// resolveLink is the whole authorization for the three public routes
// (specs/booking-link-credential): it resolves token, loads the complex
// LinkLive needs, and answers 404/410/500 itself. Each caller keeps its own
// absent/malformed shape (400 on the GETs, 422 on PublicCancel) before
// calling this — resolveLink never sees an empty token.
//
// It returns the complex it had to load for LinkLive, so PublicCancelInfo and
// PublicCancel can drop their own separate complexes.GetByID call.
func (h *Handler) resolveLink(w http.ResponseWriter, r *http.Request, token string) (*data.Booking, *data.Complex, bool) {
	booking, expiresAt, err := h.linkResolver.ResolveBooking(r.Context(), token)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.NotFound(w, r)
			return nil, nil, false
		}
		h.respond.ServerError(w, r, err)
		return nil, nil, false
	}

	// H-18: the booking resolved above can be entirely legitimate while the
	// venue it belongs to has been soft-deleted since — GetByID reads
	// active_complexes, which excludes it. That used to fall straight into
	// ServerError and answer 500 with a body the client's retry button could
	// never get past; it is not this request's fault, and it can never
	// succeed by trying again.
	complex, err := h.complexes.GetByID(r.Context(), booking.ComplexID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.Error(w, r, http.StatusGone, venueGoneMessage)
			return nil, nil, false
		}
		h.respond.ServerError(w, r, err)
		return nil, nil, false
	}

	if !pricing.LinkLive(booking, expiresAt, complex.CancellationHours, h.cfg.GracePeriod, time.Now()) {
		h.respond.Error(w, r, http.StatusGone, linkExpiredMessage)
		return nil, nil, false
	}

	return booking, complex, true
}

// courtOrGone loads a booking's court for the two read-only public routes,
// answering venueGoneMessage instead of a raw 500 when the court has been
// soft-deleted — directly (an owner retiring one court) or by the cascade
// from a soft-deleted complex, which resolveLink above now catches at the
// complex itself; this is the same failure one join further in, for the case
// where the venue is still open but this particular court is not.
//
// PublicCancel (the mutating sibling) already guards its own equivalent
// lookup and is left alone — this only gives the two read-only handlers the
// same care, per the finding: three sibling handlers, the same lookup, and
// the one that writes was the only one already careful.
func (h *Handler) courtOrGone(w http.ResponseWriter, r *http.Request, courtID uuid.UUID) (*data.Court, bool) {
	court, err := h.courts.GetByID(r.Context(), courtID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.Error(w, r, http.StatusGone, venueGoneMessage)
			return nil, false
		}
		h.respond.ServerError(w, r, err)
		return nil, false
	}
	return court, true
}

// cancellationInfo is what a client can do about their booking right now —
// computed once and read by both PublicStatus and PublicCancelInfo, so the
// two can never disagree about whether a booking can still be cancelled or
// when its refund window closes.
type cancellationInfo struct {
	// canCancel mirrors the guard PublicCancel itself enforces: a booking
	// already cancelled, completed or a no-show cannot be cancelled again.
	canCancel bool
	// withinWindow is true only when canCancel is also true — a terminal
	// booking has no window left to be inside of, regardless of what the
	// dates alone would say.
	withinWindow bool
	// deadline is the instant pricing.CanRefund stops being true, from
	// pricing.RefundDeadline — meaningful only when canCancel is true.
	deadline time.Time
}

// cancellationInfo computes what PublicStatus and PublicCancelInfo both tell
// a client about cancelling their booking right now.
func (h *Handler) cancellationInfo(booking *data.Booking, complex *data.Complex) cancellationInfo {
	canCancel := booking.Status != "cancelled" && booking.Status != "completed" && booking.Status != "no_show"
	return cancellationInfo{
		canCancel:    canCancel,
		withinWindow: canCancel && pricing.CanRefund(booking, complex.CancellationHours, h.cfg.GracePeriod),
		deadline:     pricing.RefundDeadline(booking, complex.CancellationHours, h.cfg.GracePeriod),
	}
}

// PublicStatus handles GET /api/v1/book/status. It is public because the
// client has no account; the access token they were given is the credential
// (specs/booking-link-credential) — the booking's primary key authorizes
// nothing here.
//
// The public success page renders entirely from this response: a client who
// opens the MercadoPago redirect in a different browser, or opens the link
// later, has no cached copy of the booking to fall back on, so the response
// carries the whole picture rather than just the two status fields.
func (h *Handler) PublicStatus(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	token := httpx.ReadString(qs, booklink.QueryParam, "")
	if token == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("%s query parameter is required", booklink.QueryParam))
		return
	}

	booking, complex, ok := h.resolveLink(w, r, token)
	if !ok {
		return
	}

	court, ok := h.courtOrGone(w, r, booking.CourtID)
	if !ok {
		return
	}

	payment, err := h.payments.GetByBookingID(r.Context(), booking.ID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, h.publicStatusResponse(booking, complex, court, payment))
}

// publicStatusResponse builds PublicStatus's payload. payment is nil when the
// booking carries no payment row yet (data.ErrRecordNotFound), in which case
// service_fee reads 0 rather than failing the request.
func (h *Handler) publicStatusResponse(booking *data.Booking, complex *data.Complex, court *data.Court, payment *data.Payment) httpx.Envelope {
	serviceFee := 0
	if payment != nil {
		serviceFee = payment.ServiceFee
	}

	remaining := booking.Price - booking.DepositAmount
	if remaining < 0 {
		remaining = 0
	}

	info := h.cancellationInfo(booking, complex)
	cancellation := httpx.Envelope{
		"can_cancel":         info.canCancel,
		"can_refund_now":     info.withinWindow,
		"cancellation_hours": complex.CancellationHours,
	}
	if info.canCancel {
		cancellation["refund_deadline"] = info.deadline.Format(time.RFC3339)
	} else {
		cancellation["refund_deadline"] = nil
	}

	bookingEnvelope := httpx.Envelope{
		"status":            booking.Status,
		"collection_status": booking.CollectionStatus,
		"refund_status":     booking.RefundStatus,
		"complex_name":      complex.Name,
		"complex_address":   complex.Address,
		"court_name":        court.Name,
		"sport":             court.Sport,
		"court_type":        court.CourtType,
		"date":              booking.Date.Format("2006-01-02"),
		"start_time":        booking.StartTime,
		"starts_at":         booking.StartsAt.Format(time.RFC3339),
		"ends_at":           booking.EndsAt.Format(time.RFC3339),
		"duration_minutes":  booking.DurationMinutes,
		"price":             booking.Price,
		"deposit_amount":    booking.DepositAmount,
		"service_fee":       serviceFee,
		"remaining_amount":  remaining,
		"cancellation":      cancellation,
	}
	if complex.Phone != "" {
		bookingEnvelope["complex_phone"] = complex.Phone
	}

	return httpx.Envelope{"booking": bookingEnvelope}
}

// PublicCancelInfo handles GET /api/v1/book/cancel-info, telling the client
// whether cancelling now would return their deposit.
func (h *Handler) PublicCancelInfo(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	token := httpx.ReadString(qs, booklink.QueryParam, "")
	if token == "" {
		h.respond.BadRequest(w, r, fmt.Errorf("%s query parameter is required", booklink.QueryParam))
		return
	}

	booking, complex, ok := h.resolveLink(w, r, token)
	if !ok {
		return
	}

	court, ok := h.courtOrGone(w, r, booking.CourtID)
	if !ok {
		return
	}

	h.respond.JSON(w, r, http.StatusOK, h.publicCancelInfoResponse(r.Context(), booking, complex, court))
}

// publicCancelInfoResponse builds PublicCancelInfo's payload: the same
// booking detail the success page shows (publicStatusResponse), plus the
// money actually at stake if the client cancels right now.
func (h *Handler) publicCancelInfoResponse(ctx context.Context, booking *data.Booking, complex *data.Complex, court *data.Court) httpx.Envelope {
	info := h.cancellationInfo(booking, complex)

	// The window decides whether the money is owed. The payment decides whether
	// anything can return it. Answering from the window alone is what let this
	// endpoint promise an automatic refund on a booking paid in cash — after which
	// the booking was cancelled, the refund path declined without a word, and the
	// only record that the cash was owed was the client's memory.
	method := h.refundMethod(ctx, booking)
	canRefund := info.withinWindow && method != refundNotApplicable

	refundAmount, paidAmount := h.cancelPreviewAmounts(ctx, booking, canRefund, method)

	bookingEnvelope := httpx.Envelope{
		"status":           booking.Status,
		"date":             booking.Date.Format("2006-01-02"),
		"start_time":       booking.StartTime,
		"starts_at":        booking.StartsAt.Format(time.RFC3339),
		"ends_at":          booking.EndsAt.Format(time.RFC3339),
		"duration_minutes": booking.DurationMinutes,
		"court_name":       court.Name,
		"sport":            court.Sport,
		"court_type":       court.CourtType,
		"complex_name":     complex.Name,
	}
	if complex.Address != "" {
		bookingEnvelope["complex_address"] = complex.Address
	}

	return httpx.Envelope{
		"booking":    bookingEnvelope,
		"can_cancel": info.canCancel,
		"can_refund": canRefund,
		// How it would come back: "mercadopago" is automatic, "manual" means the
		// complex has to hand it over, "none" means there is nothing to return.
		"refund_method":      method,
		"cancellation_hours": complex.CancellationHours,
		// refund_amount is what an automatic MercadoPago refund would return if
		// the client cancels right now, computed row by row the same way the
		// automatic refund itself does.
		"refund_amount": refundAmount,
		// paid_amount is what has actually been paid so far, regardless of
		// whether any of it comes back — so the page can say what was paid even
		// when cancelling now returns nothing.
		"paid_amount": paidAmount,
	}
}

// cancelPreviewAmounts computes, from the booking's payment rows, the two
// money figures the cancel page needs: paidAmount (what has actually been
// paid so far, any method) and refundAmount (what an automatic MercadoPago
// refund would return if the client cancels right now).
//
// refundAmount mirrors internal/payments/refund.go's own row selection —
// owed := Amount + ServiceFee - RefundAmount over MercadoPago rows — so this
// preview can never promise more than the automatic refund path can pay.
// canRefund false or method "none" means nothing is owed automatically
// (either the window has closed or the booking still isn't paid), so
// refundAmount is 0 without needing to look at the rows at all.
func (h *Handler) cancelPreviewAmounts(ctx context.Context, booking *data.Booking, canRefund bool, method string) (refundAmount, paidAmount int) {
	payments, err := h.payments.ListByBookingID(ctx, booking.ID)
	if err != nil {
		// An unreadable ledger has nothing to compute from; refundMethod already
		// fell back to refundByHand for the same reason.
		return 0, 0
	}

	for _, payment := range payments {
		if payment.Status == "unpaid" {
			continue
		}
		paidAmount += payment.Amount + payment.ServiceFee
	}

	if !canRefund || method == refundNotApplicable {
		return 0, paidAmount
	}

	for _, payment := range payments {
		if payment.Status != "deposit_paid" && payment.Status != "fully_paid" {
			continue
		}
		if payment.MPPaymentID == nil || *payment.MPPaymentID == "" {
			continue
		}
		refundAmount += payment.Amount + payment.ServiceFee - payment.RefundAmount
	}

	return refundAmount, paidAmount
}

// PublicCancel handles POST /api/v1/book/cancel.
//
// Outside the refund window the booking is still cancelled — a client should
// not have to show up — but the deposit is not returned.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) PublicCancel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Token != "", "token", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	booking, complex, ok := h.resolveLink(w, r, input.Token)
	if !ok {
		return
	}

	if booking.Status == "cancelled" || booking.Status == "completed" || booking.Status == "no_show" {
		h.respond.Error(w, r, http.StatusBadRequest, "the booking has already been cancelled or completed")
		return
	}

	// Determine if the cancellation qualifies for a refund (standard window or grace period).
	withinRefundWindow := pricing.CanRefund(booking, complex.CancellationHours, h.cfg.GracePeriod)

	// owesRefund feeds both the marker write below and the refund dispatch in
	// the switch further down — one expression, not two, so the switch's
	// default (RefundNotEligible) branch is exactly !owesRefund, and an edit
	// that marks a declined cancellation is, by construction, the same edit
	// that refunds it (refund-intent-durability spec's load-bearing property).
	owesRefund := booking.CollectionStatus != data.CollectionStatusUnpaid && withinRefundWindow

	// Cancel the booking.
	booking.Status = "cancelled"
	cancelNote := "Cancelado por el cliente"
	if !withinRefundWindow {
		cancelNote = "Cancelado por el cliente (fuera de plazo, sin reembolso)"
	}
	if booking.Notes != nil {
		cancelNote = *booking.Notes + " | " + cancelNote
	}
	booking.Notes = &cancelNote

	if owesRefund {
		now := time.Now()
		booking.RefundIntentAt = &now
	}

	err = h.store.Update(r.Context(), booking)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.realtime.PublishBookingChanged(booking.ComplexID)

	// Recorded at the same point the staff path records its own cancellation
	// (actions.go): after the update commits, before any money moves. The
	// refund decision travels with it because it was made above, in this
	// request, out of the complex's window — and an out-of-window cancellation
	// never reaches internal/payments at all, so if this entry did not say the
	// deposit was kept, nothing would.
	h.record(r, booking.ComplexID, "public_cancel", &booking.ID, publicCancellation{
		Actor:              publicActor,
		ClientID:           booking.ClientID,
		Booking:            booking,
		WithinRefundWindow: withinRefundWindow,
		OwesRefund:         owesRefund,
	})

	// The refund window governs the refund and nothing else.
	//
	// It used to wrap this whole block, so an out-of-window cancellation skipped
	// expiring the MercadoPago preference as well — and expiring the preference is
	// not a refund policy, it is closing the checkout link this booking still
	// holds. The slot was freed, somebody else booked and paid for it, and the
	// first client's link was still live: they could pay, get charged, and be
	// auto-refunded with no message, while the venue lost the late-cancellation
	// penalty the window exists to collect.
	//
	// The staff path deliberately has no window at all. That difference stays.
	var outcome data.RefundOutcome
	switch {
	case booking.CollectionStatus == data.CollectionStatusUnpaid:
		h.expireCheckoutPreference(r.Context(), booking, complex)
		outcome = data.RefundOutcome{Result: data.RefundNone, Reason: "the booking was never paid"}
	case owesRefund:
		outcome = h.refunds.AutoRefundIfPaid(r.Context(), booking)
	default:
		outcome = data.RefundOutcome{
			Result:         data.RefundNotEligible,
			AmountCentavos: booking.DepositAmount,
			Reason:         "cancelled outside the complex's refund window",
		}
	}

	// Send cancellation notification (email + optionally WhatsApp).
	court, courterr := h.courts.GetByID(r.Context(), booking.CourtID)
	client, cerr := h.clients.GetByID(r.Context(), booking.ClientID)
	if cerr == nil && courterr == nil {
		clientEmail := ""
		if client.Email != nil {
			clientEmail = *client.Email
		}
		// The refund decision made above travels with the message. It used to
		// stop here: the client was told their booking was off and nothing
		// about their deposit, and found out about the money — or did not —
		// from a separate email that only fires when one is actually sent.
		refundLine, refundAmount := refundNotice(outcome)
		h.notify.BookingCancelled(notifications.Cancellation{
			Email:        clientEmail,
			Phone:        client.Phone,
			ComplexName:  complex.Name,
			CourtName:    court.Name,
			Date:         booking.Date.Format("02/01"),
			StartTime:    timezone.HoursLabel(booking.StartsAt, booking.EndsAt),
			RefundLine:   refundLine,
			RefundAmount: refundAmount,
			BookPath:     booklink.BookPath(complex.Slug),
			BookURL:      booklink.Book(h.cfg.FrontendURL, complex.Slug),
		})
	}

	// The slot is free now; the checkout lock that was holding it is not.
	if courterr == nil {
		h.releaseHeldSlots(r.Context(), booking)
	}

	// ClaimRefund clears the marker in the database but cannot reach this
	// in-memory *Booking, so it is cleared here unconditionally: the field was
	// either never set, cleared inside a committed ClaimRefund, or cleared by
	// AutoRefundIfPaid's other exits — nil is correct in every case.
	booking.RefundIntentAt = nil

	// Return minimal info — don't expose internal booking details to public endpoint.
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"booking": httpx.Envelope{
			"status":            booking.Status,
			"collection_status": booking.CollectionStatus,
			"refund_status":     refundStatusAfter(booking, outcome),
		},
		// Kept for the clients already reading it, and now true when the money
		// actually came back rather than never. It stays true even when
		// "refund_status" above reads "partial": the automatic half genuinely
		// came back, and "manual_amount" in "refund" below is what says a
		// cash/transfer balance is still owed.
		"refunded": outcome.MoneyReturned(),
		"refund":   refundEnvelope(outcome),
	})
}

// createMPPreferenceWithRetry creates a MercadoPago preference, retrying once
// with a token refresh if the seller's access token has expired.
func (h *Handler) createMPPreferenceWithRetry(ctx context.Context, prefInput mp.CreatePreferenceInput, complex *data.Complex) (*mp.Preference, error) {
	pref, err := h.checkout.CreatePreference(ctx, prefInput)

	refreshTok, refreshTokErr := complex.SellerRefreshToken()
	if err != nil && mp.IsUnauthorized(err) && refreshTokErr == nil {
		h.logger.Info("public booking: seller token expired, refreshing", "complex_id", complex.ID)
		newTokens, refreshErr := h.checkout.RefreshOAuthToken(ctx, refreshTok)
		if refreshErr == nil {
			mpUserID := fmt.Sprintf("%d", newTokens.UserID)
			if credErr := h.complexes.UpdateMPCredentials(ctx, complex.ID, newTokens.AccessToken, newTokens.RefreshToken, mpUserID, newTokens.ExpiresIn); credErr != nil {
				h.logger.Error("public booking: failed to persist refreshed MP credentials", "error", credErr, "complex_id", complex.ID)
				sentry.CaptureMessage(fmt.Sprintf("MP OAuth refreshed-credential persist FAILED (checkout retry): complex_id=%s error=%v", complex.ID, credErr))
			}
			refreshedSeller, callerErr := mp.AsSeller(newTokens.AccessToken)
			if callerErr != nil {
				// MercadoPago answered the refresh without an access token.
				// Retrying without one would have meant retrying as the
				// platform, so there is nothing left to retry with: the
				// original 401 is the answer.
				h.logger.Error("public booking: the refreshed seller token is unusable, keeping the original failure",
					"error", callerErr, "complex_id", complex.ID)
				return pref, err
			}
			prefInput.Caller = refreshedSeller
			pref, err = h.checkout.CreatePreference(ctx, prefInput)
		} else {
			h.logger.Error("public booking: failed to refresh seller token", "error", refreshErr, "complex_id", complex.ID)
		}
	}

	return pref, err
}

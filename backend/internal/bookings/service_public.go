// service_public.go — the rules behind the four routes a client with no
// account reaches: booking a court, reading a booking's status, previewing a
// cancellation and making one.
package bookings

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
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
// encode — LinkToken is `json:"-"` (internal/bookings/store/bookings.go), which is what
// keeps the access token that authorizes the three public routes out of the
// trail, the same way complexstore.Complex's tags keep MercadoPago credentials out of
// it. TestPublicBookAuditEntryCarriesNoLinkToken pins that.
type publicBooking struct {
	Actor    string                `json:"actor"`
	ClientID uuid.UUID             `json:"client_id,omitzero"`
	Booking  *bookingstore.Booking `json:"booking"`
}

// publicCancellation is the audit value for a cancellation a client made
// without an account.
//
// It records the refund window decision alongside the booking because that
// decision is this path's alone. The staff path deliberately ignores the
// complex's window (see Service.Cancel), so there is nothing to record there;
// here, whether the client's deposit comes back is decided in this request,
// from a policy value read off the complex, and nothing else in the system
// writes down that it was applied. What then became of the money is
// internal/payments' entry to write, not this one's.
type publicCancellation struct {
	Actor              string                `json:"actor"`
	ClientID           uuid.UUID             `json:"client_id,omitzero"`
	Booking            *bookingstore.Booking `json:"booking"`
	WithinRefundWindow bool                  `json:"within_refund_window"`
	OwesRefund         bool                  `json:"owes_refund"`
}

// PublicBookInput is a validated request to book a court from the public page.
type PublicBookInput struct {
	ComplexID uuid.UUID
	CourtID   uuid.UUID
	Date      time.Time
	// RawDate is the date exactly as the client sent it, which is what the
	// MercadoPago preference quotes back to them.
	RawDate         string
	StartTime       string
	DurationMinutes int
	ClientFirstName string
	ClientLastName  string
	ClientPhone     string
	ClientEmail     string
	ClientNotes     string
	// HoldToken is the slot hold the visitor took when they picked the court.
	// Empty is the pre-hold flow, which a storefront still running that code
	// keeps using during a deploy.
	HoldToken string
}

// slot is the part of the request that names the hours.
func (in PublicBookInput) slot() publicSlotRequest {
	return publicSlotRequest{
		ComplexID: in.ComplexID, CourtID: in.CourtID, Date: in.Date,
		StartTime: in.StartTime, DurationMinutes: in.DurationMinutes,
	}
}

// PublicBookResult is a booking held for a client while they pay for it.
type PublicBookResult struct {
	Booking *bookingstore.Booking
	Complex *complexstore.Complex
	Court   *courtstore.Court
	// Preference is nil when MercadoPago could not be reached; in that case the
	// booking has already been cancelled again and the slot freed.
	Preference *mp.Preference
	// ServiceFee and TotalClientPays are what the checkout was created for.
	ServiceFee      int
	TotalClientPays int
}

// ErrCheckoutUnavailable reports that MercadoPago would not create the checkout
// link. The booking this request created has already been cancelled again and
// its slot released, so the client may simply try again.
var ErrCheckoutUnavailable = errors.New("bookings: the checkout link could not be created")

// ErrMercadoPagoNotConnected reports a venue that cannot take money: it has no
// usable seller credential, so there is no party to create the checkout as.
var ErrMercadoPagoNotConnected = errors.New("bookings: the complex has no usable MercadoPago credential")

// ErrClientBlocked reports a client the venue has blocked.
var ErrClientBlocked = errors.New("bookings: the client is blocked")

// ErrSlotTaken reports a slot somebody else holds, whether by a lock or by a
// committed booking.
var ErrSlotTaken = errors.New("bookings: the selected time slot is no longer available")

// checkoutCleanupBudget bounds the detached writes a failed checkout makes: the
// cancellation of its booking and the release of its slot lock. Each is one
// statement and fits well inside it. The bound is needed because
// context.WithoutCancel strips the caller's deadline as well as its cancellation,
// and this repository sets no statement timeout.
const checkoutCleanupBudget = 5 * time.Second

// PublicBook creates the pending storefront booking and its MercadoPago
// checkout. The booking is only confirmed when the payment webhook lands.
//
// With a hold token it converts the slot hold the visitor took when they
// picked the court (bookFromHold, hold.go). Without one it is the flow the
// storefront used before holds existed, kept unchanged for a storefront still
// running that code during a deploy: it checks the slot, holds it under a slot
// lock, inserts the booking and creates the checkout. If anything fails after
// the slot is held it is released, or it stays locked until the TTL expires
// and nobody can book it.
//
// It is one cohesive request lifecycle — check, hold the slot, insert, create
// the checkout — and splitting it further would spread a single
// transaction-like sequence across helpers.
//
//nolint:funlen // see the cohesion note above
func (s *Service) PublicBook(ctx context.Context, actor Actor, in PublicBookInput) (PublicBookResult, error) {
	if in.HoldToken != "" {
		return s.bookFromHold(ctx, actor, in)
	}

	complex, court, err := s.publicVenue(ctx, in.slot())
	if err != nil {
		return PublicBookResult{}, err
	}

	// No client is created here (owner rule 2026-11-10: a client exists only
	// once a booking succeeds). The booking is held under a snapshot of who it
	// is for, and the transaction that records its payment gets or creates the
	// client (bookingstore.AttachClientFromSnapshot). The blocked rule still
	// holds, read by phone without writing anything: a phone the venue blocked
	// is refused, an unknown one carries on.
	if err := s.refuseBlockedPhone(ctx, complex.ID, in.ClientPhone); err != nil {
		return PublicBookResult{}, err
	}

	offer, err := s.publicOffer(ctx, complex, court, in.slot())
	if err != nil {
		return PublicBookResult{}, err
	}

	// The slot lock is keyed on a date and two times of day, so it still needs
	// the clock reading the booking's hours end on. slots.Add wraps at
	// midnight, which is right for slot_locks — a lock is asked for and
	// released by the same pair of strings and never compared as an interval —
	// and is exactly what was wrong about storing it on the booking, which
	// no longer stores.
	lockEndTime := slots.Add(in.StartTime, in.DurationMinutes)

	booking := &bookingstore.Booking{
		ComplexID: complex.ID,
		CourtID:   in.CourtID,
		ClientSnapshot: &bookingstore.ClientSnapshot{
			FirstName: in.ClientFirstName,
			LastName:  in.ClientLastName,
			Phone:     in.ClientPhone,
			Email:     in.ClientEmail,
		},
		Date:             offer.startDate,
		StartTime:        in.StartTime,
		DurationMinutes:  in.DurationMinutes,
		Price:            offer.price,
		DepositAmount:    offer.deposit,
		Status:           "pending",
		CollectionStatus: bookingstore.CollectionStatusUnpaid,
		RefundStatus:     bookingstore.RefundStatusNone,
	}

	if in.ClientNotes != "" {
		booking.Notes = &in.ClientNotes
	}

	// Acquire a slot lock over the whole booking span to prevent the race
	// condition where another user books the same slot while this user is
	// completing the MP payment (~15 min window).
	//
	// This used to be one lock per fixed-size chunk, one per consecutive slot
	// the booking spanned. Now that a booking's length is chosen per request
	// rather than assembled from repeats of the court's own slot length, there
	// is one span and one lock for it.
	//
	// holder names this checkout's lock. Every release this checkout makes names
	// it too, so a lock whose TTL ran out and was taken over by another checkout
	// is never freed from here.
	holder := uuid.NewString()
	if lockErr := s.locks.AcquireLock(ctx, in.CourtID, offer.startDate, in.StartTime, lockEndTime, nil, holder, s.cfg.SlotLockTTL); lockErr != nil {
		if errors.Is(lockErr, bookingstore.ErrSlotLocked) {
			return PublicBookResult{}, ErrSlotTaken
		}
		return PublicBookResult{}, lockErr
	}

	if err := s.store.InsertSafe(ctx, booking); err != nil {
		// Release the slot lock on insert failure.
		s.releaseCheckoutLock(ctx, in.CourtID, offer.startDate, in.StartTime, holder)
		if errors.Is(err, bookingstore.ErrDuplicateBooking) || errors.Is(err, bookingstore.ErrSlotUnavailable) {
			return PublicBookResult{}, ErrSlotTaken
		}
		// H-22: the venue deleted the court between the GetByID above and this
		// commit. Not a slot conflict — the slot message would tell the visitor
		// to pick another hour on a court that no longer exists — and not a
		// 500 either. The same NotFound the check above answers.
		return PublicBookResult{}, err
	}

	s.realtime.PublishBookingChanged(complex.ID)
	s.recordPublicBook(actor, booking)

	result, err := s.openCheckout(ctx, checkoutRequest{
		booking: booking, complex: complex, court: court, seller: offer.seller,
		rawDate: in.RawDate, expiresIn: s.cfg.PaymentExpiry, payer: in,
	})
	if err != nil {
		s.releaseCheckoutLock(ctx, in.CourtID, offer.startDate, in.StartTime, holder)
		return PublicBookResult{}, err
	}
	return result, nil
}

// releaseCheckoutLock frees the slot lock this checkout took, and only while the
// lock still names this checkout's holder token. A plain release would free the
// lock of whichever checkout took the slot over after this one's TTL ran out.
// Errors are logged rather than returned: the lock's own TTL is the backstop,
// as it is for releaseSlotLock.
//
// Detached from the caller's cancellation, for the reason checkoutCleanupBudget
// gives: every call site is a failure path, and the client may have left.
func (s *Service) releaseCheckoutLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, holder string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), checkoutCleanupBudget)
	defer cancel()

	if relErr := s.locks.ReleaseHeldLock(ctx, courtID, date, startTime, holder); relErr != nil {
		s.logger.Error("failed to release checkout slot lock", "error", relErr, "court_id", courtID, "slot_start", startTime)
	}
}

// createMPPreferenceWithRetry creates a MercadoPago preference, retrying once
// with a token refresh if the seller's access token has expired.
func (s *Service) createMPPreferenceWithRetry(ctx context.Context, prefInput mp.CreatePreferenceInput, complex *complexstore.Complex) (*mp.Preference, error) {
	pref, err := s.checkout.CreatePreference(ctx, prefInput)
	if err == nil || !mp.IsUnauthorized(err) {
		return pref, err
	}

	// An unreadable access token leaves the stale value empty, which makes the
	// refresh unconditional rather than skipped.
	staleAccess, accessErr := complex.SellerAccessToken()
	if accessErr != nil {
		staleAccess = ""
	}

	s.logger.Info("public booking: seller token expired, refreshing", "complex_id", complex.ID)
	accessToken, refreshErr := s.complexes.RefreshMPCredentials(ctx, complex.ID, staleAccess)
	if refreshErr != nil {
		if !errors.Is(refreshErr, mpcred.ErrMPRefreshNotPersisted) {
			s.logger.Error("public booking: failed to refresh seller token", "error", refreshErr, "complex_id", complex.ID)
			return pref, err
		}
		// MercadoPago did issue new tokens; only storing them failed, and the
		// refresh has already alerted. The access token that came back is good
		// for this request, so the client is not failed for a write they have
		// no part in.
		s.logger.Error("public booking: refreshed MP credentials were not stored, continuing with the new access token for this request",
			"error", refreshErr, "complex_id", complex.ID)
	}

	refreshedSeller, callerErr := mp.AsSeller(accessToken)
	if callerErr != nil {
		// MercadoPago answered the refresh without an access token.
		// Retrying without one would have meant retrying as the
		// platform, so there is nothing left to retry with: the
		// original 401 is the answer.
		s.logger.Error("public booking: the refreshed seller token is unusable, keeping the original failure",
			"error", callerErr, "complex_id", complex.ID)
		return pref, err
	}
	prefInput.Caller = refreshedSeller
	return s.checkout.CreatePreference(ctx, prefInput)
}

// ResolveLink is the whole authorization for the three public routes
// (specs/booking-link-credential): it resolves the token and loads the complex
// pricing.LinkLive needs. Each caller keeps its own absent/malformed shape
// check before calling this — ResolveLink never sees an empty token.
//
// It returns the complex it had to load, so the cancel routes need no separate
// complexes.GetByID call of their own.
func (s *Service) ResolveLink(ctx context.Context, token string) (*bookingstore.Booking, *complexstore.Complex, error) {
	booking, expiresAt, err := s.linkResolver.ResolveBooking(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	// H-18: the booking resolved above can be entirely legitimate while the
	// venue it belongs to has been soft-deleted since — GetByID reads
	// active_complexes, which excludes it. That used to fall straight into a
	// 500 with a body the client's retry button could never get past; it is not
	// this request's fault, and it can never succeed by trying again.
	complex, err := s.complexes.GetByID(ctx, booking.ComplexID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, nil, ErrVenueGone
		}
		return nil, nil, err
	}

	if !pricing.LinkLive(booking, expiresAt, complex.CancellationHours, s.cfg.GracePeriod, time.Now()) {
		return nil, nil, ErrLinkExpired
	}

	return booking, complex, nil
}

// courtOrGone loads a booking's court for the two read-only public routes,
// answering ErrVenueGone instead of a raw failure when the court has been
// soft-deleted — directly (an owner retiring one court) or by the cascade from
// a soft-deleted complex, which ResolveLink above catches at the complex
// itself; this is the same failure one join further in, for the case where the
// venue is still open but this particular court is not.
func (s *Service) courtOrGone(ctx context.Context, courtID uuid.UUID) (*courtstore.Court, error) {
	court, err := s.courts.GetByID(ctx, courtID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrVenueGone
		}
		return nil, err
	}
	return court, nil
}

// Cancellation is what a client can do about their booking right now —
// computed once and read by both PublicStatus and PublicCancelInfo, so the two
// can never disagree about whether a booking can still be cancelled or when its
// refund window closes.
type Cancellation struct {
	// CanCancel mirrors the guard PublicCancel itself enforces: a booking
	// already cancelled, completed or a no-show cannot be cancelled again.
	CanCancel bool
	// WithinWindow is true only when CanCancel is also true — a terminal
	// booking has no window left to be inside of, regardless of what the
	// dates alone would say.
	WithinWindow bool
	// Deadline is the instant pricing.CanRefund stops being true, from
	// pricing.RefundDeadline — meaningful only when CanCancel is true.
	Deadline time.Time
}

// cancellationInfo computes what PublicStatus and PublicCancelInfo both tell a
// client about cancelling their booking right now.
func (s *Service) cancellationInfo(booking *bookingstore.Booking, complex *complexstore.Complex) Cancellation {
	canCancel := booking.Status != "cancelled" && booking.Status != "completed" && booking.Status != "no_show"
	return Cancellation{
		CanCancel:    canCancel,
		WithinWindow: canCancel && pricing.CanRefund(booking, complex.CancellationHours, s.cfg.GracePeriod),
		Deadline:     pricing.RefundDeadline(booking, complex.CancellationHours, s.cfg.GracePeriod),
	}
}

// StatusView is everything the public success page renders. A client who opens
// the MercadoPago redirect in a different browser, or opens the link later, has
// no cached copy of the booking to fall back on, so it carries the whole
// picture rather than just the two status fields.
type StatusView struct {
	Booking *bookingstore.Booking
	Complex *complexstore.Complex
	Court   *courtstore.Court
	// Payment is nil when the booking carries no payment row yet.
	Payment      *paymentstore.Payment
	Cancellation Cancellation
}

// PublicStatus answers a booking link with the booking's current state.
func (s *Service) PublicStatus(ctx context.Context, token string) (StatusView, error) {
	booking, complex, err := s.ResolveLink(ctx, token)
	if err != nil {
		return StatusView{}, err
	}

	court, err := s.courtOrGone(ctx, booking.CourtID)
	if err != nil {
		return StatusView{}, err
	}

	payment, err := s.payments.GetByBookingID(ctx, booking.ID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		return StatusView{}, err
	}

	return StatusView{
		Booking:      booking,
		Complex:      complex,
		Court:        court,
		Payment:      payment,
		Cancellation: s.cancellationInfo(booking, complex),
	}, nil
}

// CancelInfoView is everything the cancel page renders: the same booking detail
// the success page shows, plus the money actually at stake if the client
// cancels right now.
type CancelInfoView struct {
	Booking      *bookingstore.Booking
	Complex      *complexstore.Complex
	Court        *courtstore.Court
	Cancellation Cancellation
	// RefundMethod is how the money would come back: "mercadopago" is
	// automatic, "manual" means the complex has to hand it over, "none" means
	// there is nothing to return.
	RefundMethod string
	CanRefund    bool
	// RefundAmount is what an automatic MercadoPago refund would return if the
	// client cancels right now, computed row by row the same way the automatic
	// refund itself does.
	RefundAmount int
	// PaidAmount is what has actually been paid so far, regardless of whether
	// any of it comes back — so the page can say what was paid even when
	// cancelling now returns nothing.
	PaidAmount int
}

// PublicCancelInfo tells a client whether cancelling now would return their
// deposit, and how.
//
// The window decides whether the money is owed. The payment decides whether
// anything can return it. Answering from the window alone is what let this
// promise an automatic refund on a booking paid in cash — after which the
// booking was cancelled, the refund path declined without a word, and the only
// record that the cash was owed was the client's memory.
func (s *Service) PublicCancelInfo(ctx context.Context, token string) (CancelInfoView, error) {
	booking, complex, err := s.ResolveLink(ctx, token)
	if err != nil {
		return CancelInfoView{}, err
	}

	court, err := s.courtOrGone(ctx, booking.CourtID)
	if err != nil {
		return CancelInfoView{}, err
	}

	info := s.cancellationInfo(booking, complex)
	method := s.refundMethod(ctx, booking)
	canRefund := info.WithinWindow && method != refundNotApplicable
	refundAmount, paidAmount := s.cancelPreviewAmounts(ctx, booking, canRefund, method)

	return CancelInfoView{
		Booking:      booking,
		Complex:      complex,
		Court:        court,
		Cancellation: info,
		RefundMethod: method,
		CanRefund:    canRefund,
		RefundAmount: refundAmount,
		PaidAmount:   paidAmount,
	}, nil
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
func (s *Service) cancelPreviewAmounts(ctx context.Context, booking *bookingstore.Booking, canRefund bool, method string) (refundAmount, paidAmount int) {
	payments, err := s.payments.ListByBookingID(ctx, booking.ID)
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

// PublicCancelResult is what a client's own cancellation produced.
type PublicCancelResult struct {
	Booking *bookingstore.Booking
	Outcome paymentstore.RefundOutcome
}

// PublicCancel cancels a booking from the link the client holds.
//
// Outside the refund window the booking is still cancelled — a client should
// not have to show up — but the deposit is not returned.
//
// It is one cohesive state transition — cancel, refund, notify, release — and
// splitting it would relocate sequential steps into helpers without reducing
// what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) PublicCancel(ctx context.Context, actor Actor, token string) (PublicCancelResult, error) {
	booking, complex, err := s.ResolveLink(ctx, token)
	if err != nil {
		return PublicCancelResult{}, err
	}

	if booking.Status == "cancelled" || booking.Status == "completed" || booking.Status == "no_show" {
		return PublicCancelResult{}, &StateError{Message: "the booking has already been cancelled or completed"}
	}

	// Determine if the cancellation qualifies for a refund (standard window or grace period).
	withinRefundWindow := pricing.CanRefund(booking, complex.CancellationHours, s.cfg.GracePeriod)

	// owesRefund feeds both the marker write below and the refund dispatch in
	// the switch further down — one expression, not two, so the switch's
	// default (RefundNotEligible) branch is exactly !owesRefund, and an edit
	// that marks a declined cancellation is, by construction, the same edit
	// that refunds it (refund-intent-durability spec's load-bearing property).
	owesRefund := booking.CollectionStatus != bookingstore.CollectionStatusUnpaid && withinRefundWindow

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

	if err := s.store.Update(ctx, booking); err != nil {
		return PublicCancelResult{}, err
	}

	s.realtime.PublishBookingChanged(booking.ComplexID)

	// Recorded at the same point the staff path records its own cancellation
	// (Service.Cancel): after the update commits, before any money moves. The
	// refund decision travels with it because it was made above, in this
	// request, out of the complex's window — and an out-of-window cancellation
	// never reaches internal/payments at all, so if this entry did not say the
	// deposit was kept, nothing would.
	s.record(actor, booking.ComplexID, "public_cancel", &booking.ID, publicCancellation{
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
	var outcome paymentstore.RefundOutcome
	switch {
	case booking.CollectionStatus == bookingstore.CollectionStatusUnpaid:
		s.expireCheckoutPreference(ctx, booking, complex)
		outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "the booking was never paid"}
	case owesRefund:
		outcome = s.refunds.AutoRefundIfPaid(ctx, booking)
	default:
		outcome = paymentstore.RefundOutcome{
			Result:         paymentstore.RefundNotEligible,
			AmountCentavos: booking.DepositAmount,
			Reason:         "cancelled outside the complex's refund window",
		}
	}

	s.notifyCancelledAndRelease(ctx, booking, complex, outcome, notifications.SourceClientCancel)

	// ClaimRefund clears the marker in the database but cannot reach this
	// in-memory *Booking, so it is cleared here unconditionally: the field was
	// either never set, cleared inside a committed ClaimRefund, or cleared by
	// AutoRefundIfPaid's other exits — nil is correct in every case.
	booking.RefundIntentAt = nil

	return PublicCancelResult{Booking: booking, Outcome: outcome}, nil
}

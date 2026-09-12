// cron.go — the four scheduled sweeps of the booking domain.
//
// They used to live in cmd/api/cron.go, written directly against the stores,
// which made that file a second place booking rules were decided: the expiry
// sweep carried its own copy of the preference-expiry code the cancel path
// already had, and the two had already drifted. cmd/api now keeps only the
// schedule — one line per job — and the rules are here, beside the request
// paths that share them.
package bookings

import (
	"context"
	"fmt"
	"time"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Reminder2h sends the two-hour reminder (email, and WhatsApp when it is
// configured). Reminders are informational only — bookings are never
// auto-cancelled for lack of confirmation.
func (s *Service) Reminder2h(ctx context.Context) {
	// time.Now() and not timezone.Now(): the store compares instants, so the
	// location this value carries changes nothing. Argentina enters the
	// calculation once, inside booking_starts_at, where the stored date and
	// start_time are read as local wall clock.
	bookings, err := s.store.GetForReminder2hEnriched(ctx, time.Now())
	if err != nil {
		s.logger.Error("cron_reminder_2h: failed to get bookings", "error", err)
		return
	}

	sent := 0
	for _, b := range bookings {
		if err := s.store.MarkReminderSent2h(ctx, b.ID); err != nil {
			s.logger.Error("cron_reminder_2h: failed to mark sent", "error", err, "booking_id", b.ID)
			continue
		}

		// A fresh access token per reminder, rather than the one the booking
		// was confirmed with: booking_link_tokens stores only a hash, so the
		// plaintext minted at confirmation is unreadable from here — see
		// booklinkstore.Store.Mint on why every process that emits a link
		// mints its own row. A mint that fails costs the WhatsApp cancel
		// button and nothing else: the reminder still goes out, by email and
		// without the button, rather than not at all.
		cancelPath, cancelURL := "", ""
		token, mintErr := s.linkTokens.Mint(ctx, b.ID, b.EndsAt.Add(s.cfg.LinkTokenBuffer))
		if mintErr != nil {
			s.logger.Error("cron_reminder_2h: failed to mint a cancel link, sending the reminder without one",
				"error", mintErr, "booking_id", b.ID)
		} else {
			cancelPath = booklink.CancelPath(b.ComplexSlug, token)
			cancelURL = booklink.Cancel(s.cfg.FrontendURL, b.ComplexSlug, token)
		}

		s.notify.ReminderDue(notifications.Reminder{
			Email:       b.ClientEmail,
			Phone:       b.ClientPhone,
			ComplexName: b.ComplexName,
			CourtName:   b.CourtName,
			Date:        b.Date.Format("02/01"),
			StartTime:   timezone.HoursLabel(b.StartsAt, b.EndsAt),
			Address:     booklink.Address(b.ComplexAddress, b.ComplexCity),
			// What is left to pay at the venue. Two hours out this is the fact
			// a client acts on, and the confirmation that carried it went out
			// days ago.
			BalanceAmount: notifications.BalanceAmount(b.Price, b.DepositAmount, b.CollectionStatus),
			MapsQuery:     booklink.MapsQuery(b.ComplexName, b.ComplexAddress, b.ComplexCity, b.ComplexLatitude, b.ComplexLongitude),
			MapsURL:       booklink.MapsURL(b.ComplexName, b.ComplexAddress, b.ComplexCity, b.ComplexLatitude, b.ComplexLongitude),
			CancelPath:    cancelPath,
			CancelURL:     cancelURL,
		})

		s.logger.Info("cron_reminder_2h: sent reminder", "booking_id", b.ID)
		sent++
	}

	s.logger.Info("cron_reminder_2h: completed", "candidates", len(bookings), "sent", sent)
}

// ReleaseExpiredPayments cancels public bookings that are still unpaid past the
// payment window, freeing the slot they were holding.
//
// The checkout link is closed first, through the same helper the two
// cancellation paths use, so the client cannot pay for hours this sweep is
// about to put back on sale. That sharing is the point: this job used to carry
// its own copy of the expiry code, and the copy had already drifted — one
// attempt against the cancel path's two, and a credential it could not decrypt
// quietly became a call made as the platform.
func (s *Service) ReleaseExpiredPayments(ctx context.Context) {
	bookings, err := s.store.GetExpiredPendingEnriched(ctx, s.cfg.PaymentExpiry)
	if err != nil {
		s.logger.Error("cron_release_expired_payments: failed to get bookings", "error", err)
		return
	}

	released := 0
	for _, b := range bookings {
		// Expire the MP preference FIRST so the client can't pay while we cancel.
		s.expireCheckoutPreference(ctx, &b.Booking, b)

		// Cancel booking after preference is expired.
		b.Status = "cancelled"
		notes := fmt.Sprintf("Cancelado automáticamente: tiempo de pago expirado (%v)", s.cfg.PaymentExpiry)
		if b.Notes != nil {
			notes = *b.Notes + " | " + notes
		}
		b.Notes = &notes

		if err := s.store.Update(ctx, &b.Booking); err != nil {
			s.logger.Error("cron_release_expired_payments: failed to cancel booking", "error", err, "booking_id", b.ID)
			continue
		}

		s.realtime.PublishBookingChanged(b.ComplexID)

		s.notify.BookingCancelled(notifications.Cancellation{
			Email:       b.ClientEmail,
			Phone:       b.ClientPhone,
			ComplexName: b.ComplexName,
			CourtName:   b.CourtName,
			Date:        b.Date.Format("02/01"),
			StartTime:   timezone.HoursLabel(b.StartsAt, b.EndsAt),
			// This booking was never paid, so none of the six refund outcomes
			// applies and there is no amount. The client's question here is
			// why their booking vanished, which the generic cancellation copy
			// never answered.
			RefundLine: notifications.ExpiredUnpaidRefundLine,
			BookPath:   booklink.BookPath(b.ComplexSlug),
			BookURL:    booklink.Book(s.cfg.FrontendURL, b.ComplexSlug),
		})

		s.logger.Info("cron_release_expired_payments: released expired booking", "booking_id", b.ID)
		released++
	}

	s.logger.Info("cron_release_expired_payments: completed", "candidates", len(bookings), "released", released)
}

// CompletePastBookings marks confirmed bookings whose hours have passed as
// completed.
func (s *Service) CompletePastBookings(ctx context.Context) {
	count, err := s.store.CompletePastBookings(ctx)
	if err != nil {
		s.logger.Error("cron_complete_bookings: failed", "error", err)
		return
	}
	s.logger.Info("cron_complete_bookings: completed", "count", count)
}

// LinkTokenRetention is how long past its stored expiry a terminal booking's
// link token is kept before CleanLinkTokens deletes it. Purely housekeeping —
// DeleteExpiredTerminal's terminal-status predicate already guarantees a live
// booking's token is never touched, so this window exists only to keep the
// table from growing without bound, not to protect a link still in use.
const LinkTokenRetention = 24 * time.Hour

// CleanLinkTokens deletes booking link tokens whose booking has reached a
// terminal state and whose stored expiry is safely in the past. See
// booklinkstore.Store.DeleteExpiredTerminal: a pending or confirmed booking's
// token is never touched here, however old, so a sweep can never turn a live
// link into a 404.
func (s *Service) CleanLinkTokens(ctx context.Context) {
	if err := s.linkTokens.DeleteExpiredTerminal(ctx, LinkTokenRetention); err != nil {
		s.logger.Error("cron_clean_booking_link_tokens: failed", "error", err)
		return
	}
	s.logger.Info("cron_clean_booking_link_tokens: completed")
}

// cronBookingCredentials is the compile-time check that the enriched booking
// the expiry sweep reads can stand in for a complex wherever a seller
// credential is all that is needed.
var _ sellerCredentials = (*bookingstore.CronBooking)(nil)

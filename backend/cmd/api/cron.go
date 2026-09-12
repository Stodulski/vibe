package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/scheduler"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// startCronJobs hands the recurring jobs to the scheduler, which owns the
// intervals and the cross-instance locking.
//
// Every job is wrapped in observed, so a healthy deployment emits a steady
// heartbeat rather than the two lines a day it used to. That was the whole
// diagnostic problem: most jobs logged only when they had done something
// (`if count > 0`), one logged nothing ever, and an instance that could not
// take the advisory lock returned without a line at all — so a completely dead
// scheduler and a healthy idle one produced identical output, which is to say
// none.
func (app *application) startCronJobs() {
	app.scheduler.Start(app.shutdown, app.cronJobs()...)

	app.logger.Info("cron: all jobs started")
}

// cronJobs is the schedule itself, built separately from starting it.
//
// A job that is written, wired and never registered here does not run, and that
// is invisible from every other angle: the store method compiles, its tests
// pass, and production simply never calls it. Returning the list makes the
// registration something a test can read.
func (app *application) cronJobs() []scheduler.Job {
	job := func(name string, every time.Duration, run func(context.Context)) scheduler.Job {
		return scheduler.Job{Name: name, Every: every, Run: app.observed(name, run)}
	}

	return []scheduler.Job{
		job("reminder_2h", 5*time.Minute, app.cronReminder2h),
		job("release_expired_payments", 5*time.Minute, app.cronReleaseExpiredPayments),
		job("clean_slot_locks", 5*time.Minute, app.cronCleanSlotLocks),
		job("report_queue_depth", 5*time.Minute, app.cronReportQueueDepth),
		job("retry_refunds", 2*time.Minute, app.payments.RetryFailedRefunds),
		job("sweep_webhook_events", 2*time.Minute, app.payments.ProcessPendingWebhookEvents),
		job("sweep_refund_intents", 2*time.Minute, app.payments.SweepOrphanedRefundIntents),
		job("sweep_booking_link_tokens", 2*time.Minute, app.cronCleanBookingLinkTokens),
		job("complete_bookings", 30*time.Minute, app.cronCompleteBookings),
		job("refresh_mp_tokens", 12*time.Hour, app.cronRefreshMPTokens),
		job("clean_tokens", 24*time.Hour, app.cronCleanExpiredTokens),
		job("clean_webhook_events", 24*time.Hour, app.cronCleanWebhookEvents),
		job("clean_failed_refunds", 24*time.Hour, app.cronCleanFailedRefunds),
		job("clean_unverified_users", 24*time.Hour, app.cronCleanUnverifiedUsers),

		// The token blacklist is an in-process fallback when Redis is absent,
		// so every instance prunes its own — this one takes no lock.
		{Name: "blacklist_cleanup", Every: 5 * time.Minute, Local: true,
			Run: app.observed("blacklist_cleanup", func(context.Context) { app.blacklist.Cleanup() })},
	}
}

// observed wraps a job so that every run leaves exactly one line, whether or
// not the job found any work.
//
// The line carries the connection-pool figures as well. A cron job is the only
// thing on this process guaranteed to run on a fixed interval whether or not
// anybody is issuing requests, which makes it the cheapest place to put a
// bounded, always-on sample of the number that names pool starvation.
//
// The counterpart to this is observedLocker in adapters.go: this reports the
// runs that happened, and that one reports the runs that were skipped because
// another instance held the lock. A deployment needs both, or "no line" stays
// ambiguous between "not scheduled", "not this instance" and "not running".
func (app *application) observed(name string, run func(context.Context)) func(context.Context) {
	return func(ctx context.Context) {
		start := time.Now()
		// Every sweep here is cross-tenant by definition: expiring pending
		// payments, retrying refunds and completing past bookings all walk the
		// whole platform, and none of them has a complex to be scoped to. This
		// is the one place a cron job's context is built, so it is the one
		// place the bypass the tenant policies look for has to be set —
		// without it a sweep would find no rows and report that it had nothing
		// to do, which is the worst possible way for this to fail.
		ctx = data.ContextWithTenantBypass(ctx)
		run(ctx)

		args := []any{"job", name, "duration_ms", time.Since(start).Milliseconds()}
		app.logger.Info("cron: run finished", append(args, app.dbPoolLogArgs()...)...)
	}
}

// cronReportQueueDepth logs how much work is waiting in the durable queues.
//
// The sweepers cannot answer this: each logs a completion count bounded by its
// own batch limit, so a backlog of fifty and a backlog of fifty thousand
// produce the same line. Nothing counted the rows, and nothing reported how
// long the oldest due item had been waiting — which is the figure that
// separates a queue draining steadily from one that has stopped.
//
// It runs under the lock like any other job, so one instance reports and the
// others do not triple the line.
func (app *application) cronReportQueueDepth(ctx context.Context) {
	if app.queues == nil {
		app.logger.Warn("cron_report_queue_depth: no queue reporter configured")
		return
	}

	stats, err := app.queues.QueueStats(ctx)
	if err != nil {
		app.logger.Error("cron_report_queue_depth: failed", "error", err)
		return
	}

	for _, q := range stats {
		app.logger.Info("cron_report_queue_depth: backlog",
			"queue", q.Name,
			"pending", q.Pending,
			"processing", q.Processing,
			"exhausted", q.Exhausted,
			"oldest_due_seconds", q.OldestDueSeconds,
		)

		// Exhausted rows on these two queues are money: a payment never
		// reconciled, a refund a client is still owed. They are reported one
		// Sentry message per row at the moment they exhaust, which during an
		// incident is N alerts and no way to see N. This is the N.
		if q.Exhausted > 0 {
			app.logger.Error("cron_report_queue_depth: items have exhausted their retries and need a human",
				"queue", q.Name, "exhausted", q.Exhausted)
		}
	}
}

// cronReminder2h sends 2-hour reminder notifications (email + optionally WhatsApp).
// Reminders are informational only — bookings are never auto-cancelled for lack of confirmation.
func (app *application) cronReminder2h(ctx context.Context) {
	// time.Now() and not timezone.Now(): the store compares instants, so the
	// location this value carries changes nothing. Argentina enters the
	// calculation once, inside booking_starts_at, where the stored date and
	// start_time are read as local wall clock.
	bookings, err := app.models.Bookings.GetForReminder2hEnriched(ctx, time.Now())
	if err != nil {
		app.logger.Error("cron_reminder_2h: failed to get bookings", "error", err)
		return
	}

	sent := 0
	for _, b := range bookings {
		if err := app.models.Bookings.MarkReminderSent2h(ctx, b.ID); err != nil {
			app.logger.Error("cron_reminder_2h: failed to mark sent", "error", err, "booking_id", b.ID)
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
		token, mintErr := app.models.BookingLinkTokens.Mint(ctx, b.ID, b.EndsAt.Add(app.config.booking.linkTokenBuffer))
		if mintErr != nil {
			app.logger.Error("cron_reminder_2h: failed to mint a cancel link, sending the reminder without one",
				"error", mintErr, "booking_id", b.ID)
		} else {
			cancelPath = booklink.CancelPath(b.ComplexSlug, token)
			cancelURL = booklink.Cancel(app.config.frontendURL, b.ComplexSlug, token)
		}

		app.notify.ReminderDue(notifications.Reminder{
			Email:       b.ClientEmail,
			Phone:       b.ClientPhone,
			ComplexName: b.ComplexName,
			CourtName:   b.CourtName,
			Date:        b.Date.Format("02/01"),
			StartTime:   timezone.HoursLabel(b.StartsAt, b.EndsAt),
			Address:     complexAddress(b),
			// What is left to pay at the venue. Two hours out this is the fact
			// a client acts on, and the confirmation that carried it went out
			// days ago.
			BalanceAmount: notifications.BalanceAmount(b.Price, b.DepositAmount, b.CollectionStatus),
			MapsQuery:     booklink.MapsQuery(b.ComplexName, b.ComplexAddress, b.ComplexCity, b.ComplexLatitude, b.ComplexLongitude),
			MapsURL:       booklink.MapsURL(b.ComplexName, b.ComplexAddress, b.ComplexCity, b.ComplexLatitude, b.ComplexLongitude),
			CancelPath:    cancelPath,
			CancelURL:     cancelURL,
		})

		app.logger.Info("cron_reminder_2h: sent reminder", "booking_id", b.ID)
		sent++
	}

	app.logger.Info("cron_reminder_2h: completed", "candidates", len(bookings), "sent", sent)
}

// cronReleaseExpiredPayments cancels public bookings that remain unpaid after 15 minutes.
func (app *application) cronReleaseExpiredPayments(ctx context.Context) {
	bookings, err := app.models.Bookings.GetExpiredPendingEnriched(ctx, app.config.booking.paymentExpiry)
	if err != nil {
		app.logger.Error("cron_release_expired_payments: failed to get bookings", "error", err)
		return
	}

	released := 0
	for _, b := range bookings {
		// Expire the MP preference FIRST so the client can't pay while we cancel.
		payment, perr := app.models.Payments.GetByBookingID(ctx, b.ID)
		if perr == nil && payment.MPPreferenceID != nil && *payment.MPPreferenceID != "" {
			// Who the expiry call is made as. Any credential this sweep cannot
			// read leaves the platform as the only party left to ask, and
			// MercadoPago rejecting that costs one failed call — cheaper than
			// leaving a payable link on a booking nobody holds. The arm is
			// written out because it used to be what an empty token quietly
			// meant inside mp.UpdatePreferenceExpired.
			caller := mp.AsPlatform()
			if tok, err := b.SellerAccessToken(); err == nil {
				if seller, callerErr := mp.AsSeller(tok); callerErr == nil {
					caller = seller
				}
			}
			if expErr := app.mp.UpdatePreferenceExpired(ctx, *payment.MPPreferenceID, caller); expErr != nil {
				app.logger.Error("cron_release_expired_payments: first attempt to expire MP preference failed, retrying",
					"error", expErr, "booking_id", b.ID)
				// Retry with short backoff — non-blocking to other bookings in the batch.
				retryCtx, retryCancel := context.WithTimeout(ctx, 10*time.Second)
				if retryErr := app.mp.UpdatePreferenceExpired(retryCtx, *payment.MPPreferenceID, caller); retryErr != nil {
					app.logger.Error("cron_release_expired_payments: retry failed to expire MP preference",
						"error", retryErr, "booking_id", b.ID, "preference_id", *payment.MPPreferenceID)
					sentry.CaptureMessage(fmt.Sprintf("PREFERENCE EXPIRATION FAILED (client may still pay): booking_id=%s preference_id=%s", b.ID, *payment.MPPreferenceID))
				}
				retryCancel()
			}
		}

		// Cancel booking after preference is expired.
		b.Status = "cancelled"
		notes := fmt.Sprintf("Cancelado automáticamente: tiempo de pago expirado (%v)", app.config.booking.paymentExpiry)
		if b.Notes != nil {
			notes = *b.Notes + " | " + notes
		}
		b.Notes = &notes

		if err := app.models.Bookings.Update(ctx, &b.Booking); err != nil {
			app.logger.Error("cron_release_expired_payments: failed to cancel booking", "error", err, "booking_id", b.ID)
			continue
		}

		//nolint:contextcheck // notifyBookingChanged->publish is a fire-and-forget SSE broadcast
		// with its own bounded 2s internal Redis timeout (see sse.go); it must not be cancelled
		// by the ctx of this cron job (which itself has its own bounded lifetime, unrelated to
		// the broadcast). Same shared-helper shape as the payments/bookings call sites (PR2b/2c-i).
		app.events.PublishBookingChanged(b.ComplexID)

		app.notify.BookingCancelled(notifications.Cancellation{
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
			BookURL:    booklink.Book(app.config.frontendURL, b.ComplexSlug),
		})

		app.logger.Info("cron_release_expired_payments: released expired booking", "booking_id", b.ID)
		released++
	}

	app.logger.Info("cron_release_expired_payments: completed", "candidates", len(bookings), "released", released)
}

// cronCleanExpiredTokens deletes expired refresh and verification tokens.
func (app *application) cronCleanExpiredTokens(ctx context.Context) {
	if err := app.models.Tokens.DeleteExpired(ctx); err != nil {
		app.logger.Error("cron_clean_tokens: failed to clean refresh tokens", "error", err)
	}
	if err := app.models.EmailVerification.DeleteExpired(ctx); err != nil {
		app.logger.Error("cron_clean_tokens: failed to clean verification tokens", "error", err)
	}
	app.logger.Info("cron_clean_tokens: completed")
}

// cronCleanUnverifiedUsers deletes accounts that were never verified after 7 days.
func (app *application) cronCleanUnverifiedUsers(ctx context.Context) {
	if err := app.models.Users.DeleteUnverifiedStale(ctx); err != nil {
		app.logger.Error("cron_clean_unverified_users: failed", "error", err)
		return
	}
	app.logger.Info("cron_clean_unverified_users: completed")
}

// cronCompleteBookings marks past confirmed bookings as completed.
func (app *application) cronCompleteBookings(ctx context.Context) {
	count, err := app.models.Bookings.CompletePastBookings(ctx)
	if err != nil {
		app.logger.Error("cron_complete_bookings: failed", "error", err)
		return
	}
	app.logger.Info("cron_complete_bookings: completed", "count", count)
}

// bookingLinkTokenRetention is how long past its stored expiry a terminal
// booking's link token is kept before the sweep deletes it. Purely
// housekeeping — DeleteExpiredTerminal's terminal-status predicate already
// guarantees a live booking's token is never touched, so this window exists
// only to keep the table from growing without bound, not to protect a link
// still in use.
const bookingLinkTokenRetention = 24 * time.Hour

// cronCleanBookingLinkTokens deletes booking link tokens whose booking has
// reached a terminal state and whose stored expiry is safely in the past. See
// internal/data/booking_link_tokens.go's DeleteExpiredTerminal: a pending or
// confirmed booking's token is never touched here, however old, so a sweep
// can never turn a live link into a 404.
func (app *application) cronCleanBookingLinkTokens(ctx context.Context) {
	if err := app.models.BookingLinkTokens.DeleteExpiredTerminal(ctx, bookingLinkTokenRetention); err != nil {
		app.logger.Error("cron_clean_booking_link_tokens: failed", "error", err)
		return
	}
	app.logger.Info("cron_clean_booking_link_tokens: completed")
}

// cronCleanSlotLocks removes expired slot locks to free up slots.
func (app *application) cronCleanSlotLocks(ctx context.Context) {
	count, err := app.models.SlotLocks.CleanExpired(ctx)
	if err != nil {
		app.logger.Error("cron_clean_slot_locks: failed", "error", err)
		return
	}
	app.logger.Info("cron_clean_slot_locks: completed", "count", count)
}

// webhookEventRetention is how long a processed webhook event is kept.
//
// Ninety days is chosen to outlast the window in which a delivered event can still
// be questioned: MercadoPago chargebacks and buyer claims are opened weeks after
// the payment, and when one arrives the recorded payload is the only evidence of
// what the provider actually told us, as opposed to what we later read back from
// its API. It also spans a full quarter, so a reconciliation still has its source
// rows. Past that the row is only volume — and this table takes a row per
// delivery, several per payment, so it must not grow without bound.
//
// Only 'processed' rows are eligible. Pending, in-flight and exhausted events
// describe money that is still unresolved and are never deleted on a timer.
const webhookEventRetention = 90 * 24 * time.Hour

// cronCleanWebhookEvents drops webhook events that were processed long enough ago
// to have no forensic value left.
func (app *application) cronCleanWebhookEvents(ctx context.Context) {
	count, err := app.models.WebhookEvents.DeleteProcessed(ctx, webhookEventRetention)
	if err != nil {
		app.logger.Error("cron_clean_webhook_events: failed", "error", err)
		return
	}
	app.logger.Info("cron_clean_webhook_events: completed", "count", count)
}

// resolvedRefundRetention is how long a resolved refund attempt is kept.
//
// The window is deliberately webhookEventRetention: both tables are the forensic
// record of one payment's journey, they are read together when a chargeback or a
// client claim is investigated, and keeping one of them three months while the
// other goes at thirty days would leave half a story. See the reasoning there.
//
// Only 'resolved' rows are eligible. A pending, in-flight or exhausted attempt is
// money that has not come back yet, and no timer may delete that.
const resolvedRefundRetention = webhookEventRetention

// cronCleanFailedRefunds drops refund attempts that were resolved long enough ago
// to have no forensic value left.
//
// Since refunds became claim-first this table takes a row per refund rather than
// per failed refund — ClaimRefund writes one before MercadoPago is called — and
// the overwhelming majority resolve within seconds and are never read again.
// Nothing deleted any of them, so it grew for the life of the deployment and the
// sweeper's every-two-minutes query got slower with it.
func (app *application) cronCleanFailedRefunds(ctx context.Context) {
	count, err := app.models.FailedRefunds.DeleteResolved(ctx, resolvedRefundRetention)
	if err != nil {
		app.logger.Error("cron_clean_failed_refunds: failed", "error", err)
		return
	}
	app.logger.Info("cron_clean_failed_refunds: completed", "count", count)
}

// cronRefreshMPTokens proactively refreshes OAuth tokens for all connected complexes.
// MP tokens expire after ~6 months; refreshing every 12h keeps them fresh.
func (app *application) cronRefreshMPTokens(ctx context.Context) {
	// Narrowed to complexes whose token has no known expiry or expires within
	// 30 days, instead of GetWithMPConnected's every-connected-complex: MP
	// tokens live ~180 days, and refreshing all of them on every 12h tick was
	// pure waste once the expiry was actually tracked (mp_token_expires_at).
	complexes, err := app.models.Complexes.ListComplexesNeedingMPRefresh(ctx)
	if err != nil {
		app.logger.Error("cron_refresh_mp_tokens: failed to get complexes", "error", err)
		sentry.CaptureMessage(fmt.Sprintf("cron_refresh_mp_tokens: CRITICAL - cannot fetch complexes: %v", err))
		return
	}

	refreshed := 0
	failed := 0
	for _, c := range complexes {
		switch app.refreshOneMPToken(ctx, c) {
		case mpRefreshOK:
			refreshed++
		case mpRefreshFailed:
			failed++
		case mpRefreshSkipped:
			// Not connected (ErrMPNotConnected): ListComplexesNeedingMPRefresh
			// already filters on mp_refresh_token IS NOT NULL, so this is
			// defensive rather than expected — neither a success nor a
			// failure of this cron run.
		}
	}

	if failed > 0 {
		sentry.CaptureMessage(fmt.Sprintf("cron_refresh_mp_tokens: %d/%d complexes failed to refresh", failed, len(complexes)))
	}

	app.logger.Info("cron_refresh_mp_tokens: completed",
		"total", len(complexes),
		"refreshed", refreshed,
		"failed", failed,
	)
}

// mpRefreshResult is what happened to one complex's refresh attempt.
type mpRefreshResult int

const (
	mpRefreshOK mpRefreshResult = iota
	mpRefreshFailed
	// mpRefreshSkipped means there was nothing to refresh — the credential
	// read as not-connected, which ListComplexesNeedingMPRefresh's own
	// filter should already have excluded. Defensive, not expected.
	mpRefreshSkipped
)

// refreshOneMPToken refreshes a single complex's MercadoPago OAuth token and
// persists the result. Every failure branch alerts Sentry itself, so the
// caller only has to count.
func (app *application) refreshOneMPToken(ctx context.Context, c *complexstore.Complex) mpRefreshResult {
	refreshTok, refreshErr := c.SellerRefreshToken()
	if refreshErr != nil {
		if errors.Is(refreshErr, mpcred.ErrMPCredentialUnreadable) {
			sentry.CaptureMessage(fmt.Sprintf("MP refresh token UNREADABLE (skipping refresh): complex=%s (%s) error=%v", c.Name, c.ID, refreshErr))
			return mpRefreshFailed
		}
		return mpRefreshSkipped
	}

	newTokens, err := app.mpOAuth.RefreshOAuthToken(ctx, refreshTok)
	if err != nil {
		// A 4xx here is ordinarily MercadoPago answering invalid_grant — the
		// seller revoked access, or the refresh token itself expired — which
		// is an expected, unactionable-by-retry outcome, not an operational
		// failure of this cron job. Anything else (5xx, network, decode) is.
		if refreshTokenWasRejected(err) {
			app.logger.Warn("cron_refresh_mp_tokens: MercadoPago rejected the refresh token",
				"error", err, "complex_id", c.ID, "complex_name", c.Name)
		} else {
			app.logger.Error("cron_refresh_mp_tokens: failed to refresh token",
				"error", err, "complex_id", c.ID, "complex_name", c.Name)
		}
		sentry.CaptureMessage(fmt.Sprintf("MP OAuth refresh FAILED: complex=%s (%s) error=%v", c.Name, c.ID, err))
		return mpRefreshFailed
	}

	mpUserID := fmt.Sprintf("%d", newTokens.UserID)
	if updateErr := app.models.Complexes.UpdateMPCredentials(ctx, c.ID, newTokens.AccessToken, newTokens.RefreshToken, mpUserID, newTokens.ExpiresIn); updateErr != nil {
		app.logger.Error("cron_refresh_mp_tokens: failed to save new tokens", "error", updateErr, "complex_id", c.ID)
		sentry.CaptureMessage(fmt.Sprintf("MP OAuth token save FAILED: complex=%s (%s) error=%v", c.Name, c.ID, updateErr))
		return mpRefreshFailed
	}

	app.logger.Info("cron_refresh_mp_tokens: refreshed token", "complex_id", c.ID, "complex_name", c.Name)
	return mpRefreshOK
}

// refreshTokenWasRejected reports whether a RefreshOAuthToken failure was
// MercadoPago's 4xx invalid_grant-style answer — the seller revoked the
// grant, or the refresh token itself expired — rather than an outage (5xx,
// network, decode failure) worth an Error-level page.
func refreshTokenWasRejected(err error) bool {
	var apiErr *mp.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
}

// complexAddress is the venue's address as a person reads it, city included.
//
// A complex with no address on file yields the empty string rather than a
// stray comma, and internal/mailer and the WhatsApp reminder both leave the
// line out when it is empty. It delegates to booklink.Address, which the
// confirmation email now needs the same join from.
func complexAddress(b *bookingstore.CronBooking) string {
	return booklink.Address(b.ComplexAddress, b.ComplexCity)
}

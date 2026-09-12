package main

import (
	"context"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/scheduler"
)

// jobRetentionStore lets clean_jobs be tested without a database.
type jobRetentionStore interface {
	DeleteDone(ctx context.Context, olderThan time.Duration) (int64, error)
}

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
		job("retry_refunds", 2*time.Minute, app.paymentsService.RetryFailedRefunds),
		job("sweep_webhook_events", 2*time.Minute, app.paymentsService.ProcessPendingWebhookEvents),
		job("sweep_refund_intents", 2*time.Minute, app.paymentsService.SweepOrphanedRefundIntents),
		job("sweep_booking_link_tokens", 2*time.Minute, app.cronCleanBookingLinkTokens),
		job("complete_bookings", 30*time.Minute, app.cronCompleteBookings),
		job("refresh_mp_tokens", 12*time.Hour, app.cronRefreshMPTokens),
		job("clean_tokens", 24*time.Hour, app.cronCleanExpiredTokens),
		job("clean_webhook_events", 24*time.Hour, app.cronCleanWebhookEvents),
		job("clean_failed_refunds", 24*time.Hour, app.cronCleanFailedRefunds),
		job("clean_unverified_users", 24*time.Hour, app.cronCleanUnverifiedUsers),
		job("clean_jobs", 24*time.Hour, app.cronCleanJobs),

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

// cronReminder2h sends the 2-hour reminder notifications.
//
// The sweep itself lives in bookings.Service, beside the rules it shares with
// the request paths. This wrapper exists so the scheduler still names one
// method per job.
func (app *application) cronReminder2h(ctx context.Context) {
	app.bookingsService.Reminder2h(ctx)
}

// cronReleaseExpiredPayments cancels public bookings that remain unpaid past
// the payment window. The sweep lives in bookings.Service; see cronReminder2h.
func (app *application) cronReleaseExpiredPayments(ctx context.Context) {
	app.bookingsService.ReleaseExpiredPayments(ctx)
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

// cronCleanJobs deletes finished jobs.Store rows past Retention, which is
// also what frees a done job's DedupKey again. See jobs.DedupKey.
func (app *application) cronCleanJobs(ctx context.Context) {
	if app.jobRetention == nil {
		return
	}
	count, err := app.jobRetention.DeleteDone(ctx, jobs.DefaultConfig().Retention)
	if err != nil {
		app.logger.Error("cron_clean_jobs: failed", "error", err)
		return
	}
	app.logger.Info("cron_clean_jobs: completed", "count", count)
}

// cronCompleteBookings marks past confirmed bookings as completed. The sweep
// lives in bookings.Service; see cronReminder2h.
func (app *application) cronCompleteBookings(ctx context.Context) {
	app.bookingsService.CompletePastBookings(ctx)
}

// cronCleanBookingLinkTokens deletes the link tokens of bookings that have
// reached a terminal state. The sweep, and the retention window it uses
// (bookings.LinkTokenRetention), live in bookings.Service; see cronReminder2h.
func (app *application) cronCleanBookingLinkTokens(ctx context.Context) {
	app.bookingsService.CleanLinkTokens(ctx)
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

// cronRefreshMPTokens proactively refreshes OAuth tokens for all connected
// complexes. MP tokens expire after ~6 months; refreshing every 12h keeps them
// fresh.
//
// The sweep itself lives in complexes.Service: it is the same credential
// lifecycle the connect and disconnect routes open and close, and keeping it
// beside them is what stops this file from being a second place venue rules
// are written. This wrapper exists so the scheduler still names one method per
// job.
func (app *application) cronRefreshMPTokens(ctx context.Context) {
	app.complexesService.RefreshMPTokens(ctx)
}

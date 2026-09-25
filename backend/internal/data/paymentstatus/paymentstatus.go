// Package paymentstatus is the one place that knows which payment_status
// values mean money actually stayed collected — the revenue-reporting
// counterpart to internal/data/slotguard's ReleasedBookingStatuses.
//
// It exists because of a real bug: every dashboard/admin revenue sum used to
// filter WHERE status != 'refunded', which reads as "everything except a
// refund" but actually also counts 'unpaid' — the row internal/bookings'
// public checkout inserts the instant a MercadoPago preference is created
// (service_public.go), before the player has paid anything. If the player
// abandons checkout, cron.ReleaseExpiredPayments cancels the booking but
// leaves that payment row 'unpaid' forever (payment_status has no
// "expired"/"cancelled" value), so every != 'refunded' sum quietly counted an
// abandoned checkout as revenue.
package paymentstatus

// CollectedStatuses are the payment_status values (db/migrations/001_init.sql:
// 'unpaid', 'deposit_paid', 'fully_paid', 'refunded', 'refund_pending',
// 'partial_refund') where money is currently held by the venue — the
// definition every "how much came in" sum should filter on.
//
// Excluded, and why:
//   - 'unpaid': checkout started, nothing was ever collected (the bug above).
//   - 'refunded': fully returned, nothing is left to count as revenue.
//
// Included, and why:
//   - 'deposit_paid', 'fully_paid': the ordinary collected states.
//   - 'refund_pending': a refund was requested but has not completed yet —
//     the money has not left the venue's account, so it stays revenue until
//     it does. Same behavior the old != 'refunded' filter already had; kept
//     rather than "fixed", because a refund request is not a refund.
//   - 'partial_refund': part of the payment came back, part stayed collected.
//     Every sum here still totals the gross p.amount, the same arithmetic
//     every other included status already gets (nothing in this package nets
//     p.amount against p.refund_amount) — narrowing only which STATUSES
//     count, not changing how a counted row is summed, keeps this fix to the
//     one bug reported instead of quietly reshaping refund accounting too.
const CollectedStatuses = `('deposit_paid', 'fully_paid', 'refund_pending', 'partial_refund')`

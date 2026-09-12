//go:build integration

package store_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// MarkProcessing flips a queued refund to 'processing' before the provider call and
// nothing moves it back if the process dies mid-attempt. A pending-only query therefore
// loses the row permanently: money is owed, nothing retries it, nothing logs it.
// GetPendingDue reclaims rows that have sat in 'processing' past a fifteen-minute
// window — and only those, or the queue would hand a live in-flight refund to a second
// worker and pay the client twice.
func TestGetPendingDueReclaimsStaleProcessingRefunds(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	stale := createFailedRefund(f, t, "08:00", "09:30")
	fresh := createFailedRefund(f, t, "10:00", "11:30")

	for _, id := range []uuid.UUID{stale.ID, fresh.ID} {
		if err := f.Stores.FailedRefunds.MarkProcessing(ctx, id); err != nil {
			t.Fatalf("MarkProcessing(%s): %v", id, err)
		}
	}

	// Only the first attempt is aged past the window; the second stays as it was
	// marked a moment ago, which is what a healthy in-flight retry looks like.
	f.BackdateFailedRefundUpdatedAt(t, stale.ID, 20*time.Minute)

	due, err := f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	found := idsOf(due)

	if !found[stale.ID] {
		t.Error("a refund abandoned in 'processing' must be picked up again, or the money is owed forever")
	}
	if found[fresh.ID] {
		t.Error("a refund marked 'processing' moments ago is still in flight and must not be handed to a second worker")
	}
}

// The reclaim clause must not swallow the ordinary case it was added beside.
func TestGetPendingDueStillRespectsTheRetrySchedule(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	due := createFailedRefund(f, t, "08:00", "09:30")

	backedOff := createFailedRefund(f, t, "10:00", "11:30")
	if err := f.Stores.FailedRefunds.IncrementRetry(ctx, backedOff.ID, 0, "mercadopago unavailable"); err != nil {
		t.Fatalf("IncrementRetry: %v", err)
	}

	pending, err := f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	found := idsOf(pending)

	if !found[due.ID] {
		t.Error("a pending refund whose next_retry_at has passed must be returned")
	}
	if found[backedOff.ID] {
		t.Error("a refund backed off into the future must wait for its next_retry_at")
	}
}

// createFailedRefund queues a refund that is already due for retry, backed by a real
// payment and booking so the foreign keys hold. The slot times are explicit because
// several refunds in one test share a court and date, and
// bookings(court_id, date, start_time) is uniquely indexed.
func createFailedRefund(f *datatest.Fixture, t *testing.T, startTime, endTime string) *paymentstore.FailedRefund {
	t.Helper()

	booking := f.CreateBooking(t, datatest.BookingOptions{StartTime: startTime, EndTime: endTime})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	fr := &paymentstore.FailedRefund{
		PaymentID:    payment.ID,
		BookingID:    booking.ID,
		ComplexID:    f.ComplexID,
		Amount:       157_500,
		MPPaymentID:  mpPaymentID,
		ErrorMessage: "mercadopago unavailable",
		NextRetryAt:  time.Now().Add(-time.Minute),
	}
	if err := f.Stores.FailedRefunds.Insert(context.Background(), fr); err != nil {
		t.Fatalf("queueing failed refund: %v", err)
	}
	return fr
}

// idsOf indexes a result set by ID. GetPendingDue is global rather than scoped to one
// complex, so the assertions above look for specific rows instead of counting.
func idsOf(refunds []*paymentstore.FailedRefund) map[uuid.UUID]bool {
	found := make(map[uuid.UUID]bool, len(refunds))
	for _, fr := range refunds {
		found[fr.ID] = true
	}
	return found
}

// FINDING 5. A refund whose budget ran out is not abandoned forever.
//
// 'exhausted' was terminal: neither sweeper selected it and nothing — no
// endpoint, no job, no command — ever moved a row out of it. A MercadoPago
// outage longer than five hours and twenty minutes put every queued refund there
// at once, permanently, for a reason that had nothing to do with the refunds.
// The status now means "paused and alerting" rather than "given up on".
func TestAnExhaustedRefundComesBackWhenItsBackoffElapses(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	fr := createFailedRefund(f, t, "08:00", "09:30")
	exhaustRefund(f, t, fr.ID)

	due, err := f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if !idsOf(due)[fr.ID] {
		t.Fatal("a refund whose retry budget is spent is never looked at again, so the money is owed forever")
	}

	if err := f.Stores.FailedRefunds.MarkProcessing(ctx, fr.ID); err != nil {
		t.Errorf("a revived attempt must be claimable: %v", err)
	}
}

// A refund still waiting out the tail of its backoff is not revived early; the
// pause has to be a real one or an unfixable refund becomes a hot loop.
func TestAnExhaustedRefundWaitsOutItsBackoff(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	fr := createFailedRefund(f, t, "08:00", "09:30")
	exhaustRefund(f, t, fr.ID)
	if _, err := f.Pool.Exec(ctx,
		`UPDATE failed_refunds SET next_retry_at = NOW() + INTERVAL '4 hours' WHERE id = $1`, fr.ID); err != nil {
		t.Fatalf("scheduling the revival: %v", err)
	}

	due, err := f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if idsOf(due)[fr.ID] {
		t.Error("an exhausted refund was revived before its backoff elapsed")
	}
}

// FINDING 4's premise. Two instances see the same row on the same tick, and the
// claim — not the advisory lock — is what decides which of them calls
// MercadoPago. An unconditional MarkProcessing let both through and left the
// provider's idempotency key as the only thing between a client and two refunds.
func TestOnlyOneWorkerCanTakeARefundAttempt(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	fr := createFailedRefund(f, t, "08:00", "09:30")

	if err := f.Stores.FailedRefunds.MarkProcessing(ctx, fr.ID); err != nil {
		t.Fatalf("the first claim must succeed: %v", err)
	}

	err := f.Stores.FailedRefunds.MarkProcessing(ctx, fr.ID)
	if err == nil {
		t.Error("a second worker took an attempt the first is still working, so the client can be refunded twice")
	} else if !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("want ErrRecordNotFound for an attempt that cannot be taken; got %v", err)
	}
}

// FINDING 6. An attempt abandoned mid-flight is dark until the window elapses,
// so the window is no longer than it has to be: five minutes outlasts every
// honest attempt — a MercadoPago call takes eight seconds and a whole sweep is
// capped at two minutes — where the old fifteen was three times the longest
// thing it had to outlast.
func TestAnAbandonedRefundIsReclaimedOnceItsAttemptCannotStillBeHonest(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	fr := createFailedRefund(f, t, "08:00", "09:30")
	if err := f.Stores.FailedRefunds.MarkProcessing(ctx, fr.ID); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	// Older than any attempt can honestly be — the sweep that claimed it has
	// long since hit its own two-minute budget — but well inside the old window.
	f.BackdateFailedRefundUpdatedAt(t, fr.ID, 6*time.Minute)

	due, err := f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if !idsOf(due)[fr.ID] {
		t.Error("an attempt that cannot still be in flight is invisible, and the money it owes with it")
	}
}

// FINDING 6. The sweep is sequential, capped at two minutes, and spends about
// eight seconds per row in MercadoPago, so it reads what it can work rather than
// fifty rows it will drop.
func TestTheRefundSweepReadsNoMoreThanARunCanWork(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	// One-hour bookings, back to back: the fixture derives duration_minutes
	// from these hours, and the schema permits only 60, 90 or 120.
	for i := range paymentstore.RefundSweepBatchForTest + 3 {
		start := fmt.Sprintf("%02d:00", 6+i)
		end := fmt.Sprintf("%02d:00", 7+i)
		createFailedRefund(f, t, start, end)
	}

	due, err := f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if len(due) > paymentstore.RefundSweepBatchForTest {
		t.Errorf("the sweep read %d rows; a run can only work %d of them, and the rest are fetched and dropped",
			len(due), paymentstore.RefundSweepBatchForTest)
	}
}

// FINDING 7. This table takes a row per refund rather than per failure — claims
// are written before MercadoPago is called — and almost all of them resolve
// within seconds. Nothing deleted any of them.
func TestResolvedRefundAttemptsAreDeletedAndNothingElse(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	const retention = 90 * 24 * time.Hour

	old := createFailedRefund(f, t, "08:00", "09:30")
	recent := createFailedRefund(f, t, "10:00", "11:30")
	owed := createFailedRefund(f, t, "12:00", "13:30")

	for _, id := range []uuid.UUID{old.ID, recent.ID} {
		if err := f.Stores.FailedRefunds.MarkResolved(ctx, id); err != nil {
			t.Fatalf("MarkResolved(%s): %v", id, err)
		}
	}
	exhaustRefund(f, t, owed.ID)
	if _, err := f.Pool.Exec(ctx,
		`UPDATE failed_refunds SET resolved_at = NOW() - $2::interval WHERE id = $1`,
		old.ID, (retention + 24*time.Hour).String()); err != nil {
		t.Fatalf("ageing the resolved attempt: %v", err)
	}

	// The concrete model rather than f.Stores.FailedRefunds: retention is not on
	// the FailedRefundStore interface yet, because adding it there means editing
	// models.go and the mock beside it. See the note in DeleteResolved.
	store := &paymentstore.FailedRefunds{DB: data.NewDB(f.Pool)}
	if _, err := store.DeleteResolved(ctx, retention); err != nil {
		t.Fatalf("DeleteResolved: %v", err)
	}

	if refundExists(f, t, old.ID) {
		t.Error("a refund resolved months ago is kept forever, so this table only ever grows")
	}
	if !refundExists(f, t, recent.ID) {
		t.Error("a refund resolved inside the retention window was deleted")
	}
	if !refundExists(f, t, owed.ID) {
		t.Error("an exhausted refund is money still owed to a client and must never be deleted on a timer")
	}
}

// FINDING 7. The sweeper's index has to answer the sweeper's query. It was
// partial on status = 'pending' while the query had long since started selecting
// abandoned and — now — exhausted rows too, so PostgreSQL read the whole table
// every two minutes.
//
// Sequential scans are disabled for the plan below, which is what makes the
// assertion about the index rather than about how many rows this database
// happens to hold: with an index that cannot answer the predicate, PostgreSQL
// scans anyway and says so.
func TestTheRefundSweepQueryIsAnsweredByItsIndex(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatalf("disabling sequential scans: %v", err)
	}

	rows, err := tx.Query(ctx, `
		EXPLAIN SELECT id FROM failed_refunds
		WHERE next_retry_at <= NOW()
		  AND (status IN ('pending', 'exhausted')
		       OR (status = 'processing' AND updated_at < NOW() - $1::interval))
		ORDER BY next_retry_at ASC
		LIMIT $2`, paymentstore.StaleRefundProcessingForTest.String(), paymentstore.RefundSweepBatchForTest)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("reading the plan: %v", err)
		}
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the plan: %v", err)
	}

	if !strings.Contains(plan.String(), "idx_failed_refunds_pending") {
		t.Errorf("the sweep does not use its index, so it reads the whole table every two minutes:\n%s", plan.String())
	}
	if strings.Contains(plan.String(), "Seq Scan") {
		t.Errorf("the sweep falls back to a sequential scan:\n%s", plan.String())
	}
}

// FINDING 5. An attempt that never reached the provider is not evidence about
// the refund, so it must not spend part of the budget that bounds how many times
// we ask a provider that is answering us.
func TestAProviderOutageDoesNotSpendARefundsRetryBudget(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	fr := createFailedRefund(f, t, "08:00", "09:30")

	// Two attempts have already been made and answered.
	if err := f.Stores.FailedRefunds.IncrementRetry(ctx, fr.ID, 2, "mp: refund failed with status 400: invalid amount"); err != nil {
		t.Fatalf("IncrementRetry: %v", err)
	}
	if got := readRefund(f, t, fr.ID).retryCount; got != 2 {
		t.Fatalf("a rejection must spend a retry; want 2, got %d", got)
	}

	if err := f.Stores.FailedRefunds.IncrementRetry(ctx, fr.ID, 3, "mp: refund request failed: dial tcp: i/o timeout"); err != nil {
		t.Fatalf("IncrementRetry: %v", err)
	}

	state := readRefund(f, t, fr.ID)
	if state.retryCount != 2 {
		t.Errorf("a provider outage spent a retry; want the count left at 2, got %d — five hours of one "+
			"outage still abandons every queued refund", state.retryCount)
	}
	if state.status != "pending" {
		t.Errorf("want the attempt left queued; got %q", state.status)
	}
}

// exhaustRefund spends an attempt's whole retry budget, as a run of provider
// rejections would, and leaves it due.
func exhaustRefund(f *datatest.Fixture, t *testing.T, id uuid.UUID) {
	t.Helper()

	_, err := f.Pool.Exec(context.Background(), `
		UPDATE failed_refunds
		SET status = 'exhausted', retry_count = max_retries, next_retry_at = NOW() - INTERVAL '1 minute'
		WHERE id = $1`, id)
	if err != nil {
		t.Fatalf("exhausting refund %s: %v", id, err)
	}
}

// refundState is a queued attempt's bookkeeping, read straight from the table.
type refundState struct {
	status      string
	retryCount  int
	nextRetryAt time.Time
}

func readRefund(f *datatest.Fixture, t *testing.T, id uuid.UUID) refundState {
	t.Helper()

	var s refundState
	err := f.Pool.QueryRow(context.Background(),
		`SELECT status, retry_count, next_retry_at FROM failed_refunds WHERE id = $1`, id,
	).Scan(&s.status, &s.retryCount, &s.nextRetryAt)
	if err != nil {
		t.Fatalf("reading failed refund %s: %v", id, err)
	}
	return s
}

func refundExists(f *datatest.Fixture, t *testing.T, id uuid.UUID) bool {
	t.Helper()

	var exists bool
	if err := f.Pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM failed_refunds WHERE id = $1)`, id).Scan(&exists); err != nil {
		t.Fatalf("checking failed refund %s: %v", id, err)
	}
	return exists
}

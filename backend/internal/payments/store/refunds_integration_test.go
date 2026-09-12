//go:build integration

package store_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// The refund flow is claim → call MercadoPago → record. These tests exercise the
// two ends of that sentence against a real PostgreSQL, because everything that
// makes the design worth having is a property of what the database actually holds
// between the two: the claim has to be committed, it has to hold no locks, and it
// has to be findable again if this process never comes back.

// A claim that is not committed by the time ClaimRefund returns is worth nothing:
// the caller is about to spend eight seconds inside MercadoPago's API, and if the
// process dies during that call, whatever was still sitting in an open transaction
// disappears with it. The previous AtomicRefund returned exactly that — an open
// transaction and a Commit closure — which is why a refund the provider had
// already made could end up recorded nowhere.
//
// The read below therefore goes through a connection of its own, outside the pool
// the store used. An uncommitted write is invisible to it by definition, so the
// row reading 'refund_pending' here is proof the claim is durable.
func TestClaimRefundIsCommittedBeforeTheProviderIsCalled(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	claim, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}
	if want := 157_500; claim.RefundCentavos != want {
		t.Errorf("a claim reserves the deposit plus the service fee: want %d, got %d", want, claim.RefundCentavos)
	}
	if claim.MPPaymentID != mpPaymentID {
		t.Errorf("the claim must carry the MercadoPago id to refund against; want %q, got %q", mpPaymentID, claim.MPPaymentID)
	}
	if claim.AttemptID == uuid.Nil {
		t.Fatal("a claim must name the durable attempt row that survives this process")
	}

	conn := separateConn(f, t)

	var status string
	if err := conn.QueryRow(ctx,
		`SELECT status FROM payments WHERE id = $1`, payment.ID,
	).Scan(&status); err != nil {
		t.Fatalf("reading the payment from a separate connection: %v", err)
	}
	if status != "refund_pending" {
		t.Errorf("another connection must already see the claim committed; got status %q", status)
	}

	var attemptStatus string
	var amount int
	if err := conn.QueryRow(ctx,
		`SELECT status, amount FROM failed_refunds WHERE id = $1`, claim.AttemptID,
	).Scan(&attemptStatus, &amount); err != nil {
		t.Fatalf("reading the attempt from a separate connection: %v", err)
	}
	if attemptStatus != "pending" {
		t.Errorf("the attempt must be queued before the provider is called; got %q", attemptStatus)
	}
	if amount != claim.RefundCentavos {
		t.Errorf("the queued attempt must carry the claimed amount; want %d, got %d", claim.RefundCentavos, amount)
	}
}

// The defect this redesign exists to remove: a transaction and a FOR UPDATE lock
// held across a synchronous MercadoPago call. The client's own timeout is eight
// seconds and the pool has twenty-five connections, so a slow provider used to
// turn into pool exhaustion for the entire API, not just for refunds.
//
// SELECT ... FOR UPDATE NOWAIT is the exact instrument for that: it takes the row
// lock if it is free and fails immediately with SQLSTATE 55P03 if anyone else
// holds it, rather than waiting. Succeeding here means no lock survived the claim.
// Under the old design the same statement would have failed while the caller sat
// inside the provider call, which is precisely the window that mattered.
func TestClaimRefundHoldsNoLockOnceItReturns(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	if _, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID); err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	// This is where the provider call happens in production. Nothing of ours may
	// be holding the payment row now.
	conn := separateConn(f, t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin on the separate connection: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id FROM payments WHERE id = $1 FOR UPDATE NOWAIT`, payment.ID,
	).Scan(&lockedID)
	if err != nil {
		t.Fatalf("the claimed payment row is still locked, so a provider call would hold a pooled "+
			"connection and a row lock for its whole duration: %v", err)
	}
	if lockedID != payment.ID {
		t.Errorf("locked the wrong row: want %s, got %s", payment.ID, lockedID)
	}

	// The booking must be free too — a refund that is not recorded yet has no
	// business blocking anyone from touching the booking.
	var bookingID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT id FROM bookings WHERE id = $1 FOR UPDATE NOWAIT`, booking.ID,
	).Scan(&bookingID); err != nil {
		t.Fatalf("the booking row is still locked after the claim: %v", err)
	}
}

// The process dying between the claim and the record is the case the whole design
// is built around: the money may or may not have moved, and nobody is left to find
// out. The claim's attempt row is the answer — once its retry time arrives the
// reaper picks it up, replays the provider call (which MercadoPago deduplicates on
// its idempotency key) and records the result.
//
// Nothing is done here after the claim, which is exactly what a crash looks like
// from the database's side.
func TestAnAbandonedClaimIsPickedUpByTheRetryQueue(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	claim, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	// A fresh claim is deliberately invisible: the refund it describes is in
	// flight, and handing it to a second worker would refund the client twice.
	due, err := f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if idsOf(due)[claim.AttemptID] {
		t.Error("an attempt claimed a moment ago is still in flight and must not be retried yet")
	}

	// ... and the process dies here. Time passes.
	expireRefundAttempt(f, t, claim.AttemptID)

	due, err = f.Stores.FailedRefunds.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if !idsOf(due)[claim.AttemptID] {
		t.Fatal("an abandoned claim must become due for retry, or the client is owed money nothing will ever return")
	}

	// The reaper has to be able to work it without going back to any state the
	// dead process held: everything the retry needs is on the row.
	for _, fr := range due {
		if fr.ID != claim.AttemptID {
			continue
		}
		if fr.PaymentID != payment.ID || fr.BookingID != booking.ID {
			t.Errorf("the queued attempt names the wrong rows: %+v", fr)
		}
		if fr.MPPaymentID != mpPaymentID {
			t.Errorf("the queued attempt must carry the MercadoPago id; want %q, got %q", mpPaymentID, fr.MPPaymentID)
		}
		if fr.Amount != claim.RefundCentavos {
			t.Errorf("the queued attempt must carry the claimed amount; want %d, got %d", claim.RefundCentavos, fr.Amount)
		}
	}
}

// Two cancellation paths can reach the same booking at the same time — the owner
// cancelling in the dashboard, the client replying NO on WhatsApp, the cron
// releasing an expired booking. Only one of them may send money back.
//
// The two claims run as real goroutines against real connections, because that is
// the only version of this test that exercises the row lock; running them one
// after the other would pass on any implementation that merely re-reads the row.
func TestTwoConcurrentClaimsOnlyOneWins(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	type result struct {
		claim *paymentstore.RefundClaim
		err   error
	}
	results := make([]result, 2)

	var start sync.WaitGroup
	start.Add(1)
	var done sync.WaitGroup
	done.Add(2)

	for i := range results {
		go func() {
			defer done.Done()
			start.Wait() // release both goroutines as close to together as possible
			claim, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID)
			results[i] = result{claim: claim, err: err}
		}()
	}
	start.Done()
	done.Wait()

	var won, refused int
	for _, r := range results {
		switch {
		case r.err == nil:
			won++
		case errors.Is(r.err, paymentstore.ErrRefundInFlight):
			refused++
		default:
			t.Errorf("a losing claim must be refused with ErrRefundInFlight; got %v", r.err)
		}
	}
	if won != 1 {
		t.Fatalf("exactly one claim may win, or the client is refunded twice; %d won", won)
	}
	if refused != 1 {
		t.Fatalf("the losing claim must be refused, not silently succeed; %d refused", refused)
	}

	// One winner means one attempt row: a second queued attempt would be a second
	// refund waiting to be sent by the retry job.
	var attempts int
	if err := f.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM failed_refunds WHERE payment_id = $1`, payment.ID,
	).Scan(&attempts); err != nil {
		t.Fatalf("counting queued attempts: %v", err)
	}
	if attempts != 1 {
		t.Errorf("want exactly one queued attempt for one payment; got %d", attempts)
	}
}

// The recording step is one transaction covering three rows, so that a crash can
// never leave two of them agreeing and the third not. In particular the attempt is
// resolved last: the retry job used to close it first, from a separate statement,
// so a crash in between left a refunded client, a court that still read as sold,
// and an attempt already marked done.
func TestRecordRefundSuccessWritesTheMoneyAndTheBookingAndTheAttempt(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	claim, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	refundTotal, err := f.Stores.Payments.RecordRefundSuccess(ctx, *claim, 0)
	if err != nil {
		t.Fatalf("RecordRefundSuccess: %v", err)
	}
	if want := 157_500; refundTotal != want {
		t.Errorf("the recorded total must be the deposit plus the service fee; want %d, got %d", want, refundTotal)
	}

	status, refundAmount := f.ReadPaymentState(t, payment.ID)
	if status != "refunded" {
		t.Errorf("the stored payment must read as refunded; got %q", status)
	}
	if refundAmount != refundTotal {
		t.Errorf("the stored refund amount must be %d; got %d", refundTotal, refundAmount)
	}

	bookingStatus, bookingCollection, bookingRefund := f.ReadBookingState(t, booking.ID)
	if bookingStatus != "cancelled" {
		t.Errorf("a fully refunded booking must be cancelled, or the court stays sold; got %q", bookingStatus)
	}
	if bookingRefund != bookingstore.RefundStatusFull {
		t.Errorf("the stored booking refund status must be full; got %q", bookingRefund)
	}
	// The payment_status split's whole point: the refund records itself on its own axis
	// and does not overwrite what the booking collected. This fixture took a
	// deposit, so the row must still say so after the money went back.
	if bookingCollection != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("refunding must not erase what was collected; want deposit_paid, got %q", bookingCollection)
	}

	// UpdateBooking always writes deposit_amount, so any path that omits it stores
	// 0. Refunding must not erase the record of what the client originally paid.
	var storedDeposit int
	if err := f.Pool.QueryRow(ctx,
		`SELECT deposit_amount FROM bookings WHERE id = $1`, booking.ID,
	).Scan(&storedDeposit); err != nil {
		t.Fatalf("reading the booking deposit back: %v", err)
	}
	if storedDeposit != booking.DepositAmount {
		t.Errorf("a refund must preserve the original deposit; want %d, got %d",
			booking.DepositAmount, storedDeposit)
	}

	attempt := readRefundAttempt(f, t, claim.AttemptID)
	if attempt.status != "resolved" {
		t.Errorf("the attempt must be resolved once the money state is written; got %q", attempt.status)
	}
	if attempt.resolvedAt == nil {
		t.Error("a resolved attempt must record when it was resolved")
	}
}

// A booking can carry a deposit paid through MercadoPago and a balance the
// owner confirmed in cash (ConfirmPayment inserts a second row rather than
// replacing the first). Before partial_refund existed, cancelRefundedBooking wrote
// the booking as fully refunded the moment the MercadoPago row's own balance
// reached zero, with no knowledge of that sibling cash row — so a split refund
// read 'refunded' in the database while cash was still owed.
// manualOwedCentavos is what tells RecordRefundSuccess to write refund_status
// 'partial' instead.
func TestRecordRefundSuccessWritesPartialRefundWhenCashIsStillOwed(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	mpPayment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)
	cashPayment := f.CreatePayment(t, booking.ID, 50_000, 0, nil)

	claim, err := f.Stores.Payments.ClaimRefund(ctx, mpPayment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	manualOwed := cashPayment.Amount + cashPayment.ServiceFee - cashPayment.RefundAmount
	refundTotal, err := f.Stores.Payments.RecordRefundSuccess(ctx, *claim, manualOwed)
	if err != nil {
		t.Fatalf("RecordRefundSuccess: %v", err)
	}
	if want := 157_500; refundTotal != want {
		t.Errorf("the recorded total for the MercadoPago row must be the deposit plus the service fee; want %d, got %d", want, refundTotal)
	}

	mpStatus, mpRefundAmount := f.ReadPaymentState(t, mpPayment.ID)
	if mpStatus != "refunded" {
		t.Errorf("the mercadopago row must read as refunded; got %q", mpStatus)
	}
	if mpRefundAmount != refundTotal {
		t.Errorf("the mercadopago row's refund amount must be %d; got %d", refundTotal, mpRefundAmount)
	}

	// The cash row is untouched — nothing automatic may send this money back.
	cashStatus, cashRefundAmount := f.ReadPaymentState(t, cashPayment.ID)
	if cashStatus == "refunded" {
		t.Error("the cash row must not read as refunded; nothing here returned that money")
	}
	if cashRefundAmount != 0 {
		t.Errorf("the cash row's refund amount must be untouched; got %d", cashRefundAmount)
	}

	bookingStatus, bookingCollection, bookingRefund := f.ReadBookingState(t, booking.ID)
	if bookingStatus != "cancelled" {
		t.Errorf("a refunded booking must be cancelled; got %q", bookingStatus)
	}
	if bookingRefund != bookingstore.RefundStatusPartial {
		t.Errorf("a booking with cash still owed must read refund_status 'partial', not 'full'; got %q", bookingRefund)
	}
	if bookingCollection != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("a partial refund must not erase what was collected; want deposit_paid, got %q", bookingCollection)
	}
}

// RecordManualRefund is the write behind the owner confirming, by hand, that
// they returned a partially refunded booking's remaining cash balance. It has
// to close out every unrefunded manual row and move the booking's refund_status
// to 'full' in the one transaction, or a crash between the two would leave either the
// payment ledger or the booking disagreeing about whether this money is
// still owed.
func TestRecordManualRefund(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	mpPayment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)
	cashPayment := f.CreatePayment(t, booking.ID, 50_000, 0, nil)

	claim, err := f.Stores.Payments.ClaimRefund(ctx, mpPayment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}
	manualOwed := cashPayment.Amount + cashPayment.ServiceFee - cashPayment.RefundAmount
	if _, err := f.Stores.Payments.RecordRefundSuccess(ctx, *claim, manualOwed); err != nil {
		t.Fatalf("RecordRefundSuccess: %v", err)
	}

	if _, _, refundStatus := f.ReadBookingState(t, booking.ID); refundStatus != bookingstore.RefundStatusPartial {
		t.Fatalf("setup: booking must read refund_status 'partial' before RecordManualRefund runs; got %q", refundStatus)
	}

	returned, err := f.Stores.Payments.RecordManualRefund(ctx, booking.ID)
	if err != nil {
		t.Fatalf("RecordManualRefund: %v", err)
	}
	if returned != manualOwed {
		t.Errorf("the returned amount must be the cash row's own balance; want %d, got %d", manualOwed, returned)
	}

	cashStatus, cashRefundAmount := f.ReadPaymentState(t, cashPayment.ID)
	if cashStatus != "refunded" {
		t.Errorf("the cash row must now read as refunded; got %q", cashStatus)
	}
	if want := cashPayment.Amount + cashPayment.ServiceFee; cashRefundAmount != want {
		t.Errorf("the cash row's refund amount must be its amount plus service fee; want %d, got %d", want, cashRefundAmount)
	}

	bookingStatus, bookingCollection, bookingRefund := f.ReadBookingState(t, booking.ID)
	if bookingStatus != "cancelled" {
		t.Errorf("the booking must stay cancelled; got %q", bookingStatus)
	}
	if bookingRefund != bookingstore.RefundStatusFull {
		t.Errorf("the booking must now read a full refund; got %q", bookingRefund)
	}
	if bookingCollection != bookingstore.CollectionStatusDepositPaid {
		t.Errorf("closing out the manual balance must not erase what was collected; want deposit_paid, got %q", bookingCollection)
	}

	// A second call finds nothing left to close out.
	if _, err := f.Stores.Payments.RecordManualRefund(ctx, booking.ID); !errors.Is(err, paymentstore.ErrNoManualRefundOwed) {
		t.Errorf("a booking that no longer reads refund_status 'partial' must be refused; got %v", err)
	}
}

// The provider refusing is the ordinary case, not a disaster: the client is still
// owed their money, so the attempt stays queued with its backoff and nothing may
// read as refunded. There is no rollback to perform — the claim never claimed the
// refund had happened, only that one was owed.
func TestRecordRefundFailureLeavesTheAttemptRetryable(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	booking := f.CreateBooking(t, datatest.BookingOptions{})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	claim, err := f.Stores.Payments.ClaimRefund(ctx, payment.ID)
	if err != nil {
		t.Fatalf("ClaimRefund: %v", err)
	}

	before := readRefundAttempt(f, t, claim.AttemptID)

	exhausted, err := f.Stores.Payments.RecordRefundFailure(ctx, *claim, "mercadopago unavailable")
	if err != nil {
		t.Fatalf("RecordRefundFailure: %v", err)
	}
	if exhausted {
		t.Error("one rejection out of five must not exhaust the retry budget")
	}

	after := readRefundAttempt(f, t, claim.AttemptID)
	if after.status != "pending" {
		t.Errorf("a rejected refund must stay queued; got status %q", after.status)
	}
	if after.retryCount != before.retryCount+1 {
		t.Errorf("the attempt must count the failed try; want %d, got %d", before.retryCount+1, after.retryCount)
	}
	if !after.nextRetryAt.After(time.Now()) {
		t.Errorf("the next attempt must be scheduled into the future, not retried immediately; got %s", after.nextRetryAt)
	}
	if !after.nextRetryAt.After(before.nextRetryAt) {
		t.Errorf("the retry must back off rather than repeat on the same schedule; was %s, now %s",
			before.nextRetryAt, after.nextRetryAt)
	}
	if after.errorMessage != "mercadopago unavailable" {
		t.Errorf("the attempt must record why the provider refused; got %q", after.errorMessage)
	}

	status, refundAmount := f.ReadPaymentState(t, payment.ID)
	if status == "refunded" {
		t.Error("a refund the provider refused must never read as refunded")
	}
	if status != "refund_pending" {
		t.Errorf("the payment must stay claimed while the refund is still owed; got %q", status)
	}
	if refundAmount != 0 {
		t.Errorf("no money came back, so nothing may be recorded as refunded; got %d", refundAmount)
	}

	bookingStatus, _, bookingRefund := f.ReadBookingState(t, booking.ID)
	if bookingRefund == bookingstore.RefundStatusFull || bookingStatus == "cancelled" {
		t.Errorf("the booking must not be settled by a refund that never happened; got status=%q refund_status=%q",
			bookingStatus, bookingRefund)
	}
}

// separateConn opens a connection outside the pool the stores use.
//
// The refund tests need a reader that cannot possibly be sharing a transaction
// with the code under test: an uncommitted write is invisible to it, and a row
// lock it cannot take is a row lock somebody else is really holding.
func separateConn(f *datatest.Fixture, t *testing.T) *pgx.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("opening a separate connection: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		_ = conn.Close(closeCtx)
	})
	return conn
}

// refundAttemptState is the queue-side state of one attempt, read straight from
// the row rather than from anything a store method returned.
type refundAttemptState struct {
	status       string
	retryCount   int
	nextRetryAt  time.Time
	errorMessage string
	resolvedAt   *time.Time
}

func readRefundAttempt(f *datatest.Fixture, t *testing.T, id uuid.UUID) refundAttemptState {
	t.Helper()

	var state refundAttemptState
	err := f.Pool.QueryRow(context.Background(), `
		SELECT status, retry_count, next_retry_at, COALESCE(error_message, ''), resolved_at
		FROM failed_refunds WHERE id = $1`, id,
	).Scan(&state.status, &state.retryCount, &state.nextRetryAt, &state.errorMessage, &state.resolvedAt)
	if err != nil {
		t.Fatalf("reading refund attempt %s: %v", id, err)
	}
	return state
}

// expireRefundAttempt brings an attempt's retry time forward into the past, which
// is what waiting out its backoff would do. next_retry_at carries no trigger, so
// an ordinary UPDATE is enough here.
func expireRefundAttempt(f *datatest.Fixture, t *testing.T, id uuid.UUID) {
	t.Helper()

	tag, err := f.Pool.Exec(context.Background(),
		`UPDATE failed_refunds SET next_retry_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, id)
	if err != nil {
		t.Fatalf("expiring refund attempt %s: %v", id, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("expiring refund attempt %s: want 1 row affected, got %d", id, tag.RowsAffected())
	}
}

// ---------------------------------------------------------------------------
// RecordRefundFailure: what a failure costs, and whether it is recorded at all
// ---------------------------------------------------------------------------

// The retry budget bounds how many times we ask a provider that is answering
// us. An attempt that never reached MercadoPago is not evidence about this
// refund, and charging it is what turned one outage of an afternoon into the
// permanent abandonment of every queued refund at once — five attempts at
// 1m/5m/15m/1h/4h elapse in five hours and twenty minutes whatever the reason.
//
// paymentstore.FailedRefunds.IncrementRetry already had this; the recorder on the money
// path did not, and it is the one the refund handler calls.
func TestAProviderOutageDoesNotSpendAClaimedRefundsRetryBudget(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	claim := claimRefund(f, t, "08:00", "09:30")
	// Two attempts have already been made and answered, so the escalating table
	// and the flat outage probe are far enough apart to tell apart: the next
	// backoff for an answer is an hour.
	setRefundRetryCount(f, t, claim.AttemptID, 2)

	exhausted, err := f.Stores.Payments.RecordRefundFailure(ctx, *claim,
		"mp: refund request failed: Post \"https://api.mercadopago.com/v1/payments/1/refunds\": dial tcp: i/o timeout")
	if err != nil {
		t.Fatalf("RecordRefundFailure: %v", err)
	}
	if exhausted {
		t.Error("a provider that never answered must not exhaust a budget meant for answers")
	}

	after := readRefundAttempt(f, t, claim.AttemptID)
	if after.retryCount != 2 {
		t.Errorf("a provider outage spent a retry; want the count left at 2, got %d — five hours of one "+
			"outage still abandons every queued refund", after.retryCount)
	}
	if after.status != "pending" {
		t.Errorf("want the attempt left queued; got %q", after.status)
	}
	if wait := time.Until(after.nextRetryAt); wait > paymentstore.ProviderOutageRetryDelayForTest+time.Minute {
		t.Errorf("want the flat %v outage probe; the attempt waits %v, which is the escalating backoff for "+
			"an answer it never got", paymentstore.ProviderOutageRetryDelayForTest, wait)
	}
}

// The provider refusing is an answer about this refund, so it still spends the
// budget and still escalates. The outage branch must not swallow the ordinary
// case it was added beside.
func TestARefusedRefundStillSpendsTheBudget(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	claim := claimRefund(f, t, "08:00", "09:30")
	setRefundRetryCount(f, t, claim.AttemptID, 2)

	if _, err := f.Stores.Payments.RecordRefundFailure(ctx, *claim,
		"mp: refund failed with status 400: the payment cannot be refunded"); err != nil {
		t.Fatalf("RecordRefundFailure: %v", err)
	}

	after := readRefundAttempt(f, t, claim.AttemptID)
	if after.retryCount != 3 {
		t.Errorf("a refusal is an answer about this refund and must spend a retry; want 3, got %d", after.retryCount)
	}
	if wait := time.Until(after.nextRetryAt); wait <= paymentstore.ProviderOutageRetryDelayForTest+time.Minute {
		t.Errorf("want the escalating backoff for an answered attempt; the attempt waits only %v", wait)
	}
}

// The case paymentstore.FailedRefunds.IncrementRetry never had to face. Both queues now
// re-select 'exhausted' rows once next_retry_at elapses, so an exhausted attempt
// is retried and can fail again on an outage — and the outage branch has to
// decide whether that flips it back to 'pending'.
//
// It must not. Nothing was earned back: the count is still at the limit, and
// 'exhausted' is the only signal that a refund needs a person — it is what
// cronReportQueueDepth alerts on. Downgrading it during an outage would silence
// that alarm for exactly as long as the outage lasts, which is when money owed
// is piling up fastest. The row is retried either way; the status only decides
// whether anybody is told.
func TestAnExhaustedRefundStaysExhaustedThroughAProviderOutage(t *testing.T) {
	f := datatest.NewFixture(t)
	ctx := context.Background()

	claim := claimRefund(f, t, "08:00", "09:30")
	exhaustRefund(f, t, claim.AttemptID)
	before := readRefundAttempt(f, t, claim.AttemptID)

	exhausted, err := f.Stores.Payments.RecordRefundFailure(ctx, *claim, "mp: circuit breaker is open")
	if err != nil {
		t.Fatalf("RecordRefundFailure: %v", err)
	}
	if !exhausted {
		t.Error("the caller must still be told the budget is spent, or the refund reads as automatically retryable")
	}

	after := readRefundAttempt(f, t, claim.AttemptID)
	if after.status != "exhausted" {
		t.Errorf("an outage downgraded an exhausted attempt to %q, silencing the only alarm an operator gets "+
			"for money that needs a person", after.status)
	}
	if after.retryCount != before.retryCount {
		t.Errorf("the outage spent a retry on an already-spent budget; want the count left at %d, got %d",
			before.retryCount, after.retryCount)
	}
}

// The moment this write matters most is the moment the caller has nothing left:
// the MercadoPago call used up the whole context and returned a timeout. Riding
// that spent context records nothing, so the attempt stays in 'processing' with
// no backoff and no recorded reason — it reads as abandoned rather than failed,
// and is invisible to every instance until it goes stale.
func TestARefundFailureIsRecordedEvenWhenTheCallerIsAlreadyDone(t *testing.T) {
	f := datatest.NewFixture(t)

	claim := claimRefund(f, t, "08:00", "09:30")
	before := readRefundAttempt(f, t, claim.AttemptID)

	spent, cancel := context.WithCancel(context.Background())
	cancel()

	const cause = "mp: refund failed with status 400: the payment cannot be refunded"
	if _, err := f.Stores.Payments.RecordRefundFailure(spent, *claim, cause); err != nil {
		t.Fatalf("the failure went unrecorded because the caller's context was already spent, which is "+
			"precisely when a refund fails: %v", err)
	}

	after := readRefundAttempt(f, t, claim.AttemptID)
	if after.retryCount != before.retryCount+1 {
		t.Errorf("the attempt did not count the failed try; want %d, got %d", before.retryCount+1, after.retryCount)
	}
	if after.errorMessage != cause {
		t.Errorf("the attempt must record why the refund failed; got %q", after.errorMessage)
	}
	if !after.nextRetryAt.After(time.Now()) {
		t.Errorf("the next attempt must be scheduled into the future; got %s", after.nextRetryAt)
	}
}

// claimRefund puts a real payment and booking behind a committed refund claim,
// which is the state every RecordRefundFailure case starts from. The slot times
// are explicit because several claims in one test share a court and date, and
// bookings(court_id, date, start_time) is uniquely indexed.
func claimRefund(f *datatest.Fixture, t *testing.T, startTime, endTime string) *paymentstore.RefundClaim {
	t.Helper()

	booking := f.CreateBooking(t, datatest.BookingOptions{StartTime: startTime, EndTime: endTime})
	mpPaymentID := "mp-" + uuid.NewString()
	payment := f.CreatePayment(t, booking.ID, 150_000, 7_500, &mpPaymentID)

	claim, err := f.Stores.Payments.ClaimRefund(context.Background(), payment.ID)
	if err != nil {
		t.Fatalf("claiming a refund on payment %s: %v", payment.ID, err)
	}
	return claim
}

// setRefundRetryCount puts an attempt part-way through its budget, as a run of
// answered failures would.
func setRefundRetryCount(f *datatest.Fixture, t *testing.T, id uuid.UUID, count int) {
	t.Helper()

	tag, err := f.Pool.Exec(context.Background(),
		`UPDATE failed_refunds SET retry_count = $2 WHERE id = $1`, id, count)
	if err != nil {
		t.Fatalf("setting retry_count on %s: %v", id, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("setting retry_count on %s: want 1 row affected, got %d", id, tag.RowsAffected())
	}
}

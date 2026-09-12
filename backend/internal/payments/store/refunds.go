package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// claimedAttemptNote is the error_message a claim writes before the provider has
// been called. The column is named for the failure case because the table is,
// but a row now exists from the moment the refund is claimed rather than only
// after MercadoPago rejects it.
const claimedAttemptNote = "refund claimed, provider call not yet completed"

// firstRetryDelay is how long a claimed attempt stays invisible to the retry
// reaper. It has to outlast one provider call — the MercadoPago client's own
// timeout is 8 seconds — so a healthy in-flight refund is not handed to a second
// worker, while a process that dies mid-refund is picked up a minute later.
const firstRetryDelay = time.Minute

// RefundClaim is a refund this process has reserved and is allowed to send to
// MercadoPago. It is everything the provider call and the recording step need,
// so nothing between them has to re-read the database or trust a caller's
// possibly stale Payment struct.
//
// A claim is durable: by the time ClaimRefund returns, the payment reads
// 'refund_pending' and AttemptID names a committed failed_refunds row. If this
// process dies at any point afterwards, that row is what brings the money back.
type RefundClaim struct {
	// AttemptID is the failed_refunds row recording this attempt.
	AttemptID uuid.UUID
	PaymentID uuid.UUID
	BookingID uuid.UUID
	ComplexID uuid.UUID
	// MPPaymentID is the MercadoPago payment to refund against.
	MPPaymentID string
	// RefundCentavos is the amount to send to the provider, computed under the
	// payment lock at claim time.
	RefundCentavos int
}

// ClaimRefund reserves a payment for refund and commits, in one short transaction.
//
// It replaces the previous AtomicRefund, which returned with its transaction still
// open so the caller could commit after calling MercadoPago. That held a row lock
// and a pooled connection across a synchronous external HTTP call, and left a
// successful refund unrecorded whenever the deferred commit failed. Here the lock
// exists only for the few statements below: the caller gets a committed claim and
// calls the provider with no database resource held at all.
//
// The refundable amount is computed under the FOR UPDATE lock, so two callers
// racing to refund the same payment cannot both compute a full refund: the first
// commits 'refund_pending' and the second is refused with ErrRefundInFlight.
//
// The whole remaining balance is always claimed. Partial refunds arrive from
// MercadoPago's own webhook and are recorded by ConfirmWebhookPayment; nothing in
// this codebase initiates one, so a requested-amount parameter would only be an
// untested path through the money code.
func (m *Payments) ClaimRefund(ctx context.Context, paymentID uuid.UUID) (*RefundClaim, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim transaction: %w", err)
	}
	// Rollback is a no-op once Commit succeeds (pgx returns ErrTxClosed, which is expected).
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := m.Q.WithTx(tx)

	locked, err := qtx.GetPaymentByIDForUpdate(ctx, data.UUIDToPg(paymentID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("lock payment: %w", err)
	}

	mpPaymentID, remaining, err := claimable(locked)
	if err != nil {
		return nil, err
	}

	dbPayment, err := qtx.UpdatePayment(ctx, db.UpdatePaymentParams{
		Status:         db.PaymentStatus("refund_pending"),
		MpPaymentID:    locked.MpPaymentID,
		MpPreferenceID: locked.MpPreferenceID,
		// Written back unchanged: the money has not moved yet, so the amount
		// already refunded must not be touched by the claim.
		RefundAmount: locked.RefundAmount,
		ID:           locked.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("claim payment: %w", err)
	}

	claim := &RefundClaim{
		PaymentID:      data.PgToUUID(dbPayment.ID),
		BookingID:      data.PgToUUID(dbPayment.BookingID),
		ComplexID:      data.PgToUUID(dbPayment.ComplexID),
		MPPaymentID:    mpPaymentID,
		RefundCentavos: remaining,
	}
	if err := recordRefundAttempt(ctx, tx, claim); err != nil {
		return nil, err
	}

	// The claim just committed a durable attempt row, so the intent marker
	// (refund-intent-durability spec) has done its job for this booking: from
	// here the attempt row is the durable record, not the marker. Clearing it
	// inside this same transaction — rather than through ClearRefundIntent,
	// a separate round trip — is what keeps the crash window closed: a crash
	// between this commit and a later clear would reopen exactly the gap the
	// marker exists to cover.
	if err := clearRefundIntent(ctx, tx, claim.BookingID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim transaction: %w", err)
	}
	return claim, nil
}

// clearRefundIntent clears a booking's refund-intent marker inside the
// caller's own transaction, sibling to recordRefundAttempt above. It is
// deliberately raw SQL against bookings rather than a call through
// BookingStore: no store method may run outside the caller's transaction and
// still keep the marker-clear atomic with the claim it belongs to.
func clearRefundIntent(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`UPDATE bookings SET refund_intent_at = NULL WHERE id = $1`,
		bookingID,
	); err != nil {
		return fmt.Errorf("clear refund intent marker: %w", err)
	}
	return nil
}

// claimable decides whether a locked payment may be claimed for refund, and for
// how much. It is the whole of the money-side validation, run under the caller's
// FOR UPDATE lock so its answer cannot go stale before the claim is written.
func claimable(locked db.Payment) (mpPaymentID string, remaining int, err error) {
	// A full refund includes the service fee: it is what the client actually paid.
	totalPaid := int(locked.Amount) + int(locked.ServiceFee)
	// refund_amount is NOT NULL: sqlc's plain int32 for it reads
	// correctly with a direct conversion.
	remaining = totalPaid - int(locked.RefundAmount)

	switch {
	case string(locked.Status) == "refunded" || remaining <= 0:
		return "", 0, ErrAlreadyRefunded
	case string(locked.Status) == "refund_pending":
		// Another claim already committed, and either its provider call is in
		// flight or its queued attempt is waiting for the retry job. Either way the
		// money is on its way back exactly once.
		return "", 0, ErrRefundInFlight
	}

	if !locked.MpPaymentID.Valid || locked.MpPaymentID.String == "" {
		// Recording an attempt nobody can execute would put a permanently
		// unworkable row in the retry queue.
		return "", 0, fmt.Errorf("payment %s carries no mercadopago id to refund against", data.PgToUUID(locked.ID))
	}
	return locked.MpPaymentID.String, remaining, nil
}

// recordRefundAttempt queues the attempt row for a claim, inside the claim's
// transaction, and fills in its id.
//
// Writing it here — before the provider is called — is the whole point of the
// redesign: a row used to land in this table only after MercadoPago had already
// rejected the refund, so a refund that succeeded at the provider and then failed
// to be recorded left nothing behind for anything to retry.
//
// It is queued 'pending' with next_retry_at in the near future rather than
// 'processing', so the reaper leaves a healthy in-flight refund alone and picks up
// an abandoned one on its own schedule (see GetPendingDue).
//
// It is written as SQL here rather than through FailedRefunds because the
// claim and the attempt have to commit together or not at all: a payment moved to
// 'refund_pending' with no queued attempt is money nothing will ever retry.
func recordRefundAttempt(ctx context.Context, tx pgx.Tx, claim *RefundClaim) error {
	err := tx.QueryRow(ctx, `
		INSERT INTO failed_refunds (payment_id, booking_id, complex_id, amount, mp_payment_id, error_message, next_retry_at, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
		RETURNING id`,
		claim.PaymentID, claim.BookingID, claim.ComplexID,
		claim.RefundCentavos, claim.MPPaymentID, claimedAttemptNote,
		time.Now().Add(firstRetryDelay),
	).Scan(&claim.AttemptID)
	if err != nil {
		return fmt.Errorf("record refund attempt: %w", err)
	}
	return nil
}

// RecordRefundSuccess records a refund MercadoPago has already made, in one short
// transaction, and returns the total now refunded on the payment.
//
// The payment, the booking and the attempt move together. The retry job used to
// mark the attempt resolved *before* marking the payment refunded, from two
// separate statements: a crash between them left a client who has their money
// back, a court that still reads as sold, and a closed attempt that nothing would
// ever work again. Here the money state is written first and the attempt is
// resolved last, inside the same transaction, so neither ordering nor a crash can
// produce that state.
//
// Nothing is rolled back on failure at this point — the money has already moved.
// A failure here leaves the attempt queued exactly as the claim left it, and the
// retry job replays the provider call. That replay is safe because the refund's
// MercadoPago idempotency key is derived from the payment id and the amount (see
// internal/mp/mp.go), so a repeated call returns the original refund instead of
// sending the money a second time.
//
// manualOwedCentavos is the booking's cash/transfer balance still owed by
// hand, as the caller's own refundable()/manualBalance already computed it.
// cancelRefundedBooking used to write bookings.payment_status = 'refunded'
// unconditionally the moment this payment's own balance reached zero, with no
// knowledge of a sibling cash row ConfirmPayment may have inserted on the
// same booking — so a split refund read 'refunded' in the database while
// cash was still owed. A non-zero manualOwedCentavos here is what tells it to
// write 'partial_refund' instead.
func (m *Payments) RecordRefundSuccess(ctx context.Context, claim RefundClaim, manualOwedCentavos int) (int, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin record transaction: %w", err)
	}
	// Rollback is a no-op once Commit succeeds (pgx returns ErrTxClosed, which is expected).
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := m.Q.WithTx(tx)

	locked, err := qtx.GetPaymentByIDForUpdate(ctx, data.UUIDToPg(claim.PaymentID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, data.ErrRecordNotFound
		}
		return 0, fmt.Errorf("lock payment: %w", err)
	}

	totalPaid := int(locked.Amount) + int(locked.ServiceFee)
	// refund_amount is NOT NULL: sqlc's plain int32 for it reads
	// correctly with a direct conversion.
	refundTotal := min(int(locked.RefundAmount)+claim.RefundCentavos, totalPaid)
	isFullRefund := refundTotal >= totalPaid

	// A payment left in 'refund_pending' after the money came back would read as
	// still owing a refund forever, so the claim's marker is always cleared here.
	newStatus := "deposit_paid"
	if isFullRefund {
		newStatus = "refunded"
	}

	if _, err = qtx.UpdatePayment(ctx, db.UpdatePaymentParams{
		Status:         db.PaymentStatus(newStatus),
		MpPaymentID:    locked.MpPaymentID,
		MpPreferenceID: locked.MpPreferenceID,
		//nolint:gosec // G115: currency amount (cents), capped at totalPaid above; far below int32 range.
		// refund_amount is NOT NULL, so sqlc generates plain int32 rather than pgtype.Int4.
		RefundAmount: int32(refundTotal),
		ID:           locked.ID,
	}); err != nil {
		return 0, fmt.Errorf("record refunded payment: %w", err)
	}

	if isFullRefund {
		if err := cancelRefundedBooking(ctx, qtx, claim.BookingID, manualOwedCentavos > 0); err != nil {
			return 0, err
		}
	}

	// Resolved last: the attempt is the record that this refund still needs
	// working, and it may only be closed once the money state above is written.
	if claim.AttemptID != uuid.Nil {
		if _, err = tx.Exec(ctx,
			`UPDATE failed_refunds SET status = 'resolved', resolved_at = NOW() WHERE id = $1`,
			claim.AttemptID,
		); err != nil {
			return 0, fmt.Errorf("resolve refund attempt: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit record transaction: %w", err)
	}
	return refundTotal, nil
}

// cancelRefundedBooking cancels the booking behind a fully refunded payment,
// inside the caller's transaction.
//
// The row is read back under its own FOR UPDATE lock rather than taken from a
// caller-supplied struct that was loaded before the provider call. UpdateBooking
// always writes deposit_amount, so a stale struct would write a zero deposit
// over the record of what the client actually paid. The lock, not a version
// counter, is what serializes this against a concurrent booking update; see
// the bookings section of db/migrations/001_init.sql.
//
// manualBalanceRemains is whether this booking still carries a cash/transfer
// row nothing here can send back, per RecordRefundSuccess's own
// manualOwedCentavos. It writes refund_status 'partial' rather than 'full' when
// that balance remains — see the payment_status enum in db/migrations/001_init.sql for why "refunded" used to be wrong
// for exactly this booking shape.
//
// collection_status is written back from the locked row, unchanged. That is the
// half the payment_status split recovered: how much was collected and where the give-back
// stands are two facts, and refunding a booking that only ever took a deposit
// used to overwrite the first with the second.
func cancelRefundedBooking(ctx context.Context, qtx *db.Queries, bookingID uuid.UUID, manualBalanceRemains bool) error {
	locked, err := qtx.GetBookingByIDForUpdate(ctx, data.UUIDToPg(bookingID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("lock booking: %w", err)
	}

	newRefundStatus := bookingstore.RefundStatusFull
	if manualBalanceRemains {
		newRefundStatus = bookingstore.RefundStatusPartial
	}

	if _, err = qtx.UpdateBooking(ctx, db.UpdateBookingParams{
		Status:           db.BookingStatus("cancelled"),
		CollectionStatus: locked.CollectionStatus,
		RefundStatus:     newRefundStatus,
		Notes:            locked.Notes,
		DepositAmount:    locked.DepositAmount,
		ID:               locked.ID,
		// Written back unchanged: locked is read fresh under FOR UPDATE, so this
		// carries through whatever ClaimRefund already cleared inside its own
		// committed transaction (below), never a stale caller-supplied value.
		RefundIntentAt: locked.RefundIntentAt,
	}); err != nil {
		return fmt.Errorf("cancel refunded booking: %w", err)
	}
	return nil
}

// RecordRefundFailure leaves a claimed refund queued for another attempt, in one
// short transaction, and reports whether the retry budget is now spent.
//
// The payment deliberately stays in 'refund_pending': the client is still owed
// their money, and the attempt row this updates is what the retry job works from.
// Nothing here is a rollback — there is nothing to undo, because the claim never
// pretended the refund had happened.
//
// It runs on a context detached from the caller's cancellation, bounded by the
// ordinary transaction budget — the same shape as DetachedQueryContext, one size
// up because this is a transaction, and the same treatment
// WebhookEvents.MarkFailed already has. The reason is that the moment this
// write matters most is the moment the caller has nothing left: the MercadoPago
// call itself just timed out, or the sweep hit its two-minute budget with this
// attempt claimed. Riding the spent context there records nothing at all, so the
// attempt reads as abandoned in 'processing' rather than as failed — no backoff,
// no recorded reason, and invisible to every instance until it goes stale.
//
// WithoutCancel alone would not do: it strips the parent's deadline along with
// its cancellation, and nothing else bounds a statement in this repository — no
// statement_timeout is set anywhere — so the TxContext wrapper is what keeps a
// wedged PostgreSQL from holding this goroutine and its pooled connection
// forever. See DetachedQueryContext, where this codebase hit that trap once.
func (m *Payments) RecordRefundFailure(ctx context.Context, claim RefundClaim, cause string) (bool, error) {
	ctx, cancel := data.TxContext(context.WithoutCancel(ctx))
	defer cancel()

	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin requeue transaction: %w", err)
	}
	// Rollback is a no-op once Commit succeeds (pgx returns ErrTxClosed, which is expected).
	defer func() { _ = tx.Rollback(ctx) }()

	// Read under lock rather than from the claim: the retry budget belongs to the
	// attempt row, and two workers reclaiming the same abandoned attempt must not
	// both compute the same "next" retry count from the same stale copy.
	var retryCount, maxRetries int
	err = tx.QueryRow(ctx,
		`SELECT retry_count, max_retries FROM failed_refunds WHERE id = $1 FOR UPDATE`,
		claim.AttemptID,
	).Scan(&retryCount, &maxRetries)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, data.ErrRecordNotFound
		}
		return false, fmt.Errorf("lock refund attempt: %w", err)
	}

	newRetryCount := retryCount + 1
	delay := retryBackoff(newRetryCount)

	// A failure that was the provider being unreachable does not spend a retry.
	// The budget bounds how many times we ask a provider that is answering us; an
	// attempt that never got an answer is not evidence about this refund, and
	// charging it is what turned a single MercadoPago outage of an afternoon into
	// the permanent abandonment of every queued refund at once — five attempts at
	// 1m/5m/15m/1h/4h elapse in five hours and twenty minutes whatever the reason
	// for them. The row comes back on the flat outage probe instead. See
	// transientProviderFailure, and FailedRefunds.IncrementRetry, which is the
	// same policy on the other recorder of this table.
	if transientProviderFailure(cause) {
		newRetryCount = retryCount
		delay = providerOutageDelay()
	}

	exhausted := newRetryCount >= maxRetries

	// The status is still derived from the budget, and that settles the one case
	// the sibling never had to face: this row may already be 'exhausted', because
	// both queues now re-select exhausted rows once next_retry_at elapses, so an
	// exhausted attempt is retried and can fail again on an outage.
	//
	// It stays 'exhausted'. Flipping it back to 'pending' would be the shorter
	// code and it would be wrong: nothing was earned back — the count is still at
	// the limit — and 'exhausted' is the only signal an operator gets that a
	// refund needs a person, since cronReportQueueDepth alerts on exactly that
	// count. Downgrading it during an outage would silence that alarm for as long
	// as the outage lasts, which is when money owed piles up fastest. The row is
	// retried either way; the status decides only whether anybody is told.
	status := "pending"
	if exhausted {
		status = "exhausted"
	}

	if _, err = tx.Exec(ctx, `
		UPDATE failed_refunds
		SET retry_count = $2, error_message = $3, next_retry_at = $4, status = $5
		WHERE id = $1`,
		claim.AttemptID, newRetryCount, cause,
		time.Now().Add(delay), status,
	); err != nil {
		return false, fmt.Errorf("requeue refund attempt: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit requeue transaction: %w", err)
	}
	return exhausted, nil
}

// RecordManualRefund closes out a partially refunded booking's remaining
// cash/transfer rows once the owner confirms, by hand, that they returned
// that money to the client. It is the write behind
// POST .../manual-refund (internal/bookings/actions.go ManualRefund) — the
// only path allowed to move a booking off refund_status 'partial'.
//
// Per unrefunded row carrying no MercadoPago id, it writes exactly what
// RecordRefundSuccess writes for a full automatic refund: RefundAmount =
// Amount + ServiceFee, Status = 'refunded'. The
// payments_refund_within_amount_paid CHECK bounds it the
// same way a MercadoPago-backed refund is bounded.
//
// The booking is locked first, exactly as cancelRefundedBooking locks it,
// and every row write plus the booking's own move to 'refunded' commit
// together: nothing can observe the payments marked returned while the
// booking still reads refund_status 'partial', or the reverse. A booking that
// does not read 'partial' under that lock — never did, or another request
// already closed it out — is refused with ErrNoManualRefundOwed rather than
// silently doing nothing.
func (m *Payments) RecordManualRefund(ctx context.Context, bookingID uuid.UUID) (int, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin manual refund transaction: %w", err)
	}
	// Rollback is a no-op once Commit succeeds (pgx returns ErrTxClosed, which is expected).
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := m.Q.WithTx(tx)

	locked, err := qtx.GetBookingByIDForUpdate(ctx, data.UUIDToPg(bookingID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, data.ErrRecordNotFound
		}
		return 0, fmt.Errorf("lock booking: %w", err)
	}
	if locked.RefundStatus != bookingstore.RefundStatusPartial {
		return 0, ErrNoManualRefundOwed
	}

	rows, err := qtx.ListPaymentsByBookingID(ctx, data.UUIDToPg(bookingID))
	if err != nil {
		return 0, fmt.Errorf("list payments for manual refund: %w", err)
	}

	returned, err := applyManualRefundRows(ctx, qtx, rows)
	if err != nil {
		return 0, err
	}

	if _, err = qtx.UpdateBooking(ctx, db.UpdateBookingParams{
		Status:           locked.Status,
		CollectionStatus: locked.CollectionStatus,
		RefundStatus:     bookingstore.RefundStatusFull,
		Notes:            locked.Notes,
		DepositAmount:    locked.DepositAmount,
		ID:               locked.ID,
		// Written back unchanged: locked is read fresh under FOR UPDATE.
		RefundIntentAt: locked.RefundIntentAt,
	}); err != nil {
		return 0, fmt.Errorf("mark booking refunded after manual refund: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit manual refund transaction: %w", err)
	}
	return returned, nil
}

// applyManualRefundRows writes 'refunded' onto every unrefunded payment row
// carrying no MercadoPago id, inside RecordManualRefund's transaction, and
// returns the total moved. It is split out from RecordManualRefund only to
// keep that function's own shape — lock, list, write, commit — readable in
// one screen; the loop itself is the one step that touches more than one row.
func applyManualRefundRows(ctx context.Context, qtx *db.Queries, rows []db.Payment) (int, error) {
	returned := 0
	for _, row := range rows {
		if row.MpPaymentID.Valid && row.MpPaymentID.String != "" {
			// Automatic rows are the refund pipeline's own business, already
			// settled by the time this booking could read 'partial_refund'.
			continue
		}
		owed := int(row.Amount) + int(row.ServiceFee) - int(row.RefundAmount)
		if string(row.Status) == "refunded" || owed <= 0 {
			continue
		}
		if _, err := qtx.UpdatePayment(ctx, db.UpdatePaymentParams{
			Status:         db.PaymentStatus("refunded"),
			MpPaymentID:    row.MpPaymentID,
			MpPreferenceID: row.MpPreferenceID,
			//nolint:gosec // G115: currency amount (cents), bounded by payments_refund_within_amount_paid; far below int32 range.
			RefundAmount: row.Amount + row.ServiceFee,
			ID:           row.ID,
		}); err != nil {
			return 0, fmt.Errorf("record manual refund of payment %s: %w", data.PgToUUID(row.ID), err)
		}
		returned += owed
	}
	return returned, nil
}

// RefundResult is what became of the money on a cancelled booking.
//
// It exists because the refund entry point used to return nothing at all: it had
// four early returns — no payment row, no MercadoPago id, already refunded, and a
// claim the database refused — and every caller that cancelled a booking then had
// to guess which of them had happened. Two of those returns were silent, so a
// refund the system could not make left no log, no alert and nothing queued.
//
// The set is deliberately six values rather than a bool or an error: each one is
// a different sentence to the client and a different instruction to the operator.
// "Failed" is not among them on purpose — a refund that cannot be made
// automatically is not waiting to be retried, it is waiting for a person, and
// calling it a failure invites the retry that will never work.
type RefundResult string

const (
	// RefundNone means nothing was ever paid, so nothing comes back.
	RefundNone RefundResult = "none"
	// RefundNotEligible means money was paid and is deliberately kept, because the
	// cancellation fell outside the complex's refund window. Only a caller that
	// owns that policy returns this; the refund path itself never does.
	RefundNotEligible RefundResult = "not_eligible"
	// RefundIssued means MercadoPago accepted the refund and this system recorded it.
	RefundIssued RefundResult = "issued"
	// RefundAlreadyIssued means the money was already back before this call.
	RefundAlreadyIssued RefundResult = "already_issued"
	// RefundQueued means the money is durably owed and something automatic will
	// finish returning it — a queued attempt with its retry budget, or a claim
	// another path already holds.
	RefundQueued RefundResult = "queued"
	// RefundManual means the money is owed and nothing automatic can return it. Cash
	// or a transfer, a payment carrying no MercadoPago id, a booking that reads
	// as paid with no payment row, a claim nothing durable recorded, or a retry
	// budget that is spent. A person has to return this money, so every branch
	// that produces it also alerts.
	RefundManual RefundResult = "manual"
)

// RefundOutcome is the answer to "what happened to this client's money".
type RefundOutcome struct {
	Result RefundResult
	// AmountCentavos is the sum the Result is about: refunded, queued or owed.
	// Zero when nothing is owed, and zero when the amount could not be read.
	AmountCentavos int
	// ManualAmountCentavos is money owed that nothing in this system can
	// return, alongside whatever AmountCentavos reports. It exists because a
	// booking can carry more than one payment row — a deposit paid through
	// MercadoPago and a balance confirmed in cash — and those two halves can
	// settle differently: the deposit is issued or queued automatically while
	// the cash remainder is owed by hand. Result and AmountCentavos still
	// describe the automatic half exactly as they always have; this field is
	// the only place the manual half is not silently dropped.
	ManualAmountCentavos int
	// Reason is a short operator-facing note. It is logged and alerted on, never
	// shown to a client.
	Reason string
}

// MoneyReturned reports whether the client already has their money back.
func (o RefundOutcome) MoneyReturned() bool {
	return o.Result == RefundIssued || o.Result == RefundAlreadyIssued
}

// MoneyOwed reports whether money is still owed to the client.
func (o RefundOutcome) MoneyOwed() bool {
	return o.Result == RefundQueued || o.Result == RefundManual || o.ManualAmountCentavos > 0
}

// NeedsAHuman reports whether returning this money requires somebody to act.
//
// A split outcome — an MP deposit auto-refunded while a cash balance is
// still owed — reads as RefundIssued, because that is what happened to the
// automatic half. ManualAmountCentavos is what keeps that outcome from
// silently passing as "nothing left to do".
func (o RefundOutcome) NeedsAHuman() bool {
	return o.Result == RefundManual || o.ManualAmountCentavos > 0
}

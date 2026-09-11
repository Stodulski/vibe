package data

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Exponential backoff durations for retry attempts.
var refundRetryBackoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	4 * time.Hour,
}

// staleRefundProcessing is how long a refund attempt may sit in 'processing'
// before the sweeper takes it back.
//
// It only has to outlast one honest attempt, and an honest attempt is bounded by
// the context its caller holds: the retry job's whole run is capped at two
// minutes, and a single MercadoPago call at eight seconds. Five minutes is
// comfortably past both.
//
// It was fifteen. That number was picked when nothing else bounded an attempt,
// and it is the length of time a refund abandoned mid-flight — by a deploy, an
// OOM kill, or simply a sweep that ran out of budget with a row still claimed —
// stays invisible to every instance. Money the client is owed spends that window
// with nothing looking at it, so it is worth being no longer than it has to be.
const staleRefundProcessing = 5 * time.Minute

// refundSweepBatch is how many attempts one sweep reads.
//
// It used to be fifty, which the sweep could not work: it is sequential, its run
// is capped at two minutes, and a MercadoPago call takes about eight seconds, so
// fifteen rows is everything a run can reach. The other thirty-five were fetched,
// scanned and dropped every two minutes.
//
// Twelve is that ceiling with headroom for a slow call, and it costs nothing in
// throughput — what a sweep can drain is set by the budget, not by how many rows
// it read — while removing the wasted work and, more importantly, the row that
// used to be left claimed when the deadline landed mid-batch.
const refundSweepBatch = 12

// providerOutageRetryDelay is how long an attempt waits after a failure that was
// the provider being unreachable rather than the refund being refused.
//
// It is flat rather than exponential on purpose. The escalating table exists to
// protect a provider that is answering us — each rejection is evidence about
// this particular refund — but an unreachable provider tells us nothing about
// the refund at all, and there is nothing in the row that says how long the
// outage will last. A steady five-minute probe recovers promptly once it is
// over, and the MercadoPago client's circuit breaker is what stops the probes
// costing anything while it is not.
const providerOutageRetryDelay = 5 * time.Minute

// providerOutageMarkers are the substrings that identify a recorded failure as
// the provider being unreachable.
//
// Matching on text is not how one would choose to classify an error, and it is
// worth being explicit about why it is what happens here: the cause crosses into
// this package as a string, because that is the column's type and the store
// interface's shape, and the alternative — a typed failure threaded through
// FailedRefundStore, WebhookEventStore and every caller — is a wider change than
// the defect warrants. The markers below are matched against the strings
// internal/mp actually produces, and the failure mode of a miss is the old
// behaviour: the attempt consumes a retry, exactly as it did before.
//
// "seller credential unavailable" is the one entry not produced by
// internal/mp: internal/payments' sellerCredential refuses before MercadoPago
// is ever called when the complex fetch that reads the credential fails, and
// that failure is a transient database read exactly like the others in this
// list — it self-heals, so it must not spend the retry budget either. Only
// this one arm embeds the marker; a credential that exists but will not
// decrypt, or was never linked, is not transient and is deliberately left to
// spend the budget so the retry job keeps alerting until a person acts.
var providerOutageMarkers = []string{
	"circuit breaker is open", // internal/circuitbreaker, wrapped as "mp: %w"
	"request failed",          // "mp: refund request failed: ..." — transport, never an answer
	"connection refused",
	"connection reset",
	"no such host",
	"i/o timeout",
	"tls handshake",
	"context deadline exceeded",
	"unexpected eof",
	"seller credential unavailable",
}

// providerOutageStatus matches the HTTP statuses that mean "ask again later"
// rather than "no": 5xx is the provider's own fault, 429 is it asking us to slow
// down, 408 is it giving up on the request. Every other status is an answer
// about this refund and consumes the budget.
var providerOutageStatus = regexp.MustCompile(`failed with status (5\d\d|429|408)`)

// transientProviderFailure reports whether a recorded cause describes the
// provider being unavailable rather than the payment being refused.
//
// The distinction decides whether an attempt spends part of the retry budget.
// Five attempts at 1m/5m/15m/1h/4h is five hours and twenty minutes of
// wall-clock, and it used to elapse regardless of why the attempts failed — so a
// MercadoPago outage longer than an afternoon abandoned every queued refund and
// every unconfirmed payment at once, permanently, and no endpoint, job or
// command existed to bring any of them back. A budget is meant to bound how many
// times we ask a provider that is answering us; it should not be spent on a
// provider that never answered.
func transientProviderFailure(cause string) bool {
	lowered := strings.ToLower(cause)
	for _, marker := range providerOutageMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return providerOutageStatus.MatchString(lowered)
}

// FailedRefund tracks a refund that failed at the payment provider and is queued for retry with backoff.
type FailedRefund struct {
	ID           uuid.UUID  `json:"id"`
	PaymentID    uuid.UUID  `json:"payment_id"`
	BookingID    uuid.UUID  `json:"booking_id"`
	ComplexID    uuid.UUID  `json:"complex_id"`
	Amount       int        `json:"amount"`
	MPPaymentID  string     `json:"mp_payment_id"`
	ErrorMessage string     `json:"error_message"`
	RetryCount   int        `json:"retry_count"`
	MaxRetries   int        `json:"max_retries"`
	NextRetryAt  time.Time  `json:"next_retry_at"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// FailedRefundModel implements FailedRefundStore against PostgreSQL.
type FailedRefundModel struct {
	DB *DB
}

// Insert records a new failed refund and populates fr with its generated ID and timestamps.
func (m *FailedRefundModel) Insert(ctx context.Context, fr *FailedRefund) error {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	err := m.DB.QueryRow(ctx, `
		INSERT INTO failed_refunds (payment_id, booking_id, complex_id, amount, mp_payment_id, error_message, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		fr.PaymentID, fr.BookingID, fr.ComplexID,
		fr.Amount, fr.MPPaymentID, fr.ErrorMessage, fr.NextRetryAt,
	).Scan(&fr.ID, &fr.CreatedAt, &fr.UpdatedAt)
	return err
}

// GetPendingDue returns failed refunds that are due for retry: the ones waiting
// their turn, the ones abandoned in flight, and the ones whose budget is spent.
//
// MarkProcessing flips a row to 'processing' before calling MercadoPago and sets
// updated_at = NOW(), so updated_at measures the age of the attempt itself. If the
// process crashes, is redeployed, or is OOM-killed between MarkProcessing and the
// terminal transition, nothing ever moves the row out of 'processing' — a
// pending-only filter would leave that owed refund invisible forever, with no log
// and no alert. Rows stuck in 'processing' past the window above are therefore
// reclaimed for another attempt.
//
// 'exhausted' rows are selected for the same reason, one step further on. That
// status means the retry budget ran out, and nothing in this system ever moved a
// row out of it again: no endpoint, no job, no command. It was a permanent
// abandonment of money a client is owed, entered automatically after five hours
// of a provider being unreachable. Including it here makes it a pause instead —
// the row comes back on the tail of its own backoff, which the table caps at four
// hours, so a provider that has started answering finishes the refund by itself
// and one that has not re-alerts four times a day until a person deals with it.
// It is never deleted on a timer either; see the retention sweep below.
func (m *FailedRefundModel) GetPendingDue(ctx context.Context) ([]*FailedRefund, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT id, payment_id, booking_id, complex_id, amount,
		       COALESCE(mp_payment_id, ''), COALESCE(error_message, ''),
		       retry_count, max_retries, next_retry_at, status,
		       created_at, updated_at, resolved_at
		FROM failed_refunds
		WHERE next_retry_at <= NOW()
		  AND (status IN ('pending', 'exhausted')
		       OR (status = 'processing' AND updated_at < NOW() - $1::interval))
		ORDER BY next_retry_at ASC
		LIMIT $2`, staleRefundProcessing.String(), refundSweepBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFailedRefunds(rows)
}

// MarkProcessing takes an attempt for this worker before it calls the provider.
//
// It is a conditional UPDATE rather than an unconditional one, and it matches the
// predicate GetPendingDue selects on: two instances see the same row on the same
// tick, and only the one whose UPDATE matches may go on to MercadoPago. An
// unconditional write let both of them through, and what saved the money was the
// provider's idempotency key rather than anything here.
//
// It reports ErrRecordNotFound when there was no row to take — it does not
// exist, or another worker's attempt on it is still honest. The caller's move in
// either case is the same: leave it alone and take the next row.
func (m *FailedRefundModel) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	tag, err := m.DB.Exec(ctx, `
		UPDATE failed_refunds
		SET status = 'processing', updated_at = NOW()
		WHERE id = $1
		  AND (status IN ('pending', 'exhausted')
		       OR (status = 'processing' AND updated_at < NOW() - $2::interval))`,
		id, staleRefundProcessing.String())
	if err != nil {
		return fmt.Errorf("claim refund attempt: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrRecordNotFound
	}
	return nil
}

// MarkResolved marks a failed refund as successfully completed and records the resolution time.
func (m *FailedRefundModel) MarkResolved(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := detachedQueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`UPDATE failed_refunds SET status = 'resolved', resolved_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

// MarkExhausted marks a failed refund as permanently failed after exceeding its retry budget.
func (m *FailedRefundModel) MarkExhausted(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := detachedQueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`UPDATE failed_refunds SET status = 'exhausted', updated_at = NOW() WHERE id = $1`, id)
	return err
}

// retryBackoff returns how long an attempt waits before its retryCount-th try.
// Past the end of the table every further attempt waits the longest interval.
func retryBackoff(retryCount int) time.Duration {
	if retryCount < len(refundRetryBackoff) {
		return refundRetryBackoff[retryCount]
	}
	return refundRetryBackoff[len(refundRetryBackoff)-1]
}

// IncrementRetry records a failed retry attempt, storing errMsg and scheduling the
// next attempt.
//
// A failure that was the provider being unreachable does not spend a retry: the
// count stays where it was and the row comes back on the flat outage delay. See
// transientProviderFailure for why an outage must not consume a budget meant for
// answers.
//
// The write runs on a context detached from the caller's, because the moment it
// is most needed is the moment the caller has none left: a sweep that hit its
// two-minute budget with this row claimed. Losing this write leaves the row in
// 'processing' with no backoff and no recorded reason, invisible to every
// instance until it goes stale.
func (m *FailedRefundModel) IncrementRetry(ctx context.Context, id uuid.UUID, retryCount int, errMsg string) error {
	ctx, cancel := detachedQueryContext(ctx)
	defer cancel()

	if transientProviderFailure(errMsg) {
		// retry_count is left exactly where it was — the statement does not name
		// it — so the caller's proposed count is discarded rather than adjusted.
		_, err := m.DB.Exec(ctx, `
			UPDATE failed_refunds
			SET error_message = $2,
			    next_retry_at = $3,
			    status = CASE WHEN retry_count >= max_retries THEN 'exhausted' ELSE 'pending' END,
			    updated_at = NOW()
			WHERE id = $1`,
			id, errMsg, time.Now().Add(providerOutageRetryDelay))
		return err
	}

	// The status is derived from the budget rather than written as 'pending'
	// unconditionally: a row whose count has reached its limit is exhausted, and
	// saying otherwise would hide it from the alert that a person acts on.
	_, err := m.DB.Exec(ctx, `
		UPDATE failed_refunds
		SET retry_count = $2,
		    error_message = $3,
		    next_retry_at = $4,
		    status = CASE WHEN $2 >= max_retries THEN 'exhausted' ELSE 'pending' END,
		    updated_at = NOW()
		WHERE id = $1`,
		id, retryCount, errMsg, time.Now().Add(retryBackoff(retryCount)))
	return err
}

// DeleteResolved removes refund attempts that were resolved longer ago than
// olderThan, and returns how many were deleted.
//
// This table is no longer what its name says. Since refunds became claim-first,
// ClaimRefund writes a row before MercadoPago is called, so there is one row per
// refund rather than one per failed refund — and the overwhelming majority
// resolve within seconds and are never read again. Nothing deleted any of them,
// so it grew for the life of the deployment and the sweeper's query got slower
// with it.
//
// Only 'resolved' rows are eligible. A pending, in-flight or exhausted row
// describes money that has not come back yet, and no timer may delete that.
func (m *FailedRefundModel) DeleteResolved(ctx context.Context, olderThan time.Duration) (int64, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	tag, err := m.DB.Exec(ctx,
		`DELETE FROM failed_refunds
		 WHERE status = 'resolved' AND resolved_at < NOW() - $1::interval`,
		olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("delete resolved refund attempts: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanFailedRefunds(rows pgx.Rows) ([]*FailedRefund, error) {
	var result []*FailedRefund
	for rows.Next() {
		var fr FailedRefund
		err := rows.Scan(
			&fr.ID, &fr.PaymentID, &fr.BookingID, &fr.ComplexID,
			&fr.Amount, &fr.MPPaymentID, &fr.ErrorMessage,
			&fr.RetryCount, &fr.MaxRetries, &fr.NextRetryAt, &fr.Status,
			&fr.CreatedAt, &fr.UpdatedAt, &fr.ResolvedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed refund: %w", err)
		}
		result = append(result, &fr)
	}
	return result, rows.Err()
}

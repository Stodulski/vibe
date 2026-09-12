package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// staleWebhookProcessing is how long a webhook event may sit in 'processing'
// before the sweeper takes it back.
//
// The reasoning is the one behind staleRefundProcessing in failed_refunds.go, and
// the window is deliberately the same: Claim flips a row to 'processing' and sets
// updated_at = NOW(), so updated_at measures the age of the attempt itself. If the
// process crashes, is redeployed, or is OOM-killed between the claim and the
// terminal transition, nothing ever moves that row — and for this table an
// abandoned row means a payment that was captured and never confirmed. The window
// only has to outlast one honest attempt, which is bounded at 30 seconds by the
// processing context and at two minutes by the sweep's own budget.
const staleWebhookProcessing = staleRefundProcessing

// webhookSweepBatch is how many events one sweep reads. See refundSweepBatch:
// same sequential loop, same two-minute budget, same MercadoPago call in the
// middle of it, so the same ceiling on what a run can actually reach.
const webhookSweepBatch = refundSweepBatch

// WebhookEvent is one notification a payment provider delivered to us, recorded
// before it is acted on.
//
// The row is written and committed while the provider is still waiting for its
// HTTP response, which is what lets the endpoint answer 200 truthfully: the event
// is ours now, and every failure after that point is retryable from this row
// rather than lost with the goroutine that hit it.
type WebhookEvent struct {
	ID       uuid.UUID `json:"id"`
	Provider string    `json:"provider"`
	// ExternalID is the provider's own id for the subject of the event —
	// MercadoPago's data.id, which is the payment id. It is not unique here: the
	// same payment produces a delivery per status change.
	ExternalID string `json:"external_id"`
	EventType  string `json:"event_type"`
	// Payload is the body exactly as it was delivered, kept for forensics: when a
	// payment goes wrong this is the only record of what the provider actually
	// said, as opposed to what we later read back from its API.
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	RetryCount  int             `json:"retry_count"`
	MaxRetries  int             `json:"max_retries"`
	NextRetryAt time.Time       `json:"next_retry_at"`
	LastError   string          `json:"last_error"`
	ReceivedAt  time.Time       `json:"received_at"`
	ProcessedAt *time.Time      `json:"processed_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// WebhookEvents implements WebhookEventStore against PostgreSQL.
type WebhookEvents struct {
	DB *data.DB
}

// Insert records a delivered event and populates e with its generated ID and
// timestamps.
//
// This is the statement the 200 is worth: it has to be committed before the
// response is written, so a caller that gets an error here must refuse the
// delivery rather than acknowledge it.
func (m *WebhookEvents) Insert(ctx context.Context, e *WebhookEvent) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	provider := e.Provider
	if provider == "" {
		provider = "mercadopago"
	}

	err := m.DB.QueryRow(ctx, `
		INSERT INTO webhook_events (provider, external_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id, provider, status, retry_count, max_retries, next_retry_at, received_at, created_at, updated_at`,
		// The payload column is jsonb. The API pool runs in
		// QueryExecModeExec (cmd/api/main.go), where pgx infers parameter
		// types from the Go values instead of asking Postgres, and a []byte is
		// sent as bytea: Postgres then answers "invalid input syntax for type
		// json" and every MercadoPago delivery gets a 500. A string is sent as
		// text and cast by the column. Found by MercadoPago's webhook
		// simulator on 2026-09-03; the statement-cache mode the tests used to
		// run under knew the column type and hid it.
		provider, e.ExternalID, e.EventType, string(e.Payload),
	).Scan(&e.ID, &e.Provider, &e.Status, &e.RetryCount, &e.MaxRetries,
		&e.NextRetryAt, &e.ReceivedAt, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("record webhook event: %w", err)
	}
	return nil
}

// GetPendingDue returns events that are due to be worked: the ones waiting their
// turn, the ones abandoned mid-flight, and the ones whose retry budget is spent.
//
// Same shape and same reasoning as FailedRefunds.GetPendingDue, including
// why 'exhausted' is in the list. A pending-only filter would leave a row that
// died in 'processing' invisible forever, which here means a captured payment
// whose booking never got confirmed and which nothing would ever look at again;
// and an exhausted row is the same thing entered deliberately, by a provider
// outage that outlasted five attempts. Nothing ever moved a row out of that
// status. It comes back here on the tail of its own backoff instead.
func (m *WebhookEvents) GetPendingDue(ctx context.Context) ([]*WebhookEvent, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx, `
		SELECT id, provider, external_id, event_type, payload, status,
		       retry_count, max_retries, next_retry_at, COALESCE(last_error, ''),
		       received_at, processed_at, created_at, updated_at
		FROM webhook_events
		WHERE next_retry_at <= NOW()
		  AND (status IN ('pending', 'exhausted')
		       OR (status = 'processing' AND updated_at < NOW() - $1::interval))
		ORDER BY next_retry_at ASC
		LIMIT $2`, staleWebhookProcessing.String(), webhookSweepBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWebhookEvents(rows)
}

// Claim takes an event for this worker, reporting whether it got it.
//
// It is a single conditional UPDATE rather than a read followed by a write: two
// instances see the same row from GetPendingDue on the same tick, and only the
// one whose UPDATE matches may go on to call MercadoPago. The condition is the
// same predicate GetPendingDue selects on, so a row another worker is honestly
// still working is refused until its attempt has gone stale.
func (m *WebhookEvents) Claim(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := m.DB.Exec(ctx, `
		UPDATE webhook_events
		SET status = 'processing'
		WHERE id = $1
		  AND (status IN ('pending', 'exhausted')
		       OR (status = 'processing' AND updated_at < NOW() - $2::interval))`,
		id, staleWebhookProcessing.String())
	if err != nil {
		return false, fmt.Errorf("claim webhook event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// MarkProcessed closes an event out: the work it described is done and it is now
// only a forensic record, subject to the retention sweep.
//
// Detached from the caller's cancellation, like MarkFailed and for the same
// reason: the sweep may have spent its budget on the very row it just finished,
// and a closing write that is skipped leaves that row in 'processing' to be
// worked all over again.
func (m *WebhookEvents) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := data.DetachedQueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`UPDATE webhook_events SET status = 'processed', processed_at = NOW(), last_error = NULL WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark webhook event processed: %w", err)
	}
	return nil
}

// MarkFailed leaves an event queued for another attempt and reports whether the
// retry budget is now spent.
//
// The budget is read back under FOR UPDATE rather than taken from the caller's
// copy, for the reason RecordRefundFailure gives: two workers that reclaimed the
// same abandoned attempt must not both compute the same "next" retry count from
// the same stale struct and between them burn a single retry.
//
// It runs on a context detached from the caller's cancellation, bounded by the
// ordinary transaction budget — the same shape as DetachedQueryContext, one size
// up because this is a transaction. The reason is that the case this write
// matters most in is exactly the case where the caller has no context left: a
// sweep that hit its two-minute budget with this event claimed. Skipping the
// write there leaves the row in 'processing' with no backoff and no recorded
// reason, invisible to every instance until it goes stale — which for this table
// is a captured payment whose booking is still unconfirmed.
func (m *WebhookEvents) MarkFailed(ctx context.Context, id uuid.UUID, cause string) (bool, error) {
	ctx, cancel := data.TxContext(context.WithoutCancel(ctx))
	defer cancel()

	var exhausted bool
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, _ *db.Queries) error {
		var retryCount, maxRetries int
		err := tx.QueryRow(ctx,
			`SELECT retry_count, max_retries FROM webhook_events WHERE id = $1 FOR UPDATE`, id,
		).Scan(&retryCount, &maxRetries)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("lock webhook event: %w", err)
		}

		newRetryCount := retryCount + 1
		delay := retryBackoff(newRetryCount)

		// A failure that was the provider being unreachable does not spend a retry.
		// The budget bounds how many times we ask a provider that is answering; an
		// attempt that never got an answer is not evidence about this event, and
		// charging it was what let a MercadoPago outage of a single afternoon exhaust
		// every queued payment at once. See transientProviderFailure.
		if transientProviderFailure(cause) {
			newRetryCount = retryCount
			delay = providerOutageDelay()
		}

		exhausted = newRetryCount >= maxRetries

		// 'exhausted' is never deleted by the retention sweep: it means a payment
		// event this system could not act on, which is a person's money waiting for
		// someone to look at it. It is no longer the end of the line, though — the
		// sweeper picks such a row up again once next_retry_at comes round, so the
		// true blocked reports itself every few hours instead of once, and the one
		// that was only blocked by an outage finishes by itself.
		status := "pending"
		if exhausted {
			status = "exhausted"
		}

		if _, err = tx.Exec(ctx, `
		UPDATE webhook_events
		SET retry_count = $2, last_error = $3, next_retry_at = $4, status = $5
		WHERE id = $1`,
			id, newRetryCount, cause, time.Now().Add(delay), status,
		); err != nil {
			return fmt.Errorf("requeue webhook event: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

// DeleteProcessed removes processed events older than olderThan and returns how
// many were deleted.
//
// Only 'processed' rows are eligible. Anything still pending, in flight or
// exhausted describes unfinished money and stays until it is resolved.
func (m *WebhookEvents) DeleteProcessed(ctx context.Context, olderThan time.Duration) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	tag, err := m.DB.Exec(ctx,
		`DELETE FROM webhook_events
		 WHERE status = 'processed' AND processed_at < NOW() - $1::interval`,
		olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("delete processed webhook events: %w", err)
	}
	return tag.RowsAffected(), nil
}

func scanWebhookEvents(rows pgx.Rows) ([]*WebhookEvent, error) {
	var result []*WebhookEvent
	for rows.Next() {
		var e WebhookEvent
		err := rows.Scan(
			&e.ID, &e.Provider, &e.ExternalID, &e.EventType, &e.Payload, &e.Status,
			&e.RetryCount, &e.MaxRetries, &e.NextRetryAt, &e.LastError,
			&e.ReceivedAt, &e.ProcessedAt, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan webhook event: %w", err)
		}
		result = append(result, &e)
	}
	return result, rows.Err()
}

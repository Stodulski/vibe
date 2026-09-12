//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// The webhook inbox exists so that a 200 to MercadoPago means "this event is
// durably ours". Everything worth having about it is a property of what the
// database actually holds: the row has to be committed before the provider is
// answered, exactly one worker may claim it, a failure has to leave it queued
// with a backoff, and an attempt that died has to become visible again.

// webhookFixture is one test's worth of webhook events, scoped by an external id
// nothing else uses.
//
// It wraps a datatest.Fixture rather than opening its own pool: webhook_events
// carries no complex_id, so the seeded owner/complex/court/client this table
// never touches ride along unused, and every store call goes through the same
// handle whether that fixture is Isolated or Shared.
type webhookFixture struct {
	*datatest.Fixture
	Store      *paymentstore.WebhookEvents
	ExternalID string
}

// newWebhookFixture is the default: every statement runs inside one
// transaction the harness rolls back, so there is nothing here for a delete to
// scope by external id.
func newWebhookFixture(t *testing.T) *webhookFixture {
	t.Helper()
	return wrapWebhookFixture(datatest.Isolated(t))
}

// newSharedWebhookFixture is for the one test that must read a row back
// through a genuinely separate connection to prove it was committed; see
// datatest.Fixture. Its rows are real, so cleanup deletes them by external id
// the way the pre-transactional fixture always did.
func newSharedWebhookFixture(t *testing.T) *webhookFixture {
	t.Helper()

	f := wrapWebhookFixture(datatest.Shared(t))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := f.DB.Exec(ctx,
			`DELETE FROM webhook_events WHERE external_id = $1`, f.ExternalID); err != nil {
			t.Errorf("deleting webhook events: %v", err)
		}
	})
	return f
}

func wrapWebhookFixture(f *datatest.Fixture) *webhookFixture {
	return &webhookFixture{
		Fixture:    f,
		Store:      &paymentstore.WebhookEvents{DB: f.DB},
		ExternalID: "mp-" + uuid.NewString(),
	}
}

// record inserts one event through the real store.
func (f *webhookFixture) record(t *testing.T, eventType string) *paymentstore.WebhookEvent {
	t.Helper()

	e := &paymentstore.WebhookEvent{
		Provider:   "mercadopago",
		ExternalID: f.ExternalID,
		EventType:  eventType,
		Payload:    json.RawMessage(`{"type":"payment","action":"payment.updated","data":{"id":"` + f.ExternalID + `"}}`),
	}
	if err := f.Store.Insert(context.Background(), e); err != nil {
		t.Fatalf("recording webhook event: %v", err)
	}
	return e
}

// webhookEventState is the row's state, read straight from the table rather than
// from anything a store method returned.
type webhookEventState struct {
	status      string
	retryCount  int
	maxRetries  int
	nextRetryAt time.Time
	lastError   string
	processedAt *time.Time
	updatedAt   time.Time
}

func (f *webhookFixture) readEvent(t *testing.T, id uuid.UUID) webhookEventState {
	t.Helper()

	var s webhookEventState
	err := f.DB.QueryRow(context.Background(), `
		SELECT status, retry_count, max_retries, next_retry_at,
		       COALESCE(last_error, ''), processed_at, updated_at
		FROM webhook_events WHERE id = $1`, id,
	).Scan(&s.status, &s.retryCount, &s.maxRetries, &s.nextRetryAt,
		&s.lastError, &s.processedAt, &s.updatedAt)
	if err != nil {
		t.Fatalf("reading webhook event %s: %v", id, err)
	}
	return s
}

// backdateWebhookEvent ages a row's updated_at, which is what a worker dying
// mid-attempt eventually looks like.
//
// webhook_events carries a BEFORE UPDATE trigger that overwrites updated_at with
// NOW() on every write, so an ordinary UPDATE cannot age a row. The trigger is
// disabled for the duration of this single statement — inside a transaction, so
// it is restored even if the UPDATE fails. Same shape as
// backdateFailedRefundUpdatedAt.
func (f *webhookFixture) backdateWebhookEvent(t *testing.T, id uuid.UUID, age time.Duration) {
	t.Helper()

	ctx := context.Background()

	tx, err := f.DB.Begin(ctx)
	if err != nil {
		t.Fatalf("begin backdate tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `ALTER TABLE webhook_events DISABLE TRIGGER set_updated_at`); err != nil {
		t.Fatalf("disabling updated_at trigger: %v", err)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE webhook_events SET updated_at = NOW() - $2::interval WHERE id = $1`,
		id, age.String())
	if err != nil {
		t.Fatalf("backdating webhook event: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("backdating webhook event %s: want 1 row affected, got %d", id, tag.RowsAffected())
	}

	if _, err := tx.Exec(ctx, `ALTER TABLE webhook_events ENABLE TRIGGER set_updated_at`); err != nil {
		t.Fatalf("re-enabling updated_at trigger: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit backdate tx: %v", err)
	}
}

// separateConn opens a connection outside the pool the store used, so a read
// through it can only see committed data.
func (f *webhookFixture) separateConn(t *testing.T) *pgx.Conn {
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

// dueContains reports whether id is among the events the sweeper would pick up.
// The queue is global, so tests look for their own row rather than counting.
func dueContains(events []*paymentstore.WebhookEvent, id uuid.UUID) bool {
	for _, e := range events {
		if e.ID == id {
			return true
		}
	}
	return false
}

// An event that is not committed by the time Insert returns is worth nothing: the
// handler is about to answer 200 on the strength of it, and MercadoPago never
// redelivers what it has been told is fine. The read below goes through a
// connection of its own, outside the pool the store used, so an uncommitted write
// would be invisible to it by definition.
func TestRecordingAWebhookEventCommitsBeforeItIsAcknowledged(t *testing.T) {
	f := newSharedWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")

	if event.ID == uuid.Nil {
		t.Fatal("a recorded event must come back with the id its worker will claim")
	}
	if event.Status != "pending" {
		t.Errorf("a recorded event starts pending; got %q", event.Status)
	}
	if event.MaxRetries <= 0 {
		t.Errorf("a recorded event must carry a retry budget; got %d", event.MaxRetries)
	}

	conn := f.separateConn(t)

	var status, externalID, eventType string
	var payload []byte
	if err := conn.QueryRow(ctx,
		`SELECT status, external_id, event_type, payload FROM webhook_events WHERE id = $1`, event.ID,
	).Scan(&status, &externalID, &eventType, &payload); err != nil {
		t.Fatalf("reading the event from a separate connection: %v", err)
	}
	if status != "pending" {
		t.Errorf("another connection must already see the event committed; got status %q", status)
	}
	if externalID != f.ExternalID || eventType != "payment" {
		t.Errorf("the committed row must carry what was delivered; got external_id=%q event_type=%q", externalID, eventType)
	}
	if len(payload) == 0 {
		t.Error("the committed row must keep the payload; it is the only record of what MercadoPago actually said")
	}
}

// MercadoPago legitimately delivers several notifications for one payment as its
// status moves. Each is its own row: the table is an inbox and a forensic log,
// and deduplication stays with the advisory lock in internal/payments.
func TestTwoDeliveriesForOnePaymentAreBothRecorded(t *testing.T) {
	f := newWebhookFixture(t)

	first := f.record(t, "payment")
	second := f.record(t, "payment")

	if first.ID == second.ID {
		t.Fatal("each delivery must be its own row")
	}

	var count int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM webhook_events WHERE provider = 'mercadopago' AND external_id = $1`, f.ExternalID,
	).Scan(&count); err != nil {
		t.Fatalf("counting deliveries: %v", err)
	}
	if count != 2 {
		t.Errorf("(provider, external_id) must not be unique; got %d rows for one payment", count)
	}
}

// Two workers see the same due row on the same tick. Only one may go on to call
// MercadoPago, and the claim is what decides it.
func TestOnlyOneWorkerCanClaimAnEvent(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")

	claimed, err := f.Store.Claim(ctx, event.ID)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if !claimed {
		t.Fatal("a pending event must be claimable")
	}

	again, err := f.Store.Claim(ctx, event.ID)
	if err != nil {
		t.Fatalf("second Claim: %v", err)
	}
	if again {
		t.Error("an event another worker is honestly still working must not be claimed twice")
	}

	if state := f.readEvent(t, event.ID); state.status != "processing" {
		t.Errorf("a claimed event reads as in flight; got %q", state.status)
	}

	// And it is no longer offered to the sweeper while the attempt is fresh.
	due, err := f.Store.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if dueContains(due, event.ID) {
		t.Error("an event with a live attempt on it must not be handed out again")
	}
}

// The defect this table exists to remove: an attempt that dies leaves nothing
// behind. MarkProcessed is what turns a captured payment into a closed one.
func TestAProcessedEventIsClosedOutAndNoLongerDue(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")
	if _, err := f.Store.Claim(ctx, event.ID); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := f.Store.MarkProcessed(ctx, event.ID); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	state := f.readEvent(t, event.ID)
	if state.status != "processed" {
		t.Errorf("want status processed; got %q", state.status)
	}
	if state.processedAt == nil {
		t.Error("a processed event must record when it was processed")
	}

	due, err := f.Store.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if dueContains(due, event.ID) {
		t.Error("a processed event must never be worked again")
	}

	// A processed event is also unclaimable, so a stale sweep holding an old copy
	// of the row cannot reopen it.
	claimed, err := f.Store.Claim(ctx, event.ID)
	if err != nil {
		t.Fatalf("Claim after processing: %v", err)
	}
	if claimed {
		t.Error("a processed event must not be claimable")
	}
}

// A failed attempt has to leave the event queued with a backoff and a reason.
// Silently dropping it is the whole defect.
func TestAFailedAttemptQueuesTheEventWithBackoff(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")
	if _, err := f.Store.Claim(ctx, event.ID); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	before := time.Now()
	exhausted, err := f.Store.MarkFailed(ctx, event.ID, "mercadopago unavailable")
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if exhausted {
		t.Fatal("one failure out of five must not exhaust the budget")
	}

	state := f.readEvent(t, event.ID)
	if state.status != "pending" {
		t.Errorf("a failed attempt returns the event to the queue; got %q", state.status)
	}
	if state.retryCount != 1 {
		t.Errorf("the attempt must be counted; got retry_count %d", state.retryCount)
	}
	if state.lastError != "mercadopago unavailable" {
		t.Errorf("the reason must be recorded; got %q", state.lastError)
	}
	if !state.nextRetryAt.After(before) {
		t.Errorf("the retry must be scheduled into the future; next_retry_at %v is not after %v", state.nextRetryAt, before)
	}

	// Backoff, not a busy loop: the row is not due again immediately.
	due, err := f.Store.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if dueContains(due, event.ID) {
		t.Error("an event waiting out its backoff must not be handed out yet")
	}

	// The second failure waits longer than the first.
	firstDelay := time.Until(state.nextRetryAt)
	if _, err := f.Store.Claim(ctx, event.ID); err == nil {
		if _, err := f.Store.MarkFailed(ctx, event.ID, "still unavailable"); err != nil {
			t.Fatalf("second MarkFailed: %v", err)
		}
	}
	secondState := f.readEvent(t, event.ID)
	if secondState.retryCount != 2 {
		t.Errorf("the second attempt must be counted too; got %d", secondState.retryCount)
	}
	if time.Until(secondState.nextRetryAt) <= firstDelay {
		t.Errorf("the backoff must grow: first waited %v, second waits %v", firstDelay, time.Until(secondState.nextRetryAt))
	}
}

// An event nothing can ever process must stop retrying and start shouting.
// 'exhausted' is terminal and is deliberately never touched by the retention
// sweep: it describes money nobody has resolved.
func TestAnEventExhaustsItsRetryBudget(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")
	state := f.readEvent(t, event.ID)

	var exhausted bool
	for attempt := range state.maxRetries {
		var err error
		exhausted, err = f.Store.MarkFailed(ctx, event.ID, "permanently broken")
		if err != nil {
			t.Fatalf("MarkFailed attempt %d: %v", attempt, err)
		}
	}

	if !exhausted {
		t.Errorf("the budget of %d retries must run out", state.maxRetries)
	}
	final := f.readEvent(t, event.ID)
	if final.status != "exhausted" {
		t.Errorf("want status exhausted; got %q", final.status)
	}

	due, err := f.Store.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if dueContains(due, event.ID) {
		t.Error("an exhausted event needs a human, not another automatic attempt")
	}
}

// The reason GetPendingDue does not filter on 'pending' alone.
//
// Claim flips a row to 'processing' before MercadoPago is called. If the process
// crashes, is redeployed or is OOM-killed before the terminal transition, nothing
// ever moves that row — and here that means a payment that was captured and whose
// booking was never confirmed. Past the stale window it is handed out again.
func TestAnEventAbandonedInProcessingIsReclaimed(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")
	claimed, err := f.Store.Claim(ctx, event.ID)
	if err != nil || !claimed {
		t.Fatalf("Claim: claimed=%v err=%v", claimed, err)
	}

	// The worker dies here, with the row reading 'processing' forever.
	f.backdateWebhookEvent(t, event.ID, paymentstore.StaleWebhookProcessingForTest+5*time.Minute)

	due, err := f.Store.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if !dueContains(due, event.ID) {
		t.Fatal("an abandoned attempt must become visible again, or the payment behind it is lost with no log and no alert")
	}

	reclaimed, err := f.Store.Claim(ctx, event.ID)
	if err != nil {
		t.Fatalf("reclaiming: %v", err)
	}
	if !reclaimed {
		t.Error("the sweeper must be able to take an abandoned attempt back")
	}

	// Reclaiming refreshes the attempt clock, so a second sweep leaves the new
	// worker alone.
	if state := f.readEvent(t, event.ID); time.Since(state.updatedAt) > time.Minute {
		t.Errorf("the reclaimed attempt must restart its own clock; updated_at is %v", state.updatedAt)
	}
}

// Retention: the table takes a row per delivery, several per payment, so
// processed rows cannot be kept forever. Nothing unresolved is ever deleted on a
// timer.
func TestRetentionDeletesOldProcessedEventsAndNothingElse(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	old := f.record(t, "payment")
	if err := f.Store.MarkProcessed(ctx, old.ID); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	if _, err := f.DB.Exec(ctx,
		`UPDATE webhook_events SET processed_at = NOW() - INTERVAL '200 days' WHERE id = $1`, old.ID); err != nil {
		t.Fatalf("ageing the processed event: %v", err)
	}

	recent := f.record(t, "payment")
	if err := f.Store.MarkProcessed(ctx, recent.ID); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	stillPending := f.record(t, "payment")

	stuck := f.record(t, "payment")
	for range 5 {
		if _, err := f.Store.MarkFailed(ctx, stuck.ID, "permanently broken"); err != nil {
			t.Fatalf("MarkFailed: %v", err)
		}
	}
	if _, err := f.DB.Exec(ctx,
		`UPDATE webhook_events SET processed_at = NOW() - INTERVAL '200 days' WHERE id = $1`, stuck.ID); err != nil {
		t.Fatalf("ageing the exhausted event: %v", err)
	}

	deleted, err := f.Store.DeleteProcessed(ctx, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("DeleteProcessed: %v", err)
	}
	if deleted < 1 {
		t.Errorf("the aged processed event must be deleted; deleted %d", deleted)
	}

	var survives bool
	if err := f.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhook_events WHERE id = $1)`, old.ID).Scan(&survives); err != nil {
		t.Fatalf("checking the aged event: %v", err)
	}
	if survives {
		t.Error("a processed event past retention must be gone")
	}

	for _, kept := range []struct {
		name string
		id   uuid.UUID
	}{
		{"a recently processed event", recent.ID},
		{"an event still waiting to be worked", stillPending.ID},
		{"an exhausted event nobody has resolved", stuck.ID},
	} {
		var exists bool
		if err := f.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM webhook_events WHERE id = $1)`, kept.id).Scan(&exists); err != nil {
			t.Fatalf("checking %s: %v", kept.name, err)
		}
		if !exists {
			t.Errorf("%s must not be deleted by the retention sweep", kept.name)
		}
	}
}

// FINDING 5. A budget bounds how many times we ask a provider that is answering
// us. An attempt that never got an answer is not evidence about the event, and
// charging it was what let one MercadoPago outage abandon every captured payment
// at once: five attempts at 1m/5m/15m/1h/4h is five hours and twenty minutes of
// wall clock, after which the row went to 'exhausted' and nothing ever brought
// it back.
func TestAProviderOutageDoesNotSpendTheRetryBudget(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")

	exhausted, err := f.Store.MarkFailed(ctx, event.ID, "mp: get payment request failed: dial tcp 1.2.3.4:443: i/o timeout")
	if err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if exhausted {
		t.Error("one unreachable-provider attempt reported the budget as spent")
	}

	state := f.readEvent(t, event.ID)
	if state.retryCount != 0 {
		t.Errorf("an outage spent a retry; want the count left at 0, got %d", state.retryCount)
	}
	if state.status != "pending" {
		t.Errorf("want the event left queued; got %q", state.status)
	}
	if wait := time.Until(state.nextRetryAt); wait > paymentstore.ProviderOutageRetryDelayForTest+time.Minute || wait < time.Minute {
		t.Errorf("want a steady outage probe of about %v; got %v", paymentstore.ProviderOutageRetryDelayForTest, wait)
	}
}

// The other half of the same rule: a provider that answered — with a rejection,
// a malformed body, anything that is a decision about this event — does spend a
// retry, or nothing would ever exhaust and a genuinely unworkable event would
// retry forever without ever alerting.
func TestAProviderRejectionStillSpendsTheRetryBudget(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")

	if _, err := f.Store.MarkFailed(ctx, event.ID, "mp: get payment failed with status 404: not found"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	if got := f.readEvent(t, event.ID).retryCount; got != 1 {
		t.Errorf("an answer from the provider must spend a retry; want 1, got %d", got)
	}
}

// A 5xx is the provider's own fault and a 429 is it asking us to slow down.
// Neither is an answer about this payment.
func TestAProviderServerErrorIsTreatedAsAnOutage(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")

	if _, err := f.Store.MarkFailed(ctx, event.ID, "mp: get payment failed with status 503: service unavailable"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	if got := f.readEvent(t, event.ID).retryCount; got != 0 {
		t.Errorf("a 503 spent a retry; want the count left at 0, got %d", got)
	}
}

// FINDING 5. 'exhausted' meant a captured payment whose booking was never
// confirmed, entered automatically and never revisited by anything. It is a
// pause now: the row comes back once its backoff elapses, so a provider that has
// started answering finishes the work by itself and one that has not keeps
// alerting until a person acts.
func TestAnExhaustedEventComesBackWhenItsBackoffElapses(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	event := f.record(t, "payment")
	if _, err := f.DB.Exec(ctx, `
		UPDATE webhook_events
		SET status = 'exhausted', retry_count = max_retries, next_retry_at = NOW() - INTERVAL '1 minute'
		WHERE id = $1`, event.ID); err != nil {
		t.Fatalf("exhausting the event: %v", err)
	}

	due, err := f.Store.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if !dueContains(due, event.ID) {
		t.Fatal("an event whose budget is spent is never looked at again, so the payment stays unconfirmed forever")
	}

	claimed, err := f.Store.Claim(ctx, event.ID)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if !claimed {
		t.Error("a revived event is due but cannot be claimed, so the sweep spins on it")
	}
}

// FINDING 6. The sweep is sequential with a two-minute budget and about eight
// seconds of MercadoPago per row, so the deadline lands mid-batch with one event
// claimed. The failure has to be recorded anyway: without it the row keeps its
// claim, gets no backoff and no reason, and is invisible to every instance until
// its attempt goes stale.
func TestAFailedAttemptIsRecordedEvenWhenTheSweepIsOutOfTime(t *testing.T) {
	f := newWebhookFixture(t)

	event := f.record(t, "payment")
	if claimed, err := f.Store.Claim(context.Background(), event.ID); err != nil || !claimed {
		t.Fatalf("claiming the event: claimed=%v err=%v", claimed, err)
	}

	// The sweep's budget is gone — this is the row it was working when the two
	// minutes ran out.
	spent, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := f.Store.MarkFailed(spent, event.ID, "mp: get payment failed with status 404: not found"); err != nil {
		t.Fatalf("MarkFailed on a spent context: %v", err)
	}

	state := f.readEvent(t, event.ID)
	if state.status == "processing" {
		t.Error("the event kept its claim, so it is dark until its attempt goes stale")
	}
	if state.retryCount != 1 {
		t.Errorf("the attempt was not recorded; want retry_count 1, got %d", state.retryCount)
	}
	if state.lastError == "" {
		t.Error("the reason for the failure was lost, so nobody can see why this payment is stuck")
	}
}

// FINDING 6. Closing an event out has the same problem from the other side: the
// work succeeded and only the bookkeeping is left, and losing it means the whole
// dispatch — including a MercadoPago fetch — is replayed for nothing.
func TestASuccessfulAttemptIsClosedOutEvenWhenTheSweepIsOutOfTime(t *testing.T) {
	f := newWebhookFixture(t)

	event := f.record(t, "payment")
	if claimed, err := f.Store.Claim(context.Background(), event.ID); err != nil || !claimed {
		t.Fatalf("claiming the event: claimed=%v err=%v", claimed, err)
	}

	spent, cancel := context.WithCancel(context.Background())
	cancel()

	if err := f.Store.MarkProcessed(spent, event.ID); err != nil {
		t.Fatalf("MarkProcessed on a spent context: %v", err)
	}

	if state := f.readEvent(t, event.ID); state.status != "processed" {
		t.Errorf("want the event closed out; got %q, which the sweeper will work all over again", state.status)
	}
}

// FINDING 6. The sweep reads what a run can work rather than fifty rows it will
// drop.
func TestTheWebhookSweepReadsNoMoreThanARunCanWork(t *testing.T) {
	f := newWebhookFixture(t)
	ctx := context.Background()

	for range paymentstore.WebhookSweepBatchForTest + 3 {
		f.record(t, "payment")
	}

	due, err := f.Store.GetPendingDue(ctx)
	if err != nil {
		t.Fatalf("GetPendingDue: %v", err)
	}
	if len(due) > paymentstore.WebhookSweepBatchForTest {
		t.Errorf("the sweep read %d events; a run can only work %d of them", len(due), paymentstore.WebhookSweepBatchForTest)
	}
}

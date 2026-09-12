package data

import "time"

// Exports for the package's external test binary (package data_test).
//
// These are implementation details a test asserts against — the exact SQL a
// query plan is measured on, and the retention and backoff constants a sweep
// is timed against. They are deliberately not part of the package's API: a
// caller has no use for them, and a test that needs them says so here rather
// than by forcing them exported.
const (
	// LeaseGraceForTest is the slack added to a lease's TTL before it is
	// considered abandoned.
	LeaseGraceForTest = leaseGrace
	// AbandonedLeaseRetentionForTest is how long an abandoned lease row is kept.
	AbandonedLeaseRetentionForTest = abandonedLeaseRetention
	// ProviderOutageRetryDelayForTest is the delay a provider outage reschedules on.
	ProviderOutageRetryDelayForTest = providerOutageRetryDelay
	// StaleRefundProcessingForTest is how long a claimed refund may stay in
	// processing before the sweep reclaims it.
	StaleRefundProcessingForTest time.Duration = staleRefundProcessing
	// RefundSweepBatchForTest is how many refund attempts one sweep reads.
	RefundSweepBatchForTest = refundSweepBatch
	// StaleWebhookProcessingForTest is the webhook half of StaleRefundProcessingForTest.
	StaleWebhookProcessingForTest time.Duration = staleWebhookProcessing
	// WebhookSweepBatchForTest is the webhook half of RefundSweepBatchForTest.
	WebhookSweepBatchForTest = webhookSweepBatch
)

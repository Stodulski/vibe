package store

import "time"

// Test hooks for the integration tests that time this store's sweeps.
//
// They are ordinary declarations rather than export_test.go ones only because
// those tests still live in internal/data. When they move next to this package
// these become export_test.go entries and this file goes away; nothing outside
// a test may use them.
const (
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

package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRefundRetryBackoff_Length(t *testing.T) {
	if len(refundRetryBackoff) != 5 {
		t.Errorf("refundRetryBackoff has %d entries, want 5", len(refundRetryBackoff))
	}
}

func TestRefundRetryBackoff_Values(t *testing.T) {
	expected := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
		4 * time.Hour,
	}

	for i, want := range expected {
		if refundRetryBackoff[i] != want {
			t.Errorf("refundRetryBackoff[%d] = %v, want %v", i, refundRetryBackoff[i], want)
		}
	}
}

func TestRefundRetryBackoff_Increasing(t *testing.T) {
	for i := 1; i < len(refundRetryBackoff); i++ {
		if refundRetryBackoff[i] <= refundRetryBackoff[i-1] {
			t.Errorf("refundRetryBackoff[%d] (%v) should be greater than [%d] (%v)",
				i, refundRetryBackoff[i], i-1, refundRetryBackoff[i-1])
		}
	}
}

func TestFailedRefund_StructFields(t *testing.T) {
	now := time.Now()
	resolvedAt := now.Add(1 * time.Hour)
	fr := FailedRefund{
		ID:           uuid.New(),
		PaymentID:    uuid.New(),
		BookingID:    uuid.New(),
		ComplexID:    uuid.New(),
		Amount:       15000,
		MPPaymentID:  "mp-12345",
		ErrorMessage: "connection refused",
		RetryCount:   2,
		MaxRetries:   5,
		NextRetryAt:  now.Add(15 * time.Minute),
		Status:       "pending",
		CreatedAt:    now,
		UpdatedAt:    now,
		ResolvedAt:   &resolvedAt,
	}

	if fr.Amount != 15000 {
		t.Errorf("Amount = %d, want %d", fr.Amount, 15000)
	}
	if fr.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want %d", fr.RetryCount, 2)
	}
	if fr.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want %d", fr.MaxRetries, 5)
	}
	if fr.Status != "pending" {
		t.Errorf("Status = %q, want %q", fr.Status, "pending")
	}
	if fr.ResolvedAt == nil {
		t.Error("ResolvedAt should not be nil")
	}
}

func TestFailedRefund_NilResolvedAt(t *testing.T) {
	fr := FailedRefund{
		ID:     uuid.New(),
		Status: "pending",
	}

	if fr.ResolvedAt != nil {
		t.Error("ResolvedAt should be nil for pending refund")
	}
}

// TestFailedRefundModel_RequiresDB documents that all FailedRefunds methods
// require a database connection.
func TestFailedRefundModel_RequiresDB(t *testing.T) {
	t.Skip("FailedRefunds methods all require *pgxpool.Pool")
}

// The line between "the provider said no" and "the provider said nothing" is
// what decides whether an attempt spends part of the retry budget, so it is
// worth pinning against the strings internal/mp actually produces.
func TestTransientProviderFailure(t *testing.T) {
	outages := []string{
		"mp: circuit breaker is open",
		"mp: refund request failed: Post \"https://api.mercadopago.com\": dial tcp: connect: connection refused",
		"mp: get payment request failed: context deadline exceeded",
		"mp: get payment failed with status 500: internal server error",
		"mp: get payment failed with status 502: bad gateway",
		"mp: refund failed with status 429: too many requests",
		"fetch payment mp-1 from mercadopago: mp: refund request failed: read tcp: connection reset by peer",
		// internal/payments.sellerCredential's UNAVAILABLE arm (the complex
		// fetch itself failed) never reaches internal/mp at all, so its cause
		// carries this marker instead of one of the strings above — a
		// transient database read that may well succeed on the next attempt,
		// and must not spend the retry budget either. The underlying error
		// text is deliberately something no other marker matches, so this
		// case actually exercises the "seller credential unavailable" entry
		// rather than passing by accident through an unrelated one.
		"seller credential unavailable: fetch complex c1 for its seller credential: pool exhausted",
	}
	for _, cause := range outages {
		if !transientProviderFailure(cause) {
			t.Errorf("an unreachable provider must not spend a retry, but %q does", cause)
		}
	}

	answers := []string{
		"mp: refund failed with status 400: the refund amount exceeds the payment",
		"mp: get payment failed with status 404: payment not found",
		"mp: webhook signature verification failed",
		"payment mp-1 carries no mercadopago id to refund against",
		"record the refund of payment mp-1 and cancel its booking: duplicate key value",
		// Neither UNREADABLE nor MISSING self-heals on its own: retrying does
		// not decrypt a bad key or connect an account, so both are left to
		// spend the budget and re-alert until a person acts (design.md,
		// "MISSING and UNREADABLE still spend the retry budget").
		"seller credential unreadable for complex c1: data: stored MercadoPago credential could not be read",
		"seller credential missing for complex c1: data: MercadoPago is not connected for this complex",
	}
	for _, cause := range answers {
		if transientProviderFailure(cause) {
			t.Errorf("an answer about this payment must spend a retry, but %q does not", cause)
		}
	}
}

// TestEveryRetryDelayIsSpread is OUT-02 for the payment queues. The tables are
// fixed, so without the spread every row that failed against one MercadoPago
// outage carries the same next_retry_at to the second: the provider's first
// moment back up is met by the whole backlog at once, and the rows that fail
// again line up on the next entry of the same table.
//
// The assertion is a band rather than a value, which is the point — a delay
// that is always the same number is the defect.
func TestEveryRetryDelayIsSpread(t *testing.T) {
	tests := []struct {
		name string
		base time.Duration
		next func() time.Duration
	}{
		{"the first entry of the ladder", time.Minute, func() time.Duration { return retryBackoff(0) }},
		{"the last entry of the ladder", 4 * time.Hour, func() time.Duration { return retryBackoff(4) }},
		{"past the end of the ladder", 4 * time.Hour, func() time.Duration { return retryBackoff(99) }},
		{"the flat provider-outage probe", providerOutageRetryDelay, providerOutageDelay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			low := time.Duration(float64(tt.base) * 0.8)
			high := time.Duration(float64(tt.base) * 1.2)

			seen := make(map[time.Duration]struct{})
			for range 200 {
				got := tt.next()
				if got < low || got > high {
					t.Fatalf("delay %v is outside [%v, %v]; the spread is not ±20%% of %v", got, low, high, tt.base)
				}
				seen[got] = struct{}{}
			}
			if len(seen) < 2 {
				t.Errorf("200 draws produced %d distinct delays; the table is still fixed and a backlog still stampedes", len(seen))
			}
		})
	}
}

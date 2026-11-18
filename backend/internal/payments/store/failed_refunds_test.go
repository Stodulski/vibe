package store

import (
	"testing"
	"time"
)

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

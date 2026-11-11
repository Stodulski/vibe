package bookings

import (
	"strings"
	"testing"

	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// A booking cancelled outside the window may have been paid in full, not
// only with a deposit, so the sentence the client reads in the API answer,
// the email and WhatsApp must not call the money "la seña".
func TestRefundMessageNotEligibleDoesNotAssumeADeposit(t *testing.T) {
	cases := map[string]struct {
		amount int
		want   string
	}{
		"with an amount": {amount: 1_500_000, want: "fuera de plazo, lo que pagaste ($15.000) no se devuelve."},
		"without one":    {amount: 0, want: "fuera de plazo, lo que pagaste no se devuelve."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := refundMessage(paymentstore.RefundNotEligible, tc.amount)
			if got != tc.want {
				t.Fatalf("refundMessage(RefundNotEligible, %d) = %q, want %q", tc.amount, got, tc.want)
			}
			if strings.Contains(got, "seña") {
				t.Fatalf("refundMessage(RefundNotEligible, %d) = %q, must not say \"seña\"", tc.amount, got)
			}
		})
	}
}

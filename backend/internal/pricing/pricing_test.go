package pricing

import "testing"

// The fee is the same for every amount: it never grows with the payment.
func TestServiceFeeIsConstant(t *testing.T) {
	for amount := 0; amount <= 30_000_000; amount += 10_000 {
		if got := ServiceFee(amount); got != serviceFeeCentavos {
			t.Fatalf("ServiceFee(%d) = %d, want the flat %d", amount, got, serviceFeeCentavos)
		}
	}
}

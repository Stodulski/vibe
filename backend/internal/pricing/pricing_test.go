package pricing

import "testing"

func TestServiceFee(t *testing.T) {
	tests := []struct {
		name           string
		amountCentavos int
	}{
		{"zero amount", 0},
		{"small amount", 500_000},
		{"amount equal to the fee", 100_000},
		{"amount that used to sit on the old percentage break-even", 1_428_580},
		{"large amount", 10_000_000},
		{"very large amount", 1_000_000_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ServiceFee(tt.amountCentavos); got != 100_000 {
				t.Errorf("ServiceFee(%d) = %d; want a flat 100000", tt.amountCentavos, got)
			}
		})
	}
}

// The fee is the same for every amount: it never grows with the payment.
func TestServiceFeeIsConstant(t *testing.T) {
	for amount := 0; amount <= 30_000_000; amount += 10_000 {
		if got := ServiceFee(amount); got != serviceFeeCentavos {
			t.Fatalf("ServiceFee(%d) = %d, want the flat %d", amount, got, serviceFeeCentavos)
		}
	}
}

package pricing

import "testing"

func TestServiceFee(t *testing.T) {
	tests := []struct {
		name            string
		depositCentavos int
		want            int
	}{
		{"zero deposit still charges the floor", 0, 100_000},
		{"small deposit charges the floor", 500_000, 100_000},
		// 7% of 1_428_571 is 99_999, one centavo under the floor.
		{"just under the floor", 1_428_571, 100_000},
		// The break-even point: 7% of 1_428_580 clears 100_000.
		{"just over the floor", 1_500_000, 105_000},
		{"large deposit charges the percentage", 10_000_000, 700_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ServiceFee(tt.depositCentavos); got != tt.want {
				t.Errorf("ServiceFee(%d) = %d; want %d", tt.depositCentavos, got, tt.want)
			}
		})
	}
}

// The fee is never below the floor, whatever the deposit.
func TestServiceFeeNeverBelowFloor(t *testing.T) {
	for deposit := 0; deposit <= 3_000_000; deposit += 10_000 {
		if got := ServiceFee(deposit); got < feeMinCentavos {
			t.Fatalf("ServiceFee(%d) = %d, below the %d floor", deposit, got, feeMinCentavos)
		}
	}
}

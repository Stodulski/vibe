package store

import "testing"

func TestBalanceDueFollowsCollectionStatus(t *testing.T) {
	tests := []struct {
		name       string
		collection CollectionStatus
		deposit    int
		want       int
	}{
		{"nothing collected owes the whole price", CollectionStatusUnpaid, 0, 500_000},
		{"deposit leaves the rest owing", CollectionStatusDepositPaid, 150_000, 350_000},
		{"fully paid owes nothing", CollectionStatusFullyPaid, 500_000, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Booking{Price: 500_000, DepositAmount: tt.deposit, CollectionStatus: tt.collection}
			if got := b.BalanceDue(); got != tt.want {
				t.Fatalf("BalanceDue() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestRecordCounterPaymentOfTheFullBalanceFullyPays(t *testing.T) {
	b := &Booking{Status: "confirmed", Price: 500_000, DepositAmount: 150_000, CollectionStatus: CollectionStatusDepositPaid}
	b.RecordCounterPayment(350_000)
	if b.CollectionStatus != CollectionStatusFullyPaid {
		t.Fatalf("collection = %q; want fully_paid", b.CollectionStatus)
	}
	if b.Status != "confirmed" {
		t.Fatalf("status = %q; want confirmed", b.Status)
	}
}

func TestRecordCounterPaymentOfAPendingBookingConfirmsIt(t *testing.T) {
	b := &Booking{Status: "pending", Price: 500_000, CollectionStatus: CollectionStatusUnpaid}
	b.RecordCounterPayment(200_000)
	if b.Status != "confirmed" {
		t.Fatalf("status = %q; want confirmed", b.Status)
	}
	if b.CollectionStatus != CollectionStatusDepositPaid || b.DepositAmount != 200_000 {
		t.Fatalf("collection = %q deposit = %d; want deposit_paid 200000", b.CollectionStatus, b.DepositAmount)
	}
}

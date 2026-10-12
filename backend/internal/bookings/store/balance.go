package store

// BalanceDue is what the booking still owes: its price less the part already
// collected. Zero once it is fully paid.
func (b *Booking) BalanceDue() int {
	if b.CollectionStatus == CollectionStatusFullyPaid {
		return 0
	}
	if b.CollectionStatus == CollectionStatusDepositPaid {
		return b.Price - b.DepositAmount
	}
	return b.Price
}

// RecordCounterPayment moves the booking through the transition a payment the
// owner took at the counter causes: a pending booking becomes confirmed, and
// the collection status becomes fully_paid once the price is covered, or
// deposit_paid with DepositAmount holding the running total otherwise. It does
// not check anything. The callers own the refusals (cancelled, already paid)
// and persist the result.
func (b *Booking) RecordCounterPayment(amount int) {
	if b.Status == "pending" {
		b.Status = "confirmed"
	}
	totalPaid := amount
	if b.CollectionStatus == CollectionStatusDepositPaid {
		totalPaid += b.DepositAmount
	}
	if totalPaid >= b.Price {
		b.CollectionStatus = CollectionStatusFullyPaid
		return
	}
	b.CollectionStatus = CollectionStatusDepositPaid
	b.DepositAmount = totalPaid
}

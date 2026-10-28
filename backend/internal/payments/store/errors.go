package store

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by this store.
var (
	// ErrAlreadyRefunded is returned when a refund is attempted on an already-refunded payment.
	ErrAlreadyRefunded = errors.New("payment already refunded")
	// ErrRefundInFlight is returned when a refund is attempted on a payment another
	// claim has already reserved.
	//
	// It is deliberately not ErrAlreadyRefunded: that one means the money is back and
	// there is nothing left to do, while this one means the money has not necessarily
	// moved yet but a durable attempt already exists and will be worked exactly once.
	// A caller must not queue a second attempt for either, but only the first may tell
	// a client their refund is done.
	ErrRefundInFlight = errors.New("refund already in flight")
	// ErrNoManualRefundOwed is returned by RecordManualRefund when the booking
	// it locks does not read 'partial_refund' at write time — either it never
	// did, or another request already closed it out.
	ErrNoManualRefundOwed = errors.New("no manual refund owed")
	// ErrMercadoPagoRefundPending is returned by RecordManualRefund while a
	// MercadoPago payment on the booking still reads refund_pending. Its money is
	// on its way back (or has exhausted its retries and is still owed), so closing
	// the booking out now would read as fully returned while that payment is still
	// unsettled. The close-out waits for it.
	ErrMercadoPagoRefundPending = errors.New("mercadopago refund pending on booking")
	// ErrAmountExceedsBalance is returned by ConfirmInTx when a payment would collect
	// more than the booking still owes. The check reads the booking row under its lock,
	// because two counter collections can both pass the caller's check on an unlocked
	// snapshot and only the second one, read after the first commits, is over the balance.
	ErrAmountExceedsBalance = errors.New("payment amount exceeds the outstanding balance")
)

// AmountExceedsBalanceError is ErrAmountExceedsBalance carrying the balance the locked
// booking still owed, so the caller can name that figure to the owner rather than
// one read before the lock. errors.Is(err, ErrAmountExceedsBalance) holds for it.
type AmountExceedsBalanceError struct {
	Balance int
}

func (e *AmountExceedsBalanceError) Error() string {
	return fmt.Sprintf("%s: the outstanding balance is %d", ErrAmountExceedsBalance, e.Balance)
}

// Unwrap makes the sentinel match with errors.Is.
func (e *AmountExceedsBalanceError) Unwrap() error {
	return ErrAmountExceedsBalance
}

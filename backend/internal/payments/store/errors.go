package store

import "errors"

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
)

// Package paymentmethod is the one place that knows which payment_method
// enum values a counter (manual) transaction may use.
//
// Every value except "mercadopago" — which the online checkout owns
// exclusively, since it carries mp_payment_id and drives an automatic
// MercadoPago API refund that a counter transaction (cash, transfer, a card
// swiped in person, a QR/wallet scan) has no id for. Owner decision,
// 2026-09-22 (pos-cashbox's feature document): "any method except
// mercadopago, expressed once, not a hand-maintained list per call site."
//
// internal/bookings originally kept this list to itself
// (isCounterPaymentMethod); it moved here, unexported list and all, the
// moment a second domain — internal/cashbox, restricting cash_movements.method
// the same way — needed the identical rule. bookings.isCounterPaymentMethod
// now delegates to IsCounter rather than repeating the list, so "expressed
// once" still means once.
package paymentmethod

import "slices"

// counterMethods lists every payment_method value staff may record directly
// — at the booking counter or in the cashbox — as a manual transaction.
var counterMethods = []string{"cash", "transfer", "debit_card", "credit_card", "qr_wallet"}

// Message is the validation error every call site shows, naming every
// accepted value so a caller does not have to guess which ones changed.
const Message = "must be one of: cash, transfer, debit_card, credit_card, qr_wallet"

// IsCounter reports whether method is one a counter (manual) transaction may
// record — see counterMethods.
func IsCounter(method string) bool {
	return slices.Contains(counterMethods, method)
}

// CounterMethods returns every accepted value, for the one caller
// (internal/bookings' test suite) that ranges over the list itself rather
// than only calling IsCounter. A copy, so a caller cannot mutate the package's
// own list through the slice it returns.
func CounterMethods() []string {
	return slices.Clone(counterMethods)
}

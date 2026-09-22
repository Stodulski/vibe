package bookings

import "slices"

// counterPaymentMethods lists every payment_method value staff may record
// directly at the counter — every method except "mercadopago", which the
// online checkout owns exclusively: it carries mp_payment_id and drives an
// automatic MercadoPago API refund, and a counter QR payment has no id for
// that call to use. Expressed once here rather than as a hand-copied
// "== cash || == transfer" check per call site, so a new counter method (or a
// tightened list) is one edit, not one per handler.
var counterPaymentMethods = []string{"cash", "transfer", "debit_card", "credit_card", "qr_wallet"}

// counterPaymentMethodsMessage is the validation error every call site shows,
// naming every accepted value so a caller does not have to guess which ones
// changed.
const counterPaymentMethodsMessage = "must be one of: cash, transfer, debit_card, credit_card, qr_wallet"

// isCounterPaymentMethod reports whether method is one Create and
// ConfirmPayment may record — see counterPaymentMethods.
func isCounterPaymentMethod(method string) bool {
	return slices.Contains(counterPaymentMethods, method)
}

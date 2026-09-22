package bookings

import paymentmethod "github.com/stodulski/vibe-server/internal/paymentmethod"

// counterPaymentMethods lists every payment_method value staff may record
// directly at the counter — sourced from internal/paymentmethod, the shared
// home for this rule since internal/cashbox needed the identical list for
// cash_movements.method. Kept as a package-level var here, rather than
// switching every call site to paymentmethod.IsCounter directly, only because
// handlers_test.go and payment_method_test.go range over the values
// themselves, not just the predicate.
var counterPaymentMethods = paymentmethod.CounterMethods()

// counterPaymentMethodsMessage is the validation error every call site shows,
// naming every accepted value so a caller does not have to guess which ones
// changed.
const counterPaymentMethodsMessage = paymentmethod.Message

// isCounterPaymentMethod reports whether method is one Create and
// ConfirmPayment may record — see counterPaymentMethods.
func isCounterPaymentMethod(method string) bool {
	return paymentmethod.IsCounter(method)
}

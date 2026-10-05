// Package pricing holds the money rules the platform applies on top of a
// court's own price: today, the service fee charged on each online payment.
package pricing

// serviceFeeCentavos is the flat service fee charged on every online payment:
// 1000 ARS, in centavos.
//
// This is the source of truth: it is what a client is actually charged. The
// same amount is restated in the frontend fallback, in the owner-facing copy
// and on the landing, none of which can import it.
// Changing it alone fails CI: .github/scripts/check-service-fee.mjs compares this against the other copies.
const serviceFeeCentavos = 100_000

// ServiceFee returns the service fee charged on an online payment, in centavos.
//
// It is a flat 1000 ARS whatever the amount, and the client pays it. The
// complex owner is not charged this fee; the only cost they absorb is
// MercadoPago's own processing cut, which MercadoPago deducts directly.
//
// The amount is kept in the signature so every caller states what it is
// charging the fee on, but it no longer changes the result.
func ServiceFee(_ int) int {
	return serviceFeeCentavos
}

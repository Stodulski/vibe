// Package pricing holds the money rules the platform applies on top of a
// court's own price: today, the service fee charged on a booking deposit.
package pricing

// feeRatePercent is the share of the deposit taken as a service fee.
//
// This is the source of truth: it is what a client is actually charged. The
// same number is restated in the frontend fallback, in the owner-facing copy
// and on the landing, none of which can import it.
// Changing it alone fails CI: .github/scripts/check-service-fee.mjs compares this against the other three copies.
const feeRatePercent = 7

// feeMinCentavos is the floor for the service fee: 1000 ARS, in centavos.
const feeMinCentavos = 100_000

// ServiceFee returns the service fee charged on a deposit, in centavos.
//
// It is 7% of the deposit with a 1000 ARS floor, and the client pays it. The
// complex owner is not charged this fee; the only cost they absorb is
// MercadoPago's own processing cut, which MercadoPago deducts directly.
func ServiceFee(depositCentavos int) int {
	return max(depositCentavos*feeRatePercent/100, feeMinCentavos)
}

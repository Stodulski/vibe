package bookings

import "testing"

func TestIsCounterPaymentMethod(t *testing.T) {
	for _, method := range counterPaymentMethods {
		if !isCounterPaymentMethod(method) {
			t.Errorf("want %q accepted as a counter payment method", method)
		}
	}

	for _, method := range []string{"mercadopago", "", "crypto"} {
		if isCounterPaymentMethod(method) {
			t.Errorf("want %q refused as a counter payment method", method)
		}
	}
}

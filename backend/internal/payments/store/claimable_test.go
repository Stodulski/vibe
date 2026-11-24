package store

import (
	"errors"
	"testing"

	"github.com/stodulski/vibe-server/internal/db"
)

// A payment with no MercadoPago id can never be refunded automatically, and the
// refund paths owe it by hand at once only when they can recognise the error.
// A plain message would read as a passing failure and be retried for nothing.
func TestAClaimOnAPaymentWithNoMercadoPagoIDFailsForGood(t *testing.T) {
	_, _, err := claimable(db.Payment{Amount: 150_000})
	if !errors.Is(err, ErrNoMercadoPagoID) {
		t.Fatalf("want ErrNoMercadoPagoID; got %v", err)
	}
}

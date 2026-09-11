package data

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPayment_StructFields(t *testing.T) {
	now := time.Now()
	mpPaymentID := "mp-pay-123"
	mpPrefID := "mp-pref-456"
	p := Payment{
		ID:             uuid.New(),
		BookingID:      uuid.New(),
		ComplexID:      uuid.New(),
		Amount:         15000,
		ServiceFee:     750,
		Method:         "mercadopago",
		Status:         "approved",
		MPPaymentID:    &mpPaymentID,
		MPPreferenceID: &mpPrefID,
		RefundAmount:   0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if p.Amount != 15000 {
		t.Errorf("Amount = %d, want %d", p.Amount, 15000)
	}
	if p.ServiceFee != 750 {
		t.Errorf("ServiceFee = %d, want %d", p.ServiceFee, 750)
	}
	if p.Method != "mercadopago" {
		t.Errorf("Method = %q, want %q", p.Method, "mercadopago")
	}
	if p.Status != "approved" {
		t.Errorf("Status = %q, want %q", p.Status, "approved")
	}
	if p.MPPaymentID == nil || *p.MPPaymentID != "mp-pay-123" {
		t.Errorf("MPPaymentID unexpected value")
	}
	if p.RefundAmount != 0 {
		t.Errorf("RefundAmount = %d, want %d", p.RefundAmount, 0)
	}
}

func TestPayment_NilOptionalFields(t *testing.T) {
	p := Payment{
		ID:     uuid.New(),
		Method: "cash",
		Status: "approved",
	}

	if p.MPPaymentID != nil {
		t.Error("MPPaymentID should be nil for cash payment")
	}
	if p.MPPreferenceID != nil {
		t.Error("MPPreferenceID should be nil for cash payment")
	}
}

// TestPaymentModel_RequiresDB documents that all PaymentModel methods
// require a database connection.
func TestPaymentModel_RequiresDB(t *testing.T) {
	t.Skip("PaymentModel methods all require *pgxpool.Pool and *db.Queries")
}

package cashbox

import (
	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
)

// toGenCashSession maps a store session onto the generated wire type (rule
// HTTP-08). Every field on the wire is reproduced here by name.
func toGenCashSession(s *cashboxstore.CashSession) gen.CashSession {
	return gen.CashSession{
		Id:           s.ID,
		ComplexId:    s.ComplexID,
		OpenedAt:     s.OpenedAt,
		OpenedBy:     s.OpenedBy,
		OpeningCash:  s.OpeningCash,
		ClosedAt:     s.ClosedAt,
		ClosedBy:     s.ClosedBy,
		CountedCash:  s.CountedCash,
		ExpectedCash: s.ExpectedCash,
		Difference:   s.Difference,
		OpeningNote:  s.OpeningNote,
		ClosingNote:  s.ClosingNote,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// toGenCashMovement maps a store movement onto the generated wire type.
func toGenCashMovement(m *cashboxstore.CashMovement) gen.CashMovement {
	return gen.CashMovement{
		Id:              m.ID,
		ComplexId:       m.ComplexID,
		SessionId:       m.SessionID,
		Kind:            gen.CashMovementKind(m.Kind),
		Category:        gen.CashMovementCategory(m.Category),
		Method:          gen.CashMovementMethod(m.Method),
		Amount:          m.Amount,
		Note:            m.Note,
		VoidsMovementId: m.VoidsMovementID,
		CreatedAt:       m.CreatedAt,
		CreatedBy:       m.CreatedBy,
	}
}

// toGenCashMovementTotal maps one movement-totals bucket onto the generated
// wire type.
func toGenCashMovementTotal(t cashboxstore.MovementTotal) gen.CashMovementTotal {
	return gen.CashMovementTotal{
		Method:   gen.CashMovementTotalMethod(t.Method),
		Kind:     gen.CashMovementTotalKind(t.Kind),
		Category: t.Category,
		Total:    t.Total,
		Count:    t.Count,
	}
}

// toGenCashBookingPaymentTotal maps one booking-payment method breakdown row
// onto the generated wire type.
func toGenCashBookingPaymentTotal(p reportstore.PaymentMethodSummary) gen.CashBookingPaymentTotal {
	return gen.CashBookingPaymentTotal{
		Method:     gen.CashBookingPaymentTotalMethod(p.Method),
		Count:      p.Count,
		Amount:     p.Amount,
		ServiceFee: p.ServiceFee,
		Refunded:   p.Refunded,
	}
}

// toGenCashManualRefundTotal maps one manual-refund method breakdown row onto
// the generated wire type.
func toGenCashManualRefundTotal(r reportstore.ManualRefundMethodSummary) gen.CashManualRefundTotal {
	return gen.CashManualRefundTotal{
		Method: gen.CashManualRefundTotalMethod(r.Method),
		Count:  r.Count,
		Amount: r.Amount,
	}
}

// toGenCashSessionSummary maps a session's reconciliation view onto the
// generated wire type.
func toGenCashSessionSummary(s Summary) gen.CashSessionSummary {
	movementTotals := make([]gen.CashMovementTotal, len(s.MovementTotals))
	for i, t := range s.MovementTotals {
		movementTotals[i] = toGenCashMovementTotal(t)
	}

	bookingPayments := make([]gen.CashBookingPaymentTotal, len(s.BookingPayments))
	for i, p := range s.BookingPayments {
		bookingPayments[i] = toGenCashBookingPaymentTotal(p)
	}

	manualRefunds := make([]gen.CashManualRefundTotal, len(s.ManualRefunds))
	for i, r := range s.ManualRefunds {
		manualRefunds[i] = toGenCashManualRefundTotal(r)
	}

	return gen.CashSessionSummary{
		OpeningCash:       s.OpeningCash,
		ExpectedCash:      s.ExpectedCash,
		CountedCash:       s.CountedCash,
		Difference:        s.Difference,
		CashManualRefunds: s.CashManualRefunds,
		MovementTotals:    movementTotals,
		BookingPayments:   bookingPayments,
		ManualRefunds:     manualRefunds,
	}
}

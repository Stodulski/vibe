package mailer

import (
	"strings"
	"testing"
)

// A fixed booking whose dates were moved to another court on conflict has to
// say where those dates are played: the client otherwise shows up at the
// series' own court, which somebody else holds that day.
func TestSeriesCreatedEmailNamesTheCourtOfEachMovedDate(t *testing.T) {
	got := captureSeries(t, func(m *Mailer) error {
		return m.SendSeriesCreated(t.Context(), "ana@example.com", "Vibe Padel", "Cancha 1", "lunes", "19:00 a 20:30",
			"16/11", "", "", "", []string{"23/11: Cancha 3", "30/11: Cancha 2"}, Contact{})
	})
	requireAll(t, got, "23/11: Cancha 3", "30/11: Cancha 2")
}

// Money the complex handed back itself never passes through a card or a
// MercadoPago account, so the email must not promise it will appear there in a
// few business days. A MercadoPago refund still says so.
func TestARefundReturnedByHandDoesNotPromiseProcessingDays(t *testing.T) {
	const processing = "próximos días hábiles"
	byHand := captureSeries(t, func(m *Mailer) error {
		return m.SendDepositRefunded(t.Context(), "ana@example.com", "Vibe", "$5.000", "", false, "", "", "",
			"El complejo te lo devolvió directamente.", true, Contact{})
	})
	if strings.Contains(byHand.HTMLContent, processing) {
		t.Errorf("a refund returned by hand must not promise processing days\n%s", byHand.HTMLContent)
	}
	requireAll(t, byHand, "$5.000", "El complejo te lo devolvió directamente.")

	viaMP := captureSeries(t, func(m *Mailer) error {
		return m.SendDepositRefunded(t.Context(), "ana@example.com", "Vibe", "$5.000", "", false, "", "", "", "", false, Contact{})
	})
	requireAll(t, viaMP, processing)
}

// A counter payment that leaves a balance says how much is still owed; one
// that settles the booking says nothing is.
func TestPaymentReceiptStatesWhatIsStillOwed(t *testing.T) {
	partial := captureSeries(t, func(m *Mailer) error {
		return m.SendPaymentReceived(t.Context(), "ana@example.com", "Vibe", "Cancha 1", "19/11", "18:00 a 19:30",
			"$5.000", "en efectivo", "$15.000", "", Contact{})
	})
	requireAll(t, partial, "$5.000", "en efectivo", "$15.000")
	if strings.Contains(partial.HTMLContent, "está paga") {
		t.Error("a receipt that leaves a balance must not call the booking paid")
	}

	settled := captureSeries(t, func(m *Mailer) error {
		return m.SendPaymentReceived(t.Context(), "ana@example.com", "Vibe", "Cancha 1", "19/11", "18:00 a 19:30",
			"$20.000", "por transferencia", "", "", Contact{})
	})
	requireAll(t, settled, "$20.000", "está paga")
}

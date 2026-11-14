package mailer

import "testing"

func TestEventRegisteredEmailNamesTheEventWhenAndWhereToGo(t *testing.T) {
	got := captureSeries(t, func(m *Mailer) error {
		return m.SendEventRegistered(t.Context(), "ana@example.com", "Ana", "Vibe Padel", "Canchas libres",
			"sábado 15/11", "19:00 a 21:00", "Av. Siempreviva 742", "https://maps.example/x", "$ 8.000", "https://vibe.test/c/vibe/events/1")
	})
	if got.Subject != "Su lugar en Canchas libres · Vibe Padel" {
		t.Errorf("subject = %q", got.Subject)
	}
	requireAll(t, got, "Su lugar está confirmado.", "Canchas libres", "sábado 15/11", "19:00 a 21:00",
		"$ 8.000", "Av. Siempreviva 742", "https://vibe.test/c/vibe/events/1")
}

func TestEventCancelledEmailSaysWhoCancelledAndWhatHappenedToTheMoney(t *testing.T) {
	owner := captureSeries(t, func(m *Mailer) error {
		return m.SendEventCancelled(t.Context(), "ana@example.com", "Ana", "Vibe Padel", "Canchas libres",
			"sábado 15/11", "19:00 a 21:00", true, "Le devolvimos el pago:", "$ 8.000", "https://vibe.test/c/vibe")
	})
	if owner.Subject != "Evento cancelado · Canchas libres" {
		t.Errorf("subject = %q", owner.Subject)
	}
	requireAll(t, owner, "El evento fue cancelado.", "el complejo canceló", "Le devolvimos el pago:", "$ 8.000")

	self := captureSeries(t, func(m *Mailer) error {
		return m.SendEventCancelled(t.Context(), "ana@example.com", "Ana", "Vibe Padel", "Canchas libres",
			"sábado 15/11", "19:00 a 21:00", false, "", "", "")
	})
	if self.Subject != "Inscripción cancelada · Canchas libres" {
		t.Errorf("subject = %q", self.Subject)
	}
	requireAll(t, self, "Su inscripción fue cancelada.")
}

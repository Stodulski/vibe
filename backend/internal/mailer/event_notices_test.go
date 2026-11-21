package mailer

import (
	"strings"
	"testing"
)

func TestOwnerEventEmailsNameWhoWhatAndWhen(t *testing.T) {
	n := OwnerEventNotice{ComplexName: "Vibe Padel", EventName: "Canchas abiertas", Date: "sábado 15/11",
		Hours: "19:00 a 21:00", ParticipantName: "Ana Pérez", Amount: "$ 8.000"}
	paid := captureSeries(t, func(m *Mailer) error { return m.SendOwnerEventRegistered(t.Context(), "owner@example.com", n) })
	requireAll(t, paid, "Ana Pérez", "Canchas abiertas", "sábado 15/11", "19:00 a 21:00", "$ 8.000")

	gone := captureSeries(t, func(m *Mailer) error { return m.SendOwnerEventCancelled(t.Context(), "owner@example.com", n) })
	requireAll(t, gone, "Ana Pérez", "Canchas abiertas", "sábado 15/11")
	if strings.Contains(gone.HTMLContent, "$ 8.000") {
		t.Error("the cancellation says nothing about money: the refund is the participant's business")
	}
}

func TestEventRefundedEmailQuotesTheAmountAndKeepsTheSpot(t *testing.T) {
	got := captureSeries(t, func(m *Mailer) error {
		return m.SendEventRefunded(t.Context(), "ana@example.com", "Ana", "Vibe Padel", "Canchas abiertas",
			"sábado 15/11", "$ 2.000", "https://vibe.test/e/1", Contact{})
	})
	requireAll(t, got, "$ 2.000", "Canchas abiertas", "sábado 15/11", "https://vibe.test/e/1")
}

func TestEventReminderEmailSaysWhenAndHowToGetThere(t *testing.T) {
	got := captureSeries(t, func(m *Mailer) error {
		return m.SendEventReminder(t.Context(), "ana@example.com", "Ana", "Vibe Padel", "Canchas abiertas",
			"mañana sábado 15/11 a las 19:00", "Av. Siempreviva 742", "https://maps.test/x", "https://vibe.test/e/1", Contact{})
	})
	requireAll(t, got, "mañana sábado 15/11 a las 19:00", "Av. Siempreviva 742", "https://vibe.test/e/1")
}

func TestTournamentMatchEmailsNameOpponentCourtAndTime(t *testing.T) {
	for _, kind := range []string{TournamentMatchScheduled, TournamentMatchMoved, TournamentMatchReminder} {
		got := captureSeries(t, func(m *Mailer) error {
			return m.SendTournamentNotice(t.Context(), "ana@example.com", TournamentNotice{
				Kind: kind, FirstName: "Ana", ComplexName: "Vibe Padel", TournamentName: "Copa Vibe", CategoryName: "7ma",
				When: "sábado 22/11 a las 19:00", Court: "Cancha 2", Opponent: "Pérez / Gómez",
				Address: "Av. Siempreviva 742", MapsURL: "https://maps.test/x", PageURL: "https://vibe.test/t/1",
			}, Contact{})
		})
		requireAll(t, got, "Pérez / Gómez", "sábado 22/11 a las 19:00", "Cancha 2", "Copa Vibe", "https://vibe.test/t/1")
	}
}

func TestTournamentDrawEmailsTellPublishedFromRemade(t *testing.T) {
	send := func(kind string) sentSeries {
		return captureSeries(t, func(m *Mailer) error {
			return m.SendTournamentNotice(t.Context(), "ana@example.com", TournamentNotice{
				Kind: kind, FirstName: "Ana", TournamentName: "Copa Vibe", CategoryName: "7ma", PageURL: "https://vibe.test/t/1",
			}, Contact{})
		})
	}
	published, remade := send(TournamentDrawPublished), send(TournamentDrawRemade)
	requireAll(t, published, "7ma", "https://vibe.test/t/1")
	requireAll(t, remade, "7ma", "https://vibe.test/t/1")
	if published.Subject == remade.Subject {
		t.Error("a remade draw must not read like the first one")
	}
}

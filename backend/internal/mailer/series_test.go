package mailer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var tagRX = regexp.MustCompile(`(?s)<style.*?</style>|<!--.*?-->|<[^>]*>`)

type sentSeries struct {
	Subject     string `json:"subject"`
	HTMLContent string `json:"htmlContent"`
}

func captureSeries(t *testing.T, send func(m *Mailer) error) sentSeries {
	t.Helper()
	var sent sentSeries
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"series-1"}`))
	}))
	t.Cleanup(brevo.Close)
	m := New(Config{BrevoAPIKey: "key", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL
	if err := send(m); err != nil {
		t.Fatalf("send: %v", err)
	}
	return sent
}

func requireAll(t *testing.T, got sentSeries, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got.HTMLContent, want) {
			t.Errorf("body is missing %q\n%s", want, got.HTMLContent)
		}
	}
	if strings.Contains(tagRX.ReplaceAllString(got.HTMLContent, "")+got.Subject, "!") {
		t.Error("the copy must not use exclamation marks")
	}
}

func TestSeriesCreatedEmailNamesTheWeekdayHoursCourtAndWhereToGo(t *testing.T) {
	got := captureSeries(t, func(m *Mailer) error {
		return m.SendSeriesCreated(t.Context(), "ana@example.com", "Vibe Padel", "Cancha 1", "lunes", "19:00 a 20:30",
			"16/11", "25/01", "Av. Santa Fe 1200", "https://maps.test/x")
	})
	if got.Subject != "Su turno fijo en Vibe Padel" {
		t.Errorf("subject = %q", got.Subject)
	}
	requireAll(t, got,
		"todos los <strong>lunes</strong> de <strong>19:00 a 20:30</strong> en <strong>Cancha 1</strong>",
		"desde el 16/11, hasta el 25/01",
		"Cada turno se paga en el complejo.",
		"Av. Santa Fe 1200", `href="https://maps.test/x"`)
}

func TestSeriesCreatedEmailWithoutAnEndOmitsIt(t *testing.T) {
	got := captureSeries(t, func(m *Mailer) error {
		return m.SendSeriesCreated(t.Context(), "ana@example.com", "Vibe Padel", "Cancha 1", "sábados", "19:00 a 20:30", "16/11", "", "", "")
	})
	requireAll(t, got, "todos los <strong>sábados</strong>", "desde el 16/11.")
	if strings.Contains(got.HTMLContent, "hasta el") || strings.Contains(got.HTMLContent, "Cómo llegar") {
		t.Error("no end and no address: neither line belongs")
	}
}

func TestSeriesCancelledEmailSaysFromWhenAndWhatStands(t *testing.T) {
	got := captureSeries(t, func(m *Mailer) error {
		return m.SendSeriesCancelled(t.Context(), "ana@example.com", "Vibe Padel", "Cancha 1", "lunes", "19:00 a 20:30",
			"14/12", true, "https://vibe.test/vibe/book")
	})
	if got.Subject != "Turno fijo cancelado · Vibe Padel" {
		t.Errorf("subject = %q", got.Subject)
	}
	requireAll(t, got,
		"Su turno fijo de los <strong>lunes</strong> de <strong>19:00 a 20:30</strong> en <strong>Cancha 1</strong>",
		"se canceló desde el 14/12.", "Los turnos ya pagos se mantienen.", "Nueva reserva")

	got = captureSeries(t, func(m *Mailer) error {
		return m.SendSeriesCancelled(t.Context(), "ana@example.com", "Vibe Padel", "Cancha 1", "lunes", "19:00 a 20:30", "14/12", false, "")
	})
	if strings.Contains(got.HTMLContent, "ya pagos") {
		t.Error("nothing paid, nothing to say about payments")
	}
}

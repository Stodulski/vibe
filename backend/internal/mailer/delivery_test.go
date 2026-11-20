package mailer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// brevoCapture is a fake Brevo that keeps every request body and answers 201.
func brevoCapture(t *testing.T) (*Mailer, *[]string) {
	t.Helper()
	var bodies []string
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(brevo.Close)
	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL
	return m, &bodies
}

type brevoSent struct {
	Subject     string            `json:"subject"`
	HTMLContent string            `json:"htmlContent"`
	TextContent string            `json:"textContent"`
	ReplyTo     *brevoContact     `json:"replyTo"`
	Headers     map[string]string `json:"headers"`
}

func decodeSent(t *testing.T, raw string) brevoSent {
	t.Helper()
	var s brevoSent
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func confirmation(t *testing.T, m *Mailer, complexName, deposit, balance, state string, c Contact) {
	t.Helper()
	if err := m.SendBookingConfirmation(context.Background(), "ana@example.com", complexName, "Cancha 1",
		"15/03", "09:00 a 10:30", "", "", "https://vibe.test/cancel",
		deposit, balance, "con devolución hasta 24 horas antes del turno.", state, c); err != nil {
		t.Fatal(err)
	}
}

func TestThePlainTextPartDecodesEscapedQuotesAndAmpersands(t *testing.T) {
	m, bodies := brevoCapture(t)
	confirmation(t, m, `O'Connor "Padel" & Co`, "$5.000", "$15.000", PaidDeposit, Contact{})
	text := decodeSent(t, (*bodies)[0]).TextContent
	if !strings.Contains(text, `O'Connor "Padel" & Co`) {
		t.Errorf("plain text does not read the complex name back:\n%s", text)
	}
	for _, entity := range []string{"&#34;", "&#39;", "&amp;", "&quot;"} {
		if strings.Contains(text, entity) {
			t.Errorf("plain text carries the entity %s:\n%s", entity, text)
		}
	}
}

// S8: a staff booking with nothing paid says what is owed, not a $0 seña, and
// promises no refund; a booking paid in full is not called a seña.
func TestTheConfirmationTellsWhatWasPaid(t *testing.T) {
	m, bodies := brevoCapture(t)
	confirmation(t, m, "Vibe", "$0", "$20.000", PaidNothing, Contact{})
	unpaid := decodeSent(t, (*bodies)[0]).TextContent
	if strings.Contains(unpaid, "Seña") || strings.Contains(unpaid, "$0") || strings.Contains(unpaid, "devolución") {
		t.Errorf("an unpaid booking must not mention a seña, $0 or a refund:\n%s", unpaid)
	}
	if !strings.Contains(unpaid, "$20.000") {
		t.Errorf("an unpaid booking must say what is owed:\n%s", unpaid)
	}

	confirmation(t, m, "Vibe", "$20.000", "$0", PaidFull, Contact{})
	full := decodeSent(t, (*bodies)[1]).TextContent
	if strings.Contains(full, "Seña") || !strings.Contains(full, "Pagado") || !strings.Contains(full, "$20.000") {
		t.Errorf("a fully paid booking says it is paid, not a seña:\n%s", full)
	}
}

// S7: a client email replies to the complex and offers its phone, with a
// WhatsApp link when the phone reads as an international number.
func TestClientEmailsCarryTheComplexContact(t *testing.T) {
	m, bodies := brevoCapture(t)
	confirmation(t, m, "Vibe", "$5.000", "$15.000", PaidDeposit,
		Contact{Phone: "+54 9 11 2233-4455", ReplyTo: "dueno@vibe.test"})
	got := decodeSent(t, (*bodies)[0])
	if got.ReplyTo == nil || got.ReplyTo.Email != "dueno@vibe.test" {
		t.Errorf("Reply-To = %+v, want the owner's address", got.ReplyTo)
	}
	if !strings.Contains(got.HTMLContent, "+54 9 11 2233-4455") || !strings.Contains(got.HTMLContent, `href="https://wa.me/5491122334455"`) {
		t.Errorf("the email must show the phone and its wa.me link:\n%s", got.HTMLContent)
	}

	confirmation(t, m, "Vibe", "$5.000", "$15.000", PaidDeposit, Contact{Phone: "interno 12"})
	bare := decodeSent(t, (*bodies)[1])
	if bare.ReplyTo != nil || strings.Contains(bare.HTMLContent, "wa.me") || !strings.Contains(bare.HTMLContent, "interno 12") {
		t.Errorf("an unusable phone is shown without a link and no reply-to is invented: %+v", bare.ReplyTo)
	}
}

// S3: the reminder's subject names the real start, not a fixed countdown.
func TestTheReminderSubjectNamesTheRealStart(t *testing.T) {
	m, bodies := brevoCapture(t)
	if err := m.SendReminder2h(context.Background(), "ana@example.com", "Vibe", "Cancha 1", "19/11",
		"18:00 a 19:30", "hoy a las 18:00", "", "", "$15.000", "", Contact{}); err != nil {
		t.Fatal(err)
	}
	subject := decodeSent(t, (*bodies)[0]).Subject
	if !strings.Contains(subject, "hoy a las 18:00") || strings.Contains(subject, "2 horas") {
		t.Errorf("subject = %q", subject)
	}
}

// S4: a refund that cancelled the booking tells the client the booking is off.
func TestARefundThatCancelledTheBookingTellsTheClient(t *testing.T) {
	m, bodies := brevoCapture(t)
	if err := m.SendDepositRefunded(context.Background(), "ana@example.com", "Vibe", "$5.000", "",
		true, "Cancha 1", "19/11", "18:00 a 19:30", Contact{}); err != nil {
		t.Fatal(err)
	}
	got := decodeSent(t, (*bodies)[0])
	if !strings.Contains(got.TextContent, "se canceló") || !strings.Contains(got.TextContent, "Cancha 1") ||
		!strings.Contains(got.TextContent, "$5.000") {
		t.Errorf("the client must be told the booking is cancelled and the money returned:\n%s", got.TextContent)
	}
}

// S8: the welcome email escapes the first name it puts in <title>.
func TestTheWelcomeTitleEscapesTheName(t *testing.T) {
	m, bodies := brevoCapture(t)
	if err := m.SendWelcome(context.Background(), "ana@example.com", "<b>Ana</b>"); err != nil {
		t.Fatal(err)
	}
	html := decodeSent(t, (*bodies)[0]).HTMLContent
	if strings.Contains(html, "<b>Ana</b>") {
		t.Errorf("the name reached the markup unescaped:\n%s", html)
	}
}

package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// fakeProvider answers every request with status and body, and keeps each
// request it received with its headers.
type fakeProvider struct {
	srv      *httptest.Server
	bodies   []string
	auth     []string
	requests int
}

func newFakeProvider(t *testing.T, status int, body string) *fakeProvider {
	t.Helper()
	f := &fakeProvider{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.requests++
		f.bodies = append(f.bodies, string(b))
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// mailerWith points Brevo and Resend at the given fakes. A nil fake leaves that
// provider unconfigured.
func mailerWith(brevo, resend *fakeProvider, cb *circuitbreaker.CircuitBreaker) *Mailer {
	cfg := Config{Sender: "Vibe <no-reply@vibe.com.ar>", CB: cb}
	if brevo != nil {
		cfg.BrevoAPIKey = "brevo-key"
	}
	if resend != nil {
		cfg.ResendAPIKey = "re_key"
	}
	m := New(cfg)
	if brevo != nil {
		m.apiURL = brevo.srv.URL
	}
	if resend != nil {
		m.resendURL = resend.srv.URL
	}
	return m
}

func sendHi(m *Mailer) error {
	return m.sendWith(context.Background(), "ana@example.com", "Asunto", "<p>hola</p>", outbound{
		headers: map[string]string{"List-Unsubscribe": "<https://vibe.test/baja>"},
		replyTo: "dueno@example.com",
	})
}

// Brevo's free plan stops at its daily cap; Resend carries the rest of the day.
func TestBrevoOutOfQuotaFallsBackToResend(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			brevo := newFakeProvider(t, status, `{"code":"unauthorized","message":"Maximum credits exceeded"}`)
			resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)

			if err := sendHi(mailerWith(brevo, resend, nil)); err != nil {
				t.Fatalf("send = %v; Resend should have carried it", err)
			}
			if resend.requests != 1 {
				t.Fatalf("Resend got %d requests, want 1", resend.requests)
			}
		})
	}
}

func TestResendGetsTheWholeMessage(t *testing.T) {
	brevo := newFakeProvider(t, http.StatusTooManyRequests, "")
	resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)

	if err := sendHi(mailerWith(brevo, resend, nil)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		From    string            `json:"from"`
		To      []string          `json:"to"`
		Subject string            `json:"subject"`
		HTML    string            `json:"html"`
		Text    string            `json:"text"`
		ReplyTo string            `json:"reply_to"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(resend.bodies[0]), &got); err != nil {
		t.Fatal(err)
	}
	if got.From != "Vibe <no-reply@vibe.com.ar>" || len(got.To) != 1 || got.To[0] != "ana@example.com" ||
		got.Subject != "Asunto" || got.HTML != "<p>hola</p>" || got.Text != "hola" ||
		got.ReplyTo != "dueno@example.com" || got.Headers["List-Unsubscribe"] != "<https://vibe.test/baja>" {
		t.Errorf("Resend received %+v", got)
	}
	if resend.auth[0] != "Bearer re_key" {
		t.Errorf("Authorization = %q, want the Resend key as a bearer token", resend.auth[0])
	}
}

func TestBrevoSuccessNeverTouchesResend(t *testing.T) {
	brevo := newFakeProvider(t, http.StatusCreated, `{"messageId":"<a@brevo>"}`)
	resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)

	if err := sendHi(mailerWith(brevo, resend, nil)); err != nil {
		t.Fatal(err)
	}
	if resend.requests != 0 {
		t.Fatal("Resend sent a message Brevo had already accepted")
	}
}

// A message Brevo refused on its own merits gets the same refusal anywhere.
func TestAMessageBrevoRefusedIsNotSentAgainOverResend(t *testing.T) {
	brevo := newFakeProvider(t, http.StatusBadRequest, `{"message":"invalid email"}`)
	resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)

	err := sendHi(mailerWith(brevo, resend, nil))
	if !isPermanent(err) {
		t.Fatalf("send = %v; want Brevo's permanent refusal", err)
	}
	if resend.requests != 0 {
		t.Fatal("Resend was tried for a message Brevo refused on its merits")
	}
}

// A timeout after the request went out leaves Brevo's answer unknown: it may
// already have queued the message, and Resend would send it a second time.
func TestABrevoTimeoutAfterTheRequestDoesNotFallBack(t *testing.T) {
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()
	resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)

	m := mailerWith(nil, resend, nil)
	m.apiKey = "brevo-key"
	m.apiURL = brevo.URL
	m.httpClient = &http.Client{Timeout: 100 * time.Millisecond}

	if err := sendHi(m); err == nil {
		t.Fatal("send = nil; the timeout must reach the queue for a retry")
	}
	if resend.requests != 0 {
		t.Fatal("Resend sent a message Brevo may already have accepted")
	}
}

// A connection Brevo refused never carried the message, so Resend may send it.
func TestARefusedBrevoConnectionFallsBackToResend(t *testing.T) {
	resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)
	m := mailerWith(nil, resend, nil)
	m.apiKey = "brevo-key"
	m.apiURL = fmt.Sprintf("http://127.0.0.1:%d/v3/smtp/email", closedPort(t))

	if err := sendHi(m); err != nil {
		t.Fatalf("send = %v; Resend should have carried it", err)
	}
	if resend.requests != 1 {
		t.Fatal("Resend was never tried")
	}
}

// Both providers out of quota: the queue must retry later, and the breaker
// counts it, since no message can leave until a quota resets.
func TestBothProvidersOutOfQuotaIsRetriedAndCounted(t *testing.T) {
	brevo := newFakeProvider(t, http.StatusUnauthorized, `{"message":"Maximum credits exceeded"}`)
	resend := newFakeProvider(t, http.StatusTooManyRequests, `{"name":"daily_quota_exceeded"}`)

	var opened bool
	cb := circuitbreaker.New(circuitbreaker.Config{
		Name: "mail", MaxFailures: 1, ResetTimeout: time.Hour,
		OnStateChange: func(_ string, _, to circuitbreaker.State) {
			if to == circuitbreaker.StateOpen {
				opened = true
			}
		},
	})

	err := sendHi(mailerWith(brevo, resend, cb))
	if err == nil {
		t.Fatal("send = nil with both providers refusing")
	}
	if isPermanent(err) {
		t.Fatalf("send = %v; a quota resets, so the message must be retried, not dead-lettered", err)
	}
	if !opened {
		t.Error("both providers out of quota must count against the breaker")
	}
}

// When Brevo fails but Resend delivers, nothing is wrong for the breaker.
func TestAFallbackThatDeliversKeepsTheBreakerClosed(t *testing.T) {
	brevo := newFakeProvider(t, http.StatusTooManyRequests, "")
	resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)
	cb := circuitbreaker.New(circuitbreaker.Config{Name: "mail", MaxFailures: 1, ResetTimeout: time.Hour})

	if err := sendHi(mailerWith(brevo, resend, cb)); err != nil {
		t.Fatal(err)
	}
	if err := cb.AllowRequest(); err != nil {
		t.Fatalf("breaker = %v after a delivered message", err)
	}
}

func TestResendAloneCarriesMail(t *testing.T) {
	resend := newFakeProvider(t, http.StatusOK, `{"id":"abc"}`)
	m := mailerWith(nil, resend, nil)
	if err := sendHi(m); err != nil {
		t.Fatal(err)
	}
	if resend.requests != 1 {
		t.Fatal("Resend was not used with no Brevo key")
	}
}

func TestNoProviderIsErrNoTransport(t *testing.T) {
	if err := sendHi(mailerWith(nil, nil, nil)); !errors.Is(err, ErrNoTransport) {
		t.Fatalf("send = %v, want ErrNoTransport", err)
	}
}

// closedPort returns a port nothing is listening on, so a dial to it fails fast.
func closedPort(t *testing.T) int {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is not TCP: %T", ln.Addr())
	}
	_ = ln.Close()
	return addr.Port
}

package mailer

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// brevoPayload extracts the HTML and plain-text bodies Brevo actually
// received from one raw captured request, undoing the JSON encoding
// (including its default HTML-escaping of '<', '>' and '&') so assertions
// can search the real markup rather than its JSON-escaped form.
func brevoPayload(t *testing.T, rawRequestBody string) (htmlBody, textBody string) {
	t.Helper()
	var payload struct {
		HTMLContent string `json:"htmlContent"`
		TextContent string `json:"textContent"`
	}
	if err := json.Unmarshal([]byte(rawRequestBody), &payload); err != nil {
		t.Fatalf("captured request body is not valid JSON: %v", err)
	}
	return payload.HTMLContent, payload.TextContent
}

func TestNew(t *testing.T) {
	cfg := Config{
		BrevoAPIKey:  "test-key",
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUsername: "user",
		SMTPPassword: "pass",
		Sender:       "Vibe <noreply@vibe.com>",
		LogoURL:      "https://example.com/logo.png",
		CB:           circuitbreaker.New(circuitbreaker.Config{Name: "mail"}),
	}

	m := New(cfg)

	if m.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", m.apiKey, "test-key")
	}
	if m.smtpHost != "smtp.example.com" {
		t.Errorf("smtpHost = %q, want %q", m.smtpHost, "smtp.example.com")
	}
	if m.smtpPort != 587 {
		t.Errorf("smtpPort = %d, want %d", m.smtpPort, 587)
	}
	if m.smtpUsername != "user" {
		t.Errorf("smtpUsername = %q, want %q", m.smtpUsername, "user")
	}
	if m.smtpPassword != "pass" {
		t.Errorf("smtpPassword = %q, want %q", m.smtpPassword, "pass")
	}
	if m.senderEmail != "noreply@vibe.com" {
		t.Errorf("senderEmail = %q, want %q", m.senderEmail, "noreply@vibe.com")
	}
	if m.senderName != "Vibe" {
		t.Errorf("senderName = %q, want %q", m.senderName, "Vibe")
	}
	if m.senderRaw != "Vibe <noreply@vibe.com>" {
		t.Errorf("senderRaw = %q, want %q", m.senderRaw, "Vibe <noreply@vibe.com>")
	}
	if m.logoURL != "https://example.com/logo.png" {
		t.Errorf("logoURL = %q, want %q", m.logoURL, "https://example.com/logo.png")
	}
	if m.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if m.cb == nil {
		t.Error("cb is nil")
	}
}

func TestNew_PlainSender(t *testing.T) {
	cfg := Config{Sender: "noreply@vibe.com"}
	m := New(cfg)

	if m.senderEmail != "noreply@vibe.com" {
		t.Errorf("senderEmail = %q, want %q", m.senderEmail, "noreply@vibe.com")
	}
	if m.senderName != "" {
		t.Errorf("senderName = %q, want empty", m.senderName)
	}
}

func TestMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "brevo_api when apiKey set",
			cfg:  Config{BrevoAPIKey: "key-123"},
			want: "brevo_api",
		},
		{
			name: "smtp when smtp credentials set",
			cfg:  Config{SMTPUsername: "user", SMTPPassword: "pass"},
			want: "smtp",
		},
		{
			name: "disabled when nothing set",
			cfg:  Config{},
			want: "disabled",
		},
		{
			name: "brevo_api with a real smtp fallback when both are set",
			cfg:  Config{BrevoAPIKey: "key", SMTPUsername: "user", SMTPPassword: "pass"},
			want: "brevo_api+smtp",
		},
		{
			name: "disabled when only username set",
			cfg:  Config{SMTPUsername: "user"},
			want: "disabled",
		},
		{
			name: "disabled when only password set",
			cfg:  Config{SMTPPassword: "pass"},
			want: "disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(tt.cfg)
			if got := m.Mode(); got != tt.want {
				t.Errorf("Mode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHtmlToPlainText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips HTML tags",
			input: "<p>Hello <strong>world</strong></p>",
			want:  "Hello world",
		},
		{
			name:  "converts block-level closing tags to newlines",
			input: "<p>First paragraph</p><p>Second paragraph</p>",
			want:  "First paragraph\nSecond paragraph",
		},
		{
			name:  "converts br to newlines",
			input: "Line one<br>Line two<br/>Line three<br />Line four",
			want:  "Line one\nLine two\nLine three\nLine four",
		},
		{
			name:  "decodes HTML entities amp lt gt quot",
			input: "<p>&amp; &lt; &gt; &quot; &#39;</p>",
			want:  "& < > \" '",
		},
		{
			name:  "decodes Spanish HTML entities",
			input: "<p>&aacute; &eacute; &iacute; &oacute; &uacute; &ntilde; &iexcl;</p>",
			want:  "\u00e1 \u00e9 \u00ed \u00f3 \u00fa \u00f1 \u00a1",
		},
		{
			name:  "collapses whitespace",
			input: "<p>Hello    world</p>",
			want:  "Hello world",
		},
		{
			name:  "collapses excessive newlines",
			input: "<p>A</p><p></p><p></p><p></p><p>B</p>",
			want:  "A\n\nB",
		},
		{
			name:  "converts nbsp to space",
			input: "<p>Hello&nbsp;world</p>",
			want:  "Hello world",
		},
		{
			name:  "handles heading closing tags",
			input: "<h1>Title</h1><h2>Subtitle</h2><p>Body</p>",
			want:  "Title\nSubtitle\nBody",
		},
		{
			name:  "handles div closing tags",
			input: "<div>Block one</div><div>Block two</div>",
			want:  "Block one\nBlock two",
		},
		{
			name:  "strips zero-width and zwnj entities",
			input: "Hello&#847;&zwnj;World",
			want:  "HelloWorld",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToPlainText(tt.input)
			if got != tt.want {
				t.Errorf("htmlToPlainText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBrevoError(t *testing.T) {
	e := &brevoError{statusCode: 429, body: "rate limited"}
	want := "mailer: brevo API returned 429: rate limited"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestIsInfraError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil returns false",
			err:  nil,
			want: false,
		},
		{
			name: "brevoError with 500 returns true",
			err:  &brevoError{statusCode: 500, body: "internal server error"},
			want: true,
		},
		{
			name: "brevoError with 502 returns true",
			err:  &brevoError{statusCode: 502, body: "bad gateway"},
			want: true,
		},
		{
			name: "brevoError with 400 returns false",
			err:  &brevoError{statusCode: 400, body: "bad request"},
			want: false,
		},
		{
			// 429 is the provider telling us it is shedding load. Backing off
			// is the whole point of the breaker, so this counts.
			name: "brevoError with 429 returns true",
			err:  &brevoError{statusCode: 429, body: "too many requests"},
			want: true,
		},
		{
			// A rotated key fails every message we will ever send. Recording
			// it as a success left the breaker closed through 100% mail loss.
			name: "brevoError with 401 returns true",
			err:  &brevoError{statusCode: 401, body: "key not found"},
			want: true,
		},
		{
			name: "generic error returns true",
			err:  &testNetErr{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInfraError(tt.err); got != tt.want {
				t.Errorf("isInfraError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// testNetErr simulates a generic network error.
type testNetErr struct{}

func (e *testNetErr) Error() string { return "network error" }

func TestGenerateMessageID(t *testing.T) {
	id := generateMessageID("example.com")

	if id[0] != '<' {
		t.Errorf("message ID should start with '<', got %q", id)
	}
	if id[len(id)-1] != '>' {
		t.Errorf("message ID should end with '>', got %q", id)
	}

	inner := id[1 : len(id)-1]
	if idx := len(inner) - len("@example.com"); idx <= 0 {
		t.Errorf("message ID too short: %q", id)
	} else if inner[idx:] != "@example.com" {
		t.Errorf("message ID should contain @example.com, got %q", id)
	}

	// Ensure uniqueness.
	id2 := generateMessageID("example.com")
	if id == id2 {
		t.Error("two calls to generateMessageID returned the same value")
	}
}

func TestPreheaderBlock(t *testing.T) {
	out := preheaderBlock("Preview text")

	if out == "" {
		t.Fatal("preheaderBlock returned empty string")
	}

	// Should be a hidden div.
	if !contains(out, `display:none`) {
		t.Error("preheaderBlock should contain display:none")
	}
	if !contains(out, "Preview text") {
		t.Error("preheaderBlock should contain the provided text")
	}
	if !contains(out, "<div") {
		t.Error("preheaderBlock should return a div element")
	}
	// Should contain padding zero-width chars.
	if !contains(out, "&#847;") {
		t.Error("preheaderBlock should contain zero-width padding")
	}
}

// contains is a helper for substring checks in test assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Failure classification
// ---------------------------------------------------------------------------

// TestBrevoErrorClassification pins the split between "the service is having
// problems" and "this message is bad", which used to be drawn at status 500 —
// so a 401 from a rotated API key was recorded as a successful send.
func TestBrevoErrorClassification(t *testing.T) {
	tests := []struct {
		status        int
		wantInfra     bool
		wantPermanent bool
		why           string
	}{
		{status: 500, wantInfra: true, why: "provider is broken"},
		{status: 502, wantInfra: true, why: "provider is broken"},
		{status: 429, wantInfra: true, why: "provider is shedding load"},
		{status: 401, wantInfra: true, why: "our credentials are refused, which fails every message"},
		{status: 403, wantInfra: true, why: "our credentials are refused, which fails every message"},
		{status: 302, wantInfra: true, why: "a JSON API answering with a redirect is misrouted"},
		{status: 400, wantPermanent: true, why: "this payload is malformed"},
		{status: 404, wantPermanent: true, why: "this request is wrong"},
		{status: 422, wantPermanent: true, why: "this recipient is unusable"},
	}

	for _, tt := range tests {
		t.Run(tt.why, func(t *testing.T) {
			err := &brevoError{statusCode: tt.status, body: "..."}

			if got := err.infra(); got != tt.wantInfra {
				t.Errorf("status %d: infra() = %v, want %v (%s)", tt.status, got, tt.wantInfra, tt.why)
			}
			if got := err.Permanent(); got != tt.wantPermanent {
				t.Errorf("status %d: Permanent() = %v, want %v (%s)", tt.status, got, tt.wantPermanent, tt.why)
			}
			if got := isInfraError(err); got != tt.wantInfra {
				t.Errorf("status %d: isInfraError() = %v, want %v (%s)", tt.status, got, tt.wantInfra, tt.why)
			}
		})
	}
}

func TestIsInfraErrorTreatsUnknownFailuresAsInfra(t *testing.T) {
	if !isInfraError(errors.New("dial tcp: connection refused")) {
		t.Error("a network failure must count against the breaker")
	}
	if isInfraError(nil) {
		t.Error("nil is not a failure")
	}
	if isInfraError(ErrNoTransport) {
		t.Error("a missing transport is a configuration fact, not a provider outage")
	}
}

func TestErrNoTransportIsPermanent(t *testing.T) {
	if !isPermanent(ErrNoTransport) {
		t.Error("ErrNoTransport must be permanent: retrying a message with nowhere to send it cannot help")
	}

	m := New(Config{Sender: "Vibe <noreply@vibe.com.ar>"})
	if err := m.send(context.Background(), "client@example.com", "hi", "<p>hi</p>"); !errors.Is(err, ErrNoTransport) {
		t.Errorf("send with no transport = %v, want ErrNoTransport", err)
	}
}

// TestOpenBreakerIsReportedAsNotAttempted keeps the wrapping that lets the task
// queue tell a refused send apart from a failed one.
func TestOpenBreakerIsReportedAsNotAttempted(t *testing.T) {
	cb := circuitbreaker.New(circuitbreaker.Config{Name: "mail", MaxFailures: 1, ResetTimeout: time.Hour})
	cb.RecordFailure() // opens it

	m := New(Config{BrevoAPIKey: "key", Sender: "Vibe <noreply@vibe.com.ar>", CB: cb})
	err := m.send(context.Background(), "client@example.com", "hi", "<p>hi</p>")

	if !errors.Is(err, circuitbreaker.ErrOpen) {
		t.Fatalf("send through an open breaker = %v, want an error wrapping circuitbreaker.ErrOpen", err)
	}
}

// ---------------------------------------------------------------------------
// SMTP fallback
// ---------------------------------------------------------------------------

// startFakeSMTP accepts one plain (non-TLS) SMTP session and returns the message
// body it received. net/smtp allows PLAIN auth without TLS to a loopback host,
// which is what makes this possible without a certificate.
func startFakeSMTP(t *testing.T) (string, int, <-chan string) {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is not TCP: %T", ln.Addr())
	}
	received := make(chan string, 1)

	go serveOneSMTPSession(ln, received)

	return "127.0.0.1", addr.Port, received
}

func serveOneSMTPSession(ln net.Listener, received chan<- string) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	reply := func(lines ...string) {
		for _, l := range lines {
			_, _ = writer.WriteString(l + "\r\n")
		}
		_ = writer.Flush()
	}

	reply("220 127.0.0.1 ESMTP fake")

	var body strings.Builder
	inData := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")

		if inData {
			if trimmed == "." {
				inData = false
				received <- body.String()
				reply("250 2.0.0 Ok: queued")
				continue
			}
			body.WriteString(line)
			continue
		}

		cmd := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			reply("250-127.0.0.1 Hello", "250 AUTH PLAIN")
		case strings.HasPrefix(cmd, "HELO"):
			reply("250 127.0.0.1 Hello")
		case strings.HasPrefix(cmd, "AUTH"):
			reply("235 2.7.0 Accepted")
		case cmd == "DATA":
			inData = true
			reply("354 End data with <CR><LF>.<CR><LF>")
		case cmd == "QUIT":
			reply("221 2.0.0 Bye")
			return
		default:
			reply("250 2.0.0 Ok")
		}
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

// TestBrevoOutageFallsBackToSMTP is the whole point of the "SMTP fallback"
// comment on the struct: the transport used to be picked once from config, so a
// Brevo outage was total mail loss on a deployment with working SMTP credentials.
func TestBrevoOutageFallsBackToSMTP(t *testing.T) {
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"service unavailable"}`))
	}))
	defer brevo.Close()

	host, port, received := startFakeSMTP(t)

	var opened bool
	cb := circuitbreaker.New(circuitbreaker.Config{
		Name: "mail", MaxFailures: 1, ResetTimeout: time.Hour,
		OnStateChange: func(_ string, _, to circuitbreaker.State) {
			if to == circuitbreaker.StateOpen {
				opened = true
			}
		},
	})

	m := New(Config{
		BrevoAPIKey: "rotated-key", SMTPHost: host, SMTPPort: port,
		SMTPUsername: "user", SMTPPassword: "pass",
		Sender: "Vibe <noreply@vibe.com.ar>", CB: cb,
	})
	m.apiURL = brevo.URL

	if err := m.send(context.Background(), "client@example.com", "Reserva confirmada", "<p>Tu reserva fue confirmada.</p>"); err != nil {
		t.Fatalf("send = %v, want nil: SMTP should have carried the message", err)
	}

	select {
	case body := <-received:
		if !strings.Contains(body, "Reserva confirmada") {
			t.Errorf("SMTP body does not carry the subject:\n%s", body)
		}
		if !strings.Contains(body, "client@example.com") {
			t.Errorf("SMTP body does not carry the recipient:\n%s", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP fallback was never attempted")
	}

	if opened {
		t.Error("the breaker opened on a send that ultimately succeeded")
	}
}

// TestBrevoAuthFailureFallsBackAndCountsAgainstTheBreaker covers the rotated-key
// case: a 401 used to be recorded as a success, so the breaker never opened and
// 100% permanent mail loss was reported as healthy.
func TestBrevoAuthFailureFallsBackAndCountsAgainstTheBreaker(t *testing.T) {
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Key not found"}`))
	}))
	defer brevo.Close()

	var opened bool
	cb := circuitbreaker.New(circuitbreaker.Config{
		Name: "mail", MaxFailures: 1, ResetTimeout: time.Hour,
		OnStateChange: func(_ string, _, to circuitbreaker.State) {
			if to == circuitbreaker.StateOpen {
				opened = true
			}
		},
	})

	m := New(Config{
		BrevoAPIKey: "rotated-key", SMTPHost: "127.0.0.1", SMTPPort: closedPort(t),
		SMTPUsername: "user", SMTPPassword: "pass",
		Sender: "Vibe <noreply@vibe.com.ar>", CB: cb,
	})
	m.apiURL = brevo.URL

	err := m.send(context.Background(), "client@example.com", "hi", "<p>hi</p>")
	if err == nil {
		t.Fatal("send = nil, want an error: both transports failed")
	}
	if !strings.Contains(err.Error(), "smtp") {
		t.Errorf("error does not mention the SMTP fallback, so it was never tried: %v", err)
	}
	if !opened {
		t.Error("a 401 from the mail provider must count against the breaker, not as a success")
	}
}

// TestPermanentRejectionSkipsTheFallback: SMTP would carry the same bytes to the
// same refusal, so falling back would only double the work.
func TestPermanentRejectionSkipsTheFallback(t *testing.T) {
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Invalid email address"}`))
	}))
	defer brevo.Close()

	var opened bool
	cb := circuitbreaker.New(circuitbreaker.Config{
		Name: "mail", MaxFailures: 1, ResetTimeout: time.Hour,
		OnStateChange: func(_ string, _, to circuitbreaker.State) {
			if to == circuitbreaker.StateOpen {
				opened = true
			}
		},
	})

	m := New(Config{
		BrevoAPIKey: "key", SMTPHost: "127.0.0.1", SMTPPort: closedPort(t),
		SMTPUsername: "user", SMTPPassword: "pass",
		Sender: "Vibe <noreply@vibe.com.ar>", CB: cb,
	})
	m.apiURL = brevo.URL

	err := m.send(context.Background(), "not-an-address", "hi", "<p>hi</p>")
	if err == nil {
		t.Fatal("send = nil, want the provider's rejection")
	}
	if strings.Contains(err.Error(), "smtp") {
		t.Errorf("the fallback ran for a message the provider itself refused: %v", err)
	}
	if !isPermanent(err) {
		t.Errorf("a 4xx rejection must be permanent so the queue stops retrying it: %v", err)
	}
	if opened {
		t.Error("one malformed recipient must not open the breaker for everybody else")
	}
}

func TestBrevoSuccessSkipsSMTP(t *testing.T) {
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"<abc@brevo>"}`))
	}))
	defer brevo.Close()

	host, port, received := startFakeSMTP(t)

	m := New(Config{
		BrevoAPIKey: "key", SMTPHost: host, SMTPPort: port,
		SMTPUsername: "user", SMTPPassword: "pass",
		Sender: "Vibe <noreply@vibe.com.ar>",
	})
	m.apiURL = brevo.URL

	if err := m.send(context.Background(), "client@example.com", "hi", "<p>hi</p>"); err != nil {
		t.Fatalf("send = %v, want nil", err)
	}

	select {
	case <-received:
		t.Error("SMTP was used even though Brevo accepted the message")
	case <-time.After(200 * time.Millisecond):
	}
}

// captureSent drives every one of the mailer's public send methods against a
// fake Brevo and returns what actually went over the wire, one entry per
// message: subject, HTML body and derived plain text, all of it.
//
// The placeholders below are deliberately neutral. This helper exists to read
// the mailer's OWN copy — the sentences that are compiled into this package —
// so nothing it passes in may itself contain a word the caller is looking for.
func captureSent(t *testing.T) []string {
	t.Helper()

	var bodies []string
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL

	ctx := context.Background()
	const (
		to      = "client@example.com"
		complex = "Vibe Palermo"
		court   = "Cancha 1"
		date    = "01/09"
		hours   = "18:00 - 19:30"
	)
	sends := []func() error{
		func() error {
			return m.SendBookingConfirmation(ctx, to, complex, court, date, hours,
				"Av. Santa Fe 1200", "https://maps.example/?q=1", "https://vibe.test/cancel",
				"DEPOSIT_AMOUNT", "BALANCE_AMOUNT", "CANCELLATION_LINE")
		},
		func() error {
			return m.SendReminder2h(ctx, to, complex, court, date, hours,
				"Av. Santa Fe 1200", "https://maps.example/?q=1", "BALANCE_AMOUNT", "https://vibe.test/cancel")
		},
		func() error {
			return m.SendBookingCancelled(ctx, to, complex, court, date, hours,
				"REFUND_LINE", "$5.000", "https://vibe.test/book")
		},
		func() error { return m.SendDepositRefunded(ctx, to, complex, "$5.000", "https://vibe.test/book") },
		func() error { return m.SendOwnerNewBooking(ctx, to, complex, court, "Ana Perez", date, hours) },
		func() error { return m.SendEmailVerification(ctx, to, "Ana", "https://vibe.test/verify?t=1") },
		func() error { return m.SendPasswordReset(ctx, to, "Ana", "https://vibe.test/reset?t=2") },
		func() error {
			return m.SendDuplicateRegistration(ctx, to, "Ana", "https://vibe.test/login", "https://vibe.test/forgot")
		},
	}
	for i, send := range sends {
		if err := send(); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if len(bodies) != len(sends) {
		t.Fatalf("want %d messages captured; got %d", len(sends), len(bodies))
	}
	return bodies
}

// TestNoEmailUsesAForbiddenWord reads every string this package can put in
// front of a client and holds it to the product's vocabulary.
//
// "seña" and "devolución", never "reembolso". "link", never "enlace".
// "complejo", never "club". Before this test, the confirmation said "cancelar
// con reembolso" twice, the refund email was headed "Reembolso realizado" over
// a body that said "devolución" — one message contradicting itself — and both
// account emails said "el enlace expira".
//
// It reads the rendered message rather than the source, so a word in a subject
// line, a preheader, a button label or a hidden preview counts exactly as much
// as one in the body. All three have carried a violation at some point.
func TestNoEmailUsesAForbiddenWord(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)\b(reembolsos?|enlaces?|clubes?|club)\b`)

	for _, body := range captureSent(t) {
		if hits := forbidden.FindAllString(body, -1); hits != nil {
			t.Errorf("an email uses forbidden vocabulary %q\n%s", hits, body)
		}
	}
}

// The two sentences about money and cancellation are rendered once, by
// internal/notifications, and quoted by both the email and the WhatsApp
// message. The email that drops one of them is the email that says nothing
// about the deposit, which is the whole defect this change is about.
func TestTheClientEmailsQuoteTheSentencesTheyAreGiven(t *testing.T) {
	bodies := captureSent(t)

	for _, c := range []struct {
		name  string
		index int
		want  []string
	}{
		{"confirmation", 0, []string{"DEPOSIT_AMOUNT", "BALANCE_AMOUNT", "CANCELLATION_LINE"}},
		{"reminder", 1, []string{"BALANCE_AMOUNT", "Av. Santa Fe 1200"}},
		{"cancellation", 2, []string{"REFUND_LINE", "$5.000"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, want := range c.want {
				if !strings.Contains(bodies[c.index], want) {
					t.Errorf("the %s email dropped %q", c.name, want)
				}
			}
		})
	}
}

// The duplicate-registration email goes to the person who already owns the
// address, and its whole subject is that a stranger just touched their account.
// It used to greet them with the first name that stranger typed at the
// registration form, so an attacker chose the words Vibe addressed the victim
// with. An unknown name is greeted without one.
func TestTheDuplicateRegistrationEmailNeverGreetsWithAnUnverifiedName(t *testing.T) {
	var body string
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL

	if err := m.SendDuplicateRegistration(context.Background(), "ana@example.com", "",
		"https://vibe.test/login", "https://vibe.test/forgot"); err != nil {
		t.Fatalf("SendDuplicateRegistration = %v", err)
	}

	if !strings.Contains(body, "Hola.") {
		t.Errorf("an email with no name must greet without one; got %s", body)
	}
	if strings.Contains(body, "Hola, <strong></strong>.") || strings.Contains(body, "Hola, .") {
		t.Errorf("the greeting rendered an empty name; got %s", body)
	}
}

// ---------------------------------------------------------------------------
// Round 3: HTML redesign — escaping, bulletproof buttons, font declarations
// ---------------------------------------------------------------------------

// TestComplexNameIsHTMLEscaped pins that a complex name carrying literal
// markup cannot inject into the rendered email: every dynamic value is
// escaped at the point it is written into the template, not trusted from the
// database.
func TestComplexNameIsHTMLEscaped(t *testing.T) {
	var raw string
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		raw = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL

	const evilName = `Padel & Co <b>`
	if err := m.SendBookingConfirmation(context.Background(), "ana@example.com", evilName, "Cancha 1",
		"01/09", "18:00 a 19:30", "", "", "https://vibe.test/cancel",
		"$5.000", "$15.000", "con devolución hasta 24 horas antes del turno."); err != nil {
		t.Fatalf("SendBookingConfirmation = %v", err)
	}

	htmlBody, _ := brevoPayload(t, raw)

	if strings.Contains(htmlBody, "<b>") {
		t.Errorf("the complex name's raw <b> leaked into the HTML unescaped:\n%s", htmlBody)
	}
	if !strings.Contains(htmlBody, "Padel &amp; Co &lt;b&gt;") {
		t.Errorf("the complex name was not escaped as expected:\n%s", htmlBody)
	}
}

// TestEveryButtonIsBulletproof pins the redesign's one non-negotiable email
// client compatibility rule: a click target is always a bgcolor'd <td>
// wrapping the anchor, never a styled <a> alone — Outlook desktop renders the
// bare anchor's own padding as a sliver, not a button.
func TestEveryButtonIsBulletproof(t *testing.T) {
	for i, raw := range captureSent(t) {
		htmlBody, _ := brevoPayload(t, raw)

		if !strings.Contains(htmlBody, `bgcolor="#1db954"`) {
			t.Errorf("email %d has no bulletproof button (bgcolor=\"#1db954\"); body:\n%s", i, htmlBody)
		}

		idx := 0
		for {
			pos := strings.Index(htmlBody[idx:], "<a ")
			if pos == -1 {
				break
			}
			pos += idx
			end := strings.Index(htmlBody[pos:], ">")
			if end == -1 {
				t.Fatalf("email %d: unterminated <a tag", i)
			}
			tag := htmlBody[pos : pos+end]
			if strings.Contains(tag, "display:inline-block") && strings.Contains(tag, "padding") {
				start := pos - 400
				if start < 0 {
					start = 0
				}
				preceding := htmlBody[start:pos]
				lastTD := strings.LastIndex(preceding, "<td")
				if lastTD == -1 || !strings.Contains(preceding[lastTD:], "bgcolor=") {
					t.Errorf("email %d has an orphaned non-bulletproof button, not inside a bgcolor <td>: %s", i, tag)
				}
			}
			idx = pos + end + 1
		}
	}
}

// TestEveryTextElementDeclaresFontFamily pins that <p>, <h1> and <a> all carry
// their own font-family, because Outlook does not inherit it from the body —
// an element that forgets it falls back to Times New Roman in production.
func TestEveryTextElementDeclaresFontFamily(t *testing.T) {
	for i, raw := range captureSent(t) {
		htmlBody, _ := brevoPayload(t, raw)

		for _, prefix := range []string{"<p ", "<h1 ", "<a "} {
			idx := 0
			for {
				pos := strings.Index(htmlBody[idx:], prefix)
				if pos == -1 {
					break
				}
				pos += idx
				end := strings.Index(htmlBody[pos:], ">")
				if end == -1 {
					t.Fatalf("email %d: unterminated %s tag", i, prefix)
				}
				tag := htmlBody[pos : pos+end]
				if !strings.Contains(tag, "font-family:") {
					t.Errorf("email %d: a %s tag has no font-family declared: %s", i, prefix, tag)
				}
				idx = pos + end + 1
			}
		}
	}
}

// TestPlainTextFallbackHasNoMarkupArtifacts renders one full email through
// the real send pipeline and checks the text/plain part it carries: the
// headline must read through, and none of the new markup's plumbing — the
// <head> block, the MSO conditional comments and their VML spacer hacks, or
// the hidden preheader's zero-width filler — may leak into it as text.
func TestPlainTextFallbackHasNoMarkupArtifacts(t *testing.T) {
	var raw string
	brevo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		raw = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer brevo.Close()

	m := New(Config{BrevoAPIKey: "k", Sender: "Vibe <noreply@vibe.com.ar>"})
	m.apiURL = brevo.URL

	if err := m.SendBookingConfirmation(context.Background(), "ana@example.com", "Vibe Palermo", "Cancha 1",
		"15/03", "09:00 a 10:30", "Av. Santa Fe 1200", "https://maps.example/?q=1", "https://vibe.test/cancel",
		"$5.000", "$15.000", "con devolución hasta 24 horas antes del turno."); err != nil {
		t.Fatalf("SendBookingConfirmation = %v", err)
	}

	_, plain := brevoPayload(t, raw)

	for _, want := range []string{
		"Su reserva está confirmada.",
		"Cancha 1", "Vibe Palermo",
		"Cancelar reserva",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain text is missing %q; got:\n%s", want, plain)
		}
	}

	lower := strings.ToLower(plain)
	for _, artifact := range []string{"<", "mso", "&#847;", "&zwnj;", "&#8203;"} {
		if strings.Contains(lower, strings.ToLower(artifact)) {
			t.Errorf("plain text leaked a markup artifact %q; got:\n%s", artifact, plain)
		}
	}
}

package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// brevoAPIURL is the transactional send endpoint. It is a field on Mailer
// rather than used inline so tests can point the client at a local server.
const brevoAPIURL = "https://api.brevo.com/v3/smtp/email"

// Mailer sends transactional emails via the Brevo HTTP API, falling back to
// SMTP when Brevo is unreachable, overloaded or refusing our credentials.
type Mailer struct {
	// Brevo API (preferred transport)
	apiKey string
	apiURL string

	// SMTP fallback. When both are configured every message is tried over
	// Brevo first and over SMTP only if Brevo failed for a reason that was not
	// about this particular message — see deliver.
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string

	// Shared
	senderName  string
	senderEmail string
	senderRaw   string
	logoURL     string
	// appURL is the dashboard origin the owner's "Ver en el panel" button
	// points at. It is a separate field from logoURL — which is the frontend
	// origin plus "/logo.png" — because a future asset-CDN split for the logo
	// must not silently move the owner's dashboard link with it.
	appURL     string
	httpClient *http.Client
	cb         *circuitbreaker.CircuitBreaker
}

// Config holds all mailer configuration.
type Config struct {
	BrevoAPIKey  string
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	Sender       string
	LogoURL      string
	// AppURL is the dashboard origin used for the owner's "Ver en el panel"
	// button on the new-booking email.
	AppURL string
	CB     *circuitbreaker.CircuitBreaker
}

// New creates a new Mailer. Uses Brevo API if apiKey is set, otherwise SMTP.
func New(cfg Config) *Mailer {
	senderName := ""
	senderEmail := cfg.Sender
	if addr, err := mail.ParseAddress(cfg.Sender); err == nil {
		senderEmail = addr.Address
		senderName = addr.Name
	}
	return &Mailer{
		apiKey:       cfg.BrevoAPIKey,
		apiURL:       brevoAPIURL,
		smtpHost:     cfg.SMTPHost,
		smtpPort:     cfg.SMTPPort,
		smtpUsername: cfg.SMTPUsername,
		smtpPassword: cfg.SMTPPassword,
		senderName:   senderName,
		senderEmail:  senderEmail,
		senderRaw:    cfg.Sender,
		logoURL:      cfg.LogoURL,
		appURL:       cfg.AppURL,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		cb:           cfg.CB,
	}
}

// smtpConfigured reports whether the SMTP transport can be used at all.
func (m *Mailer) smtpConfigured() bool {
	return m.smtpUsername != "" && m.smtpPassword != ""
}

// Mode returns which transports are usable: "brevo_api+smtp", "brevo_api",
// "smtp", or "disabled".
//
// The combined value is not cosmetic — it is the difference between a
// deployment whose mail survives a Brevo outage and one whose does not, and it
// is the only place an operator sees that difference at boot.
func (m *Mailer) Mode() string {
	switch {
	case m.apiKey != "" && m.smtpConfigured():
		return "brevo_api+smtp"
	case m.apiKey != "":
		return "brevo_api"
	case m.smtpConfigured():
		return "smtp"
	default:
		return "disabled"
	}
}

// ---------------------------------------------------------------------------
// Plain text generation
// ---------------------------------------------------------------------------

var (
	// reHeadBlock strips the whole <head> element before anything else runs.
	// The new markup's head carries a <style> block (raw CSS text, which is
	// not inside any tag so the generic tag-stripper below leaves it
	// untouched), an MSO <xml> conditional with a bare "96" text node
	// (o:PixelsPerInch), and a <title> — none of it client-visible content,
	// all of it would otherwise leak into the plain-text part verbatim.
	reHeadBlock = regexp.MustCompile(`(?is)<head\b[^>]*>.*?</head>`)
	reBlockTags = regexp.MustCompile(`(?i)</(p|h[1-6]|tr|div|li)>`)
	reBR        = regexp.MustCompile(`(?i)<br\s*/?>`)
	reAllTags   = regexp.MustCompile(`<[^>]*>`)
	reSpaces    = regexp.MustCompile(`[^\S\n]+`)
	reNewlines  = regexp.MustCompile(`\n{3,}`)
)

func htmlToPlainText(htmlBody string) string {
	s := htmlBody
	s = reHeadBlock.ReplaceAllString(s, "")
	s = reBlockTags.ReplaceAllString(s, "</$1>\n")
	s = reBR.ReplaceAllString(s, "\n")
	s = reAllTags.ReplaceAllString(s, "")
	r := strings.NewReplacer(
		"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", `"`, "&#39;", "'",
		"&aacute;", "á", "&eacute;", "é", "&iacute;", "í",
		"&oacute;", "ó", "&uacute;", "ú", "&ntilde;", "ñ",
		"&iexcl;", "¡", "&#8211;", "–", "&mdash;", "—",
		// &#847; and &zwnj; pad the hidden preheader; &#8203; is the
		// zero-width space the bulletproof button's second VML spacer hack
		// carries. All three are filler with nothing to render as text.
		"&#847;", "", "&zwnj;", "", "&#8203;", "",
	)
	s = r.Replace(s)
	s = reSpaces.ReplaceAllString(s, " ")
	s = reNewlines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// ---------------------------------------------------------------------------
// Send pipeline
// ---------------------------------------------------------------------------

// permanentErr is a rejection no retry can change. It answers Permanent() so
// the task queue can dead-letter it immediately, without internal/notifier and
// internal/mailer having to know about each other.
type permanentErr struct{ msg string }

func (e permanentErr) Error() string   { return e.msg }
func (e permanentErr) Permanent() bool { return true }

// ErrNoTransport is returned when neither Brevo nor SMTP is configured.
//
// It is permanent on purpose: retrying a message with nowhere to send it just
// burns the retry budget and hides the real problem, which is that this
// deployment has no mail credentials.
var ErrNoTransport error = permanentErr{"mailer: no transport configured (set BREVO_API_KEY or SMTP credentials)"}

// isPermanent reports whether the provider refused this specific message.
func isPermanent(err error) bool {
	var p interface{ Permanent() bool }
	return errors.As(err, &p) && p.Permanent()
}

// isInfraError reports whether the failure says something about the mail
// service's health, as opposed to something about this one message.
//
// It decides whether the circuit breaker counts a failure, so getting it wrong
// costs in both directions. Reporting a 401 as success — which this used to do
// for every status below 500 — meant a rotated API key produced 100% permanent
// mail loss with a closed breaker and no alarm. Reporting every 4xx as a
// failure would mean five malformed recipient addresses in a row stop mail for
// everybody for a minute. The split is therefore by what the status is about,
// not by its first digit.
func isInfraError(err error) bool {
	if err == nil {
		return false
	}
	// A message the provider judged on its own merits says nothing about the
	// provider's health.
	if isPermanent(err) {
		return false
	}
	var be *brevoError
	if errors.As(err, &be) {
		return be.infra()
	}
	// Network errors, SMTP dial/auth failures, marshalling errors — all infra.
	return true
}

// brevoError wraps a non-success HTTP response from Brevo.
type brevoError struct {
	statusCode int
	body       string
}

func (e *brevoError) Error() string {
	return fmt.Sprintf("mailer: brevo API returned %d: %s", e.statusCode, e.body)
}

// infra reports whether this status is about the service rather than the
// message: 5xx is the service failing, 429 is it shedding load, and 401/403 is
// it refusing our credentials — which rejects every message we will ever send,
// not this one. A 3xx from a JSON API is a misrouted request, which is also not
// about the message.
func (e *brevoError) infra() bool {
	switch {
	case e.statusCode >= 500:
		return true
	case e.statusCode == http.StatusTooManyRequests:
		return true
	case e.statusCode == http.StatusUnauthorized, e.statusCode == http.StatusForbidden:
		return true
	case e.statusCode >= 300 && e.statusCode < 400:
		return true
	default:
		return false
	}
}

// Permanent reports whether retrying would produce the same answer. Every other
// 4xx is this message being refused — a malformed address, a payload the API
// will not take — and re-sending the same bytes gets the same refusal.
func (e *brevoError) Permanent() bool {
	return e.statusCode >= 400 && e.statusCode < 500 && !e.infra()
}

func (m *Mailer) send(ctx context.Context, to, subject, htmlBody string) error {
	if err := m.cb.AllowRequest(); err != nil {
		// Wrapped rather than swallowed: the queue recognises
		// circuitbreaker.ErrOpen as "never attempted" and reschedules the task
		// without consuming a retry, so an open breaker delays a message
		// instead of destroying it.
		return fmt.Errorf("mailer: %w", err)
	}

	plainText := htmlToPlainText(htmlBody)
	err := m.deliver(ctx, to, subject, htmlBody, plainText)

	if isInfraError(err) {
		m.cb.RecordFailure()
	} else {
		m.cb.RecordSuccess()
	}
	return err
}

// deliver sends over Brevo, then over SMTP if Brevo failed for a reason that
// was not about this message.
//
// This is the fallback the struct comment has always claimed. The transport
// used to be chosen once, from config, so a Brevo outage was total mail loss on
// a deployment that had perfectly good SMTP credentials sitting unused.
func (m *Mailer) deliver(ctx context.Context, to, subject, htmlBody, plainText string) error {
	if m.apiKey == "" {
		if !m.smtpConfigured() {
			return ErrNoTransport
		}
		return m.sendSMTP(ctx, to, subject, htmlBody, plainText)
	}

	err := m.sendBrevoAPI(ctx, to, subject, htmlBody, plainText)
	if err == nil {
		return nil
	}
	if !m.smtpConfigured() {
		return err
	}
	if isPermanent(err) {
		// Brevo refused the message itself. SMTP would carry the same bytes to
		// the same rejection, so the fallback would only double the work.
		return err
	}

	smtpErr := m.sendSMTP(ctx, to, subject, htmlBody, plainText)
	if smtpErr == nil {
		return nil
	}
	// Both errors are wrapped so the breaker classification above still sees
	// the Brevo status code that caused the fallback.
	return fmt.Errorf("mailer: brevo failed (%w) and smtp fallback failed: %w", err, smtpErr)
}

// ---------------------------------------------------------------------------
// Brevo HTTP API
// ---------------------------------------------------------------------------

type brevoRequest struct {
	Sender      brevoContact   `json:"sender"`
	To          []brevoContact `json:"to"`
	Subject     string         `json:"subject"`
	HTMLContent string         `json:"htmlContent"`
	TextContent string         `json:"textContent"`
}

type brevoContact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

func (m *Mailer) sendBrevoAPI(ctx context.Context, to, subject, htmlBody, plainText string) error {
	payload := brevoRequest{
		Sender:      brevoContact{Name: m.senderName, Email: m.senderEmail},
		To:          []brevoContact{{Email: to}},
		Subject:     subject,
		HTMLContent: htmlBody,
		TextContent: plainText,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mailer: failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailer: failed to create request: %w", err)
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("api-key", m.apiKey)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mailer: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // the status is the error; a truncated body only makes it less legible
		return &brevoError{statusCode: resp.StatusCode, body: string(respBody)}
	}

	return nil
}

// ---------------------------------------------------------------------------
// SMTP with proper multipart + headers
// ---------------------------------------------------------------------------

func generateMessageID(domain string) string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("<%s.%d@%s>", hex.EncodeToString(b), time.Now().UnixNano(), domain)
}

// message, send, close) for one operation; extraction would fragment this into a chain of
// internal-only helpers each called exactly once.
//
//nolint:funlen // single cohesive SMTP send flow (dial, TLS handshake, auth, compose MIME
func (m *Mailer) sendSMTP(ctx context.Context, to, subject, htmlBody, plainText string) error {
	sanitize := func(s string) string {
		return strings.NewReplacer("\r", "", "\n", "").Replace(s)
	}

	boundary := fmt.Sprintf("----=_Vibe_%d", time.Now().UnixNano())

	domain := "vibe.com.ar"
	if parts := strings.SplitN(m.senderEmail, "@", 2); len(parts) == 2 {
		domain = parts[1]
	}

	var msg strings.Builder
	msg.WriteString("From: " + sanitize(m.senderRaw) + "\r\n")
	msg.WriteString("To: " + sanitize(to) + "\r\n")
	msg.WriteString("Subject: " + sanitize(subject) + "\r\n")
	msg.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	msg.WriteString("Message-ID: " + generateMessageID(domain) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	msg.WriteString("\r\n")

	// Plain text part
	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(plainText)
	msg.WriteString("\r\n\r\n")

	// HTML part
	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)
	msg.WriteString("\r\n\r\n")

	msg.WriteString("--" + boundary + "--\r\n")

	addr := net.JoinHostPort(m.smtpHost, fmt.Sprintf("%d", m.smtpPort))

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("mailer: smtp dial failed: %w", err)
	}

	c, err := smtp.NewClient(conn, m.smtpHost)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mailer: smtp client failed: %w", err)
	}
	defer func() { _ = c.Close() }() //nolint:errcheck // the send either succeeded or returned its own error; a close failure adds nothing

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: m.smtpHost}); err != nil {
			return fmt.Errorf("mailer: starttls failed: %w", err)
		}
	}

	auth := smtp.PlainAuth("", m.smtpUsername, m.smtpPassword, m.smtpHost)
	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("mailer: smtp auth failed: %w", err)
	}

	if err = c.Mail(m.senderEmail); err != nil {
		return fmt.Errorf("mailer: smtp MAIL FROM failed: %w", err)
	}
	if err = c.Rcpt(to); err != nil {
		return fmt.Errorf("mailer: smtp RCPT TO failed: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mailer: smtp DATA failed: %w", err)
	}
	if _, err = w.Write([]byte(msg.String())); err != nil {
		return fmt.Errorf("mailer: smtp write failed: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("mailer: smtp close data failed: %w", err)
	}

	if err := c.Quit(); err != nil {
		return fmt.Errorf("mailer: smtp quit failed: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Template helpers
// ---------------------------------------------------------------------------
//
// This mirrors the helper structure of the design reference (build.py, in the
// product owner's approved markup handoff): p, title, muted, button, link,
// addressLine, fallback and wrap. Palette:
//
//	#ffffff = page bg and card   #111111 = ink   #1db954 = accent
//	#15803d = accent ink (link/button text on white)
//
// Every text-bearing element declares font-family inline (Outlook does not
// inherit it from the body), and every button is the bulletproof table+VML
// shape rather than a styled <a> alone, because Outlook desktop ignores an
// anchor's own padding and renders a sliver instead of a button.
const (
	emailFont      = "Helvetica Neue, Helvetica, Arial, sans-serif"
	emailInk       = "#111111"
	emailAccent    = "#1db954"
	emailAccentInk = "#15803d"
)

// preheaderBlock renders the hidden preview text every email client shows
// beside the subject line, padded with filler so the client cannot fall
// through to quoting the first visible sentence instead.
func preheaderBlock(text string) string {
	pad := strings.Repeat("&#847;&zwnj;&nbsp;", 60)
	return fmt.Sprintf(`<div style="display:none;font-size:1px;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;mso-hide:all;">%s%s</div>`, text, pad)
}

// logoImg renders the 96x96 logo centered above the content. Absent
// configuration falls back to a text wordmark rather than an <img> with an
// empty src, which some clients render as a broken-image icon instead of
// nothing. When configured, the <img> itself carries styled alt-text
// (green, bold, centered, sized to fill the 96x96 box) so a client that
// blocks remote images shows "Vibe" instead of a broken-image icon.
func (m *Mailer) logoImg() string {
	if m.logoURL == "" {
		return fmt.Sprintf(`<p style="margin:0;font-family:%s;font-size:28px;font-weight:700;color:%s;">Vibe</p>`, emailFont, emailAccentInk)
	}
	return fmt.Sprintf(`<img src="%s" width="96" height="96" alt="Vibe" style="display:block;width:96px;height:96px;border:0;margin:0 auto;font-family:%s;font-size:28px;font-weight:700;color:%s;text-align:center;line-height:96px;">`,
		html.EscapeString(m.logoURL), emailFont, emailAccentInk)
}

// wrap lays out one transactional email: the XHTML head Outlook and Gmail
// both need, a hidden preheader, and a fixed 560px card (fluid to 100% under
// 600px) holding the logo and the content. There is no footer — the product
// owner's redesign drops it entirely.
func (m *Mailer) wrap(title, preheader, content string) string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html lang="es" xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head>
<meta charset="UTF-8">
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<!--[if mso]><xml><o:OfficeDocumentSettings><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml><![endif]-->
<title>`)
	b.WriteString(title)
	b.WriteString(`</title>
<style>:root{color-scheme:light;supported-color-schemes:light;} body{margin:0;padding:0;} table{border-collapse:collapse;} img{border:0;line-height:100%;outline:none;text-decoration:none;} a[x-apple-data-detectors]{color:inherit!important;text-decoration:none!important;} @media only screen and (max-width:600px){.w{width:100%!important;} .px{padding-left:20px!important;padding-right:20px!important;}}</style>
</head>
<body style="margin:0;padding:0;background-color:#ffffff;">
`)
	b.WriteString(preheaderBlock(preheader))
	b.WriteString(`
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" bgcolor="#ffffff" style="background-color:#ffffff;">
<tr><td align="center" style="padding:32px 12px;">
<!--[if mso]><table role="presentation" width="560" cellpadding="0" cellspacing="0" border="0"><tr><td><![endif]-->
<table role="presentation" class="w" width="560" cellpadding="0" cellspacing="0" border="0" style="width:560px;max-width:560px;">
<tr><td align="center" style="padding:0 8px 28px;">
`)
	b.WriteString(m.logoImg())
	b.WriteString(`
</td></tr>
<tr><td class="px" style="padding:0 8px;">
`)
	b.WriteString(content)
	b.WriteString(`
</td></tr>
</table>
<!--[if mso]></td></tr></table><![endif]-->
</td></tr>
</table>
</body>
</html>`)

	return b.String()
}

// ---------------------------------------------------------------------------
// Reusable content blocks
// ---------------------------------------------------------------------------

// p renders a body paragraph at size px, with marginBottom px of space below
// it. Every call site is responsible for escaping any caller-supplied value
// it interpolates into text before passing it here — see html.EscapeString
// at each Send* method below.
func p(text string, size, marginBottom int) string {
	return fmt.Sprintf(`<p style="margin:0 0 %dpx;font-family:%s;font-size:%dpx;line-height:1.5;color:%s;font-weight:400;">%s</p>`,
		marginBottom, emailFont, size, emailInk, text)
}

// muted is the same paragraph p renders, at the tighter 12px margin used for
// a secondary line right under the sentence it qualifies. It carries no
// lighter color or smaller size: the redesign dropped the old grey "fine
// print" treatment in favor of one ink throughout.
func muted(text string) string { return p(text, 16, 12) }

// title renders the email's single h1 headline.
func title(text string) string {
	return fmt.Sprintf(`<h1 style="margin:0 0 20px;font-family:%s;font-size:22px;line-height:1.3;color:%s;font-weight:700;">%s</h1>`,
		emailFont, emailInk, text)
}

// button renders a bulletproof email button: a bgcolor'd table cell carrying
// the click target, with the VML <i> spacer hack so Outlook desktop — which
// ignores an anchor's own padding — renders the same size box every other
// client does from padding alone. label is always one of this file's own
// fixed strings, never caller-supplied, so it is not escaped; href is.
func button(href, label string) string {
	return fmt.Sprintf(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:8px 0 24px;"><tr>`+
		`<td align="center" bgcolor="%s" style="background-color:%s;border:1px solid %s;">`+
		`<!--[if mso]><i style="mso-font-width:400%%;mso-text-raise:28pt" hidden>&nbsp;</i><![endif]-->`+
		`<a href="%s" style="display:inline-block;padding:13px 28px;font-family:%s;font-size:15px;line-height:1;font-weight:700;color:#0b0b0b;text-decoration:none;mso-padding-alt:0;">%s</a>`+
		`<!--[if mso]><i style="mso-font-width:400%%;" hidden>&nbsp;&#8203;</i><![endif]-->`+
		`</td></tr></table>`,
		emailAccent, emailAccent, emailAccent, html.EscapeString(href), emailFont, label)
}

// link renders an inline text link. text is caller-controlled only through
// fallback below, which escapes it itself; every other call site here passes
// this file's own fixed label text.
func link(href, text string) string {
	return fmt.Sprintf(`<a href="%s" style="color:%s;text-decoration:underline;font-family:%s;">%s</a>`,
		html.EscapeString(href), emailAccentInk, emailFont, text)
}

// addressLine renders the venue's address followed by a bold, underlined
// "Cómo llegar" link to mapsURL. Callers must skip this entirely when either
// address or mapsURL is empty — see SendBookingConfirmation and
// SendReminder2h, which is also why this never has to decide that for itself.
func addressLine(address, mapsURL string) string {
	return p(fmt.Sprintf(`%s. <a href="%s" style="color:%s;font-weight:700;text-decoration:underline;font-family:%s;">Cómo llegar</a>.`,
		html.EscapeString(address), html.EscapeString(mapsURL), emailAccentInk, emailFont), 16, 16)
}

// fallback renders the "copy this link" line shown under a button, for a
// client that stripped the button's markup but left plain anchors alone.
func fallback(rawURL string) string {
	return p(fmt.Sprintf("Si el botón no funciona, copie este link en el navegador:<br>%s",
		link(rawURL, html.EscapeString(rawURL))), 13, 12)
}

// ---------------------------------------------------------------------------
// Public send methods
// ---------------------------------------------------------------------------

// greeting addresses the reader by name, when the name is really theirs.
//
// It exists because one caller has no such name. The duplicate-registration
// email used to greet the account holder with whatever first name the person
// who just tried to register had typed — so "Hola Ana" could land in Ana's
// inbox written by somebody who is not Ana, in an email whose whole subject is
// that a stranger touched her account. An empty name is greeted without one
// rather than with a stranger's.
func greeting(firstName string) string {
	name := strings.TrimSpace(firstName)
	if name == "" {
		return "Hola."
	}
	return fmt.Sprintf("Hola, <strong>%s</strong>.", html.EscapeString(name))
}

// SendBookingConfirmation emails the client that their booking was confirmed.
//
// address and mapsURL render the "Cómo llegar" line and are both required for
// it to appear at all — see addressLine — because a link with nowhere to
// point is worse than no line. depositAmount, balanceAmount and
// cancellationLine arrive rendered, from internal/notifications'
// PaymentAmounts and CancellationLine. All three are also the WhatsApp
// message's parameters, which is the point: the two channels describe one
// booking, and neither may be edited into disagreeing with the other. The
// sentences below are assembled around them with the same fixed words the
// approved WhatsApp template uses ("Seña abonada: ... Resta pagar en el
// complejo: ...", "Cancelación ..."), so the two channels keep saying the
// same thing even though the template now carries the amounts as two separate
// variables. The grace period cancellationLine may quote is
// operator-configured — this function's comment claimed a hardcoded fifteen
// minutes long after the code had stopped believing it.
func (m *Mailer) SendBookingConfirmation(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine string) error {
	subject := fmt.Sprintf("Reserva confirmada · %s", complexName)

	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	dateEsc, startEsc := html.EscapeString(date), html.EscapeString(startTime)

	content := title("Su reserva está confirmada.") +
		p(fmt.Sprintf("<strong>%s</strong> en <strong>%s</strong>, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc), 16, 16)
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	content += p(fmt.Sprintf("Seña abonada: <strong>%s</strong>. Resta pagar en el complejo: <strong>%s</strong>.",
		html.EscapeString(depositAmount), html.EscapeString(balanceAmount)), 16, 16)
	content += muted("Cancelación " + html.EscapeString(cancellationLine))
	content += button(cancelURL, "Cancelar reserva")
	content += p("Gracias por su reserva.", 16, 0)

	preheader := fmt.Sprintf("%s en %s, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc)
	return m.send(ctx, to, subject, m.wrap("Reserva confirmada", preheader, content))
}

// SendReminder2h emails the client a reminder that their booking starts in
// about 2 hours.
//
// address and mapsURL render the same "Cómo llegar" line the confirmation
// does, two hours closer to when it matters; balanceAmount is the plain
// formatted amount from internal/notifications' BalanceAmount, in the same
// fixed words the approved WhatsApp template uses ("Resta pagar en el
// complejo: ...").
func (m *Mailer) SendReminder2h(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, balanceAmount, cancelURL string) error {
	subject := fmt.Sprintf("Su reserva comienza en 2 horas · %s", complexName)

	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	dateEsc, startEsc := html.EscapeString(date), html.EscapeString(startTime)

	content := title("Su reserva comienza en 2 horas.") +
		p(fmt.Sprintf("<strong>%s</strong> en <strong>%s</strong>, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc), 16, 16)
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	content += p(fmt.Sprintf("Resta pagar en el complejo: <strong>%s</strong>.", html.EscapeString(balanceAmount)), 16, 16)
	if cancelURL != "" {
		content += button(cancelURL, "Cancelar reserva")
	}
	content += p("Le esperamos.", 16, 0)

	preheader := fmt.Sprintf("%s en %s, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc)
	return m.send(ctx, to, subject, m.wrap("Su reserva es pronto", preheader, content))
}

// SendBookingCancelled emails the client that their booking was cancelled, and
// what became of their deposit.
//
// refundLine is the value substituted after the fixed "Devolución: " lead-in,
// covering all six cancellation outcomes (see internal/bookings'
// refundMessage) — it is never empty in production, so the preheader always
// has something to quote; refundAmount is that same money formatted, or empty
// when none is coming back, and is what the preheader quotes on its own,
// since the sentence around it is fixed text a client's inbox preview line
// need not repeat.
func (m *Mailer) SendBookingCancelled(ctx context.Context, to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL string) error {
	subject := fmt.Sprintf("Reserva cancelada · %s", complexName)

	content := title("Su reserva fue cancelada.") +
		p(fmt.Sprintf("<strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			html.EscapeString(courtName), html.EscapeString(complexName), html.EscapeString(date), html.EscapeString(startTime)), 16, 16) +
		p("Devolución: "+html.EscapeString(refundLine), 16, 16) +
		muted("Ante cualquier consulta, contacte al complejo.")
	if bookURL != "" {
		content += button(bookURL, "Nueva reserva")
	}

	preheader := fmt.Sprintf("Su reserva en %s fue cancelada.", html.EscapeString(complexName))
	switch {
	case refundAmount != "":
		preheader = fmt.Sprintf("Devolución: %s.", html.EscapeString(refundAmount))
	case refundLine != "":
		preheader = "Devolución: " + html.EscapeString(refundLine)
	}
	return m.send(ctx, to, subject, m.wrap("Reserva cancelada", preheader, content))
}

// SendEmailVerification emails the user a link to verify their account, valid
// for 24 hours.
func (m *Mailer) SendEmailVerification(ctx context.Context, to, firstName, verifyURL string) error {
	const subject = "Verifique su email · Vibe"

	content := title("Verifique su email.") +
		p(greeting(firstName)+" Para activar su cuenta, presione el botón:", 16, 16) +
		button(verifyURL, "Verificar email") +
		muted("El link expira en 24 horas.") +
		fallback(verifyURL)

	return m.send(ctx, to, subject, m.wrap("Verifique su email", "Active su cuenta de Vibe.", content))
}

// SendDepositRefunded emails the client that their booking deposit came back.
//
// "Devolución", in the subject and the page title as well as the body. The
// same email used to be headed "Reembolso realizado" over a body that said
// "Se procesó la devolución" — one message contradicting itself on the one
// word this product has a rule about.
func (m *Mailer) SendDepositRefunded(ctx context.Context, to, complexName, amount, bookURL string) error {
	subject := fmt.Sprintf("Devolución realizada · %s", complexName)
	amountEsc := html.EscapeString(amount)

	content := title("Devolución realizada.") +
		p(fmt.Sprintf("Se realizó la devolución de <strong>%s</strong> de la seña de su reserva en <strong>%s</strong>.",
			amountEsc, html.EscapeString(complexName)), 16, 16) +
		muted("Se acredita en los próximos días hábiles, en el medio de pago utilizado.")
	if bookURL != "" {
		content += button(bookURL, "Nueva reserva")
	}

	preheader := fmt.Sprintf("Se realizó la devolución de %s de la seña de su reserva.", amountEsc)
	return m.send(ctx, to, subject, m.wrap("Devolución realizada", preheader, content))
}

// SendPasswordReset emails the user a link to reset their password, valid for
// 1 hour.
func (m *Mailer) SendPasswordReset(ctx context.Context, to, firstName, resetURL string) error {
	const subject = "Restablecer contraseña · Vibe"

	content := title("Restablecer contraseña.") +
		p(greeting(firstName)+" Recibimos una solicitud para restablecer su contraseña.", 16, 16) +
		button(resetURL, "Restablecer contraseña") +
		muted("El link expira en 1 hora. Si no solicitó este cambio, ignore este email.") +
		fallback(resetURL)

	return m.send(ctx, to, subject, m.wrap("Restablecer contraseña", "Siga el link para restablecer su contraseña.", content))
}

// SendEmailChangeRequested emails the account's CURRENT address that a
// session asked to move it to newEmail, valid for expiresIn — the same TTL
// convention as a password reset link, and the same reason: an old link
// recovers nothing once its window has passed. expiresIn is already rendered
// Spanish copy (see notifications.EmailChangeRequestedEmail.ExpiresIn), not a
// second hardcoded "1 hora" that could drift from the actual TTL. Ignoring the
// email leaves the account's address untouched; only following it and
// completing the confirmation moves it.
func (m *Mailer) SendEmailChangeRequested(ctx context.Context, to, firstName, newEmail, confirmURL, expiresIn string) error {
	const subject = "Confirme el cambio de email · Vibe"

	content := title("Confirme el cambio de email.") +
		p(fmt.Sprintf("%s Alguien con acceso a su cuenta solicitó cambiar el email a <strong>%s</strong>.",
			greeting(firstName), html.EscapeString(newEmail)), 16, 16) +
		button(confirmURL, "Confirmar cambio de email") +
		muted(fmt.Sprintf("El link expira en %s. Si no fue usted, ignore este email: su email actual no cambiará.",
			html.EscapeString(expiresIn))) +
		fallback(confirmURL)

	return m.send(ctx, to, subject, m.wrap("Confirme el cambio de email", "Confirme el cambio de email de su cuenta Vibe.", content))
}

// SendDuplicateRegistration emails the account owner that someone tried to
// register with their already-registered email, offering login and password
// reset links.
//
// firstName is the name on the existing account, or empty. It must never be
// the name the person attempting the registration typed: see greeting.
func (m *Mailer) SendDuplicateRegistration(ctx context.Context, to, firstName, loginURL, resetURL string) error {
	const subject = "Intento de registro · Vibe"

	content := title("Intento de registro.") +
		p(greeting(firstName)+" Alguien intentó crear una cuenta con su email. Si fue usted, su cuenta ya existe.", 16, 16) +
		button(loginURL, "Iniciar sesión") +
		muted(fmt.Sprintf("¿Olvidó su contraseña? %s.", link(resetURL, "Restablecerla"))) +
		muted("Si no fue usted, ignore este email.")

	return m.send(ctx, to, subject, m.wrap("Intento de registro", "Alguien intentó registrarse con su email en Vibe.", content))
}

// SendOwnerNewBooking emails the complex owner that a new booking was made.
//
// Only for a booking that came in on its own: internal/notifications
// suppresses this for the dashboard's own create route, where the recipient
// is the person who just made the booking. Its button points at m.appURL —
// the dashboard origin — rather than the client-facing booking page every
// other button here links to.
func (m *Mailer) SendOwnerNewBooking(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error {
	subject := fmt.Sprintf("Nueva reserva · %s, %s %s", courtName, date, startTime)

	clientEsc, courtEsc := html.EscapeString(clientName), html.EscapeString(courtName)
	complexEsc := html.EscapeString(complexName)
	dateEsc, startEsc := html.EscapeString(date), html.EscapeString(startTime)

	content := title("Nueva reserva.") +
		p(fmt.Sprintf("<strong>%s</strong> reservó <strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			clientEsc, courtEsc, complexEsc, dateEsc, startEsc), 16, 16) +
		button(m.appURL+"/bookings", "Ver en el panel")

	preheader := fmt.Sprintf("%s reservó %s el %s de %s.", clientEsc, courtEsc, dateEsc, startEsc)
	return m.send(ctx, to, subject, m.wrap("Nueva reserva", preheader, content))
}

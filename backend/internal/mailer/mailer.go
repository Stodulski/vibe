package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// brevoAPIURL is the transactional send endpoint. It is a field on Mailer
// rather than used inline so tests can point the client at a local server.
const brevoAPIURL = "https://api.brevo.com/v3/smtp/email"

// Mailer sends transactional emails via the Brevo HTTP API.
type Mailer struct {
	apiKey string
	apiURL string

	senderName  string
	senderEmail string
	senderRaw   string
	logoURL     string
	// appURL is the dashboard origin the owner's "Ver en el panel" button
	// points at. It is a separate field from logoURL — which is the frontend
	// origin plus "/logo.png" — because a future asset-CDN split for the logo
	// must not silently move the owner's dashboard link with it.
	appURL string
	// publicURL is the storefront origin the welcome email's tutorial links point
	// at. It is config.PublicBaseURL, the same origin the public site lives on.
	publicURL  string
	httpClient *http.Client
	cb         *circuitbreaker.CircuitBreaker
}

// Config holds all mailer configuration.
type Config struct {
	BrevoAPIKey string
	Sender      string
	LogoURL     string
	// AppURL is the dashboard origin used for the owner's "Ver en el panel"
	// button on the new-booking email.
	AppURL string
	// PublicURL is the storefront origin the welcome email's tutorial links point at.
	PublicURL string
	CB        *circuitbreaker.CircuitBreaker
}

// New creates a new Mailer. With no Brevo API key every send fails with ErrNoTransport.
func New(cfg Config) *Mailer {
	senderName := ""
	senderEmail := cfg.Sender
	if addr, err := mail.ParseAddress(cfg.Sender); err == nil {
		senderEmail = addr.Address
		senderName = addr.Name
	}
	return &Mailer{
		apiKey:      cfg.BrevoAPIKey,
		apiURL:      brevoAPIURL,
		senderName:  senderName,
		senderEmail: senderEmail,
		senderRaw:   cfg.Sender,
		logoURL:     cfg.LogoURL,
		appURL:      cfg.AppURL,
		publicURL:   cfg.PublicURL,
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		cb:          cfg.CB,
	}
}

// Mode returns "brevo_api" when mail can be sent, or "disabled". It is the
// only place an operator sees at boot that a deployment has no mail transport.
func (m *Mailer) Mode() string {
	if m.apiKey != "" {
		return "brevo_api"
	}
	return "disabled"
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

// reAnchor matches one link, its address and its inner markup.
var reAnchor = regexp.MustCompile(`(?is)<a\s[^>]*?href="([^"]*)"[^>]*>(.*?)</a>`)

// anchorAsText writes a link for the plain-text part as its words followed by
// its address in parentheses, so a reader whose client shows only that part can
// still follow it. A link whose words already are its address is written once.
func anchorAsText(anchor string) string {
	m := reAnchor.FindStringSubmatch(anchor)
	href := m[1]
	words := strings.TrimSpace(reSpaces.ReplaceAllString(reAllTags.ReplaceAllString(m[2], ""), " "))
	if words == "" || words == href || words == html.EscapeString(href) {
		return href
	}
	return words + " (" + href + ")"
}

func htmlToPlainText(htmlBody string) string {
	s := htmlBody
	s = reHeadBlock.ReplaceAllString(s, "")
	s = reAnchor.ReplaceAllStringFunc(s, anchorAsText)
	s = reBlockTags.ReplaceAllString(s, "</$1>\n")
	s = reBR.ReplaceAllString(s, "\n")
	s = reAllTags.ReplaceAllString(s, "")
	// &#847; and &zwnj; pad the hidden preheader; &#8203; is the zero-width
	// space the bulletproof button's second VML spacer hack carries. All three
	// are filler with nothing to render as text, so they go before decoding.
	s = strings.NewReplacer("&#847;", "", "&zwnj;", "", "&#8203;", "").Replace(s)
	// Every other entity is decoded, numeric ones included: html.EscapeString
	// writes a quote as &#34; and an apostrophe as &#39;, and a client name
	// like O'Connor or a complex called "El Galpón" used to reach the
	// plain-text part as O&#39;Connor. A non-breaking space reads as a space.
	s = strings.ReplaceAll(html.UnescapeString(s), "\u00a0", " ")
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

// ErrNoTransport is returned when no Brevo API key is configured.
//
// It is permanent on purpose: retrying a message with nowhere to send it just
// burns the retry budget and hides the real problem, which is that this
// deployment has no mail credentials.
var ErrNoTransport error = permanentErr{"mailer: no transport configured (set BREVO_API_KEY)"}

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
	// Network errors, marshalling errors — all infra.
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
	return m.sendWith(ctx, to, subject, htmlBody, outbound{})
}

// outbound is what a message carries beyond its recipient, subject and bodies.
// The transactional emails send none of it; the lifecycle nudges send the
// List-Unsubscribe headers and a reply-to address.
type outbound struct {
	// headers are extra message headers by name. Each transport writes exactly
	// these and adds none of its own.
	headers map[string]string
	// replyTo is where a reply goes, when it is not the sender.
	replyTo string
}

// sendWith is send with the outbound options.
func (m *Mailer) sendWith(ctx context.Context, to, subject, htmlBody string, opts outbound) error {
	if err := m.cb.AllowRequest(); err != nil {
		// Wrapped rather than swallowed: the queue recognises
		// circuitbreaker.ErrOpen as "never attempted" and reschedules the task
		// without consuming a retry, so an open breaker delays a message
		// instead of destroying it.
		return fmt.Errorf("mailer: %w", err)
	}

	plainText := htmlToPlainText(htmlBody)
	err := m.deliver(ctx, to, subject, htmlBody, plainText, opts)

	if isInfraError(err) {
		m.cb.RecordFailure()
	} else {
		m.cb.RecordSuccess()
	}
	return err
}

// deliver sends the message over the Brevo API, the only transport. Outbound
// SMTP is blocked on the Railway plan the server runs on, so an SMTP fallback
// could never carry a message there.
func (m *Mailer) deliver(ctx context.Context, to, subject, htmlBody, plainText string, opts outbound) error {
	if m.apiKey == "" {
		return ErrNoTransport
	}
	return m.sendBrevoAPI(ctx, to, subject, htmlBody, plainText, opts)
}

// ---------------------------------------------------------------------------
// Brevo HTTP API
// ---------------------------------------------------------------------------

type brevoRequest struct {
	Sender      brevoContact      `json:"sender"`
	To          []brevoContact    `json:"to"`
	ReplyTo     *brevoContact     `json:"replyTo,omitempty"`
	Subject     string            `json:"subject"`
	HTMLContent string            `json:"htmlContent"`
	TextContent string            `json:"textContent"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type brevoContact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

func (m *Mailer) sendBrevoAPI(ctx context.Context, to, subject, htmlBody, plainText string, opts outbound) error {
	payload := brevoRequest{
		Sender:      brevoContact{Name: m.senderName, Email: m.senderEmail},
		To:          []brevoContact{{Email: to}},
		Subject:     subject,
		HTMLContent: htmlBody,
		TextContent: plainText,
		Headers:     opts.headers,
	}
	if opts.replyTo != "" {
		payload.ReplyTo = &brevoContact{Email: opts.replyTo}
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
	return p(fmt.Sprintf("Si el botón no funciona, copiá este link en el navegador:<br>%s",
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

// The states a booking's payment can be in, as the confirmation tells it.
// An empty state is a confirmation queued before the state was carried, and
// reads as a paid deposit, the one wording every confirmation used to have.
const (
	PaidNothing = "unpaid"
	PaidDeposit = "deposit"
	PaidFull    = "full"
)

// SendBookingConfirmation emails the client that their booking was confirmed.
//
// cancelURL is empty when the booking link token could not be minted, and then the
// cancel button is left out rather than rendered with an empty href, the way
// SendReminder2h leaves it out.
//
// address and mapsURL render the "Cómo llegar" line and are both required for
// it to appear at all — see addressLine — because a link with nowhere to
// point is worse than no line. depositAmount, balanceAmount and
// cancellationLine arrive rendered, from internal/notifications'
// PaymentAmounts and CancellationLine, so the sentences below quote the figures
// the caller recorded. paymentState (PaidNothing, PaidDeposit, PaidFull) picks
// the sentence: a booking entered with nothing collected says what is owed and
// promises no refund, and one paid in full is not called a "seña".
func (m *Mailer) SendBookingConfirmation(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine, paymentState string, contact Contact) error {
	subject := fmt.Sprintf("Reserva confirmada · %s", complexName)

	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	dateEsc, startEsc := html.EscapeString(date), html.EscapeString(startTime)

	content := title("Tu reserva está confirmada.") +
		p(fmt.Sprintf("<strong>%s</strong> en <strong>%s</strong>, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc), 16, 16)
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	switch paymentState {
	case PaidNothing:
		if balanceAmount != "" && balanceAmount != "$0" {
			content += p(fmt.Sprintf("A pagar en el complejo: <strong>%s</strong>.", html.EscapeString(balanceAmount)), 16, 16)
		}
	case PaidFull:
		content += p(fmt.Sprintf("Pagado: <strong>%s</strong>. No tenés nada pendiente de pago.", html.EscapeString(depositAmount)), 16, 16)
		content += muted("Cancelación " + html.EscapeString(cancellationLine))
	default:
		content += p(fmt.Sprintf("Seña abonada: <strong>%s</strong>. Resta pagar en el complejo: <strong>%s</strong>.",
			html.EscapeString(depositAmount), html.EscapeString(balanceAmount)), 16, 16)
		content += muted("Cancelación " + html.EscapeString(cancellationLine))
	}
	if cancelURL != "" {
		content += button(cancelURL, "Cancelar reserva")
	}
	content += contactLine(contact)
	content += p("Gracias por reservar.", 16, 0)

	preheader := fmt.Sprintf("%s en %s, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Reserva confirmada", preheader, content), contact)
}

// SendReminder2h emails the client a reminder two hours before their booking
// starts.
//
// when is the start as the client reads it on the day the email arrives ("hoy
// a las 18:00", "mañana a las 00:30"): the subject names the real start instead
// of a countdown that was wrong for every reminder not sent exactly two hours
// out. address and mapsURL render the same "Cómo llegar" line the confirmation
// does; balanceAmount is the plain formatted amount from internal/notifications'
// BalanceAmount.
func (m *Mailer) SendReminder2h(ctx context.Context, to, complexName, courtName, date, startTime, when, address, mapsURL, balanceAmount, cancelURL string, contact Contact) error {
	subject := fmt.Sprintf("Tu turno es %s · %s", when, complexName)

	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	dateEsc, startEsc := html.EscapeString(date), html.EscapeString(startTime)

	content := title(fmt.Sprintf("Tu turno es %s.", html.EscapeString(when))) +
		p(fmt.Sprintf("<strong>%s</strong> en <strong>%s</strong>, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc), 16, 16)
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	if balanceAmount == "" || balanceAmount == "$0" {
		content += p("No tenés nada pendiente de pago.", 16, 16)
	} else {
		content += p(fmt.Sprintf("Resta pagar en el complejo: <strong>%s</strong>.", html.EscapeString(balanceAmount)), 16, 16)
	}
	if cancelURL != "" {
		content += button(cancelURL, "Cancelar reserva")
	}
	content += contactLine(contact)
	content += p("Te esperamos.", 16, 0)

	preheader := fmt.Sprintf("%s en %s, el %s de %s.", courtEsc, complexEsc, dateEsc, startEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Tu turno es pronto", preheader, content), contact)
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
func (m *Mailer) SendBookingCancelled(ctx context.Context, to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL string, contact Contact) error {
	subject := fmt.Sprintf("Reserva cancelada · %s", complexName)

	content := title("Tu reserva se canceló.") +
		p(fmt.Sprintf("<strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			html.EscapeString(courtName), html.EscapeString(complexName), html.EscapeString(date), html.EscapeString(startTime)), 16, 16) +
		p("Devolución: "+html.EscapeString(refundLine), 16, 16) +
		contactLine(contact)
	if bookURL != "" {
		content += button(bookURL, "Nueva reserva")
	}

	preheader := fmt.Sprintf("Tu reserva en %s se canceló.", html.EscapeString(complexName))
	switch {
	case refundAmount != "":
		preheader = fmt.Sprintf("Devolución: %s.", html.EscapeString(refundAmount))
	case refundLine != "":
		preheader = "Devolución: " + html.EscapeString(refundLine)
	}
	return m.sendToClient(ctx, to, subject, m.wrap("Reserva cancelada", preheader, content), contact)
}

// SendOwnerBookingRefunded emails the complex owner that a refund MercadoPago
// issued on its own side cancelled a booking, so the slot is free to sell again.
//
// Only for a refund that cancelled a live booking: internal/notifications sends
// this from internal/payments, and a refund Vibe issued itself is announced to
// the client alone. Its button points at the dashboard's bookings page, the
// same origin SendOwnerBookingCancelled uses.
//
// The subject says "devolución", not "reembolso": TestNoEmailUsesAForbiddenWord
// holds every message, subject included, to the product's vocabulary.
func (m *Mailer) SendOwnerBookingRefunded(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error {
	subject := fmt.Sprintf("Se canceló una reserva por una devolución de MercadoPago: %s %s %s", courtName, date, startTime)

	clientEsc, courtEsc := html.EscapeString(clientName), html.EscapeString(courtName)
	complexEsc := html.EscapeString(complexName)
	dateEsc, startEsc := html.EscapeString(date), html.EscapeString(startTime)

	content := title("Se canceló una reserva.") +
		p(fmt.Sprintf("<strong>%s</strong> reservó <strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			clientEsc, courtEsc, complexEsc, dateEsc, startEsc), 16, 16) +
		p("MercadoPago devolvió el pago de esta reserva y la cancelamos. El turno quedó libre para reservar.", 16, 16) +
		button(m.appURL+"/bookings", "Ver en el panel")

	preheader := fmt.Sprintf("%s el %s de %s quedó libre.", courtEsc, dateEsc, startEsc)
	return m.send(ctx, to, subject, m.wrap("Se canceló una reserva", preheader, content))
}

// SendEmailVerification emails the user a link to verify their account, valid
// for 24 hours.
func (m *Mailer) SendEmailVerification(ctx context.Context, to, firstName, verifyURL string) error {
	const subject = "Confirmá tu email · Vibe"

	content := title("Confirmá tu email.") +
		p(greeting(firstName)+" Para activar tu cuenta, tocá el botón:", 16, 16) +
		button(verifyURL, "Confirmar email") +
		muted("El link vence en 24 horas.") +
		fallback(verifyURL)

	return m.send(ctx, to, subject, m.wrap("Confirmá tu email", "Activá tu cuenta de Vibe.", content))
}

// SendDepositRefunded emails the client that their money came back.
//
// "Devolución", in the subject and the page title as well as the body. The
// same email used to be headed "Reembolso realizado" over a body that said
// "Se procesó la devolución" — one message contradicting itself on the one
// word this product has a rule about.
//
// cancelled says the refund also cancelled the booking (a refund MercadoPago
// issued on its own side on a live booking). The email then says so first,
// naming the booking from courtName, date and startTime: it is the client's
// only notice that the booking is off, and it used to talk about money alone.
func (m *Mailer) SendDepositRefunded(ctx context.Context, to, complexName, amount, bookURL string, cancelled bool, courtName, date, startTime string, contact Contact) error {
	subject := fmt.Sprintf("Devolución realizada · %s", complexName)
	heading := "Te devolvimos el pago."
	amountEsc, complexEsc := html.EscapeString(amount), html.EscapeString(complexName)

	var content string
	if cancelled {
		subject = fmt.Sprintf("Reserva cancelada y pago devuelto · %s", complexName)
		heading = "Tu reserva se canceló y te devolvimos el pago."
		content = title(heading) +
			p(fmt.Sprintf("Tu reserva de <strong>%s</strong> en <strong>%s</strong>, el %s de %s, se canceló porque el pago se devolvió por MercadoPago.",
				html.EscapeString(courtName), complexEsc, html.EscapeString(date), html.EscapeString(startTime)), 16, 16) +
			p(fmt.Sprintf("Te devolvimos <strong>%s</strong>.", amountEsc), 16, 16)
	} else {
		content = title(heading) +
			p(fmt.Sprintf("Te devolvimos <strong>%s</strong> de tu reserva en <strong>%s</strong>.", amountEsc, complexEsc), 16, 16)
	}
	content += muted("Lo vas a ver en los próximos días hábiles, en el medio con el que pagaste.") +
		contactLine(contact)
	if bookURL != "" {
		content += button(bookURL, "Nueva reserva")
	}

	preheader := fmt.Sprintf("Te devolvimos %s de tu reserva.", amountEsc)
	if cancelled {
		preheader = fmt.Sprintf("Tu reserva se canceló. Te devolvimos %s.", amountEsc)
	}
	return m.sendToClient(ctx, to, subject, m.wrap("Devolución realizada", preheader, content), contact)
}

// SendPasswordReset emails the user a link to reset their password, valid for
// 1 hour.
func (m *Mailer) SendPasswordReset(ctx context.Context, to, firstName, resetURL string) error {
	const subject = "Cambiá tu contraseña · Vibe"

	content := title("Cambiá tu contraseña.") +
		p(greeting(firstName)+" Recibimos un pedido para cambiar tu contraseña.", 16, 16) +
		button(resetURL, "Cambiar contraseña") +
		muted("El link vence en 1 hora. Si no lo pediste vos, ignorá este email.") +
		fallback(resetURL)

	return m.send(ctx, to, subject, m.wrap("Cambiá tu contraseña", "Seguí el link para cambiar tu contraseña.", content))
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
	const subject = "Confirmá el cambio de email · Vibe"

	content := title("Confirmá el cambio de email.") +
		p(fmt.Sprintf("%s Alguien con acceso a tu cuenta pidió cambiar el email a <strong>%s</strong>.",
			greeting(firstName), html.EscapeString(newEmail)), 16, 16) +
		button(confirmURL, "Confirmar cambio de email") +
		muted(fmt.Sprintf("El link vence en %s. Si no lo pediste vos, ignorá este email: tu email actual no cambia.",
			html.EscapeString(expiresIn))) +
		fallback(confirmURL)

	return m.send(ctx, to, subject, m.wrap("Confirmá el cambio de email", "Confirmá el cambio de email de tu cuenta de Vibe.", content))
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
		p(greeting(firstName)+" Alguien intentó crear una cuenta con tu email. Si fuiste vos, tu cuenta ya existe.", 16, 16) +
		button(loginURL, "Iniciar sesión") +
		muted(fmt.Sprintf("¿Olvidaste tu contraseña? %s.", link(resetURL, "Cambiala acá"))) +
		muted("Si no fuiste vos, ignorá este email.")

	return m.send(ctx, to, subject, m.wrap("Intento de registro", "Alguien intentó registrarse con tu email en Vibe.", content))
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

// SendOwnerBookingCancelled emails the complex owner that a client cancelled a
// booking, so the slot is free to sell again.
//
// Only for a client's own cancellation: internal/notifications sends this for
// SourceClientCancel alone. Its button points at the dashboard's bookings page,
// the same origin SendOwnerNewBooking uses.
func (m *Mailer) SendOwnerBookingCancelled(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error {
	subject := fmt.Sprintf("Se canceló una reserva: %s %s %s", courtName, date, startTime)

	clientEsc, courtEsc := html.EscapeString(clientName), html.EscapeString(courtName)
	complexEsc := html.EscapeString(complexName)
	dateEsc, startEsc := html.EscapeString(date), html.EscapeString(startTime)

	content := title("Se canceló una reserva.") +
		p(fmt.Sprintf("<strong>%s</strong> canceló su reserva de <strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			clientEsc, courtEsc, complexEsc, dateEsc, startEsc), 16, 16) +
		p("El turno quedó libre para reservar.", 16, 16) +
		button(m.appURL+"/bookings", "Ver en el panel")

	preheader := fmt.Sprintf("%s el %s de %s quedó libre.", courtEsc, dateEsc, startEsc)
	return m.send(ctx, to, subject, m.wrap("Se canceló una reserva", preheader, content))
}

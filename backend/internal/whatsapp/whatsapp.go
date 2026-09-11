package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
)

// graphAPIBaseURL is the Meta Graph API version every outbound call is made
// against.
//
// It is a named constant rather than a literal inside NewWAClient because the
// version is a deadline, not a detail: Meta expires a version roughly two years
// after it ships, and calls against an expired one start failing with 400s that
// say nothing about the cause. This deployment sat on v19.0, which expired on
// 21 May 2026 — the whole WhatsApp channel was already past its own end of
// life with nothing in the code saying so.
const graphAPIBaseURL = "https://graph.facebook.com/v25.0"

// WAClient is a circuit-breaker-wrapped HTTP client for the WhatsApp Cloud API.
type WAClient struct {
	token       string
	phoneID     string
	verifyToken string
	appSecret   string
	httpClient  *http.Client
	baseURL     string
	cb          *circuitbreaker.CircuitBreaker
}

// NewWAClient builds a WAClient authenticated with the given WhatsApp Cloud API credentials.
func NewWAClient(token, phoneID, verifyToken, appSecret string, cb *circuitbreaker.CircuitBreaker) *WAClient {
	return &WAClient{
		token:       token,
		phoneID:     phoneID,
		verifyToken: verifyToken,
		appSecret:   appSecret,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL: graphAPIBaseURL,
		cb:      cb,
	}
}

// doRequest executes an HTTP request through the circuit breaker.
func (c *WAClient) doRequest(req *http.Request) (*http.Response, error) {
	if err := c.cb.AllowRequest(); err != nil {
		return nil, fmt.Errorf("wa: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.RecordFailure()
		return nil, err
	}

	if resp.StatusCode >= 500 {
		c.cb.RecordFailure()
	} else {
		c.cb.RecordSuccess()
	}

	return resp, nil
}

// TemplateMessage represents a WhatsApp template message to send.
type TemplateMessage struct {
	TemplateName string
	LanguageCode string
	Components   []Component
}

// Component is one templated section (e.g. body or button) of a WhatsApp TemplateMessage.
type Component struct {
	Type       string      `json:"type"`
	SubType    string      `json:"sub_type,omitempty"`
	Index      string      `json:"index,omitempty"`
	Parameters []Parameter `json:"parameters"`
}

// Parameter is a single placeholder value substituted into a template Component.
type Parameter struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SendTemplate sends a template message to the given phone number.
func (c *WAClient) SendTemplate(ctx context.Context, to string, msg TemplateMessage) error {
	components := make([]map[string]any, len(msg.Components))
	for i, comp := range msg.Components {
		params := make([]map[string]string, len(comp.Parameters))
		for j, p := range comp.Parameters {
			params[j] = map[string]string{
				"type": p.Type,
				// Sanitised here rather than at each builder: this is the one
				// place every parameter of every template passes through, so a
				// template added later cannot forget it.
				"text": sanitizeParam(p.Text),
			}
		}
		c := map[string]any{
			"type":       comp.Type,
			"parameters": params,
		}
		if comp.SubType != "" {
			c["sub_type"] = comp.SubType
		}
		if comp.Index != "" {
			c["index"] = comp.Index
		}
		components[i] = c
	}

	body := map[string]any{
		"messaging_product": "whatsapp",
		// RecipientID, not the stored number: an Argentine mobile stored
		// without its 9 is valid E.164 and unroutable on WhatsApp.
		"to":   RecipientID(to),
		"type": "template",
		"template": map[string]any{
			"name":       msg.TemplateName,
			"language":   map[string]string{"code": msg.LanguageCode},
			"components": components,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("wa: failed to marshal message body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+c.phoneID+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("wa: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("wa: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("wa: send template failed with status %d (failed to read body: %w)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("wa: send template failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// VerifyWebhook handles the GET verification request from Meta when configuring the webhook.
func (c *WAClient) VerifyWebhook(r *http.Request) (string, error) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "subscribe" {
		return "", fmt.Errorf("wa: invalid hub.mode: %s", mode)
	}

	if token != c.verifyToken {
		return "", fmt.Errorf("wa: verify token mismatch")
	}

	return challenge, nil
}

// VerifySignature verifies the X-Hub-Signature-256 HMAC-SHA256 signature on incoming webhook POSTs.
func (c *WAClient) VerifySignature(r *http.Request, body []byte) error {
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		return fmt.Errorf("wa: missing X-Hub-Signature-256 header")
	}

	// Format: sha256={hex}
	if !strings.HasPrefix(signature, "sha256=") {
		return fmt.Errorf("wa: invalid signature format")
	}
	receivedHex := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(c.appSecret))
	mac.Write(body)
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedHex), []byte(receivedHex)) {
		return fmt.Errorf("wa: webhook signature verification failed")
	}

	return nil
}

// The four templates below are the whole client-facing WhatsApp surface. Each
// one has an approved counterpart in WhatsApp Manager whose body text is
// written out in docs/whatsapp-templates.md, and that document is the source
// of truth: the parameters here are positional, so adding, removing or
// reordering one silently changes which sentence a value lands in. Meta gives
// no error for that — the message goes out reading "Cancha: 15/03".
//
// Every dynamic sentence arrives pre-rendered from internal/notifications'
// copy helpers rather than being assembled here, so the WhatsApp message and
// the email of the same event say the same thing by construction.

// text is a body/button parameter. Every parameter these templates send is a
// text parameter; nothing here binds a currency, date or media component.
func text(s string) Parameter { return Parameter{Type: "text", Text: s} }

// urlButton is one dynamic URL button. index is the button's position in the
// template as it was approved, counted from zero, and it is what binds this
// value to that button rather than to the other one.
func urlButton(index, suffix string) Component {
	return Component{
		Type:       "button",
		SubType:    "url",
		Index:      index,
		Parameters: []Parameter{text(suffix)},
	}
}

// BookingConfirmationTemplate builds the booking_confirmation template message.
//
// depositAmount and balanceAmount are what the client paid as a deposit and
// what they still owe at the venue, each a plain formatted amount rather than
// a sentence; cancellationRule states the cancellation rule, lowercase,
// because the template's own fixed word ("Cancelación") comes right before
// it. All three come from internal/notifications (PaymentAmounts,
// CancellationLine) so the confirmation email states the same facts in the
// same words.
//
// Every parameter here sits between two of the template's fixed words —
// Meta's validator rejects a template with two variables directly adjacent —
// which is also why depositAmount and balanceAmount travel as two positional
// parameters rather than one pre-joined sentence: "Seña abonada: {{5}}. Resta
// pagar en el complejo: {{6}}." has fixed text before, between and after both.
//
// mapsQuery is the dynamic suffix for the "Ver ubicación" button, built by
// internal/booklink.MapsQuery. cancelPath is the suffix for the "Cancelar
// reserva" button, built by internal/booklink.CancelPath — a relative path
// ending in the booking's access token under internal/booklink.QueryParam's
// name.
func BookingConfirmationTemplate(courtName, complexName, date, startTime, depositAmount, balanceAmount, cancellationRule, mapsQuery, cancelPath string) TemplateMessage {
	return TemplateMessage{
		TemplateName: "booking_confirmation",
		LanguageCode: "es_AR",
		Components: []Component{
			{
				Type: "body",
				Parameters: []Parameter{
					text(courtName),
					text(complexName),
					text(date),
					text(startTime),
					text(depositAmount),
					text(balanceAmount),
					text(cancellationRule),
				},
			},
			urlButton("0", mapsQuery),
			urlButton("1", cancelPath),
		},
	}
}

// Reminder2hTemplate builds the reminder_2h template message.
//
// It carries the venue's address and the balance still owed there — via
// balanceAmount, a plain formatted amount from internal/notifications'
// BalanceAmount, which the confirmation two hours earlier is too far back to
// be re-read for — and the same two buttons: a client two hours out either
// needs directions or needs to call it off.
//
// address comes before balanceAmount because that is the order the approved
// body binds them in: "La dirección es {{5}}. Resta pagar en el complejo:
// {{6}}." The body also has to close on fixed text rather than {{6}}: Meta's
// validator rejects a template ending on a variable, a trailing period on the
// value does not count as text, so "Le esperamos." stands alone as the last
// line rather than "Le esperamos en {{6}}." the way the address used to be
// bound.
func Reminder2hTemplate(courtName, complexName, date, startTime, address, balanceAmount, mapsQuery, cancelPath string) TemplateMessage {
	return TemplateMessage{
		TemplateName: "reminder_2h",
		LanguageCode: "es_AR",
		Components: []Component{
			{
				Type: "body",
				Parameters: []Parameter{
					text(courtName),
					text(complexName),
					text(date),
					text(startTime),
					text(address),
					text(balanceAmount),
				},
			},
			urlButton("0", mapsQuery),
			urlButton("1", cancelPath),
		},
	}
}

// BookingCancelledTemplate builds the booking_cancelled template message.
//
// refundLine is the value substituted after the template's fixed
// "Devolución: ", rendered by internal/bookings from the same six outcomes
// the cancellation response carries (or, for a booking auto-cancelled for
// non-payment, by the cron that cancelled it). A cancellation that says
// nothing about the deposit is the message clients ask about, so this
// parameter is never empty.
//
// bookPath is the suffix for the "Nueva reserva" button, built by
// internal/booklink.BookPath.
func BookingCancelledTemplate(courtName, complexName, date, startTime, refundLine, bookPath string) TemplateMessage {
	return TemplateMessage{
		TemplateName: "booking_cancelled",
		LanguageCode: "es_AR",
		Components: []Component{
			{
				Type: "body",
				Parameters: []Parameter{
					text(courtName),
					text(complexName),
					text(date),
					text(startTime),
					text(refundLine),
				},
			},
			urlButton("0", bookPath),
		},
	}
}

// DepositRefundedTemplate builds the deposit_returned template message.
//
// amount comes before complexName because that is the order the approved
// body binds them in: "Se realizó la devolución de {{1}} de la seña de su
// reserva en {{2}}."
//
// The MercadoPago settlement time ("los próximos días hábiles") is literal
// text in the approved template rather than a parameter: it is a fact about
// the provider, not about this refund, and a fact in the template cannot be
// left blank by a caller.
func DepositRefundedTemplate(amount, complexName, bookPath string) TemplateMessage {
	return TemplateMessage{
		TemplateName: "deposit_returned",
		LanguageCode: "es_AR",
		Components: []Component{
			{
				Type: "body",
				Parameters: []Parameter{
					text(amount),
					text(complexName),
				},
			},
			urlButton("0", bookPath),
		},
	}
}

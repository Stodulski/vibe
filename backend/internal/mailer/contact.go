package mailer

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/stodulski/vibe-server/internal/validator"
)

// Contact is how a client reaches the complex an email is about: the complex's
// phone, and the address a reply to the email goes to (the owner's).
//
// Every client email about a booking, a fixed booking or an event carries it.
// Since the WhatsApp notices were removed the email is the only message the
// client gets, so "escribile al complejo" has to come with a way to do it.
// Either field may be empty; the email then offers what it has.
type Contact struct {
	Phone   string
	ReplyTo string
}

// waLink is the wa.me link for a phone, or empty when the phone does not read
// as an international number: a link to the wrong chat is worse than none.
func waLink(phone string) string {
	normalized, err := validator.NormalizePhone(phone)
	if err != nil {
		return ""
	}
	return "https://wa.me/" + strings.TrimPrefix(normalized, "+")
}

// contactLine is the closing line of a client email: who to write to, and how.
func contactLine(c Contact) string {
	phone := strings.TrimSpace(c.Phone)
	reply := ""
	if c.ReplyTo != "" {
		reply = " También podés responder este email."
	}
	if phone == "" {
		if c.ReplyTo != "" {
			return muted("Si tenés dudas, respondé este email y le llega al complejo.")
		}
		return muted("Si tenés dudas, escribile al complejo.")
	}
	line := fmt.Sprintf("Si tenés dudas, escribile al complejo al <strong>%s</strong>", html.EscapeString(phone))
	if wa := waLink(phone); wa != "" {
		line += " o por " + link(wa, "WhatsApp")
	}
	return muted(line + "." + reply)
}

// sendToClient sends a client email with the complex's reply-to address.
func (m *Mailer) sendToClient(ctx context.Context, to, subject, htmlBody string, c Contact) error {
	return m.sendWith(ctx, to, subject, htmlBody, outbound{replyTo: c.ReplyTo})
}

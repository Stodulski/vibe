package mailer

import (
	"context"
	"fmt"
	"html"
)

// SendEventRegistered tells a person their spot in an open-play event ("canchas
// libres") is confirmed. date is dd/mm with the weekday, hours the span as a
// person reads it, amount what they paid. The copy uses voseo, like every other
// client email, and closes with the complex's contact.
func (m *Mailer) SendEventRegistered(ctx context.Context, to, firstName, complexName, eventName, date, hours, address, mapsURL, amount, eventURL string, contact Contact) error {
	subject := "Tu lugar en " + eventName + " · " + complexName

	nameEsc, eventEsc, complexEsc := html.EscapeString(firstName), html.EscapeString(eventName), html.EscapeString(complexName)
	dateEsc, hoursEsc := html.EscapeString(date), html.EscapeString(hours)

	when := fmt.Sprintf("el <strong>%s</strong>, de <strong>%s</strong>", dateEsc, hoursEsc)
	if hours == "" {
		when = fmt.Sprintf("desde el <strong>%s</strong>", dateEsc)
	}
	content := title("Tu lugar está confirmado.") +
		p(fmt.Sprintf("Hola %s, quedó confirmada tu inscripción a <strong>%s</strong> en <strong>%s</strong>: %s.",
			nameEsc, eventEsc, complexEsc, when), 16, 16)
	if amount != "" {
		content += p(fmt.Sprintf("Registramos tu pago de <strong>%s</strong>.", html.EscapeString(amount)), 16, 16)
	}
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	if eventURL != "" {
		content += button(eventURL, "Ver el evento")
	}
	content += contactLine(contact)

	preheader := fmt.Sprintf("%s · %s, %s.", eventEsc, dateEsc, hoursEsc)
	if hours == "" {
		preheader = fmt.Sprintf("%s · desde el %s.", eventEsc, dateEsc)
	}
	return m.sendToClient(ctx, to, subject, m.wrap("Inscripción confirmada", preheader, content), contact)
}

// SendEventCancelled tells a person their spot in an event is over: the owner
// cancelled the event (byOwner), or they cancelled their own spot. refundLine is
// the sentence about their money, refundAmount the figure when something is
// coming back.
func (m *Mailer) SendEventCancelled(ctx context.Context, to, firstName, complexName, eventName, date, hours string, byOwner bool, refundLine, refundAmount, eventsURL string, contact Contact) error {
	subject := "Inscripción cancelada · " + eventName
	heading := "Tu inscripción se canceló."
	if byOwner {
		subject = "Evento cancelado · " + eventName
		heading = "El evento se canceló."
	}

	eventEsc, complexEsc := html.EscapeString(eventName), html.EscapeString(complexName)
	dateEsc, hoursEsc := html.EscapeString(date), html.EscapeString(hours)

	whenText := dateEsc + ", " + hoursEsc
	if hours == "" {
		whenText = "desde el " + dateEsc
	}
	lead := fmt.Sprintf("Hola %s, tu inscripción a <strong>%s</strong> en <strong>%s</strong> (%s) se canceló.",
		html.EscapeString(firstName), eventEsc, complexEsc, whenText)
	if byOwner {
		lead = fmt.Sprintf("Hola %s, el complejo canceló <strong>%s</strong> en <strong>%s</strong> (%s).",
			html.EscapeString(firstName), eventEsc, complexEsc, whenText)
	}
	content := title(heading) + p(lead, 16, 16)
	if refundLine != "" {
		line := html.EscapeString(refundLine)
		if refundAmount != "" {
			line += " <strong>" + html.EscapeString(refundAmount) + "</strong>"
		}
		content += p(line, 16, 16)
	}
	content += contactLine(contact)
	if eventsURL != "" {
		content += button(eventsURL, "Ver la agenda")
	}

	preheader := fmt.Sprintf("%s · %s.", eventEsc, dateEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Inscripción cancelada", preheader, content), contact)
}

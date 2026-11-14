package mailer

import (
	"context"
	"fmt"
	"html"
)

// SendEventRegistered tells a person their spot in an open-play event ("canchas
// libres") is confirmed. date is dd/mm with the weekday, hours the span as a
// person reads it, amount what they paid. The copy addresses them as "usted",
// like every other client email.
func (m *Mailer) SendEventRegistered(ctx context.Context, to, firstName, complexName, eventName, date, hours, address, mapsURL, amount, eventURL string) error {
	subject := "Su lugar en " + eventName + " · " + complexName

	nameEsc, eventEsc, complexEsc := html.EscapeString(firstName), html.EscapeString(eventName), html.EscapeString(complexName)
	dateEsc, hoursEsc := html.EscapeString(date), html.EscapeString(hours)

	content := title("Su lugar está confirmado.") +
		p(fmt.Sprintf("Hola %s, quedó confirmada su inscripción a <strong>%s</strong> en <strong>%s</strong>: el <strong>%s</strong>, de <strong>%s</strong>.",
			nameEsc, eventEsc, complexEsc, dateEsc, hoursEsc), 16, 16)
	if amount != "" {
		content += p(fmt.Sprintf("Se registró su pago de <strong>%s</strong>.", html.EscapeString(amount)), 16, 16)
	}
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	if eventURL != "" {
		content += button(eventURL, "Ver el evento")
	}

	preheader := fmt.Sprintf("%s · %s, %s.", eventEsc, dateEsc, hoursEsc)
	return m.send(ctx, to, subject, m.wrap("Inscripción confirmada", preheader, content))
}

// SendEventCancelled tells a person their spot in an event is over: the owner
// cancelled the event (byOwner), or they cancelled their own spot. refundLine is
// the sentence about their money, refundAmount the figure when something is
// coming back.
func (m *Mailer) SendEventCancelled(ctx context.Context, to, firstName, complexName, eventName, date, hours string, byOwner bool, refundLine, refundAmount, eventsURL string) error {
	subject := "Inscripción cancelada · " + eventName
	heading := "Su inscripción fue cancelada."
	if byOwner {
		subject = "Evento cancelado · " + eventName
		heading = "El evento fue cancelado."
	}

	eventEsc, complexEsc := html.EscapeString(eventName), html.EscapeString(complexName)
	dateEsc, hoursEsc := html.EscapeString(date), html.EscapeString(hours)

	lead := fmt.Sprintf("Hola %s, su inscripción a <strong>%s</strong> en <strong>%s</strong> (%s, %s) se canceló.",
		html.EscapeString(firstName), eventEsc, complexEsc, dateEsc, hoursEsc)
	if byOwner {
		lead = fmt.Sprintf("Hola %s, el complejo canceló <strong>%s</strong> en <strong>%s</strong> (%s, %s).",
			html.EscapeString(firstName), eventEsc, complexEsc, dateEsc, hoursEsc)
	}
	content := title(heading) + p(lead, 16, 16)
	if refundLine != "" {
		line := html.EscapeString(refundLine)
		if refundAmount != "" {
			line += " <strong>" + html.EscapeString(refundAmount) + "</strong>"
		}
		content += p(line, 16, 16)
	}
	content += muted("Ante cualquier consulta, contacte al complejo.")
	if eventsURL != "" {
		content += button(eventsURL, "Ver la agenda")
	}

	preheader := fmt.Sprintf("%s · %s.", eventEsc, dateEsc)
	return m.send(ctx, to, subject, m.wrap("Inscripción cancelada", preheader, content))
}

package mailer

import (
	"context"
	"fmt"
	"html"
)

// SendSeriesCreated tells a client that a fixed weekly booking ("turno fijo")
// was set up for them. weekdays is the plural after "todos los" ("lunes"),
// hours the span as a person reads it, from and until are dd/mm (until is empty
// when the series has no end). The copy uses voseo, like every other client
// email, and closes with the complex's contact.
func (m *Mailer) SendSeriesCreated(ctx context.Context, to, complexName, courtName, weekdays, hours, from, until, address, mapsURL string, contact Contact) error {
	subject := "Tu turno fijo en " + complexName

	weekdaysEsc, hoursEsc := html.EscapeString(weekdays), html.EscapeString(hours)
	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	span := "desde el " + html.EscapeString(from)
	if until != "" {
		span += ", hasta el " + html.EscapeString(until)
	}

	content := title("Tu turno fijo está reservado.") +
		p(fmt.Sprintf("Tu turno fijo quedó reservado: todos los <strong>%s</strong> de <strong>%s</strong> en <strong>%s</strong> (%s), %s. Cada turno se paga en el complejo.",
			weekdaysEsc, hoursEsc, courtEsc, complexEsc, span), 16, 16)
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	content += contactLine(contact)

	preheader := fmt.Sprintf("Todos los %s de %s en %s.", weekdaysEsc, hoursEsc, complexEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Turno fijo reservado", preheader, content), contact)
}

// SendSeriesCancelled tells a client that the owner cancelled their fixed
// weekly booking from a date on. keptPaid says some already-booked dates carry a
// payment and stand.
func (m *Mailer) SendSeriesCancelled(ctx context.Context, to, complexName, courtName, weekdays, hours, from string, keptPaid bool, bookURL string, contact Contact) error {
	subject := "Turno fijo cancelado · " + complexName

	weekdaysEsc, hoursEsc := html.EscapeString(weekdays), html.EscapeString(hours)
	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	fromEsc := html.EscapeString(from)

	content := title("Tu turno fijo se canceló.") +
		p(fmt.Sprintf("Tu turno fijo de los <strong>%s</strong> de <strong>%s</strong> en <strong>%s</strong> (%s) se canceló desde el %s.",
			weekdaysEsc, hoursEsc, courtEsc, complexEsc, fromEsc), 16, 16)
	if keptPaid {
		content += p("Los turnos ya pagos se mantienen.", 16, 16)
	}
	content += contactLine(contact)
	if bookURL != "" {
		content += button(bookURL, "Nueva reserva")
	}

	preheader := fmt.Sprintf("Tu turno fijo en %s se canceló desde el %s.", complexEsc, fromEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Turno fijo cancelado", preheader, content), contact)
}

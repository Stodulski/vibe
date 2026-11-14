package mailer

import (
	"context"
	"fmt"
	"html"
)

// SendSeriesCreated tells a client that a fixed weekly booking ("turno fijo")
// was set up for them. weekdays is the plural after "todos los" ("lunes"),
// hours the span as a person reads it, from and until are dd/mm (until is empty
// when the series has no end). The copy addresses the client as "usted", like
// every other client email.
func (m *Mailer) SendSeriesCreated(ctx context.Context, to, complexName, courtName, weekdays, hours, from, until, address, mapsURL string) error {
	subject := "Su turno fijo en " + complexName

	weekdaysEsc, hoursEsc := html.EscapeString(weekdays), html.EscapeString(hours)
	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	span := "desde el " + html.EscapeString(from)
	if until != "" {
		span += ", hasta el " + html.EscapeString(until)
	}

	content := title("Su turno fijo está reservado.") +
		p(fmt.Sprintf("Su turno fijo quedó reservado: todos los <strong>%s</strong> de <strong>%s</strong> en <strong>%s</strong> (%s), %s. Cada turno se paga en el complejo.",
			weekdaysEsc, hoursEsc, courtEsc, complexEsc, span), 16, 16)
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}

	preheader := fmt.Sprintf("Todos los %s de %s en %s.", weekdaysEsc, hoursEsc, complexEsc)
	return m.send(ctx, to, subject, m.wrap("Turno fijo reservado", preheader, content))
}

// SendSeriesCancelled tells a client that the owner cancelled their fixed
// weekly booking from a date on. keptPaid says some already-booked dates carry a
// payment and stand.
func (m *Mailer) SendSeriesCancelled(ctx context.Context, to, complexName, courtName, weekdays, hours, from string, keptPaid bool, bookURL string) error {
	subject := "Turno fijo cancelado · " + complexName

	weekdaysEsc, hoursEsc := html.EscapeString(weekdays), html.EscapeString(hours)
	courtEsc, complexEsc := html.EscapeString(courtName), html.EscapeString(complexName)
	fromEsc := html.EscapeString(from)

	content := title("Su turno fijo fue cancelado.") +
		p(fmt.Sprintf("Su turno fijo de los <strong>%s</strong> de <strong>%s</strong> en <strong>%s</strong> (%s) se canceló desde el %s.",
			weekdaysEsc, hoursEsc, courtEsc, complexEsc, fromEsc), 16, 16)
	if keptPaid {
		content += p("Los turnos ya pagos se mantienen.", 16, 16)
	}
	content += muted("Ante cualquier consulta, contacte al complejo.")
	if bookURL != "" {
		content += button(bookURL, "Nueva reserva")
	}

	preheader := fmt.Sprintf("Su turno fijo en %s se canceló desde el %s.", complexEsc, fromEsc)
	return m.send(ctx, to, subject, m.wrap("Turno fijo cancelado", preheader, content))
}

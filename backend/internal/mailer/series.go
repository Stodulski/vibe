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
//
// moved lists the dates played on another court because the series' own court
// was taken, one line each ("12/12: Cancha 2"); empty when there are none.
func (m *Mailer) SendSeriesCreated(ctx context.Context, to, complexName, courtName, weekdays, hours, from, until, address, mapsURL string, moved []string, contact Contact) error {
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
	if len(moved) > 0 {
		content += p("Estas fechas las jugás en otra cancha:", 16, 8)
		for _, line := range moved {
			content += p(html.EscapeString(line), 8, 8)
		}
	}
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

// SendOwnerSeriesDatesLost tells the owner that the scheduler could not book
// some dates of a fixed weekly booking, one line per date with why. Nobody
// decided those dates were lost, so this is the only place the owner learns
// the client has no court on them.
func (m *Mailer) SendOwnerSeriesDatesLost(ctx context.Context, to, complexName, courtName, clientName, weekdays, hours string, lines []string) error {
	subject := "No pudimos reservar fechas de un turno fijo · " + complexName

	clientEsc, courtEsc := html.EscapeString(clientName), html.EscapeString(courtName)
	weekdaysEsc, hoursEsc := html.EscapeString(weekdays), html.EscapeString(hours)

	content := title("Hay fechas de un turno fijo sin reservar.") +
		p(fmt.Sprintf("El turno fijo de <strong>%s</strong> (todos los %s de %s en %s) no se pudo reservar en estas fechas:",
			clientEsc, weekdaysEsc, hoursEsc, courtEsc), 16, 16)
	for _, line := range lines {
		content += p(html.EscapeString(line), 8, 8)
	}
	content += p("El resto del turno fijo sigue igual. Si hace falta, avisale al cliente.", 16, 16) +
		button(m.appURL+"/bookings", "Ver en el panel")

	preheader := fmt.Sprintf("Turno fijo de %s: %d fechas sin reservar.", clientEsc, len(lines))
	return m.send(ctx, to, subject, m.wrap("Fechas de un turno fijo sin reservar", preheader, content))
}

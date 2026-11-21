package mailer

import (
	"context"
	"fmt"
	"html"
)

// The booking notices below close the moments a client or an owner used to be
// told nothing: a booking marked as not attended, a payment taken at the
// counter, money a complex owes back by hand, and a repeated payment
// MercadoPago returned on its own. internal/notifications sends them.

// bookButton is the "Nueva reserva" button the client booking emails close
// with, or nothing when there is no booking page to link.
func bookButton(bookURL string) string {
	if bookURL == "" {
		return ""
	}
	return button(bookURL, "Nueva reserva")
}

// SendBookingNoShow tells a client that the complex marked their booking as
// not attended. It is short and neutral on purpose: it says what was recorded
// and how to reach the complex if it was a mistake, and nothing about
// consequences.
func (m *Mailer) SendBookingNoShow(ctx context.Context, to, complexName, courtName, date, startTime, bookURL string, contact Contact) error {
	subject := "Tu reserva quedó como no asistida · " + complexName

	complexEsc, dateEsc := html.EscapeString(complexName), html.EscapeString(date)
	content := title("Tu reserva quedó como no asistida.") +
		p(fmt.Sprintf("<strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			html.EscapeString(courtName), complexEsc, dateEsc, html.EscapeString(startTime)), 16, 16) +
		p("El complejo la registró como no asistida. Si fuiste y es un error, escribile al complejo y lo corrigen.", 16, 16) +
		contactLine(contact) +
		bookButton(bookURL)

	preheader := fmt.Sprintf("%s registró tu reserva del %s como no asistida.", complexEsc, dateEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Reserva no asistida", preheader, content), contact)
}

// SendPaymentReceived is the client's receipt for a payment the complex took
// at the counter. method is how it was paid, already a phrase ("en
// efectivo"); balanceAmount is what is still owed, empty when nothing is.
func (m *Mailer) SendPaymentReceived(ctx context.Context, to, complexName, courtName, date, startTime, amount, method, balanceAmount, bookURL string, contact Contact) error {
	subject := "Recibimos tu pago · " + complexName

	amountEsc, complexEsc := html.EscapeString(amount), html.EscapeString(complexName)
	paid := "<strong>" + amountEsc + "</strong>"
	if method != "" {
		paid += " " + html.EscapeString(method)
	}
	content := title("Recibimos tu pago.") +
		p(fmt.Sprintf("El complejo registró %s por tu reserva de <strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			paid, html.EscapeString(courtName), complexEsc, html.EscapeString(date), html.EscapeString(startTime)), 16, 16)
	if balanceAmount != "" {
		content += p(fmt.Sprintf("Te queda por pagar <strong>%s</strong> en el complejo.", html.EscapeString(balanceAmount)), 16, 16)
	} else {
		content += p("Tu reserva está paga.", 16, 16)
	}
	content += contactLine(contact) + bookButton(bookURL)

	preheader := fmt.Sprintf("Registramos tu pago de %s en %s.", amountEsc, complexEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Pago recibido", preheader, content), contact)
}

// SendRefundOwedByVenue tells a client that the complex owes them amount back
// and returns it directly, because that money was not paid through
// MercadoPago and nothing automatic can send it.
func (m *Mailer) SendRefundOwedByVenue(ctx context.Context, to, complexName, courtName, date, startTime, amount, bookURL string, contact Contact) error {
	subject := "Tu devolución · " + complexName

	amountEsc, complexEsc := html.EscapeString(amount), html.EscapeString(complexName)
	content := title("El complejo te va a devolver tu dinero.") +
		p(fmt.Sprintf("Por tu reserva de <strong>%s</strong> en <strong>%s</strong>, el %s de %s, el complejo te va a devolver <strong>%s</strong> directamente.",
			html.EscapeString(courtName), complexEsc, html.EscapeString(date), html.EscapeString(startTime), amountEsc), 16, 16) +
		p("Escribile para coordinar cómo te lo devuelve.", 16, 16) +
		contactLine(contact) +
		bookButton(bookURL)

	preheader := fmt.Sprintf("%s te va a devolver %s.", complexEsc, amountEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Tu devolución", preheader, content), contact)
}

// SendOwnerRefundOwed tells the owner that a client is owed amount back by
// hand: money taken outside MercadoPago on a booking whose refund the
// automatic path could not finish. The button opens the bookings page, where
// the owner marks it returned.
func (m *Mailer) SendOwnerRefundOwed(ctx context.Context, to, complexName, courtName, clientName, date, startTime, amount string) error {
	subject := fmt.Sprintf("Tenés una devolución pendiente: %s %s %s", courtName, date, startTime)

	clientEsc, amountEsc := html.EscapeString(clientName), html.EscapeString(amount)
	content := title("Tenés una devolución pendiente.") +
		p(fmt.Sprintf("<strong>%s</strong> tiene que recibir <strong>%s</strong> de su reserva de <strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			clientEsc, amountEsc, html.EscapeString(courtName), html.EscapeString(complexName),
			html.EscapeString(date), html.EscapeString(startTime)), 16, 16) +
		p("Ese dinero se cobró fuera de MercadoPago, así que la devolución la hacés vos. Cuando se lo devuelvas, marcala en el panel.", 16, 16) +
		button(m.appURL+"/bookings", "Ver en el panel")

	preheader := fmt.Sprintf("%s tiene que recibir %s.", clientEsc, amountEsc)
	return m.send(ctx, to, subject, m.wrap("Devolución pendiente", preheader, content))
}

// SendOwnerDuplicatePaymentRefunded tells the owner that a client paid the
// same booking twice and the second payment went back through MercadoPago on
// its own. The booking stands; the owner has nothing to do.
func (m *Mailer) SendOwnerDuplicatePaymentRefunded(ctx context.Context, to, complexName, courtName, clientName, date, startTime, amount string) error {
	subject := fmt.Sprintf("Devolvimos un pago repetido: %s %s %s", courtName, date, startTime)

	clientEsc, amountEsc := html.EscapeString(clientName), html.EscapeString(amount)
	content := title("Devolvimos un pago repetido.") +
		p(fmt.Sprintf("<strong>%s</strong> pagó dos veces su reserva de <strong>%s</strong> en <strong>%s</strong>, el %s de %s.",
			clientEsc, html.EscapeString(courtName), html.EscapeString(complexName),
			html.EscapeString(date), html.EscapeString(startTime)), 16, 16) +
		p(fmt.Sprintf("Le devolvimos el segundo pago (<strong>%s</strong>) por MercadoPago. La reserva sigue en pie y no tenés que hacer nada.", amountEsc), 16, 16) +
		button(m.appURL+"/bookings", "Ver en el panel")

	preheader := fmt.Sprintf("Le devolvimos %s a %s.", amountEsc, clientEsc)
	return m.send(ctx, to, subject, m.wrap("Pago repetido devuelto", preheader, content))
}

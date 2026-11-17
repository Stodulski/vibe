package mailer

import (
	"context"
	"fmt"
	"html"
)

// SendWaitlistPlaceFreed tells someone on the place waitlist that the place they
// waited for opened up. placeLabel says what freed ("Cancha 1 el miércoles 18/11
// a las 19:00"); bookURL opens it directly. Nobody holds the place for them: the
// first to pay keeps it.
func (m *Mailer) SendWaitlistPlaceFreed(ctx context.Context, to, complexName, placeLabel, bookURL string) error {
	subject := "Se liberó un lugar en " + complexName
	complexEsc := html.EscapeString(complexName)
	placeEsc := html.EscapeString(placeLabel)

	content := title("Se liberó un lugar.") +
		p(fmt.Sprintf("Se liberó un lugar en <strong>%s</strong>, en <strong>%s</strong>.", placeEsc, complexEsc), 16, 16) +
		p("El primero que paga se queda con el lugar.", 16, 16)
	if bookURL != "" {
		content += button(bookURL, "Ir a reservar")
	}

	preheader := fmt.Sprintf("Se liberó un lugar en %s.", complexEsc)
	return m.send(ctx, to, subject, m.wrap("Se liberó un lugar", preheader, content))
}

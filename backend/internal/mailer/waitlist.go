package mailer

import (
	"context"
	"fmt"
	"html"
)

// SendWaitlistSlotFreed tells someone who asked to be told about a day that a
// slot on it has opened up again. date is the day as dd/mm.
func (m *Mailer) SendWaitlistSlotFreed(ctx context.Context, to, complexName, date, bookURL string) error {
	subject := "Se liberó un turno en " + complexName
	complexEsc := html.EscapeString(complexName)
	dateEsc := html.EscapeString(date)

	content := title("Se liberó un turno.") +
		p(fmt.Sprintf("Se liberó un turno el <strong>%s</strong> en <strong>%s</strong>. Reservalo antes que otro.",
			dateEsc, complexEsc), 16, 16)
	if bookURL != "" {
		content += button(bookURL, "Ver horarios")
	}

	preheader := fmt.Sprintf("Se liberó un turno el %s en %s.", dateEsc, complexEsc)
	return m.send(ctx, to, subject, m.wrap("Se liberó un turno", preheader, content))
}

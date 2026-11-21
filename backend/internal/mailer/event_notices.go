package mailer

import (
	"context"
	"fmt"
	"html"
)

// OwnerEventNotice is what the owner reads about one sign-up to an event or a
// tournament category: who, for what, when, and what they paid.
type OwnerEventNotice struct {
	ComplexName     string
	EventName       string
	Date            string
	Hours           string
	ParticipantName string
	Amount          string
}

// SendOwnerEventRegistered tells the owner a person paid their spot in an
// event or a tournament category.
func (m *Mailer) SendOwnerEventRegistered(ctx context.Context, to string, n OwnerEventNotice) error {
	nameEsc, eventEsc := html.EscapeString(n.ParticipantName), html.EscapeString(n.EventName)
	subject := "Nueva inscripción · " + n.EventName

	lead := fmt.Sprintf("<strong>%s</strong> se anotó en <strong>%s</strong> (%s)", nameEsc, eventEsc, ownerWhen(n))
	if n.Amount != "" {
		lead += fmt.Sprintf(" y pagó <strong>%s</strong>", html.EscapeString(n.Amount))
	}
	content := title("Nueva inscripción.") + p(lead+".", 16, 16) + button(m.appURL+"/events", "Ver en el panel")

	preheader := fmt.Sprintf("%s se anotó en %s.", nameEsc, eventEsc)
	return m.send(ctx, to, subject, m.wrap("Nueva inscripción", preheader, content))
}

// SendOwnerEventCancelled tells the owner a person who had paid gave up their
// spot, so it is free again.
func (m *Mailer) SendOwnerEventCancelled(ctx context.Context, to string, n OwnerEventNotice) error {
	nameEsc, eventEsc := html.EscapeString(n.ParticipantName), html.EscapeString(n.EventName)
	subject := "Se liberó un lugar · " + n.EventName

	content := title("Se liberó un lugar.") +
		p(fmt.Sprintf("<strong>%s</strong> canceló su inscripción a <strong>%s</strong> (%s). El lugar ya está disponible para otra persona.",
			nameEsc, eventEsc, ownerWhen(n)), 16, 16) +
		button(m.appURL+"/events", "Ver en el panel")

	preheader := fmt.Sprintf("%s canceló su inscripción a %s.", nameEsc, eventEsc)
	return m.send(ctx, to, subject, m.wrap("Inscripción cancelada", preheader, content))
}

// ownerWhen is the event's day and hours as the owner's emails write them.
func ownerWhen(n OwnerEventNotice) string {
	if n.Hours == "" {
		return "desde el " + html.EscapeString(n.Date)
	}
	return html.EscapeString(n.Date) + ", " + html.EscapeString(n.Hours)
}

// SendEventRefunded tells a person that part of what they paid for their spot
// came back, while the spot itself stays theirs.
func (m *Mailer) SendEventRefunded(ctx context.Context, to, firstName, complexName, eventName, date, amount, eventURL string, contact Contact) error {
	eventEsc, complexEsc := html.EscapeString(eventName), html.EscapeString(complexName)
	subject := "Te devolvimos dinero · " + eventName

	content := title("Te devolvimos dinero.") +
		p(fmt.Sprintf("Hola %s, el complejo te devolvió <strong>%s</strong> de tu inscripción a <strong>%s</strong> en <strong>%s</strong> (%s). Lo vas a ver acreditado en los próximos días.",
			html.EscapeString(firstName), html.EscapeString(amount), eventEsc, complexEsc, html.EscapeString(date)), 16, 16) +
		p("Tu lugar sigue confirmado.", 16, 16)
	if eventURL != "" {
		content += button(eventURL, "Ver el evento")
	}
	content += contactLine(contact)

	preheader := fmt.Sprintf("%s · %s.", eventEsc, html.EscapeString(amount))
	return m.sendToClient(ctx, to, subject, m.wrap("Devolución", preheader, content), contact)
}

// SendEventReminder reminds a person of the event or tournament they signed up
// for, the day before. when is the start as they read it ("mañana sábado 15/11
// a las 19:00").
func (m *Mailer) SendEventReminder(ctx context.Context, to, firstName, complexName, eventName, when, address, mapsURL, eventURL string, contact Contact) error {
	eventEsc, complexEsc, whenEsc := html.EscapeString(eventName), html.EscapeString(complexName), html.EscapeString(when)
	subject := "Te esperamos · " + eventName

	content := title("Te esperamos.") +
		p(fmt.Sprintf("Hola %s, te recordamos que <strong>%s</strong> en <strong>%s</strong> es <strong>%s</strong>.",
			html.EscapeString(firstName), eventEsc, complexEsc, whenEsc), 16, 16)
	if address != "" && mapsURL != "" {
		content += addressLine(address, mapsURL)
	}
	if eventURL != "" {
		content += button(eventURL, "Ver el evento")
	}
	content += contactLine(contact)

	preheader := fmt.Sprintf("%s · %s.", eventEsc, whenEsc)
	return m.sendToClient(ctx, to, subject, m.wrap("Recordatorio", preheader, content), contact)
}

// Kinds of tournament notice SendTournamentNotice writes.
const (
	TournamentDrawPublished   = "draw_published"
	TournamentDrawRemade      = "draw_remade"
	TournamentMatchScheduled  = "match_scheduled"
	TournamentMatchMoved      = "match_rescheduled"
	TournamentMatchReminder   = "match_reminder"
	TournamentOpponentUnknown = "a definir"
)

// TournamentNotice is one player's notice about their tournament: the draw, or
// one of their matches.
type TournamentNotice struct {
	Kind           string
	FirstName      string
	ComplexName    string
	TournamentName string
	CategoryName   string
	// The match: when it is as the player reads it, the court, and the pair
	// on the other side ("Pérez / Gómez", or TournamentOpponentUnknown). Empty
	// for a draw notice.
	When     string
	Court    string
	Opponent string
	Address  string
	MapsURL  string
	// PageURL is the tournament's public page, where the draw is.
	PageURL string
}

// SendTournamentNotice tells a player about their tournament: the draw came out
// or was remade, or one of their matches was scheduled, moved or is about to
// start.
func (m *Mailer) SendTournamentNotice(ctx context.Context, to string, n TournamentNotice, contact Contact) error {
	what := html.EscapeString(n.CategoryName)
	if n.TournamentName != "" {
		what = html.EscapeString(n.TournamentName) + " · " + what
	}
	hello := "Hola " + html.EscapeString(n.FirstName) + ", "
	match := fmt.Sprintf("contra <strong>%s</strong>, <strong>%s</strong> en <strong>%s</strong>",
		html.EscapeString(n.Opponent), html.EscapeString(n.When), html.EscapeString(n.Court))

	var subject, heading, lead, label string
	switch n.Kind {
	case TournamentDrawPublished:
		subject, heading, label = "Ya salió el cuadro · "+n.CategoryName, "Ya salió el cuadro.", "Cuadro publicado"
		lead = hello + fmt.Sprintf("ya está el cuadro de <strong>%s</strong>. Entrá a ver tu grupo y tus partidos.", what)
	case TournamentDrawRemade:
		subject, heading, label = "El cuadro cambió · "+n.CategoryName, "El cuadro cambió.", "Cuadro actualizado"
		lead = hello + fmt.Sprintf("el complejo armó de nuevo el cuadro de <strong>%s</strong>. Revisá tu grupo y tus partidos.", what)
	case TournamentMatchMoved:
		subject, heading, label = "Tu partido cambió de horario · "+n.CategoryName, "Tu partido cambió de horario.", "Partido reprogramado"
		lead = hello + fmt.Sprintf("tu partido de <strong>%s</strong> ahora es %s.", what, match)
	case TournamentMatchReminder:
		subject, heading, label = "Hoy jugás · "+n.CategoryName, "Se viene tu partido.", "Recordatorio"
		lead = hello + fmt.Sprintf("te recordamos tu partido de <strong>%s</strong>: %s.", what, match)
	default:
		subject, heading, label = "Tu partido ya tiene horario · "+n.CategoryName, "Tu partido ya tiene horario.", "Partido programado"
		lead = hello + fmt.Sprintf("tu partido de <strong>%s</strong> es %s.", what, match)
	}

	content := title(heading) + p(lead, 16, 16)
	if n.When != "" && n.Address != "" && n.MapsURL != "" {
		content += addressLine(n.Address, n.MapsURL)
	}
	if n.PageURL != "" {
		content += button(n.PageURL, "Ver el torneo")
	}
	content += contactLine(contact)

	preheader := what
	if n.When != "" {
		preheader += " · " + html.EscapeString(n.When)
	}
	return m.sendToClient(ctx, to, subject, m.wrap(label, preheader+".", content), contact)
}

package mailer

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// One-off campaign (2026-10). Remove after it has been sent.

// benefitsSegment is the last line of the email and the link that follows it.
type benefitsSegment struct {
	line, label string
}

// benefitsSegments are the three endings, keyed by the campaign's segment names.
var benefitsSegments = map[string]benefitsSegment{
	"create_complex": {"Cargá tu complejo en 5 minutos.", "Cargar mi complejo"},
	"connect_mp":     {"Conectá MercadoPago para cobrar señas.", "Conectar MercadoPago"},
	"share_page":     {"Compartí tu página de reservas.", "Ver mi página"},
}

// benefitsItems are the five things Vibe does, in the order the email lists them.
var benefitsItems = []struct {
	heading, text string
}{
	{"Menos tiempo con el WhatsApp",
		"Tus clientes ven los horarios libres en tu página y reservan solos, a cualquier hora."},
	{"Señas cobradas, sin perseguir a nadie",
		"La seña entra por MercadoPago al reservar. Si cancelan, la reserva se libera sola."},
	{"Turnos que no quedan vacíos",
		"Si alguien cancela, te enterás al instante. Y si un día está completo, tus clientes se anotan en una lista y les avisamos cuando se libera un turno."},
	{"La caja, clara al cierre",
		"Cobrás el turno y las paletas en un solo paso. Al final del día sabés cuánto entró y por dónde."},
	{"El mes, listo para tu contador",
		"Reportes de cobros del mes, que exportás a Excel con un clic."},
}

const (
	benefitsSubject   = "Lo que Vibe hace por tu complejo"
	benefitsPreheader = "Menos tiempo en el teléfono, más turnos ocupados."
)

// SendBenefitsCampaign emails the one-off "beneficios de Vibe" copy to one owner. It
// is a marketing email, so it carries the same List-Unsubscribe headers and footer as
// the lifecycle nudges. segment picks the last line; an unknown segment is a permanent
// failure, so the queue never retries a message that cannot be built.
func (m *Mailer) SendBenefitsCampaign(ctx context.Context, to, firstName, segment, segmentURL, panelURL, unsubscribeURL, optOutURL string) error {
	seg, ok := benefitsSegments[segment]
	if !ok {
		return permanentErr{msg: fmt.Sprintf("mailer: unknown benefits segment %q", segment)}
	}

	var items strings.Builder
	for i, it := range benefitsItems {
		margin := 12
		if i == len(benefitsItems)-1 {
			margin = 16
		}
		items.WriteString(p("<strong>"+it.heading+"</strong><br>"+it.text, 16, margin))
	}

	content := p(benefitsGreeting(firstName), 16, 12) +
		p("Vibe está para que tu complejo funcione solo y vos te dediques a lo importante.", 16, 16) +
		items.String() +
		button(panelURL, "Ir a mi panel") +
		fallback(panelURL) +
		p(seg.line+" "+link(segmentURL, seg.label), 16, 16) +
		p("Vibe es gratis para tu complejo. Si querés que te ayude a configurarlo, respondé este mail.", 16, 16) +
		p("Máximo, de Vibe", 16, 24) +
		p("Si no querés recibir más estos mails, "+link(optOutURL, "date de baja acá")+".", 13, 0)

	return m.sendWith(ctx, to, benefitsSubject, m.wrap(benefitsSubject, benefitsPreheader, content), outbound{
		headers: listHeaders(unsubscribeURL),
		replyTo: supportEmail,
	})
}

// benefitsGreeting addresses the owner by the first name they gave, and says just
// "Hola," when there is none, so an empty name never leaves a gap.
func benefitsGreeting(firstName string) string {
	name := strings.TrimSpace(firstName)
	if name == "" {
		return "Hola,"
	}
	return "Hola " + html.EscapeString(name) + ","
}

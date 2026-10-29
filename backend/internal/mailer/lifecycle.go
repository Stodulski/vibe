package mailer

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// supportEmail is where an owner replies to a nudge, and the mailto fallback of
// its one-click unsubscribe. Nudges are sent from the no-reply sender, so the
// reply address is what makes "respondé este mail" work.
const supportEmail = "hola@vibe.com.ar"

// nudgeCopy is one lifecycle email's wording. The strings are the product's
// approved copy; change them here and nowhere else.
type nudgeCopy struct {
	subject   string
	preheader string
	body      string
	cta       string
}

// nudgeCopyFor returns the copy for a kind, or false for a kind this package does
// not know. The first and complex names are escaped here, because they go into
// the HTML body verbatim.
func nudgeCopyFor(kind, firstName, complexName string) (nudgeCopy, bool) {
	hi := nudgeGreeting(firstName)
	venue := html.EscapeString(strings.TrimSpace(complexName))
	if venue == "" {
		venue = "Tu complejo"
	}

	switch kind {
	case "verify_email":
		return nudgeCopy{
			subject:   "Te falta confirmar tu email en Vibe",
			preheader: "Te falta confirmar tu email para entrar a Vibe.",
			body:      hi + " Empezaste a crear tu cuenta en Vibe y te falta confirmar tu email. Tocá el botón y seguís donde quedaste.",
			cta:       "Confirmar mi email",
		}, true
	case "create_complex":
		return nudgeCopy{
			subject:   "Tu complejo te espera en Vibe",
			preheader: "Te falta cargar tu complejo. Te lleva unos minutos.",
			body:      hi + " Ya tenés tu cuenta. Te falta cargar tu complejo: nombre, dirección y horarios. Te lleva unos minutos.",
			cta:       "Cargar mi complejo",
		}, true
	case "add_courts":
		return nudgeCopy{
			subject:   "Cargá tus canchas en Vibe",
			preheader: "Cargá tus canchas para empezar a tomar reservas.",
			body:      hi + " " + venue + " ya está en Vibe. Cargá tus canchas y precios para empezar a usar el turnero y tomar reservas.",
			cta:       "Cargar mis canchas",
		}, true
	case "connect_mp":
		return nudgeCopy{
			subject:   "Conectá MercadoPago y cobrá la seña online",
			preheader: "Tus clientes pagan la seña al reservar y la plata va a tu cuenta.",
			body: hi + " " + venue + " ya tiene sus canchas en Vibe. Conectá tu cuenta de MercadoPago y tus clientes pagan " +
				"la seña cuando reservan online. La plata va directo a tu cuenta. Si algo te frenó, respondé este mail.",
			cta: "Conectar MercadoPago",
		}, true
	case "inactive":
		return nudgeCopy{
			subject:   "¿Te ayudo a arrancar con Vibe?",
			preheader: "Si algo te frenó, respondé este mail.",
			body: hi + " Vi que hace unos días no entrás a Vibe. Si algo te frenó, respondé este mail y te ayudo a dejar " +
				venue + " andando. Mientras tanto, podés cargar tus turnos de la semana en el turnero.",
			cta: "Ir a mi panel",
		}, true
	case "finish_signup":
		return nudgeCopy{
			subject:   "Tu cuenta de Vibe quedó a medio camino",
			preheader: "Terminá tu cuenta de Vibe. Es gratis para tu complejo.",
			body: hi + " Empezaste a crear tu cuenta en Vibe y no llegaste a terminarla. Vibe es gratis para tu complejo: " +
				"turnero, caja y reservas online con seña. Si algo te frenó, respondé este mail.",
			cta: "Terminar mi cuenta gratis",
		}, true
	}
	return nudgeCopy{}, false
}

// nudgeGreeting addresses the reader by the first name they gave, and says just
// "Hola." when there is none, so an empty name never leaves a gap.
func nudgeGreeting(firstName string) string {
	name := strings.TrimSpace(firstName)
	if name == "" {
		return "Hola."
	}
	return "Hola " + html.EscapeString(name) + "."
}

// listHeaders are the RFC 2369 and RFC 8058 headers that let a mail client offer
// a one-click unsubscribe. An empty URL carries none.
func listHeaders(unsubscribeURL string) map[string]string {
	if unsubscribeURL == "" {
		return nil
	}
	return map[string]string{
		"List-Unsubscribe":      "<" + unsubscribeURL + ">, <mailto:" + supportEmail + "?subject=baja>",
		"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
	}
}

// SendLifecycleNudge emails a nudge. kind picks the copy and is one of the kinds
// the lifecycle package names. ctaURL is the button. unsubscribeURL becomes the
// List-Unsubscribe header and optOutURL the footer link to the /baja page. The
// footer is part of every nudge and of no transactional email.
func (m *Mailer) SendLifecycleNudge(ctx context.Context, to, kind, firstName, complexName, ctaURL, unsubscribeURL, optOutURL string) error {
	c, ok := nudgeCopyFor(kind, firstName, complexName)
	if !ok {
		return permanentErr{msg: fmt.Sprintf("mailer: unknown lifecycle nudge kind %q", kind)}
	}

	content := p(c.body, 16, 16) +
		button(ctaURL, c.cta) +
		fallback(ctaURL) +
		p("Máximo, de Vibe", 16, 24) +
		p("Si no querés recibir más estos mails, "+link(optOutURL, "date de baja acá")+".", 13, 0)

	return m.sendWith(ctx, to, c.subject, m.wrap(c.subject, c.preheader, content), outbound{
		headers: listHeaders(unsubscribeURL),
		replyTo: supportEmail,
	})
}

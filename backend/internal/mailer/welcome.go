package mailer

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// welcomeTutorial is one tutorial the welcome links to: its slug on the
// storefront (PublicURL + /tutoriales/<slug>) and its headline. Both are copied
// from landing/src/data/tutoriales.ts. TestWelcomeTutorialsMatchTheLandingSource
// fails when either drifts from that file, so a renamed tutorial cannot send an
// owner to a 404.
type welcomeTutorial struct {
	slug  string
	title string
}

var welcomeTutorials = []welcomeTutorial{
	{slug: "grilla-de-reservas", title: "La grilla de reservas: todo el día en una pantalla"},
	{slug: "configuracion-del-complejo", title: "Configurá tu complejo: horarios y seña"},
	{slug: "precios-por-horario", title: "Precios por horario: cobrá la noche más cara"},
	{slug: "reportes-de-facturacion", title: "El reporte del mes, listo para el contador"},
}

// welcomeBullets is what Vibe does for the owner, in the order the email lists it.
var welcomeBullets = []string{
	"Ver todos los turnos del día en una sola pantalla.",
	"Cobrar en mostrador y cerrar la caja de cada turno.",
	"Cobrar señas online con MercadoPago.",
	"Tener tu página para que tus clientes reserven.",
	"Ver los reportes del mes, listos para tu contador.",
}

// welcomeSteps are the first three moves, each with the panel page that does it.
var welcomeSteps = []struct {
	text, path, label string
}{
	{"Cargá tu complejo: nombre, dirección y horarios.", "/onboarding", "Cargar mi complejo"},
	{"Sumá tus canchas y sus precios.", "/courts", "Cargar mis canchas"},
	{"Conectá MercadoPago para cobrar señas.", "/settings?tab=billing", "Conectar MercadoPago"},
}

// SendWelcome emails an owner once their account becomes usable: when the
// address is verified, or on a first Google sign-up. It is transactional, so it
// replies to the support address and carries no unsubscribe header or footer.
// Panel links are built on AppURL and tutorial links on PublicURL.
func (m *Mailer) SendWelcome(ctx context.Context, to, firstName string) error {
	subject := "Bienvenido a Vibe"
	if name := strings.TrimSpace(firstName); name != "" {
		subject += ", " + name
	}

	var bullets, steps, tutorials strings.Builder
	for i, b := range welcomeBullets {
		// The last bullet keeps the gap the next heading needs.
		margin := 8
		if i == len(welcomeBullets)-1 {
			margin = 16
		}
		bullets.WriteString(p("• "+b, 16, margin))
	}
	for i, s := range welcomeSteps {
		steps.WriteString(p(fmt.Sprintf("%d. %s %s", i+1, s.text, link(m.appURL+s.path, s.label)), 16, 12))
	}
	for _, t := range welcomeTutorials {
		tutorials.WriteString(p(link(welcomeTutorialURL(m.publicURL, t.slug), t.title), 16, 8))
	}

	dashboard := m.appURL + "/dashboard"
	content := title("Bienvenido a Vibe.") +
		p(greeting(firstName), 16, 12) +
		p("Ya tenés tu cuenta de Vibe. Es gratis para tu complejo y la podés usar desde el celular o la compu.", 16, 16) +
		p("<strong>Con Vibe podés:</strong>", 16, 8) +
		bullets.String() +
		p("<strong>Tus primeros pasos:</strong>", 16, 12) +
		steps.String() +
		button(dashboard, "Ir a mi panel") +
		p("Si el botón no funciona, copiá este link en el navegador:<br>"+link(dashboard, html.EscapeString(dashboard)), 13, 16) +
		p("<strong>Tutoriales para arrancar:</strong>", 16, 8) +
		tutorials.String() +
		p("Si te trabás en algo, respondé este mail y te ayudamos.", 16, 16) +
		p("Máximo, de Vibe", 16, 0)

	const preheader = "Ya tenés tu cuenta de Vibe. Te mostramos cómo arrancar."
	return m.sendWith(ctx, to, subject, m.wrap(subject, preheader, content), outbound{
		replyTo: supportEmail,
	})
}

// welcomeTutorialURL is the storefront page for one tutorial.
func welcomeTutorialURL(publicURL, slug string) string {
	return strings.TrimSuffix(publicURL, "/") + "/tutoriales/" + slug
}

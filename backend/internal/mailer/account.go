package mailer

import (
	"context"
	"fmt"
	"html"
)

// The security notices of an account. Each one reports something that already
// happened, so none carries a link that acts on the account: a reply reaches
// support, and the reset page is linked by its plain address. They have no
// opt-out.

// SendPasswordChanged tells an account its password was just changed.
func (m *Mailer) SendPasswordChanged(ctx context.Context, to, firstName string) error {
	const subject = "Cambiaste tu contraseña · Vibe"
	forgot := m.appURL + "/forgot-password"

	content := title("Cambiaste tu contraseña.") +
		p(greeting(firstName)+" La contraseña de tu cuenta de Vibe se cambió recién. Ya podés entrar con la nueva.", 16, 16) +
		button(m.appURL+"/login", "Iniciar sesión") +
		muted(fmt.Sprintf("Si no fuiste vos, %s y respondé este email para que te ayudemos.", link(forgot, "pedí una contraseña nueva")))

	return m.sendWith(ctx, to, subject, m.wrap("Cambiaste tu contraseña", "La contraseña de tu cuenta de Vibe se cambió.", content),
		outbound{replyTo: supportEmail})
}

// SendEmailChanged tells the account's PREVIOUS address that the account now
// answers at newEmail.
func (m *Mailer) SendEmailChanged(ctx context.Context, to, firstName, newEmail string) error {
	const subject = "Cambiaste el email de tu cuenta · Vibe"

	content := title("Cambiaste el email de tu cuenta.") +
		p(fmt.Sprintf("%s Desde ahora tu cuenta de Vibe usa <strong>%s</strong>. A esta dirección no te vamos a escribir más.",
			greeting(firstName), html.EscapeString(newEmail)), 16, 16) +
		button("mailto:"+supportEmail, "Escribinos") +
		muted("Si no fuiste vos, escribinos o respondé este email y te ayudamos a recuperar tu cuenta.")

	return m.sendWith(ctx, to, subject, m.wrap("Cambiaste el email de tu cuenta", "Tu cuenta de Vibe ahora usa otro email.", content),
		outbound{replyTo: supportEmail})
}

// SendAccountDeleted confirms an account was deleted.
func (m *Mailer) SendAccountDeleted(ctx context.Context, to, firstName string) error {
	const subject = "Eliminaste tu cuenta · Vibe"

	content := title("Eliminaste tu cuenta.") +
		p(greeting(firstName)+" Tu cuenta de Vibe y los datos de tu complejo se eliminaron.", 16, 16) +
		p("Gracias por haber usado Vibe. Si algún día querés volver, podés crear una cuenta nueva cuando quieras.", 16, 16) +
		button(m.publicURL, "Ir a Vibe") +
		muted("Si no fuiste vos, respondé este email y lo vemos juntos.")

	return m.sendWith(ctx, to, subject, m.wrap("Eliminaste tu cuenta", "Tu cuenta de Vibe se eliminó.", content),
		outbound{replyTo: supportEmail})
}

// The owner's notices about a complex's connections. Their buttons open the
// panel page that manages the connection.

// SendOwnerMPConnected tells the owner the complex can take payments online.
func (m *Mailer) SendOwnerMPConnected(ctx context.Context, to, complexName string) error {
	subject := fmt.Sprintf("Conectaste MercadoPago · %s", complexName)
	complexEsc := html.EscapeString(complexName)

	content := title("Conectaste MercadoPago.") +
		p(fmt.Sprintf("<strong>%s</strong> ya cobra señas online con tu cuenta de MercadoPago.", complexEsc), 16, 16) +
		button(m.appURL+"/settings?tab=billing", "Ver en el panel") +
		muted("Si no fuiste vos, respondé este email y te ayudamos.")

	return m.sendWith(ctx, to, subject, m.wrap("Conectaste MercadoPago", fmt.Sprintf("%s ya cobra señas online.", complexEsc), content),
		outbound{replyTo: supportEmail})
}

// SendOwnerMPDisconnected tells the owner the complex stopped taking payments
// online.
func (m *Mailer) SendOwnerMPDisconnected(ctx context.Context, to, complexName string) error {
	subject := fmt.Sprintf("Desconectaste MercadoPago · %s", complexName)
	complexEsc := html.EscapeString(complexName)

	content := title("Desconectaste MercadoPago.") +
		p(fmt.Sprintf("<strong>%s</strong> dejó de cobrar señas online. Las reservas siguen entrando y las cobrás en el complejo.", complexEsc), 16, 16) +
		p("Cuando quieras volver a cobrar online, conectalo de nuevo desde el panel.", 16, 16) +
		button(m.appURL+"/settings?tab=billing", "Conectar MercadoPago") +
		muted("Si no fuiste vos, respondé este email y te ayudamos.")

	return m.sendWith(ctx, to, subject, m.wrap("Desconectaste MercadoPago", fmt.Sprintf("%s dejó de cobrar señas online.", complexEsc), content),
		outbound{replyTo: supportEmail})
}

// SendOwnerMPRefreshFailed tells the owner MercadoPago stopped renewing the
// connection, so online payments stop until they reconnect.
func (m *Mailer) SendOwnerMPRefreshFailed(ctx context.Context, to, complexName string) error {
	subject := fmt.Sprintf("Volvé a conectar MercadoPago · %s", complexName)
	complexEsc := html.EscapeString(complexName)

	content := title("Volvé a conectar MercadoPago.") +
		p(fmt.Sprintf("MercadoPago dejó de aceptar la conexión de <strong>%s</strong> con Vibe. Mientras no la renueves, no vas a poder cobrar señas online.", complexEsc), 16, 16) +
		p("Se arregla en un minuto: entrá al panel y conectá MercadoPago de nuevo.", 16, 16) +
		button(m.appURL+"/settings?tab=billing", "Conectar MercadoPago") +
		muted("Si te trabás, respondé este email y te ayudamos.")

	return m.sendWith(ctx, to, subject, m.wrap("Volvé a conectar MercadoPago", "Conectá MercadoPago de nuevo para seguir cobrando señas online.", content),
		outbound{replyTo: supportEmail})
}

// SendOwnerInstagramReconnect tells the owner the complex's Instagram account
// stopped accepting Vibe, so stories pause until they reconnect.
func (m *Mailer) SendOwnerInstagramReconnect(ctx context.Context, to, complexName string) error {
	subject := fmt.Sprintf("Volvé a conectar Instagram · %s", complexName)
	complexEsc := html.EscapeString(complexName)

	content := title("Volvé a conectar Instagram.") +
		p(fmt.Sprintf("Instagram dejó de aceptar la conexión de <strong>%s</strong> con Vibe, así que pausamos las historias automáticas.", complexEsc), 16, 16) +
		p("Se arregla en un minuto: entrá al panel y conectá Instagram de nuevo.", 16, 16) +
		button(m.appURL+"/settings?tab=instagram", "Conectar Instagram") +
		muted("Si te trabás, respondé este email y te ayudamos.")

	return m.sendWith(ctx, to, subject, m.wrap("Volvé a conectar Instagram", "Conectá Instagram de nuevo para seguir publicando historias.", content),
		outbound{replyTo: supportEmail})
}

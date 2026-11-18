package mailer

import (
	"context"
	"fmt"
	"html"
)

// One-off campaign (2026-11). Remove after it has been sent.

const (
	kitSubject   = "Tu kit para redes ya está listo"
	kitPreheader = "Historias y posts con el link y el QR de tu complejo."
	// kitImageAlt describes the post when the client blocks images.
	kitImageAlt = "Post con el link y el QR de tu complejo"
)

// SendSocialKitCampaign emails the one-off social kit announcement to one owner. It
// shows the owner's own "Link de reservas" post, links it and the button to the kit
// in the panel, and carries the same List-Unsubscribe headers and footer as the
// benefits email.
func (m *Mailer) SendSocialKitCampaign(ctx context.Context, to, firstName, imageURL, kitURL, unsubscribeURL, optOutURL string) error {
	content := p(benefitsGreeting(firstName), 16, 12) +
		p("Armamos un kit para que muestres tu complejo en redes. Son historias y posts con el link de tus reservas y un código QR.", 16, 16) +
		kitImage(imageURL, kitURL) +
		p("Tus clientes escanean el código y reservan desde el celular, a cualquier hora.", 16, 16) +
		button(kitURL, "Descargar mi kit") +
		fallback(kitURL) +
		p("Están listos para subir a Instagram, Facebook o WhatsApp. Si querés que te ayude a usarlo, respondé este mail.", 16, 16) +
		p("Máximo, de Vibe", 16, 24) +
		p("Si no querés recibir más estos mails, "+link(optOutURL, "date de baja acá")+".", 13, 0)

	return m.sendWith(ctx, to, kitSubject, m.wrap(kitSubject, kitPreheader, content), outbound{
		headers: listHeaders(unsubscribeURL),
		replyTo: supportEmail,
	})
}

// kitImage renders the owner's post at 280 by 373 px, its 3:4 ratio, linked to the
// kit. The explicit size keeps the layout steady while the image loads or when the
// client blocks it.
func kitImage(imageURL, kitURL string) string {
	return fmt.Sprintf(`<p style="margin:0 0 16px;font-family:%s;"><a href="%s" style="text-decoration:none;font-family:%s;">`+
		`<img src="%s" width="280" height="373" alt="%s" style="display:block;width:280px;height:373px;border:0;border-radius:12px;font-family:%s;font-size:14px;color:%s;">`+
		`</a></p>`,
		emailFont, html.EscapeString(kitURL), emailFont, html.EscapeString(imageURL), kitImageAlt, emailFont, emailInk)
}

package mailer

import (
	"context"
	"html"
)

// SendSocialPublishReport tells the operator how one of Vibe's posts ended:
// its status, where it went live, or the error that stopped it. The subject is
// built by the caller. Each line is plain text, escaped here.
func (m *Mailer) SendSocialPublishReport(ctx context.Context, to, subject string, lines []string) error {
	content := title("Publicación en redes.")
	for _, line := range lines {
		content += p(html.EscapeString(line), 16, 8)
	}
	return m.send(ctx, to, subject, m.wrap("Publicación en redes", html.EscapeString(subject), content))
}

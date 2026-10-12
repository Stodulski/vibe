package social

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// TaskReport is the durable-queue task that sends one report email.
const TaskReport = "social_publish_report"

// ReportMailer sends the report email. The mailer package implements it.
type ReportMailer interface {
	SendSocialPublishReport(ctx context.Context, to, subject string, lines []string) error
}

// Enqueuer records one durable job. The notifications queue satisfies it.
type Enqueuer interface {
	Enqueue(taskType string, payload any, dedupKey string)
}

// ReportPayload is the job payload of one report email.
type ReportPayload struct {
	To      string   `json:"to"`
	Subject string   `json:"subject"`
	Lines   []string `json:"lines"`
}

// QueueReporter reports a finished post by queueing one email. The dedup key
// is the post and its final status, so a redelivered job sends one email.
type QueueReporter struct {
	queue Enqueuer
	to    string
}

// NewQueueReporter returns a reporter that writes to queue and mails to.
func NewQueueReporter(queue Enqueuer, to string) *QueueReporter {
	return &QueueReporter{queue: queue, to: to}
}

// Report queues the report email for r.
func (q *QueueReporter) Report(_ context.Context, r Report) error {
	payload := BuildReport(q.to, r)
	key := jobs.DedupKey(TaskReport, q.to, r.Post.ID+":"+string(r.Post.Status))
	q.queue.Enqueue(TaskReport, payload, key)
	return nil
}

// BuildReport writes the Spanish subject and body for one finished post. The
// lines are plain text, with no exclamation marks and no secrets.
func BuildReport(to string, r Report) ReportPayload {
	label := statusLabel(r.Post.Status)
	lines := []string{
		"Publicación: " + r.Post.ID + ".",
		"Estado: " + label + ".",
	}
	if slices.Contains(r.Post.Networks, NetworkInstagram) {
		lines = append(lines, "Instagram: "+instagramLine(r))
	}
	if slices.Contains(r.Post.Networks, NetworkFacebook) {
		lines = append(lines, "Facebook: "+facebookLine(r.Post))
	}
	if r.Post.Status != StatusPublished && r.Post.LastError != "" {
		lines = append(lines, "Último error: "+r.Post.LastError)
	}
	return ReportPayload{
		To:      to,
		Subject: "Publicación en redes: " + r.Post.ID + " " + label,
		Lines:   lines,
	}
}

func statusLabel(status Status) string {
	switch status {
	case StatusPublished:
		return "publicada"
	case StatusPartial:
		return "publicada parcialmente"
	case StatusFailed:
		return "fallida"
	case StatusScheduled:
		return "programada"
	case StatusPublishing:
		return "en curso"
	case StatusCancelled:
		return "cancelada"
	default:
		return string(status)
	}
}

func instagramLine(r Report) string {
	switch {
	case r.Post.IGMediaID == "":
		return "no publicada."
	case r.IGPermalink == "":
		return "publicada, sin enlace."
	default:
		return r.IGPermalink
	}
}

func facebookLine(post Post) string {
	switch {
	case post.FBPostID == "":
		return "no publicada."
	case post.Kind == TypeReel:
		return "https://www.facebook.com/reel/" + post.FBPostID
	default:
		return "https://www.facebook.com/" + post.FBPostID
	}
}

// DeliverReport returns the queue handler that sends a queued report email.
func DeliverReport(mailer ReportMailer) func(ctx context.Context, raw json.RawMessage) error {
	return func(ctx context.Context, raw json.RawMessage) error {
		var p ReportPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("social: decode report payload: %w", err)
		}
		if err := mailer.SendSocialPublishReport(ctx, p.To, p.Subject, p.Lines); err != nil {
			return fmt.Errorf("social: send report: %w", err)
		}
		return nil
	}
}

// Package leads captures a visitor-provided email that doesn't otherwise
// reach a full signup — right now: someone who started registering but left
// before finishing. It forwards those to the same spreadsheet the landing
// page's mailing-list signup writes to, tagged with a distinct origin so the
// two flows stay distinguishable downstream.
package leads

import (
	"context"
	"net/http"
	"time"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// LeadStore keeps the people who left a registration form. The row is what the
// owner-lifecycle nudge reads, so it outlives a spreadsheet outage.
type LeadStore interface {
	Upsert(ctx context.Context, email, firstName, lastName, phone string) error
}

// Handler records abandoned-registration leads and forwards them to the
// configured webhook.
type Handler struct {
	respond    *httpx.Responder
	httpClient *http.Client
	webhookURL string
	token      string
	store      LeadStore
}

// Config configures the outbound webhook.
type Config struct {
	// WebhookURL is a Google Apps Script Web App /exec URL. Blank disables
	// forwarding: the endpoint still accepts and validates requests, it
	// just drops them — useful for environments without the spreadsheet
	// webhook configured.
	WebhookURL string
	Token      string
}

// NewHandler returns a Handler that keeps each lead in store and posts it to the
// configured webhook.
func NewHandler(respond *httpx.Responder, cfg Config, store LeadStore) *Handler {
	return &Handler{
		respond:    respond,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		webhookURL: cfg.WebhookURL,
		token:      cfg.Token,
		store:      store,
	}
}

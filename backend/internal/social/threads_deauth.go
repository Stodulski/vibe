package social

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/jobs"
)

const (
	threadsDisconnectSubject = "Threads: se desconectó la cuenta"
	deauthMaxBody            = 16 << 10
	reasonUninstalled        = "se quitó la app de Threads"
	reasonDeletionRequested  = "se pidió eliminar los datos desde Meta"
)

// ThreadsDeauthConfig tunes Meta's uninstall and data-deletion callbacks.
type ThreadsDeauthConfig struct {
	// Configured is false when THREADS_APP_ID or THREADS_APP_SECRET is unset.
	Configured bool
	// AppSecret verifies signed_request. It is never logged or returned.
	AppSecret string
	// To receives the notice that the connection was removed.
	To string
	// StatusURL is the page the data-deletion answer points the owner to.
	StatusURL string
}

// ThreadsDeauth answers Meta's uninstall and data-deletion callbacks. Meta
// sends them without a session, so the signed_request is the authentication.
type ThreadsDeauth struct {
	store   ThreadsDisconnecter
	queue   Enqueuer
	cfg     ThreadsDeauthConfig
	respond *httpx.Responder
	logger  *slog.Logger
}

// NewThreadsDeauth returns the uninstall and data-deletion callbacks.
func NewThreadsDeauth(store ThreadsDisconnecter, queue Enqueuer, cfg ThreadsDeauthConfig, respond *httpx.Responder, logger *slog.Logger) *ThreadsDeauth {
	return &ThreadsDeauth{store: store, queue: queue, cfg: cfg, respond: respond, logger: logger}
}

// Uninstall answers the callback sent when the owner removes the app from
// Threads. It removes the connection and answers 200.
func (d *ThreadsDeauth) Uninstall(w http.ResponseWriter, r *http.Request) {
	if !d.disconnect(w, r, reasonUninstalled) {
		return
	}
	d.respond.JSON(w, r, http.StatusOK, httpx.Envelope{})
}

// Delete answers the data-deletion callback. It removes the connection, then
// answers with the status page and the confirmation code Meta shows the owner.
func (d *ThreadsDeauth) Delete(w http.ResponseWriter, r *http.Request) {
	if !d.disconnect(w, r, reasonDeletionRequested) {
		return
	}
	code, err := newConfirmationCode()
	if err != nil {
		d.respond.ServerError(w, r, err)
		return
	}
	d.logger.Info("social: threads data deletion confirmed", "confirmation_code", code)
	d.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"url": d.cfg.StatusURL, "confirmation_code": code})
}

// disconnect verifies the signed request and removes the connection it names.
// A valid request for another account removes nothing and still succeeds, so
// Meta does not retry it. It reports false after writing the refusal itself.
func (d *ThreadsDeauth) disconnect(w http.ResponseWriter, r *http.Request, reason string) bool {
	if !d.cfg.Configured {
		d.respond.Refuse(w, r, httpx.NotImplemented("threads is not configured: THREADS_APP_ID and THREADS_APP_SECRET must be set"))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, deauthMaxBody)
	if err := r.ParseForm(); err != nil {
		d.respond.BadRequest(w, r, err)
		return false
	}
	payload, err := ParseSignedRequest(r.PostForm.Get("signed_request"), d.cfg.AppSecret)
	if err != nil {
		d.logger.Warn("social: threads callback refused", "reason", reason, "error", err)
		d.respond.Refuse(w, r, httpx.BadRequest("signed_request is not signed by this app"))
		return false
	}
	username, removed, err := d.store.DisconnectThreads(r.Context(), payload.UserID.String())
	if err != nil {
		d.respond.ServerError(w, r, err)
		return false
	}
	if removed {
		d.logger.Info("social: threads disconnected by meta callback", "reason", reason)
		d.queue.Enqueue(TaskReport, ReportPayload{
			To:      d.cfg.To,
			Subject: threadsDisconnectSubject,
			Lines: []string{
				"La cuenta @" + username + " dejó de estar conectada a Vibe.",
				"Motivo: " + reason + ".",
				"Para volver a conectarla, hay que autorizar la cuenta de nuevo desde el panel.",
			},
		}, jobs.DedupKey(TaskReport, d.cfg.To, "threads-disconnect:"+payload.UserID.String()+":"+strconv.FormatInt(payload.IssuedAt, 10)))
	}
	return true
}

func newConfirmationCode() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("social: confirmation code: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

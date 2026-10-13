package social

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/stodulski/vibe-server/internal/jobs"
)

// The Threads token rules. Threads refuses to refresh a token younger than a
// day, a refreshed token lasts 60 days, and Meta asks for a new authorization
// about 90 days after the connection, so the owner is asked 14 days early.
const (
	threadsRefreshAfter  = 24 * time.Hour
	threadsRefreshWithin = 15 * 24 * time.Hour
	threadsConsentLife   = 90 * 24 * time.Hour
	threadsRemindBefore  = 14 * 24 * time.Hour
)

// ThreadsOutcome is what one maintenance run did, for the log.
type ThreadsOutcome string

// The outcomes of one run.
const (
	ThreadsNoConnection  ThreadsOutcome = "no_connection"
	ThreadsKept          ThreadsOutcome = "kept"
	ThreadsRefreshed     ThreadsOutcome = "refreshed"
	ThreadsRefreshFailed ThreadsOutcome = "refresh_failed"
	ThreadsReminded      ThreadsOutcome = "reminded"
)

// ThreadsMaintenanceConfig tunes the daily job.
type ThreadsMaintenanceConfig struct {
	// To receives the reconnect email.
	To string
	// ConnectURL is the link the owner opens to reconnect.
	ConnectURL string
	Now        func() time.Time
}

// ThreadsMaintenance refreshes the Threads token when it is due and asks the
// owner to reconnect when it cannot be refreshed or the consent is near its end.
type ThreadsMaintenance struct {
	store  ThreadsStore
	api    ThreadsAPI
	queue  Enqueuer
	cfg    ThreadsMaintenanceConfig
	logger *slog.Logger
}

// NewThreadsMaintenance returns the job.
func NewThreadsMaintenance(store ThreadsStore, api ThreadsAPI, queue Enqueuer, cfg ThreadsMaintenanceConfig, logger *slog.Logger) *ThreadsMaintenance {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &ThreadsMaintenance{store: store, api: api, queue: queue, cfg: cfg, logger: logger}
}

// Run does one pass. A refresh failure is not an error of the job: it becomes
// the reconnect email, so the returned error is only for a store failure.
func (m *ThreadsMaintenance) Run(ctx context.Context) (ThreadsOutcome, error) {
	conn, ok, err := m.store.LoadThreads(ctx)
	if err != nil {
		return "", fmt.Errorf("social: load threads connection: %w", err)
	}
	if !ok {
		return ThreadsNoConnection, nil
	}

	now := m.cfg.Now()
	if refreshDue(conn, now) {
		tok, err := m.api.Refresh(ctx, conn.AccessToken)
		if err != nil {
			m.logger.Error("social: threads token refresh failed", "error", err)
			m.remind(conn, "el token no se pudo renovar")
			return ThreadsRefreshFailed, nil
		}
		conn.AccessToken = tok.AccessToken
		conn.RefreshedAt = now
		conn.ExpiresAt = now.Add(tok.ExpiresIn)
		if err := m.store.SaveThreads(ctx, conn); err != nil {
			return "", fmt.Errorf("social: store refreshed threads token: %w", err)
		}
		m.logger.Info("social: threads token refreshed", "expires_at", conn.ExpiresAt)
		return ThreadsRefreshed, nil
	}

	if consentNeedsOwner(conn, now) {
		m.remind(conn, "Meta pide volver a autorizar la cuenta dentro de unos días")
		return ThreadsReminded, nil
	}
	return ThreadsKept, nil
}

// refreshDue is true when the token is at least a day old and expires within
// fifteen days.
func refreshDue(conn ThreadsConnection, now time.Time) bool {
	return now.Sub(conn.RefreshedAt) >= threadsRefreshAfter && conn.ExpiresAt.Sub(now) <= threadsRefreshWithin
}

// consentNeedsOwner is true when the owner must authorize again: the consent is
// about to run out, or the token has already expired.
func consentNeedsOwner(conn ThreadsConnection, now time.Time) bool {
	return !now.Before(conn.ConnectedAt.Add(threadsConsentLife-threadsRemindBefore)) || !now.Before(conn.ExpiresAt)
}

// remind queues the reconnect email. The dedup key is the connection itself, so
// a connection produces one email however many runs find it in the same state.
func (m *ThreadsMaintenance) remind(conn ThreadsConnection, reason string) {
	lines := []string{
		"La cuenta de Threads @" + conn.Username + " necesita reconectarse: " + reason + ".",
		"Iniciá sesión como superadmin en la app y abrí este enlace: " + m.cfg.ConnectURL,
	}
	key := jobs.DedupKey(TaskThreadsReconnect, m.cfg.To, conn.ConnectedAt.UTC().Format(time.RFC3339))
	m.queue.Enqueue(TaskThreadsReconnect, ReportPayload{
		To:      m.cfg.To,
		Subject: "Threads: hay que reconectar la cuenta",
		Lines:   lines,
	}, key)
}

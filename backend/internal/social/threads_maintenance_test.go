package social

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

const connectURL = "https://api.vibe.test/api/v1/social/threads/connect"

func runMaintenance(t *testing.T, store *fakeThreadsStore, api *fakeThreadsAPI, q *fakeEnqueuer) ThreadsOutcome {
	t.Helper()
	m := NewThreadsMaintenance(store, api, q, ThreadsMaintenanceConfig{
		To:         "owner@vibe.test",
		ConnectURL: connectURL,
		Now:        func() time.Time { return t0 },
	}, slog.New(slog.DiscardHandler))
	out, err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return out
}

func storedConn(connectedAgo, refreshedAgo, expiresIn time.Duration) *fakeThreadsStore {
	return &fakeThreadsStore{ok: true, conn: ThreadsConnection{
		UserID:      "42",
		Username:    "vibe.padel",
		AccessToken: "long-old",
		ConnectedAt: t0.Add(-connectedAgo),
		RefreshedAt: t0.Add(-refreshedAgo),
		ExpiresAt:   t0.Add(expiresIn),
	}}
}

func TestMaintenanceDoesNothingWithoutAConnection(t *testing.T) {
	t.Parallel()
	api := &fakeThreadsAPI{}
	q := &fakeEnqueuer{}
	if out := runMaintenance(t, &fakeThreadsStore{}, api, q); out != ThreadsNoConnection {
		t.Fatalf("outcome = %q; want no_connection", out)
	}
	if api.refreshedWith != "" || len(q.calls) != 0 {
		t.Fatal("no connection means no refresh and no email")
	}
}

func TestMaintenanceRefreshesADayOldTokenThatExpiresWithinFifteenDays(t *testing.T) {
	t.Parallel()
	store := storedConn(30*24*time.Hour, 25*time.Hour, 10*24*time.Hour)
	api := &fakeThreadsAPI{refreshToken: "long-new"}
	q := &fakeEnqueuer{}

	if out := runMaintenance(t, store, api, q); out != ThreadsRefreshed {
		t.Fatalf("outcome = %q; want refreshed", out)
	}
	if api.refreshedWith != "long-old" {
		t.Fatalf("refreshed %q; want the stored token", api.refreshedWith)
	}
	got := store.conn
	if got.AccessToken != "long-new" || !got.RefreshedAt.Equal(t0) || !got.ExpiresAt.Equal(t0.Add(sixtyDays)) {
		t.Fatalf("stored = %+v; want the new token issued now", got)
	}
	if !got.ConnectedAt.Equal(t0.Add(-30 * 24 * time.Hour)) {
		t.Fatal("a refresh must not move the connection date")
	}
	if len(q.calls) != 0 {
		t.Fatalf("a successful refresh sends no email; got %d", len(q.calls))
	}
}

func TestMaintenanceKeepsAFreshTokenUntouched(t *testing.T) {
	t.Parallel()
	store := storedConn(10*24*time.Hour, 25*time.Hour, 40*24*time.Hour)
	api := &fakeThreadsAPI{}
	if out := runMaintenance(t, store, api, &fakeEnqueuer{}); out != ThreadsKept {
		t.Fatalf("outcome = %q; want kept", out)
	}
	if api.refreshedWith != "" {
		t.Fatal("a token with 40 days left is not refreshed yet")
	}
}

func TestMaintenanceWaitsADayBeforeRefreshing(t *testing.T) {
	t.Parallel()
	store := storedConn(10*24*time.Hour, 2*time.Hour, 10*24*time.Hour)
	api := &fakeThreadsAPI{}
	if out := runMaintenance(t, store, api, &fakeEnqueuer{}); out != ThreadsKept {
		t.Fatalf("outcome = %q; want kept", out)
	}
	if api.refreshedWith != "" {
		t.Fatal("Threads refuses to refresh a token younger than a day")
	}
}

func TestMaintenanceQueuesOneReconnectEmailWhenRefreshFails(t *testing.T) {
	t.Parallel()
	store := storedConn(30*24*time.Hour, 25*time.Hour, 10*24*time.Hour)
	api := &fakeThreadsAPI{refreshErr: errors.New("meta answered 400 (code 190)")}
	q := &fakeEnqueuer{}

	if out := runMaintenance(t, store, api, q); out != ThreadsRefreshFailed {
		t.Fatalf("outcome = %q; want refresh_failed", out)
	}
	if store.saves != 0 {
		t.Fatal("a failed refresh must not overwrite the stored token")
	}
	if len(q.calls) != 1 || q.calls[0].taskType != TaskThreadsReconnect {
		t.Fatalf("calls = %+v; want one reconnect email", q.calls)
	}
	first := q.calls[0]

	runMaintenance(t, store, api, q)
	if len(q.calls) != 2 || q.calls[1].key != first.key {
		t.Fatalf("a second run must reuse the dedup key so the queue sends one email")
	}
}

func TestMaintenanceReminderNamesTheConnectLinkAndNoToken(t *testing.T) {
	t.Parallel()
	store := storedConn(77*24*time.Hour, time.Hour, 30*24*time.Hour)
	api := &fakeThreadsAPI{}
	q := &fakeEnqueuer{}

	if out := runMaintenance(t, store, api, q); out != ThreadsReminded {
		t.Fatalf("outcome = %q; want reminded", out)
	}
	if len(q.calls) != 1 {
		t.Fatalf("calls = %d; want one reminder", len(q.calls))
	}
	p := q.calls[0].payload
	if p.To != "owner@vibe.test" || p.Subject != "Threads: hay que reconectar la cuenta" {
		t.Fatalf("payload = %+v", p)
	}
	body := strings.Join(p.Lines, "\n")
	if !strings.Contains(body, connectURL) {
		t.Fatalf("the reminder must carry the connect link; got %q", body)
	}
	if strings.Contains(body, "long-old") {
		t.Fatal("the reminder must never carry a token")
	}
}

func TestMaintenanceRemindsBeforeTheConsentWindowAndNotBefore(t *testing.T) {
	t.Parallel()
	store := storedConn(60*24*time.Hour, time.Hour, 30*24*time.Hour)
	q := &fakeEnqueuer{}
	if out := runMaintenance(t, store, &fakeThreadsAPI{}, q); out != ThreadsKept || len(q.calls) != 0 {
		t.Fatalf("outcome = %q with %d emails; want kept and none at day 60", out, len(q.calls))
	}
}

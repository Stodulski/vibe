package complexes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

// refreshWorld is the state the refresh routine reads and writes: one venue's
// stored credentials, the lock between instances, and a MercadoPago that
// rotates the refresh token on every use. Each double is the real thing's
// contract in miniature, so a race the real thing would lose is lost here too.
type refreshWorld struct {
	f       *fixture
	id      uuid.UUID
	store   *credentialStore
	locks   *memoryLocks
	mercado *rotatingMP
}

func newRefreshWorld(t *testing.T) *refreshWorld {
	t.Helper()

	f := newFixture(t)
	w := &refreshWorld{f: f, id: uuid.New(), locks: &memoryLocks{held: map[string]bool{}}}
	w.store = &credentialStore{stubStore: f.store, id: w.id, access: "access-0", refresh: "refresh-0"}
	w.mercado = &rotatingMP{store: w.store}

	f.service.venues = w.store
	f.service.credentials = w.store
	f.service.oauth = w.mercado
	f.service.locks = w.locks
	f.service.refreshLockPoll = time.Millisecond
	f.service.refreshLockWait = 2 * time.Second
	return w
}

// credentialStore keeps one venue's tokens the way the database does: a read
// returns what the last write left.
type credentialStore struct {
	*stubStore
	id uuid.UUID

	mu        sync.Mutex
	access    string
	refresh   string
	updateErr error
	writes    int
}

func (c *credentialStore) GetByID(_ context.Context, id uuid.UUID) (*complexstore.Complex, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	access, refresh := c.access, c.refresh
	complex := complexstore.NewComplexForTest(id, &access, &refresh)
	complex.Name = "Vibe"
	return complex, nil
}

func (c *credentialStore) UpdateMPCredentials(_ context.Context, _ uuid.UUID, access, refresh, _ string, _ int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.updateErr != nil {
		return c.updateErr
	}
	c.access, c.refresh = access, refresh
	c.writes++
	return nil
}

func (c *credentialStore) tokens() (access, refresh string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.access, c.refresh
}

// rotatingMP issues a new pair on every refresh and kills the old refresh
// token, which is what makes two uncoordinated refreshes fail.
type rotatingMP struct {
	store *credentialStore
	delay time.Duration
	err   error

	mu     sync.Mutex
	calls  int
	issued int
	spent  map[string]bool
}

func (m *rotatingMP) RefreshOAuthToken(_ context.Context, refreshToken string) (*mp.OAuthTokens, error) {
	m.mu.Lock()
	m.calls++
	m.issued++
	n := m.issued
	if m.spent == nil {
		m.spent = map[string]bool{}
	}
	spent := m.spent[refreshToken]
	m.spent[refreshToken] = true
	m.mu.Unlock()

	time.Sleep(m.delay)

	if m.err != nil {
		return nil, m.err
	}
	if spent {
		return nil, &mp.APIError{StatusCode: http.StatusBadRequest, Body: `{"error":"invalid_grant"}`}
	}
	return &mp.OAuthTokens{
		AccessToken:  fmt.Sprintf("access-%d", n),
		RefreshToken: fmt.Sprintf("refresh-%d", n),
		UserID:       42,
		ExpiresIn:    3600,
	}, nil
}

func (m *rotatingMP) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// memoryLocks is data.LockStore's contract without Postgres: TryAdvisory takes
// a key only when nobody holds it, and release frees it.
type memoryLocks struct {
	mu   sync.Mutex
	held map[string]bool
	// never refuses every attempt, as a lease held by another instance would.
	never bool
	err   error
	keys  []string
}

func (l *memoryLocks) TryAdvisory(_ context.Context, key string) (bool, func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.keys = append(l.keys, key)
	if l.err != nil {
		return false, func() {}, l.err
	}
	if l.never || l.held[key] {
		return false, func() {}, nil
	}
	l.held[key] = true
	return true, func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		delete(l.held, key)
	}, nil
}

// sentryMessages binds a Sentry client that records what it is handed.
func sentryMessages(t *testing.T) func() []string {
	t.Helper()

	tr := &recordingTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Dsn: "", Transport: tr})
	if err != nil {
		t.Fatalf("building a test sentry client: %v", err)
	}
	hub := sentry.CurrentHub()
	previous := hub.Client()
	hub.BindClient(client)
	t.Cleanup(func() { hub.BindClient(previous) })

	return func() []string {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		out := make([]string, 0, len(tr.events))
		for _, e := range tr.events {
			out = append(out, e.Message)
		}
		return out
	}
}

type recordingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (r *recordingTransport) Flush(time.Duration) bool              { return true }
func (r *recordingTransport) FlushWithContext(context.Context) bool { return true }
func (r *recordingTransport) Configure(sentry.ClientOptions)        {}
func (r *recordingTransport) Close()                                {}
func (r *recordingTransport) SendEvent(event *sentry.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

// Two callers that both found access-0 rejected must spend the refresh token
// once between them. The second one, uncoordinated, would have presented a
// refresh token MercadoPago already retired.
func TestConcurrentRefreshesOfOneVenueCallMercadoPagoOnce(t *testing.T) {
	w := newRefreshWorld(t)
	w.mercado.delay = 20 * time.Millisecond

	const callers = 6
	tokens := make([]string, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			tokens[i], errs[i] = w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0")
		})
	}
	wg.Wait()

	if got := w.mercado.callCount(); got != 1 {
		t.Fatalf("want MercadoPago called once; got %d", got)
	}
	for i := range callers {
		if errs[i] != nil {
			t.Errorf("caller %d: %v", i, errs[i])
		}
		if tokens[i] != "access-1" {
			t.Errorf("caller %d: want the one refreshed token access-1; got %q", i, tokens[i])
		}
	}
	if access, refresh := w.store.tokens(); access != "access-1" || refresh != "refresh-1" {
		t.Errorf("want the single refresh stored; got access=%q refresh=%q", access, refresh)
	}
	for _, key := range w.locks.keys {
		if key != "mp_refresh:"+w.id.String() {
			t.Errorf("the lock must be keyed on the venue; got %q", key)
		}
	}
}

// A caller that arrives after somebody else refreshed gets what is stored and
// MercadoPago is not asked again.
func TestAStaleCallerGetsTheAlreadyRefreshedToken(t *testing.T) {
	w := newRefreshWorld(t)
	w.store.access, w.store.refresh = "access-5", "refresh-5"

	got, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "access-5" {
		t.Errorf("want the stored token; got %q", got)
	}
	if calls := w.mercado.callCount(); calls != 0 {
		t.Errorf("MercadoPago must not be called when the token was already renewed; got %d call(s)", calls)
	}
	if w.store.writes != 0 {
		t.Errorf("nothing may be written; got %d write(s)", w.store.writes)
	}
}

// A caller that cannot name the token it saw (the sweep, when the stored
// access token will not open) refreshes unconditionally.
func TestAnEmptyStaleTokenAlwaysRefreshes(t *testing.T) {
	w := newRefreshWorld(t)

	got, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "")

	if err != nil || got != "access-1" {
		t.Fatalf("want a refresh to access-1; got %q, %v", got, err)
	}
}

// The lock is released when the refresh ends, success or not, so the next
// caller is not left waiting out a lease.
func TestTheRefreshLockIsReleased(t *testing.T) {
	w := newRefreshWorld(t)
	w.mercado.err = errors.New("mercadopago down")

	if _, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0"); err == nil {
		t.Fatal("want the MercadoPago failure")
	}
	if len(w.locks.held) != 0 {
		t.Errorf("the lock must be released after a failed refresh; still held: %v", w.locks.held)
	}
}

// MercadoPago issued new tokens and the write failed: the stored refresh token
// is dead, so this must be an error, an error-level log and a Sentry event that
// names the venue — and the access token still comes back for the request in
// hand.
func TestARefreshThatCannotBeStoredIsAnErrorAndAnAlert(t *testing.T) {
	w := newRefreshWorld(t)
	w.store.updateErr = errors.New("database unavailable")
	messages := sentryMessages(t)

	got, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0")

	if !errors.Is(err, mpcred.ErrMPRefreshNotPersisted) {
		t.Fatalf("want ErrMPRefreshNotPersisted; got %v", err)
	}
	if got != "access-1" {
		t.Errorf("the new access token must still be returned; got %q", got)
	}
	var alert string
	for _, m := range messages() {
		if strings.Contains(m, "refreshed-credential persist FAILED") {
			alert = m
		}
	}
	if alert == "" {
		t.Fatalf("an unstorable refresh must alert Sentry; captured %q", messages())
	}
	if !strings.Contains(alert, w.id.String()) {
		t.Errorf("the alert must name the venue; got %q", alert)
	}
	if logs := w.f.logs.String(); !strings.Contains(logs, "level=ERROR") {
		t.Errorf("want an error-level log; got %q", logs)
	}
}

// The persist must survive the caller going away, or the tokens MercadoPago
// just issued are lost with the request that asked for them.
func TestTheRefreshIsStoredEvenIfTheCallerHasGone(t *testing.T) {
	w := newRefreshWorld(t)
	ctx, cancel := context.WithCancel(t.Context())
	w.mercado.delay = 10 * time.Millisecond
	go func() {
		time.Sleep(2 * time.Millisecond)
		cancel()
	}()

	if _, err := w.f.service.RefreshMPCredentials(ctx, w.id, "access-0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if access, _ := w.store.tokens(); access != "access-1" {
		t.Errorf("the refreshed token must be stored even though the caller's context ended; got %q", access)
	}
}

// invalid_grant is a final answer from MercadoPago; callers recognize it
// through IsMPRefreshRejected and still reach the underlying APIError.
func TestARejectedRefreshTokenIsRecognizable(t *testing.T) {
	w := newRefreshWorld(t)
	w.mercado.err = &mp.APIError{StatusCode: http.StatusBadRequest, Body: `{"error":"invalid_grant"}`}

	_, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0")

	if !IsMPRefreshRejected(err) {
		t.Fatalf("want a 4xx recognized as a rejection; got %v", err)
	}
	var apiErr *mp.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("want the APIError reachable through the wrap; got %v", err)
	}
	if IsMPRefreshRejected(&mp.APIError{StatusCode: http.StatusBadGateway}) {
		t.Error("a 5xx is an outage, not a rejection")
	}
}

func TestARefreshGivesUpWhenTheLockStaysTaken(t *testing.T) {
	w := newRefreshWorld(t)
	w.locks.never = true
	w.f.service.refreshLockWait = 20 * time.Millisecond

	_, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0")

	if !errors.Is(err, ErrMPRefreshBusy) {
		t.Fatalf("want ErrMPRefreshBusy; got %v", err)
	}
	if calls := w.mercado.callCount(); calls != 0 {
		t.Errorf("nothing may be refreshed without the lock; got %d call(s)", calls)
	}
}

func TestARefreshStopsWaitingWhenTheCallerIsGone(t *testing.T) {
	w := newRefreshWorld(t)
	w.locks.never = true
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := w.f.service.RefreshMPCredentials(ctx, w.id, "access-0")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled; got %v", err)
	}
}

func TestARefreshFailsClosedWhenTheLockCannotBeTaken(t *testing.T) {
	w := newRefreshWorld(t)
	w.locks.err = errors.New("database unavailable")

	_, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0")

	if err == nil {
		t.Fatal("want an error")
	}
	if calls := w.mercado.callCount(); calls != 0 {
		t.Errorf("nothing may be refreshed without the lock; got %d call(s)", calls)
	}
}

// ---------------------------------------------------------------------------
// The sweep
// ---------------------------------------------------------------------------

// The sweep refreshes what it lists, through the same routine, and stores it.
func TestTheSweepRefreshesThroughTheSerializedRoutine(t *testing.T) {
	w := newRefreshWorld(t)
	access, refresh := w.store.tokens()
	w.f.store.needingRefresh = []*complexstore.Complex{complexstore.NewComplexForTest(w.id, &access, &refresh)}

	w.f.service.RefreshMPTokens(t.Context())

	if got, _ := w.store.tokens(); got != "access-1" {
		t.Errorf("want the sweep to store the refreshed token; got %q", got)
	}
	if len(w.locks.keys) != 1 {
		t.Errorf("the sweep must take the refresh lock; keys %v", w.locks.keys)
	}
	if logs := w.f.logs.String(); !strings.Contains(logs, "cron_refresh_mp_tokens: refreshed token") {
		t.Errorf("want the refresh reported; got %q", logs)
	}
}

// The listing the sweep works from is older than the lock: a checkout that
// refreshed in between leaves the sweep with a stale row, and the sweep must
// neither spend the dead refresh token nor overwrite the newer pair.
func TestTheSweepDoesNotRefreshAVenueCheckoutAlreadyRefreshed(t *testing.T) {
	w := newRefreshWorld(t)
	oldAccess, oldRefresh := "access-0", "refresh-0"
	w.f.store.needingRefresh = []*complexstore.Complex{complexstore.NewComplexForTest(w.id, &oldAccess, &oldRefresh)}
	// Checkout got there first.
	if _, err := w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	messages := sentryMessages(t)

	w.f.service.RefreshMPTokens(t.Context())

	if calls := w.mercado.callCount(); calls != 1 {
		t.Errorf("only checkout's refresh may reach MercadoPago; got %d call(s)", calls)
	}
	if access, refresh := w.store.tokens(); access != "access-1" || refresh != "refresh-1" {
		t.Errorf("the newer pair must survive the sweep; got access=%q refresh=%q", access, refresh)
	}
	if got := messages(); len(got) != 0 {
		t.Errorf("a venue that was already refreshed is not a failure; captured %q", got)
	}
}

func TestTheSweepAlertsWhenMercadoPagoFails(t *testing.T) {
	w := newRefreshWorld(t)
	access, refresh := w.store.tokens()
	w.f.store.needingRefresh = []*complexstore.Complex{complexstore.NewComplexForTest(w.id, &access, &refresh)}
	w.mercado.err = &mp.APIError{StatusCode: http.StatusInternalServerError, Body: "boom"}
	messages := sentryMessages(t)

	w.f.service.RefreshMPTokens(t.Context())

	var sawFailure, sawSummary bool
	for _, m := range messages() {
		sawFailure = sawFailure || strings.Contains(m, "MP OAuth refresh FAILED")
		sawSummary = sawSummary || strings.Contains(m, "1/1 complexes failed to refresh")
	}
	if !sawFailure || !sawSummary {
		t.Errorf("want the per-venue alert and the run summary; captured %q", messages())
	}
}

// The sweep counts an unstorable refresh as a failure and does not alert twice
// for it: the routine already did.
func TestTheSweepAlertsOnceForAnUnstorableRefresh(t *testing.T) {
	w := newRefreshWorld(t)
	access, refresh := w.store.tokens()
	w.f.store.needingRefresh = []*complexstore.Complex{complexstore.NewComplexForTest(w.id, &access, &refresh)}
	w.store.updateErr = errors.New("database unavailable")
	messages := sentryMessages(t)

	w.f.service.RefreshMPTokens(t.Context())

	persistAlerts := 0
	for _, m := range messages() {
		if strings.Contains(m, "persist FAILED") {
			persistAlerts++
		}
		if strings.Contains(m, "MP OAuth refresh FAILED") {
			t.Errorf("the persist failure must not be reported a second time as a refresh failure: %q", m)
		}
	}
	if persistAlerts != 1 {
		t.Errorf("want exactly one persist alert; got %d in %q", persistAlerts, messages())
	}
}

// A venue with no refresh token is skipped, not failed.
func TestTheSweepSkipsAVenueWithNoRefreshToken(t *testing.T) {
	w := newRefreshWorld(t)
	w.store.refresh = ""
	access, empty := "access-0", ""
	w.f.store.needingRefresh = []*complexstore.Complex{complexstore.NewComplexForTest(w.id, &access, &empty)}
	messages := sentryMessages(t)

	w.f.service.RefreshMPTokens(t.Context())

	if calls := w.mercado.callCount(); calls != 0 {
		t.Errorf("nothing to refresh with; got %d call(s)", calls)
	}
	if got := messages(); len(got) != 0 {
		t.Errorf("a skipped venue must not alert; captured %q", got)
	}
}

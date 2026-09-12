package bookings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

type stubStore struct {
	booking   *data.Booking
	list      []*data.Booking
	getErr    error
	listErr   error
	insertErr error

	// onUpdate runs inside Update, so a test can put an event — a client
	// disconnecting, in particular — between two steps of a handler.
	onUpdate func()

	inserted []*data.Booking
	updated  []*data.Booking
}

func (s *stubStore) GetByComplex(context.Context, uuid.UUID, time.Time, time.Time, data.Filters) ([]*data.Booking, data.Metadata, error) {
	return s.list, data.Metadata{}, s.listErr
}

func (s *stubStore) GetByID(context.Context, uuid.UUID) (*data.Booking, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.booking == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.booking, nil
}

func (s *stubStore) InsertSafe(_ context.Context, b *data.Booking) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	b.ID = uuid.New()
	// The real InsertSafe returns lower(span) and upper(span) and writes them
	// onto the booking (internal/data/bookings.go), so every caller downstream
	// of an insert holds the two instants. The confirmation copy is rendered
	// from them, so a stub that left them zero would make a handler look like
	// it had nothing to say about the hours it just sold.
	b.StartsAt = slots.At(timezone.Day(b.Date), b.StartTime)
	b.EndsAt = b.StartsAt.Add(time.Duration(b.DurationMinutes) * time.Minute)
	// The real InsertSafe mints a booking link token inside the same
	// transaction (internal/data/bookings.go) and sets it here; mirror that so
	// a test asserting on booking.LinkToken sees the same shape production
	// does.
	b.LinkToken = "stub-link-token-" + b.ID.String()
	s.inserted = append(s.inserted, b)
	return nil
}

func (s *stubStore) Update(_ context.Context, b *data.Booking) error {
	if s.onUpdate != nil {
		s.onUpdate()
	}
	// A shallow copy, not the caller's own pointer: the real store writes
	// whatever the caller held at the moment of the call, and every handler
	// under test keeps mutating its own *data.Booking afterward (clearing
	// RefundIntentAt beside paymentStatusAfter, among others). Recording the
	// live pointer would let a later in-memory mutation silently overwrite
	// what a test believes it already asserted on — the exact
	// mutation-testing false negative this harness must not produce.
	snapshot := *b
	s.updated = append(s.updated, &snapshot)
	return nil
}

type stubClients struct {
	client               *data.Client
	err                  error
	created              []string
	noShows              []uuid.UUID
	updated              []*data.Client
	allowNameUpdateCalls []bool
}

func (s *stubClients) GetByID(context.Context, uuid.UUID) (*data.Client, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.client == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.client, nil
}

// allowNameUpdate is recorded rather than acted on: the stub always hands
// back whatever s.client already holds, and it is the real ClientModel's SQL
// (internal/data/clients.go) that the R1-client-name-overwrite fix actually
// lives in. Recording it here is what lets a test confirm which value its
// caller passed — the whole point of the fix is that create.go and public.go
// must not pass the same one.
func (s *stubClients) GetOrCreate(_ context.Context, _ uuid.UUID, firstName, _, phone, _ string, allowNameUpdate bool) (*data.Client, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.created = append(s.created, phone)
	s.allowNameUpdateCalls = append(s.allowNameUpdateCalls, allowNameUpdate)
	if s.client != nil {
		return s.client, nil
	}
	return &data.Client{ID: uuid.New(), FirstName: firstName, Phone: phone}, nil
}

func (s *stubClients) Update(_ context.Context, c *data.Client) error {
	s.updated = append(s.updated, c)
	return nil
}

func (s *stubClients) IncrementNoShows(_ context.Context, id uuid.UUID) error {
	s.noShows = append(s.noShows, id)
	return nil
}

type stubComplexes struct {
	complex   *data.Complex
	schedules []*data.Schedule
	err       error
	// credentialsErr fails UpdateMPCredentials, standing in for the database
	// refusing the write that stores a freshly refreshed MercadoPago token.
	// MercadoPago rotates the refresh token on use, so a refused write leaves
	// the stored one dead — a stub that always persisted is how a silent
	// failure of that write would ship green.
	credentialsErr error

	// credentials records every persisted refresh, so a test can assert the
	// write was attempted rather than trusting the caller reached it.
	credentials []mpCredentialUpdate
}

// mpCredentialUpdate is one attempt to store a refreshed MercadoPago credential.
type mpCredentialUpdate struct {
	complexID    uuid.UUID
	accessToken  string
	refreshToken string
	mpUserID     string
	expiresIn    int
}

func (s *stubComplexes) GetByID(context.Context, uuid.UUID) (*data.Complex, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.complex == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.complex, nil
}

func (s *stubComplexes) GetSchedules(context.Context, uuid.UUID) ([]*data.Schedule, error) {
	return s.schedules, nil
}

func (s *stubComplexes) UpdateMPCredentials(_ context.Context, complexID uuid.UUID, accessToken, refreshToken, mpUserID string, expiresIn int) error {
	s.credentials = append(s.credentials, mpCredentialUpdate{
		complexID:    complexID,
		accessToken:  accessToken,
		refreshToken: refreshToken,
		mpUserID:     mpUserID,
		expiresIn:    expiresIn,
	})
	if s.credentialsErr != nil {
		return s.credentialsErr
	}
	return nil
}

type stubCourts struct {
	court  *data.Court
	prices []*data.CourtPrice
	// blocked is what the owner has taken off sale, keyed by date so a stub
	// that returns the same list for every day cannot make a test pass on a
	// date the block does not cover.
	blocked map[string][]*data.BlockedSlot
	// blockedErr fails the read, standing in for the database being
	// unavailable while a write path is deciding whether the hours are on
	// sale. A read that cannot fail cannot prove the caller refuses rather
	// than assuming "not blocked".
	blockedErr error
	// blockedQueries records every lookup, so a test can assert the write path
	// actually asked rather than trusting that it reached the line.
	blockedQueries []blockedQuery
}

// blockedQuery is one call to GetBlockedSlots, with the arguments it was made
// with. Discarding them is how a check against the wrong court or the wrong
// date would ship green.
type blockedQuery struct {
	courtID uuid.UUID
	date    string
}

func (s *stubCourts) GetByID(context.Context, uuid.UUID) (*data.Court, error) {
	if s.court == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.court, nil
}

func (s *stubCourts) GetPrices(context.Context, uuid.UUID) ([]*data.CourtPrice, error) {
	return s.prices, nil
}

// GetBlockedSlots honours ctx for the same reason stubLocks.ReleaseLock does:
// CourtModel hands it to queryContext and then to pgx, so a finished caller
// context fails the read rather than answering "nothing is blocked".
func (s *stubCourts) GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*data.BlockedSlot, error) {
	s.blockedQueries = append(s.blockedQueries, blockedQuery{courtID: courtID, date: date.Format("2006-01-02")})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.blockedErr != nil {
		return nil, s.blockedErr
	}
	return s.blocked[date.Format("2006-01-02")], nil
}

// block takes hours off sale on a court for one date, the way the owner's
// blocked-slot write would.
func (s *stubCourts) block(courtID uuid.UUID, date time.Time, start, end string) {
	if s.blocked == nil {
		s.blocked = map[string][]*data.BlockedSlot{}
	}
	key := date.Format("2006-01-02")
	s.blocked[key] = append(s.blocked[key], &data.BlockedSlot{
		ID: uuid.New(), CourtID: courtID, Date: date, StartTime: start, EndTime: end,
	})
}

type stubPayments struct {
	payment *data.Payment
	// ledger overrides ListByBookingID with the booking's full payment history,
	// for a test that needs more than the single row `payment` can represent
	// (e.g. an MP deposit plus a cash balance). Nil falls back to `payment`.
	ledger []*data.Payment
	// getErr fails GetByBookingID with something other than "no such row",
	// standing in for the database being unavailable while a cancellation is
	// deciding what can be refunded.
	getErr    error
	inserted  []*data.Payment
	confirmed []*data.Payment
	updated   []*data.Payment
	// insertAndConfirmErr drives InsertAndConfirmBooking's failure path — the
	// H-15 race and its ErrBookingNotConfirmable sentinel in particular, which
	// a handler test cannot otherwise reach since the real guard lives behind
	// a database transaction this stub never opens.
	insertAndConfirmErr error
	// manualRefundErr fails RecordManualRefund, standing in for the database
	// being unavailable, or for the booking's refund_status no longer reading
	// 'partial' by the time the write locks it.
	manualRefundErr    error
	manualRefundAmount int
	manualRefundCalls  int
}

func (s *stubPayments) Insert(_ context.Context, p *data.Payment) error {
	p.ID = uuid.New()
	s.inserted = append(s.inserted, p)
	return nil
}

// GetByBookingID honours ctx for the same reason stubLocks.ReleaseLock does:
// PaymentModel hands it to queryContext and then to pgx, so a finished caller
// context fails the read. expireCheckoutPreference starts with this call, and
// whether it survives a disconnected client is exactly what a test has to be
// able to see.
func (s *stubPayments) GetByBookingID(ctx context.Context, _ uuid.UUID) (*data.Payment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.payment == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.payment, nil
}

func (s *stubPayments) InsertAndConfirmBooking(_ context.Context, p *data.Payment, _ *data.Booking) error {
	if s.insertAndConfirmErr != nil {
		return s.insertAndConfirmErr
	}
	s.confirmed = append(s.confirmed, p)
	return nil
}

// ListByBookingID mirrors s.payment, the only row this stub knows about, so
// tests seeding one payment see it in the ledger too. An unseeded stub
// returns nil — an empty ledger, not an error — the same way the real store's
// ListByBookingID does for a booking with no payments. getErr fails this the
// same way it fails GetByBookingID, so a test simulating "the database would
// not answer" reaches whichever of the two lookups a caller now makes.
func (s *stubPayments) ListByBookingID(ctx context.Context, _ uuid.UUID) ([]*data.Payment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.ledger != nil {
		return s.ledger, nil
	}
	if s.payment == nil {
		return nil, nil
	}
	return []*data.Payment{s.payment}, nil
}

func (s *stubPayments) Update(_ context.Context, p *data.Payment) error {
	s.updated = append(s.updated, p)
	return nil
}

func (s *stubPayments) RecordManualRefund(_ context.Context, _ uuid.UUID) (int, error) {
	s.manualRefundCalls++
	if s.manualRefundErr != nil {
		return 0, s.manualRefundErr
	}
	return s.manualRefundAmount, nil
}

type stubLocks struct {
	acquireErr error
	// releaseErr fails ReleaseLock, standing in for the database refusing to free
	// a held slot. A release that cannot fail cannot prove the caller survives it.
	releaseErr error
	acquired   []string
	released   []string
	// abandoned is every release that never reached the database because the
	// context handed to it was already finished.
	abandoned []string
}

// SlotLockModel hands its ctx to queryContext and then straight to pgx, so a
// caller context that is already cancelled fails the statement before it reaches
// PostgreSQL — the lock stays. A stub that ignores its ctx cannot tell a release
// that happened from one that was abandoned, and on the disconnect path that is
// the entire question. Honouring ctx.Err() here is what makes the difference
// observable.
func (s *stubLocks) AcquireLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, _ string, _ *uuid.UUID, _ time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.acquireErr != nil {
		return s.acquireErr
	}
	s.acquired = append(s.acquired, slotKey(courtID, date, startTime))
	return nil
}

func (s *stubLocks) ReleaseLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime string) error {
	if err := ctx.Err(); err != nil {
		s.abandoned = append(s.abandoned, slotKey(courtID, date, startTime))
		return err
	}
	if s.releaseErr != nil {
		return s.releaseErr
	}
	s.released = append(s.released, slotKey(courtID, date, startTime))
	return nil
}

func slotKey(courtID uuid.UUID, date time.Time, startTime string) string {
	return courtID.String() + "@" + date.Format("2006-01-02") + " " + startTime
}

// expiryAttempt is one call to UpdatePreferenceExpired, and it records who the
// call was made as along with the preference.
//
// The stub used to discard that with `_ string`, which is precisely the kind of
// blind double this suite must not have: the whole question on the credential
// path is *which* party the call was made as, and a stub that throws the
// argument away cannot tell a refusal from a call authenticated as the wrong
// one. mp.Caller is comparable, so a test says which one it expected by
// building it — mp.AsPlatform() or mustSeller. Every attempt is recorded,
// failed ones included, because "did it retry" is not answerable from
// successes alone.
type expiryAttempt struct {
	preferenceID string
	caller       mp.Caller
}

// mustSeller is mp.AsSeller for the tokens a test hard-codes and knows are
// non-empty, so a fixture never quietly builds the unnamed caller the client
// refuses.
func mustSeller(t *testing.T, token string) mp.Caller {
	t.Helper()
	caller, err := mp.AsSeller(token)
	if err != nil {
		t.Fatalf("mp.AsSeller(%q): %v", token, err)
	}
	return caller
}

type stubCheckout struct {
	preference *mp.Preference
	err        error
	// expireErr fails UpdatePreferenceExpired on every attempt, standing in for
	// MercadoPago refusing to close a checkout link that is still live.
	expireErr error
	// expireFailFirst fails only the first attempt, so a retry that succeeds can
	// be told apart from a single call that did.
	expireFailFirst bool
	// onCreate runs inside CreatePreference, before it answers. It is how a test
	// puts something between "the handler called MercadoPago" and "MercadoPago
	// answered" — a client closing the tab, in particular.
	onCreate func()
	created  int
	expired  []string
	// attempts is every call, in order, whatever its outcome.
	attempts []expiryAttempt
	// lastInput records what CreatePreference was actually called with, so a
	// test can assert on the input rather than trusting that the caller built
	// it correctly — the stub used to ignore its argument entirely.
	lastInput mp.CreatePreferenceInput
}

func (s *stubCheckout) CreatePreference(_ context.Context, input mp.CreatePreferenceInput) (*mp.Preference, error) {
	s.created++
	s.lastInput = input
	if s.onCreate != nil {
		s.onCreate()
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.preference == nil {
		return &mp.Preference{ID: "pref-1", InitPoint: "https://mp.test/checkout"}, nil
	}
	return s.preference, nil
}

func (s *stubCheckout) RefreshOAuthToken(context.Context, string) (*mp.OAuthTokens, error) {
	return &mp.OAuthTokens{AccessToken: "refreshed"}, nil
}

func (s *stubCheckout) UpdatePreferenceExpired(_ context.Context, preferenceID string, caller mp.Caller) error {
	s.attempts = append(s.attempts, expiryAttempt{preferenceID: preferenceID, caller: caller})
	if s.expireErr != nil {
		return s.expireErr
	}
	if s.expireFailFirst && len(s.attempts) == 1 {
		return errPreferenceExpiryUnavailable
	}
	s.expired = append(s.expired, preferenceID)
	return nil
}

// errPreferenceExpiryUnavailable stands in for MercadoPago being unreachable
// for one call — the transient failure a retry exists to survive.
var errPreferenceExpiryUnavailable = errors.New("mercadopago unavailable")

type stubWhatsApp struct {
	challenge    string
	verifyErr    error
	signatureErr error
}

func (s *stubWhatsApp) VerifyWebhook(*http.Request) (string, error) {
	return s.challenge, s.verifyErr
}

func (s *stubWhatsApp) VerifySignature(*http.Request, []byte) error { return s.signatureErr }

// stubRefunder can answer with any outcome the real refund path can produce. A
// stub that only ever recorded the call is how an endpoint that reported "no
// refund" after every successful one shipped green.
type stubRefunder struct {
	outcome data.RefundOutcome
	// onRefund runs inside AutoRefundIfPaid, standing in for what happens while
	// the MercadoPago refund call is in flight — the client closing the tab, for
	// one.
	onRefund func()
	refunded []uuid.UUID
}

func (s *stubRefunder) AutoRefundIfPaid(_ context.Context, b *data.Booking) data.RefundOutcome {
	s.refunded = append(s.refunded, b.ID)
	if s.onRefund != nil {
		s.onRefund()
	}
	if s.outcome.Result == "" {
		return data.RefundOutcome{Result: data.RefundIssued, AmountCentavos: 150_000}
	}
	return s.outcome
}

type stubNotifier struct {
	confirmed []notifications.BookingConfirmation
	cancelled []notifications.Cancellation
}

func (s *stubNotifier) BookingConfirmed(c notifications.BookingConfirmation) {
	s.confirmed = append(s.confirmed, c)
}

func (s *stubNotifier) BookingCancelled(c notifications.Cancellation) {
	s.cancelled = append(s.cancelled, c)
}

type stubBroadcaster struct{ published []uuid.UUID }

func (s *stubBroadcaster) PublishBookingChanged(id uuid.UUID) { s.published = append(s.published, id) }

type stubRecorder struct{ entries []audit.Entry }

func (s *stubRecorder) Record(e audit.Entry) { s.entries = append(s.entries, e) }

// stubLinkResolver stands in for the booking-link-token store's
// ResolveBooking method (LinkResolver). Like stubStore.GetByID before it, it
// ignores the presented token and answers from whatever booking (and expiry)
// the test configured — a test that wants to assert on the presented token
// itself sets validToken and checks it explicitly.
//
// expiresAt defaults to far in the future when unset, so a test that does not
// care about token expiry gets the same "always resolves" behavior the old
// GetByID-based stub gave every public-route test before this credential
// swap.
type stubLinkResolver struct {
	booking    *data.Booking
	expiresAt  time.Time
	err        error
	validToken string
	resolved   []string
}

func (s *stubLinkResolver) ResolveBooking(_ context.Context, token string) (*data.Booking, time.Time, error) {
	s.resolved = append(s.resolved, token)
	if s.err != nil {
		return nil, time.Time{}, s.err
	}
	if s.validToken != "" && token != s.validToken {
		return nil, time.Time{}, data.ErrRecordNotFound
	}
	if s.booking == nil {
		return nil, time.Time{}, data.ErrRecordNotFound
	}
	expiresAt := s.expiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(100 * 365 * 24 * time.Hour)
	}
	return s.booking, expiresAt, nil
}

type fixture struct {
	handler      *Handler
	store        *stubStore
	clients      *stubClients
	complexes    *stubComplexes
	courts       *stubCourts
	payments     *stubPayments
	locks        *stubLocks
	checkout     *stubCheckout
	whatsapp     *stubWhatsApp
	refunds      *stubRefunder
	linkResolver *stubLinkResolver
	notify       *stubNotifier
	realtime     *stubBroadcaster
	audit        *stubRecorder
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{
		store: &stubStore{}, clients: &stubClients{}, complexes: &stubComplexes{},
		courts: &stubCourts{}, payments: &stubPayments{}, locks: &stubLocks{},
		checkout: &stubCheckout{}, whatsapp: &stubWhatsApp{}, refunds: &stubRefunder{},
		linkResolver: &stubLinkResolver{},
		notify:       &stubNotifier{}, realtime: &stubBroadcaster{}, audit: &stubRecorder{},
	}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	f.handler = NewHandler(Dependencies{
		Store: f.store, Clients: f.clients, Complexes: f.complexes, Courts: f.courts,
		Payments: f.payments, Locks: f.locks, Checkout: f.checkout, WhatsApp: f.whatsapp,
		Refunds: f.refunds, LinkResolver: f.linkResolver, Notify: f.notify, Realtime: f.realtime, Audit: f.audit,
		Respond: httpx.NewResponder(logger), Logger: logger,
		Run: func(fn func()) { fn() },
	}, Config{
		FrontendURL: "https://vibe.test", BackendURL: "https://api.vibe.test",
		Environment: "test", GracePeriod: 15 * time.Minute,
		PaymentExpiry: 15 * time.Minute, SlotLockTTL: 15 * time.Minute,
		WhatsAppEnabled: true,
	})
	return f
}

// ownerRequest builds a request from the complex's authenticated owner.
func ownerRequest(t *testing.T, method, target string, complexID uuid.UUID, params map[string]string, body string) *http.Request {
	t.Helper()

	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	}

	r = httpx.ContextSetUser(r, &authstore.User{ID: uuid.New(), Role: "owner"})
	r = httpx.ContextSetComplex(r, &data.Complex{ID: complexID, CancellationHours: 24})

	if len(params) > 0 {
		p := make(httprouter.Params, 0, len(params))
		for k, v := range params {
			p = append(p, httprouter.Param{Key: k, Value: v})
		}
		r = r.WithContext(context.WithValue(r.Context(), httprouter.ParamsKey, p))
	}
	return r
}

func publicRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	if body == "" {
		return httptest.NewRequestWithContext(t.Context(), method, target, nil)
	}
	return httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return out
}

// futureBooking returns a confirmed booking a week out.
func futureBooking(complexID uuid.UUID) *data.Booking {
	date := timezone.Day(time.Now().In(timezone.Argentina).AddDate(0, 0, 7))
	startsAt := slots.At(date, "18:00")
	return &data.Booking{
		ID: uuid.New(), ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "confirmed", CollectionStatus: data.CollectionStatusDepositPaid,
		RefundStatus: data.RefundStatusNone, Price: 500_000,
		Date:      date,
		StartTime: "18:00", DurationMinutes: 90, CreatedAt: time.Now(),
		// The span the store reads off bookings.span. Every payload's
		// starts_at/ends_at come from these, so a fixture that left them zero
		// would assert on the year one.
		StartsAt: startsAt,
		EndsAt:   startsAt.Add(90 * time.Minute),
	}
}

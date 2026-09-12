package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
)

// stubPayments stands in for the payment store. Every step of the refund
// lifecycle can be made to fail: the old stub hardcoded a successful commit, which
// is precisely why a flow that lost refunds on a failed commit shipped green.
type stubPayments struct {
	byMPID    *paymentstore.Payment
	byBooking *paymentstore.Payment
	// byBookingAll is the whole ledger a test has seeded, in insertion order —
	// what ListByBookingID answers with. Existing tests only ever set
	// byBooking directly (bypassing remember), so ListByBookingID falls back to
	// treating that single row as the whole ledger when byBookingAll is empty,
	// which keeps every single-row fixture correct without having to touch it.
	byBookingAll []*paymentstore.Payment
	// listErr fails ListByBookingID, standing in for the database being
	// unavailable while the refund path is summing every payment row.
	listErr error

	mpIDErr    error
	bookingErr error
	// claimErr refuses the claim, standing in for a payment another path already
	// holds or for the database being unavailable.
	claimErr error
	// insertErr fails InsertAndConfirmBooking, standing in for the database being
	// unavailable — the same condition that makes a refund fail.
	insertErr error
	// confirmErr fails ConfirmWebhookPayment, standing in for the database being
	// unavailable while a captured payment is being written against its booking.
	// A stub that always confirms is how an unretryable confirmation would ship
	// green.
	confirmErr error
	// slotErr stands in for the confirmation-time slot guard, which refuses to
	// confirm a booking whose slot was taken while its payment was in flight. It
	// applies only to a write that would confirm the booking, exactly as the real
	// guard does, so the refund path — which records the same payment against a
	// cancelled booking — still goes through.
	slotErr error
	// successErr fails RecordRefundSuccess after the provider already moved the
	// money, so a test can reach the refunded-but-not-recorded path.
	successErr error
	// failureErr fails RecordRefundFailure, so the requeue itself can be broken.
	failureErr error
	// exhausted makes RecordRefundFailure report the retry budget spent.
	exhausted bool
	// claimAmount is what a claim reserves; a zero value means the usual deposit.
	claimAmount int

	updated   *paymentstore.Payment
	inserted  *paymentstore.Payment
	confirmed *paymentstore.Payment

	// insertedBooking and confirmedBooking are the booking as the store was
	// handed it, copied rather than aliased. The callers mutate the booking they
	// pass and go on using it afterwards, so a pointer here would report the
	// state at the end of the flow rather than the state that was committed —
	// and "committed together with the payment" is exactly the question the
	// refund-intent marker is asked.
	insertedBooking  *data.Booking
	confirmedBooking *data.Booking

	// insertedRows is every row this store was asked to create. `inserted` only
	// remembers the last one, so it cannot tell one insert from two — which is
	// exactly the question a redelivery has to answer.
	insertedRows []*paymentstore.Payment

	claimed         []uuid.UUID
	recordedSuccess []paymentstore.RefundClaim
	// recordedSuccessManualOwed is the manualOwedCentavos each RecordRefundSuccess
	// call carried, index-aligned with recordedSuccess — so a test can assert the
	// caller actually threaded the booking's manual balance through rather than
	// always passing zero.
	recordedSuccessManualOwed []int
	recordedFailure           []recordedFailure
}

// recordedFailure is one requeued attempt and the reason given for it.
type recordedFailure struct {
	claim paymentstore.RefundClaim
	cause string
}

func (s *stubPayments) GetByMPPaymentID(context.Context, string) (*paymentstore.Payment, error) {
	if s.mpIDErr != nil {
		return nil, s.mpIDErr
	}
	if s.byMPID == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.byMPID, nil
}

func (s *stubPayments) GetByBookingID(context.Context, uuid.UUID) (*paymentstore.Payment, error) {
	if s.bookingErr != nil {
		return nil, s.bookingErr
	}
	if s.byBooking == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.byBooking, nil
}

// ListByBookingID returns the seeded ledger in insertion order. Tests that
// seed byBooking directly (never calling remember) get that single row back,
// matching what the real store would return for a booking with exactly one
// payment.
//
// bookingErr also fails this, the same way it fails GetByBookingID: existing
// tests use it to stand in for "the database would not answer" regardless of
// which of the two lookups a caller now makes, and the real ListByBookingID
// never returns ErrRecordNotFound for an empty ledger (nil, nil instead), so
// that one sentinel is translated rather than propagated.
func (s *stubPayments) ListByBookingID(_ context.Context, _ uuid.UUID) ([]*paymentstore.Payment, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.bookingErr != nil {
		if errors.Is(s.bookingErr, data.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, s.bookingErr
	}
	if len(s.byBookingAll) > 0 {
		return s.byBookingAll, nil
	}
	if s.byBooking != nil {
		return []*paymentstore.Payment{s.byBooking}, nil
	}
	return nil, nil
}

func (s *stubPayments) InsertAndConfirmBooking(_ context.Context, p *paymentstore.Payment, b *data.Booking) error {
	if s.slotErr != nil && b.Status == "confirmed" {
		return s.slotErr
	}
	if s.insertErr != nil {
		return s.insertErr
	}
	// The real store fills the generated id in, and the claim that follows needs it.
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	s.inserted = p
	s.insertedRows = append(s.insertedRows, p)
	snapshot := *b
	s.insertedBooking = &snapshot
	s.remember(p)
	return nil
}

func (s *stubPayments) ConfirmWebhookPayment(_ context.Context, p *paymentstore.Payment, b *data.Booking) error {
	if s.slotErr != nil && b.Status == "confirmed" {
		return s.slotErr
	}
	if s.confirmErr != nil {
		return s.confirmErr
	}
	s.confirmed = p
	snapshot := *b
	s.confirmedBooking = &snapshot
	s.remember(p)
	return nil
}

// remember makes a written row readable again, which a stub that only records
// what it was handed never does.
//
// Both lookups matter and for different reasons. GetByMPPaymentID mirrors the
// payments table's partial unique index on mp_payment_id: the moment a row
// carries MercadoPago's payment id, the next lookup for that id finds it.
// GetByBookingID mirrors idx_payments_booking plus GetPaymentByBookingID's
// ordering: one booking's payment is readable back from the booking alone.
//
// Without both, every redelivery looks like a first delivery and a confirmation
// that is not idempotent — one that writes a second payment row for money that
// was captured once — ships green.
func (s *stubPayments) remember(p *paymentstore.Payment) {
	if p.MPPaymentID != nil && *p.MPPaymentID != "" {
		s.byMPID = p
	}
	s.byBooking = p
	s.byBookingAll = append(s.byBookingAll, p)
}

func (s *stubPayments) Update(_ context.Context, p *paymentstore.Payment) error {
	s.updated = p
	return nil
}

func (s *stubPayments) ClaimRefund(_ context.Context, paymentID uuid.UUID) (*paymentstore.RefundClaim, error) {
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	// The real claim is a committed row-level reservation, so a second claim on a
	// payment this stub already handed out has to be refused here too — otherwise
	// a caller that double-claims looks correct in tests.
	for _, claimed := range s.claimed {
		if claimed == paymentID {
			return nil, paymentstore.ErrRefundInFlight
		}
	}
	s.claimed = append(s.claimed, paymentID)

	amount := s.claimAmount
	if amount == 0 {
		amount = 150_000
	}
	claim := &paymentstore.RefundClaim{
		AttemptID:      uuid.New(),
		PaymentID:      paymentID,
		MPPaymentID:    "mp-123",
		RefundCentavos: amount,
	}
	// The real ClaimRefund reads BookingID and ComplexID off the locked payment
	// row (internal/data/refunds.go). Mirror that here so a caller building an
	// alert from claim.BookingID/ComplexID — the seller-credential refusal path
	// — sees the fixture's values, not the zero UUID.
	if p := s.paymentFor(paymentID); p != nil {
		claim.BookingID = p.BookingID
		claim.ComplexID = p.ComplexID
	}
	return claim, nil
}

// paymentFor returns whichever fixture payment matches id, mirroring how the
// real store would locate the row a claim is being made against.
func (s *stubPayments) paymentFor(id uuid.UUID) *paymentstore.Payment {
	if s.byBooking != nil && s.byBooking.ID == id {
		return s.byBooking
	}
	if s.byMPID != nil && s.byMPID.ID == id {
		return s.byMPID
	}
	for _, p := range s.byBookingAll {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// RecordRefundSuccess mirrors the real recorder's arithmetic, which is the
// whole point of it: internal/data/refunds.go does not assign refund_amount, it
// *adds* the claim's figure to whatever the locked row already holds and caps
// the sum at amount + service_fee.
//
// Returning claim.RefundCentavos flat, as this stub used to, made the recorder
// look like an assignment — and an assignment cannot collide with the webhook
// path's own assignment. The collision is exactly the defect: two writers, two
// arithmetics, one column. A stub that cannot add cannot show a sum being
// counted twice.
func (s *stubPayments) RecordRefundSuccess(_ context.Context, claim paymentstore.RefundClaim, manualOwedCentavos int) (int, error) {
	if s.successErr != nil {
		return 0, s.successErr
	}
	s.recordedSuccess = append(s.recordedSuccess, claim)
	s.recordedSuccessManualOwed = append(s.recordedSuccessManualOwed, manualOwedCentavos)

	row := s.paymentFor(claim.PaymentID)
	if row == nil {
		// No fixture row to read back: the flat answer, as before.
		return claim.RefundCentavos, nil
	}
	totalPaid := row.Amount + row.ServiceFee
	refundTotal := min(row.RefundAmount+claim.RefundCentavos, totalPaid)
	row.RefundAmount = refundTotal
	if refundTotal >= totalPaid {
		row.Status = "refunded"
	} else {
		row.Status = "deposit_paid"
	}
	return refundTotal, nil
}

func (s *stubPayments) RecordRefundFailure(_ context.Context, claim paymentstore.RefundClaim, cause string) (bool, error) {
	if s.failureErr != nil {
		return false, s.failureErr
	}
	s.recordedFailure = append(s.recordedFailure, recordedFailure{claim: claim, cause: cause})
	return s.exhausted, nil
}

type stubBookings struct {
	booking *data.Booking
	err     error
	updated *data.Booking
}

func (s *stubBookings) GetByID(context.Context, uuid.UUID) (*data.Booking, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.booking == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.booking, nil
}

func (s *stubBookings) Update(_ context.Context, b *data.Booking) error {
	s.updated = b
	return nil
}

// stubLinkTokens stands in for the booking-link-token store's Mint method
// (LinkMinter). A stub that always succeeds is how a webhook confirmation
// that skips minting on failure would ship green, so mintErr lets a test
// reach that refusal.
type stubLinkTokens struct {
	mintErr error
	minted  []uuid.UUID
}

func (s *stubLinkTokens) Mint(_ context.Context, bookingID uuid.UUID, _ time.Time) (string, error) {
	if s.mintErr != nil {
		return "", s.mintErr
	}
	s.minted = append(s.minted, bookingID)
	return "stub-link-token-" + bookingID.String(), nil
}

type stubClients struct {
	client  *clientstore.Client
	err     error
	updated *clientstore.Client
}

func (s *stubClients) GetByID(context.Context, uuid.UUID) (*clientstore.Client, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.client == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.client, nil
}

func (s *stubClients) Update(_ context.Context, c *clientstore.Client) error {
	s.updated = c
	return nil
}

type stubComplexes struct {
	complex *complexstore.Complex
	err     error
	// calls counts GetByID invocations, so a test can assert the collector
	// check (the only caller of this method in internal/payments) runs
	// exactly once per webhook rather than once per branch it dominates.
	calls int
}

func (s *stubComplexes) GetByID(context.Context, uuid.UUID) (*complexstore.Complex, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.complex == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.complex, nil
}

type stubCourts struct{ court *courtstore.Court }

func (s *stubCourts) GetByID(context.Context, uuid.UUID) (*courtstore.Court, error) {
	if s.court == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.court, nil
}

// stubFailedRefunds is the queue side only: rows are now created and closed by the
// payment store's refund lifecycle, so this stub reads and claims, and both of
// those can fail.
type stubFailedRefunds struct {
	pending []*paymentstore.FailedRefund

	pendingErr    error
	processingErr error

	processing []uuid.UUID
}

func (s *stubFailedRefunds) GetPendingDue(context.Context) ([]*paymentstore.FailedRefund, error) {
	if s.pendingErr != nil {
		return nil, s.pendingErr
	}
	return s.pending, nil
}

func (s *stubFailedRefunds) MarkProcessing(_ context.Context, id uuid.UUID) error {
	if s.processingErr != nil {
		return s.processingErr
	}
	s.processing = append(s.processing, id)
	return nil
}

// callTrace records the order in which the stubs were used, so a test can assert
// that the event was recorded *before* any processing was attempted rather than
// only that both happened at some point.
type callTrace struct{ calls []string }

func (t *callTrace) add(name string) {
	if t != nil {
		t.calls = append(t.calls, name)
	}
}

// indexOf returns where name was first called, or -1 if it never was.
func (t *callTrace) indexOf(name string) int {
	for i, call := range t.calls {
		if call == name {
			return i
		}
	}
	return -1
}

// recordedWebhookFailure is one requeued event and the reason given for it.
type recordedWebhookFailure struct {
	id    uuid.UUID
	cause string
}

// stubWebhookEvents stands in for the durable inbox. Every step of it can be
// made to fail, because a store stub that always records successfully is exactly
// how a handler answering 200 without recording anything shipped green.
type stubWebhookEvents struct {
	trace *callTrace

	// insertErr fails the recording step, standing in for the database being
	// unavailable at the one moment the 200 depends on.
	insertErr error
	// claimRefused makes Claim report that another worker owns the row.
	claimRefused bool
	claimErr     error
	processedErr error
	failedErr    error
	// exhausted makes MarkFailed report the retry budget spent.
	exhausted bool

	pending    []*paymentstore.WebhookEvent
	pendingErr error

	inserted  []*paymentstore.WebhookEvent
	claimed   []uuid.UUID
	processed []uuid.UUID
	failed    []recordedWebhookFailure
}

func (s *stubWebhookEvents) Insert(_ context.Context, e *paymentstore.WebhookEvent) error {
	s.trace.add("record")
	if s.insertErr != nil {
		return s.insertErr
	}
	// The real store fills these in on the way back, and the worker needs the id.
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.Status = "pending"
	e.MaxRetries = 5
	s.inserted = append(s.inserted, e)
	return nil
}

func (s *stubWebhookEvents) GetPendingDue(context.Context) ([]*paymentstore.WebhookEvent, error) {
	if s.pendingErr != nil {
		return nil, s.pendingErr
	}
	return s.pending, nil
}

func (s *stubWebhookEvents) Claim(_ context.Context, id uuid.UUID) (bool, error) {
	if s.claimErr != nil {
		return false, s.claimErr
	}
	if s.claimRefused {
		return false, nil
	}
	s.claimed = append(s.claimed, id)
	return true, nil
}

func (s *stubWebhookEvents) MarkProcessed(_ context.Context, id uuid.UUID) error {
	if s.processedErr != nil {
		return s.processedErr
	}
	s.processed = append(s.processed, id)
	return nil
}

func (s *stubWebhookEvents) MarkFailed(_ context.Context, id uuid.UUID, cause string) (bool, error) {
	if s.failedErr != nil {
		return false, s.failedErr
	}
	s.failed = append(s.failed, recordedWebhookFailure{id: id, cause: cause})
	return s.exhausted, nil
}

// stubLocks grants the advisory lock unless told otherwise, so a test can
// simulate another instance already handling the same webhook.
// stubRefundIntents stands in for the reconciliation sweep's store layer.
type stubRefundIntents struct {
	orphans    []*data.Booking
	orphansErr error

	claimErr error
	// notFoundFor makes ClaimRefundIntent report ErrRecordNotFound for these
	// specific ids, standing in for another instance having already taken the
	// row.
	notFoundFor map[uuid.UUID]bool

	clearErr error

	claimed []uuid.UUID
	cleared []uuid.UUID
}

func (s *stubRefundIntents) GetRefundIntentOrphans(context.Context, time.Duration, int) ([]*data.Booking, error) {
	if s.orphansErr != nil {
		return nil, s.orphansErr
	}
	return s.orphans, nil
}

func (s *stubRefundIntents) ClaimRefundIntent(_ context.Context, id uuid.UUID, _ time.Time) error {
	if s.claimErr != nil {
		return s.claimErr
	}
	if s.notFoundFor[id] {
		return data.ErrRecordNotFound
	}
	s.claimed = append(s.claimed, id)
	return nil
}

func (s *stubRefundIntents) ClearRefundIntent(_ context.Context, id uuid.UUID) error {
	if s.clearErr != nil {
		return s.clearErr
	}
	s.cleared = append(s.cleared, id)
	return nil
}

type stubLocks struct {
	trace    *callTrace
	taken    bool
	err      error
	attempts []string
	released int
}

func (s *stubLocks) TryAdvisory(_ context.Context, key string) (bool, func(), error) {
	s.trace.add("process")
	s.attempts = append(s.attempts, key)
	if s.err != nil {
		return false, func() {}, s.err
	}
	if s.taken {
		return false, func() {}, nil
	}
	return true, func() { s.released++ }, nil
}

type stubProvider struct {
	signatureErr error
	payment      *mp.Payment
	paymentErr   error
	refundErr    error
	// refundAmount is what MercadoPago answers an accepted refund with, in
	// pesos. Zero is the real client's zero value too — an accepted refund whose
	// response carried no usable amount — so it stands for that case rather than
	// for "nothing moved".
	refundAmount float64

	verified []string
	refunds  []float64
	// callers is who each provider call was made as, in order. mp.Caller is
	// comparable, so a test says which party it expected by building the same
	// one — mp.AsPlatform() or mustSeller — rather than by matching a token
	// string the stub would have had to be handed in the clear.
	callers []mp.Caller
}

// mustSeller is mp.AsSeller for the tokens a test hard-codes and knows are
// non-empty, so a fixture never quietly builds the unnamed caller the provider
// refuses.
func mustSeller(t *testing.T, token string) mp.Caller {
	t.Helper()
	caller, err := mp.AsSeller(token)
	if err != nil {
		t.Fatalf("mp.AsSeller(%q): %v", token, err)
	}
	return caller
}

func (s *stubProvider) VerifyWebhookSignature(_ *http.Request, dataID string) error {
	s.verified = append(s.verified, dataID)
	return s.signatureErr
}

func (s *stubProvider) GetPayment(_ context.Context, _ string, caller mp.Caller) (*mp.Payment, error) {
	s.callers = append(s.callers, caller)
	if s.paymentErr != nil {
		return nil, s.paymentErr
	}
	if s.payment == nil {
		// The real client never returns a nil payment with a nil error, so
		// neither does this.
		return &mp.Payment{ID: 123, Status: "pending"}, nil
	}
	return s.payment, nil
}

func (s *stubProvider) RefundPayment(_ context.Context, _ string, amount float64, caller mp.Caller) (*mp.Refund, error) {
	s.callers = append(s.callers, caller)
	if s.refundErr != nil {
		return nil, s.refundErr
	}
	s.refunds = append(s.refunds, amount)
	return &mp.Refund{Amount: s.refundAmount, Status: "approved"}, nil
}

type stubNotifier struct {
	confirmations []notifications.BookingConfirmation
	refunds       []notifications.Refund
}

func (s *stubNotifier) BookingConfirmed(c notifications.BookingConfirmation) {
	s.confirmations = append(s.confirmations, c)
}

func (s *stubNotifier) DepositRefunded(r notifications.Refund) {
	s.refunds = append(s.refunds, r)
}

// stubRecorder keeps every audit entry it is handed, whole.
//
// A stub that counted calls would pass for a handler that writes an empty
// entry, which is the failure mode this repository has already shipped behind
// doubles that could not observe. The tests below assert on the action, the
// entity, the complex the entry is scoped to and the encoded value.
type stubRecorder struct{ entries []audit.Entry }

func (s *stubRecorder) Record(e audit.Entry) { s.entries = append(s.entries, e) }

// find returns the entries recorded under one action, in order.
func (s *stubRecorder) find(action string) []audit.Entry {
	var found []audit.Entry
	for _, e := range s.entries {
		if e.Action == action {
			found = append(found, e)
		}
	}
	return found
}

// value re-encodes an entry's NewValue the way audit.Recorder.Record does and
// decodes it back into a map, so a test asserts on the JSON that would reach
// the audit_log row rather than on the struct the handler happened to build.
func value(t *testing.T, e audit.Entry) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(e.NewValue)
	if err != nil {
		t.Fatalf("audit value does not encode: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("audit value is not a JSON object: %v", err)
	}
	return decoded
}

type stubBroadcaster struct{ published []uuid.UUID }

func (s *stubBroadcaster) PublishBookingChanged(complexID uuid.UUID) {
	s.published = append(s.published, complexID)
}

type fixture struct {
	handler       *Handler
	payments      *stubPayments
	bookings      *stubBookings
	clients       *stubClients
	complexes     *stubComplexes
	courts        *stubCourts
	failedRefunds *stubFailedRefunds
	webhookEvents *stubWebhookEvents
	refundIntents *stubRefundIntents
	linkTokens    *stubLinkTokens
	locks         *stubLocks
	provider      *stubProvider
	notify        *stubNotifier
	realtime      *stubBroadcaster
	audit         *stubRecorder
	trace         *callTrace
	logs          *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	logs := &bytes.Buffer{}
	trace := &callTrace{}
	f := &fixture{
		payments:      &stubPayments{},
		bookings:      &stubBookings{},
		clients:       &stubClients{},
		complexes:     &stubComplexes{},
		courts:        &stubCourts{},
		failedRefunds: &stubFailedRefunds{},
		webhookEvents: &stubWebhookEvents{trace: trace},
		refundIntents: &stubRefundIntents{},
		linkTokens:    &stubLinkTokens{},
		locks:         &stubLocks{trace: trace},
		provider:      &stubProvider{},
		notify:        &stubNotifier{},
		realtime:      &stubBroadcaster{},
		audit:         &stubRecorder{},
		trace:         trace,
		logs:          logs,
	}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	f.handler = NewHandler(Dependencies{
		Payments:      f.payments,
		Bookings:      f.bookings,
		Clients:       f.clients,
		Complexes:     f.complexes,
		Courts:        f.courts,
		FailedRefunds: f.failedRefunds,
		WebhookEvents: f.webhookEvents,
		RefundIntents: f.refundIntents,
		LinkTokens:    f.linkTokens,
		Locks:         f.locks,
		Provider:      f.provider,
		Notify:        f.notify,
		Realtime:      f.realtime,
		Audit:         f.audit,
		Respond:       httpx.NewResponder(logger),
		Logger:        logger,
		Run:           func(fn func()) { fn() }, // inline, so tests observe the work
	}, Config{FrontendURL: "https://vibe.test", CancellationGracePeriod: 15 * time.Minute, LinkTokenBuffer: 24 * time.Hour})
	return f
}

// paidBooking returns a confirmed, deposit-paid booking and its payment.
func paidBooking(complexID uuid.UUID) (*data.Booking, *paymentstore.Payment) {
	bookingID := uuid.New()
	mpID := "mp-123"
	return &data.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "confirmed", CollectionStatus: data.CollectionStatusDepositPaid,
		RefundStatus: data.RefundStatusNone, Price: 500_000,
		Date: time.Now().AddDate(0, 0, 7), StartTime: "18:00", DurationMinutes: 90,
		CreatedAt: time.Now(),
	}, &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID, Amount: 150_000,
		Status: "deposit_paid", MPPaymentID: &mpID,
	}
}

// sellerTestToken is the placeholder seller access token every fixture below
// uses when a test needs sellerCredential to succeed rather than exercise one
// of its three refusal arms.
const sellerTestToken = "seller-access-token"

// linkedComplex returns a complex whose stored MercadoPago credential reads
// (SellerAccessToken) as well as whose MPUserID (the collector check) are
// both set — the ordinary state of a complex that has connected MercadoPago.
// mpUserID may be "" for a test that does not exercise the collector check.
//
// Building this through data.NewComplexForTest is required: the credential
// fields are unexported, so a bare &data.Complex{} literal outside
// internal/data can never carry a seller token, and sellerCredential would
// then always refuse with ErrMPNotConnected regardless of what the test is
// actually about.
func linkedComplex(id uuid.UUID, mpUserID string) *complexstore.Complex {
	token := sellerTestToken
	c := complexstore.NewComplexForTest(id, &token, nil)
	if mpUserID != "" {
		c.MPUserID = &mpUserID
	}
	return c
}

var (
	errProvider = errors.New("mercadopago unavailable")
	errDatabase = errors.New("database unavailable")
	errRecord   = errors.New("recording the refund failed")
)

// pendingBooking returns a booking still waiting on payment together with the
// MercadoPago payment that pays for it exactly, so a test exercising the confirmation
// path clears the amount check and is decided by whatever it is actually testing.
func pendingBooking(complexID uuid.UUID) (*data.Booking, *mp.Payment) {
	booking := &data.Booking{
		ID: uuid.New(), ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "pending", CollectionStatus: data.CollectionStatusUnpaid,
		RefundStatus: data.RefundStatusNone, Price: 500_000, DepositAmount: 150_000,
		Date: time.Now().AddDate(0, 0, 7), StartTime: "18:00", DurationMinutes: 90,
		CreatedAt: time.Now(),
	}
	totalCentavos := booking.DepositAmount + pricing.ServiceFee(booking.DepositAmount)
	return booking, &mp.Payment{
		ID:                123,
		Status:            "approved",
		TransactionAmount: float64(totalCentavos) / 100.0,
	}
}

package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

type stubStore struct {
	event       *eventstore.Event
	regs        map[uuid.UUID]*eventstore.Registration
	byToken     map[string]*eventstore.Registration
	createErr   error
	created     *eventstore.Event
	createdCrts []uuid.UUID
	paid        []uuid.UUID
	cancelErr   error
	cancelled   []uuid.UUID
	signUpErr   error
	signedUp    []eventstore.SignUpInput
	released    []uuid.UUID
	participant []string
	rows        []*eventstore.RegistrationRow
	// owed is the refundOwed of every CancelByParticipant, in order; drawnAtCancel
	// is what the cancellation's own transaction found about the draw.
	owed          []bool
	drawnAtCancel bool
	// pending is what the refund sweep's read answers.
	pending []eventstore.PendingRefund
	// expired is what the hold sweep closes; due is what the reminder sweep
	// reads, and reminded the registrations it marked.
	expired  []eventstore.ExpiredHold
	due      []eventstore.DueReminder
	dueAt    []time.Time
	reminded []uuid.UUID
	// checks are the open payment checks, in the order they were opened;
	// deferred the ones pushed back until the test makes them due again.
	checks   []*eventstore.PaymentCheck
	deferred map[uuid.UUID]bool
}

// openCheck opens a payment check, as an expiry's write does.
func (s *stubStore) openCheck(id, complexID, eventID uuid.UUID, notify bool) {
	s.checks = append(s.checks, &eventstore.PaymentCheck{RegistrationID: id, ComplexID: complexID, EventID: eventID, Notify: notify})
}

// makeDue brings every deferred check's time, as the clock passing does.
func (s *stubStore) makeDue() { s.deferred = nil }

func (s *stubStore) DuePaymentChecks(_ context.Context, limit int) ([]eventstore.PaymentCheck, error) {
	var out []eventstore.PaymentCheck
	for _, c := range s.checks {
		reg, ok := s.regs[c.RegistrationID]
		if s.deferred[c.RegistrationID] || (ok && reg.Status != eventstore.RegistrationExpired) || len(out) == limit {
			continue
		}
		out = append(out, *c)
	}
	return out, nil
}

func (s *stubStore) ClosePaymentCheck(_ context.Context, id uuid.UUID) (bool, error) {
	for i, c := range s.checks {
		if c.RegistrationID == id {
			s.checks = slices.Delete(s.checks, i, i+1)
			return true, nil
		}
	}
	return false, nil
}

func (s *stubStore) DeferPaymentCheck(_ context.Context, id uuid.UUID) (int, error) {
	for _, c := range s.checks {
		if c.RegistrationID == id {
			c.Failures++
			if s.deferred == nil {
				s.deferred = map[uuid.UUID]bool{}
			}
			s.deferred[id] = true
			return c.Failures, nil
		}
	}
	return 0, data.ErrRecordNotFound
}

func (s *stubStore) CreateEvent(_ context.Context, e *eventstore.Event, courtIDs []uuid.UUID) error {
	if s.createErr != nil {
		return s.createErr
	}
	e.ID = uuid.New()
	s.created, s.createdCrts = e, courtIDs
	s.event = e
	return nil
}

func (s *stubStore) CancelEvent(_ context.Context, id, _ uuid.UUID, _ *string) (*eventstore.Event, []uuid.UUID, error) {
	if s.cancelErr != nil {
		return nil, nil, s.cancelErr
	}
	s.cancelled = append(s.cancelled, id)
	ev := *s.event
	ev.Status = eventstore.EventCancelled
	return &ev, s.paid, nil
}

func (s *stubStore) GetEvent(context.Context, uuid.UUID) (*eventstore.Event, error) {
	if s.event == nil {
		return nil, data.ErrRecordNotFound
	}
	ev := *s.event
	return &ev, nil
}

func (s *stubStore) ListEvents(context.Context, uuid.UUID, time.Time, time.Time, bool) ([]*eventstore.Event, error) {
	if s.event == nil {
		return nil, nil
	}
	return []*eventstore.Event{s.event}, nil
}

func (s *stubStore) SignUp(_ context.Context, in eventstore.SignUpInput) (*eventstore.Registration, string, error) {
	if s.signUpErr != nil {
		return nil, "", s.signUpErr
	}
	s.signedUp = append(s.signedUp, in)
	reg := &eventstore.Registration{
		ID: uuid.New(), EventID: in.EventID, ComplexID: s.event.ComplexID, FirstName: in.FirstName, LastName: in.LastName,
		Phone: in.Phone, Email: in.Email, Status: eventstore.RegistrationHeld, Price: s.event.Price, ServiceFee: in.ServiceFee,
		HeldUntil: time.Now().Add(15 * time.Minute), CreatedAt: time.Now(),
	}
	s.remember(reg, "token-"+reg.ID.String())
	return reg, "token-" + reg.ID.String(), nil
}

func (s *stubStore) remember(reg *eventstore.Registration, token string) {
	if s.regs == nil {
		s.regs, s.byToken = map[uuid.UUID]*eventstore.Registration{}, map[string]*eventstore.Registration{}
	}
	s.regs[reg.ID] = reg
	s.byToken[token] = reg
}

func (s *stubStore) GetRegistration(_ context.Context, id uuid.UUID) (*eventstore.Registration, error) {
	if reg, ok := s.regs[id]; ok {
		return reg, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubStore) GetRegistrationByToken(_ context.Context, token string) (*eventstore.Registration, error) {
	if reg, ok := s.byToken[token]; ok {
		return reg, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubStore) ListRegistrations(context.Context, uuid.UUID, bool) ([]*eventstore.RegistrationRow, error) {
	return s.rows, nil
}

func (s *stubStore) CancelRegistration(_ context.Context, id uuid.UUID, by string) (*eventstore.Registration, error) {
	reg, ok := s.regs[id]
	if !ok || (reg.Status != eventstore.RegistrationHeld && reg.Status != eventstore.RegistrationConfirmed) {
		return nil, eventstore.ErrNotCancellable
	}
	reg.Status = eventstore.RegistrationCancelled
	reg.CancelledBy = &by
	s.released = append(s.released, id)
	s.participant = append(s.participant, by)
	return reg, nil
}

func (s *stubStore) CancelByParticipant(ctx context.Context, id uuid.UUID, refundOwed bool) (*eventstore.ParticipantCancel, error) {
	reg, err := s.CancelRegistration(ctx, id, eventstore.CancelledByParticipant)
	if err != nil {
		return nil, err
	}
	s.owed = append(s.owed, refundOwed && !s.drawnAtCancel)
	return &eventstore.ParticipantCancel{Registration: reg, Drawn: s.drawnAtCancel}, nil
}

func (s *stubStore) CancelByOwner(ctx context.Context, id uuid.UUID) (*eventstore.ParticipantCancel, error) {
	reg, err := s.CancelRegistration(ctx, id, eventstore.CancelledByOwner)
	if err != nil {
		return nil, err
	}
	s.owed = append(s.owed, true)
	return &eventstore.ParticipantCancel{Registration: reg, Drawn: s.drawnAtCancel}, nil
}

func (s *stubStore) ExpireHolds(context.Context) ([]eventstore.ExpiredHold, error) {
	for _, h := range s.expired {
		if reg, ok := s.regs[h.ID]; ok {
			reg.Status = eventstore.RegistrationExpired
		}
		s.openCheck(h.ID, h.ComplexID, h.EventID, true)
	}
	expired := s.expired
	s.expired = nil
	return expired, nil
}

func (s *stubStore) DueReminders(_ context.Context, now time.Time) ([]eventstore.DueReminder, error) {
	s.dueAt = append(s.dueAt, now)
	var out []eventstore.DueReminder
	for _, d := range s.due {
		if !slices.Contains(s.reminded, d.RegistrationID) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *stubStore) MarkReminded(_ context.Context, id uuid.UUID) error {
	s.reminded = append(s.reminded, id)
	return nil
}

func (s *stubStore) OwnerCancelledStillPaid(context.Context, time.Duration, int) ([]eventstore.PendingRefund, error) {
	return s.pending, nil
}

type stubComplexes struct{ complex *complexstore.Complex }

func (s *stubComplexes) GetByID(context.Context, uuid.UUID) (*complexstore.Complex, error) {
	return s.complex, nil
}

func (s *stubComplexes) GetBySlug(context.Context, string) (*complexstore.Complex, error) {
	return s.complex, nil
}

func (s *stubComplexes) RefreshMPCredentials(context.Context, uuid.UUID, string) (string, error) {
	return "", nil
}

type stubCourts struct{ courts []*courtstore.Court }

func (s *stubCourts) GetByComplex(context.Context, uuid.UUID) ([]*courtstore.Court, error) {
	return s.courts, nil
}

type stubClients struct{ blocked bool }

func (s *stubClients) GetByPhone(context.Context, uuid.UUID, string) (*clientstore.Client, error) {
	if s.blocked {
		return &clientstore.Client{IsBlocked: true}, nil
	}
	return nil, data.ErrRecordNotFound
}

type stubPayments struct {
	inserted []*paymentstore.Payment
	refunded []uuid.UUID
	outcome  paymentstore.RefundOutcome
}

func (s *stubPayments) Insert(_ context.Context, p *paymentstore.Payment) error {
	s.inserted = append(s.inserted, p)
	return nil
}

func (s *stubPayments) RefundRegistration(_ context.Context, id uuid.UUID) paymentstore.RefundOutcome {
	s.refunded = append(s.refunded, id)
	return s.outcome
}

type stubCheckout struct {
	err   error
	input []mp.CreatePreferenceInput
	// found is what a payment search answers per reference, searchErr makes it
	// fail, and searched records each search's reference and caller.
	found     map[string][]mp.Payment
	searchErr error
	searched  []string
	callers   []mp.Caller
	// onSearch runs inside the search, for what happens meanwhile elsewhere.
	onSearch func()
}

func (s *stubCheckout) SearchPaymentsByReference(_ context.Context, reference string, caller mp.Caller) ([]mp.Payment, error) {
	s.searched = append(s.searched, reference)
	s.callers = append(s.callers, caller)
	if s.onSearch != nil {
		s.onSearch()
	}
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.found[reference], nil
}

// stubReconciler stands in for the payments service: it records each payment
// handed to it and, like the real processing on a spot with room, confirms the
// registration the payment names (confirms), or fails as an unrecordable event.
type stubReconciler struct {
	err        error
	reconciled []string
	confirms   map[string]*eventstore.Registration
}

func (s *stubReconciler) ReconcileApprovedPayment(_ context.Context, mpPaymentID string) error {
	s.reconciled = append(s.reconciled, mpPaymentID)
	if s.err != nil {
		return s.err
	}
	if reg, ok := s.confirms[mpPaymentID]; ok {
		reg.Status = eventstore.RegistrationConfirmed
	}
	return nil
}

func (s *stubCheckout) CreatePreference(_ context.Context, in mp.CreatePreferenceInput) (*mp.Preference, error) {
	s.input = append(s.input, in)
	if s.err != nil {
		return nil, s.err
	}
	return &mp.Preference{ID: "pref-1", InitPoint: "https://mp.test/pay"}, nil
}

type stubNotifier struct {
	cancelled      []notifications.EventCancellation
	ownerCancelled []notifications.OwnerEventNotice
	reminders      []notifications.EventReminder
	// refuseReminders makes EventReminderDue fail, as a refused enqueue does.
	refuseReminders bool
}

func (s *stubNotifier) OwnerEventCancelled(n notifications.OwnerEventNotice) {
	s.ownerCancelled = append(s.ownerCancelled, n)
}

func (s *stubNotifier) EventReminderDue(r notifications.EventReminder) error {
	if s.refuseReminders {
		return errBoom
	}
	s.reminders = append(s.reminders, r)
	return nil
}

func (s *stubNotifier) EventCancelled(n notifications.EventCancellation) {
	s.cancelled = append(s.cancelled, n)
}

type stubRealtime struct{ published int }

func (s *stubRealtime) PublishBookingChanged(uuid.UUID) { s.published++ }

type stubAudit struct{ entries []audit.Entry }

func (s *stubAudit) Record(e audit.Entry) { s.entries = append(s.entries, e) }

type fixture struct {
	svc       *Service
	store     *stubStore
	complexes *stubComplexes
	courts    *stubCourts
	clients   *stubClients
	payments  *stubPayments
	checkout  *stubCheckout
	reconcile *stubReconciler
	notify    *stubNotifier
	realtime  *stubRealtime
	audit     *stubAudit
	complex   *complexstore.Complex
	owner     uuid.UUID
}

// newFixture wires a venue with two active courts and MercadoPago connected, and
// an event on it three days out, for 20 people at 8.000.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	complexID := uuid.New()
	token := "seller-token"
	complex := complexstore.NewComplexForTest(complexID, &token, nil)
	complex.Name, complex.Slug, complex.IsActive, complex.CancellationHours = "Vibe Padel", "vibe", true, 24

	f := &fixture{
		store:     &stubStore{},
		complexes: &stubComplexes{complex: complex},
		courts: &stubCourts{courts: []*courtstore.Court{
			{ID: uuid.New(), ComplexID: complexID, Name: "Cancha 1", IsActive: true},
			{ID: uuid.New(), ComplexID: complexID, Name: "Cancha 2", IsActive: true},
			{ID: uuid.New(), ComplexID: complexID, Name: "Cancha vieja", IsActive: false},
		}},
		clients: &stubClients{}, payments: &stubPayments{}, checkout: &stubCheckout{}, reconcile: &stubReconciler{}, notify: &stubNotifier{},
		realtime: &stubRealtime{}, audit: &stubAudit{}, complex: complex, owner: uuid.New(),
	}
	f.store.event = &eventstore.Event{
		ID: uuid.New(), ComplexID: complexID, Name: "Canchas abiertas", Sport: "padel",
		Date: time.Now().AddDate(0, 0, 3), StartTime: "19:00", DurationMinutes: 120,
		Capacity: 20, Price: 800_000, Status: eventstore.EventActive,
	}
	f.svc = NewService(Dependencies{
		Store: f.store, Complexes: f.complexes, Courts: f.courts, Clients: f.clients, Payments: f.payments,
		Checkout: f.checkout, Reconciler: f.reconcile, Notify: f.notify, Realtime: f.realtime, Audit: f.audit,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, Config{PublicURL: "https://vibe.test", BackendURL: "https://api.vibe.test", GracePeriod: 15 * time.Minute, PaymentExpiry: 15 * time.Minute, MaxHoldsPerIP: 3})
	return f
}

func (f *fixture) ownerActor() Actor { return Actor{UserID: &f.owner, IP: "203.0.113.5"} }

var errBoom = errors.New("boom")

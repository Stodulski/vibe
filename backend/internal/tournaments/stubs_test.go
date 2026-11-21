package tournaments

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
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/events"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// stubStore is the tournaments store with the behaviour a test sets.
type stubStore struct {
	detail      *tournamentstore.Detail
	created     *tournamentstore.Tournament
	createdCats []tournamentstore.NewCategory
	cancelled   int
	drawErr     error
	moveErr     error
	resultErr   error
	scheduleErr error
	draws       []tournamentstore.DrawInput
	scheduled   []tournamentstore.ScheduleInput
	results     []tournamentstore.ResultInput
	hooksFor    []string
	pairHooks   []pairHook
	drawn       bool
	feeErr      error
	feeSet      []int
	charges     []tournamentstore.ChargeInput
	undone      []int
	// due is what the match reminder sweep reads; reminded the matches it marked.
	due      []tournamentstore.DueMatch
	reminded []uuid.UUID
}

func (s *stubStore) DueMatchReminders(context.Context, time.Time) ([]tournamentstore.DueMatch, error) {
	var out []tournamentstore.DueMatch
	for _, d := range s.due {
		if !slices.Contains(s.reminded, d.MatchID) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *stubStore) MarkMatchReminded(_ context.Context, id uuid.UUID) error {
	s.reminded = append(s.reminded, id)
	return nil
}

func (s *stubStore) Create(_ context.Context, t *tournamentstore.Tournament, cats []tournamentstore.NewCategory) error {
	t.ID = uuid.New()
	s.created, s.createdCats = t, cats
	s.detail = &tournamentstore.Detail{Tournament: *t}
	for _, c := range cats {
		s.detail.Categories = append(s.detail.Categories, &tournamentstore.Category{
			ID: uuid.New(), TournamentID: t.ID, ComplexID: t.ComplexID, EventID: uuid.New(), Name: c.Name, Format: c.Format,
			Capacity: c.Capacity, Price: c.Price, GroupSize: c.GroupSize, QualifiersPerGroup: c.QualifiersPerGroup,
			SetsToWin: c.SetsToWin, GamesPerSet: c.GamesPerSet, SuperTiebreakDecider: c.SuperTiebreakDecider,
			MatchDurationMinutes: c.MatchDurationMinutes, CourtFee: c.CourtFee,
		})
	}
	return nil
}

func (s *stubStore) Get(context.Context, uuid.UUID) (*tournamentstore.Tournament, error) {
	if s.detail == nil {
		return nil, data.ErrRecordNotFound
	}
	t := s.detail.Tournament
	return &t, nil
}

func (s *stubStore) List(context.Context, uuid.UUID, time.Time, time.Time, bool) ([]*tournamentstore.Tournament, error) {
	if s.detail == nil {
		return nil, nil
	}
	t := s.detail.Tournament
	return []*tournamentstore.Tournament{&t}, nil
}

func (s *stubStore) Matches(context.Context, uuid.UUID) ([]*tournamentstore.Match, error) {
	return s.detail.Matches, nil
}

func (s *stubStore) Categories(context.Context, uuid.UUID) ([]*tournamentstore.Category, error) {
	return s.detail.Categories, nil
}

func (s *stubStore) Category(_ context.Context, _, id uuid.UUID) (*tournamentstore.Category, error) {
	for _, c := range s.detail.Categories {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubStore) Detail(context.Context, uuid.UUID) (*tournamentstore.Detail, error) {
	if s.detail == nil {
		return nil, data.ErrRecordNotFound
	}
	return s.detail, nil
}

func (s *stubStore) Cancel(context.Context, uuid.UUID, uuid.UUID, *string) (*tournamentstore.Tournament, error) {
	s.cancelled++
	t := s.detail.Tournament
	t.Status = tournamentstore.StatusCancelled
	return &t, nil
}

func (s *stubStore) CategoryOfEvent(context.Context, uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	return s.detail.Tournament.ID, s.detail.Categories[0].ID, nil
}

func (s *stubStore) PairOf(context.Context, uuid.UUID) (*tournamentstore.Pair, error) {
	return &tournamentstore.Pair{ID: uuid.New(), Status: tournamentstore.PairActive}, nil
}

func (s *stubStore) DrawGenerated(context.Context, uuid.UUID) (bool, error) { return s.drawn, nil }

func (s *stubStore) GenerateDraw(_ context.Context, in tournamentstore.DrawInput) error {
	s.draws = append(s.draws, in)
	return s.drawErr
}

func (s *stubStore) MovePair(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return s.moveErr
}

func (s *stubStore) SetResult(_ context.Context, in tournamentstore.ResultInput) (*tournamentstore.Match, error) {
	s.results = append(s.results, in)
	return nil, s.resultErr
}

func (s *stubStore) WithdrawPair(context.Context, uuid.UUID) error { return nil }

func (s *stubStore) SetCourtFee(_ context.Context, categoryID uuid.UUID, fee int) error {
	s.feeSet = append(s.feeSet, fee)
	for _, c := range s.detail.Categories {
		if c.ID == categoryID && s.feeErr == nil {
			c.CourtFee = fee
		}
	}
	return s.feeErr
}

func (s *stubStore) ChargeCourtFee(_ context.Context, in tournamentstore.ChargeInput) (*tournamentstore.MatchFee, error) {
	s.charges = append(s.charges, in)
	if s.feeErr != nil {
		return nil, s.feeErr
	}
	return &tournamentstore.MatchFee{ID: uuid.New(), MatchID: in.MatchID, Side: in.Side, Method: in.Method, Amount: 1}, nil
}

func (s *stubStore) UndoCourtFee(_ context.Context, _ uuid.UUID, side int, _ uuid.UUID) error {
	s.undone = append(s.undone, side)
	return s.feeErr
}

func (s *stubStore) Schedule(_ context.Context, in tournamentstore.ScheduleInput) (*tournamentstore.Match, error) {
	s.scheduled = append(s.scheduled, in)
	if s.scheduleErr != nil {
		return nil, s.scheduleErr
	}
	// The match the detail holds takes the court and the time, as the store's
	// does, so a later read sees it scheduled.
	for i, m := range s.detail.Matches {
		if m.ID == in.MatchID {
			moved := *m
			start := in.StartTime
			on := in.Date
			moved.CourtID, moved.ScheduledOn, moved.StartTime, moved.Duration = &in.CourtID, &on, &start, &in.Minutes
			s.detail.Matches[i] = &moved
			return &moved, nil
		}
	}
	return &tournamentstore.Match{ID: in.MatchID}, nil
}

func (s *stubStore) Unschedule(_ context.Context, id uuid.UUID) (*tournamentstore.Match, error) {
	return &tournamentstore.Match{ID: id}, nil
}

// pairHook is the pair a sign-up's hooks were built for: its payer and partner.
type pairHook struct{ payer, partner tournamentstore.Player }

func (s *stubStore) PairHooks(payer, partner tournamentstore.Player) *eventstore.SignUpHooks {
	s.hooksFor = append(s.hooksFor, payer.Phone+"+"+partner.Phone)
	s.pairHooks = append(s.pairHooks, pairHook{payer: payer, partner: partner})
	return &eventstore.SignUpHooks{}
}

// stubEvents is the events domain with the behaviour a test sets.
type stubEvents struct {
	complex   *complexstore.Complex
	event     *eventstore.Event
	cancelled []uuid.UUID
	refunds   map[uuid.UUID][]events.Refund
	signUps   []events.SignUpInput
	hooks     []*eventstore.SignUpHooks
	signUpErr error
}

func (s *stubEvents) PublicComplex(context.Context, string) (*complexstore.Complex, error) {
	return s.complex, nil
}

func (s *stubEvents) AnyEvent(context.Context, uuid.UUID, uuid.UUID) (*eventstore.Event, error) {
	return s.event, nil
}

func (s *stubEvents) SignUpWith(_ context.Context, _ events.Actor, _ *complexstore.Complex, _ *eventstore.Event, in events.SignUpInput, hooks *eventstore.SignUpHooks) (*events.SignUpResult, error) {
	s.signUps, s.hooks = append(s.signUps, in), append(s.hooks, hooks)
	if s.signUpErr != nil {
		return nil, s.signUpErr
	}
	return &events.SignUpResult{Registration: &eventstore.Registration{ID: uuid.New()}, Preference: &mp.Preference{ID: "pref", InitPoint: "https://mp.test/pay"}}, nil
}

func (s *stubEvents) CancelAny(_ context.Context, _ *complexstore.Complex, _ events.Actor, eventID uuid.UUID, _ *string) (*events.CancelResult, error) {
	s.cancelled = append(s.cancelled, eventID)
	return &events.CancelResult{Refunds: s.refunds[eventID]}, nil
}

func (s *stubEvents) Status(context.Context, string) (*events.View, error) {
	return &events.View{Event: s.event, Registration: &eventstore.Registration{}}, nil
}

func (s *stubEvents) CancelRegistration(context.Context, events.Actor, string) (*events.CancelResultPublic, error) {
	return &events.CancelResultPublic{}, nil
}

type stubCourts struct{ courts []*courtstore.Court }

func (s *stubCourts) GetByComplex(context.Context, uuid.UUID) ([]*courtstore.Court, error) {
	return s.courts, nil
}

type stubComplexes struct{ complex *complexstore.Complex }

func (s *stubComplexes) GetByID(context.Context, uuid.UUID) (*complexstore.Complex, error) {
	return s.complex, nil
}

type stubNotifier struct {
	notices []notifications.TournamentNotice
	// refuse makes TournamentNoticeDue fail, as a refused enqueue does.
	refuse bool
}

func (s *stubNotifier) TournamentNoticeDue(n notifications.TournamentNotice) error {
	if s.refuse {
		return errors.New("queue refused")
	}
	s.notices = append(s.notices, n)
	return nil
}

type stubRealtime struct{}

func (s *stubRealtime) PublishBookingChanged(uuid.UUID) {}

type stubAudit struct{ entries []audit.Entry }

func (s *stubAudit) Record(e audit.Entry) { s.entries = append(s.entries, e) }

type fixture struct {
	svc     *Service
	store   *stubStore
	events  *stubEvents
	complex *complexstore.Complex
	owner   uuid.UUID
	court   *courtstore.Court
	audit   *stubAudit
	notify  *stubNotifier
}

// newFixture wires a venue with one court and a tournament in the store, a
// category of 8 pairs for 10.000 each.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	complexID := uuid.New()
	token := "seller-token"
	complex := complexstore.NewComplexForTest(complexID, &token, nil)
	complex.Name, complex.Slug, complex.IsActive, complex.CancellationHours = "Vibe Padel", "vibe", true, 24

	tid, cid := uuid.New(), uuid.New()
	start := time.Now().AddDate(0, 0, 3)
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	cat := &tournamentstore.Category{
		ID: cid, TournamentID: tid, ComplexID: complexID, EventID: uuid.New(), Name: "6ta",
		Format: tournamentstore.FormatElimination, Capacity: 8, Price: 1_000_000, GroupSize: 3, QualifiersPerGroup: 2,
		SetsToWin: 2, GamesPerSet: 6, SuperTiebreakDecider: true, MatchDurationMinutes: 90,
	}
	f := &fixture{
		store: &stubStore{detail: &tournamentstore.Detail{
			Tournament: tournamentstore.Tournament{
				ID: tid, ComplexID: complexID, Name: "Torneo Vibe", Sport: "padel", StartDate: day, EndDate: day.AddDate(0, 0, 1),
				Status: tournamentstore.StatusActive,
			},
			Categories: []*tournamentstore.Category{cat},
		}},
		events:  &stubEvents{complex: complex, refunds: map[uuid.UUID][]events.Refund{}},
		complex: complex, owner: uuid.New(), audit: &stubAudit{}, notify: &stubNotifier{},
		court: &courtstore.Court{ID: uuid.New(), ComplexID: complexID, Name: "Cancha 1", IsActive: true},
	}
	f.events.event = &eventstore.Event{ID: cat.EventID, ComplexID: complexID, TournamentID: &tid}
	f.svc = NewService(Dependencies{
		Store: f.store, Events: f.events, Courts: &stubCourts{courts: []*courtstore.Court{f.court}},
		Realtime: &stubRealtime{}, Audit: f.audit, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Complexes: &stubComplexes{complex: complex}, Notify: f.notify, PublicURL: "https://vibe.test",
	})
	return f
}

func (f *fixture) actor() Actor                        { return Actor{UserID: &f.owner, IP: "203.0.113.5"} }
func (f *fixture) category() *tournamentstore.Category { return f.store.detail.Categories[0] }

// eventsView is a pair's spot as the events domain reads it.
func eventsView(f *fixture) *events.View {
	return &events.View{
		Event: f.events.event, Complex: f.complex,
		Registration: &eventstore.Registration{
			ID: uuid.New(), FirstName: "Ana", LastName: "Diaz", Status: eventstore.RegistrationConfirmed,
			Price: 1_000_000, ServiceFee: 20_000, HeldUntil: time.Now(),
		},
		Cancellation: events.Cancellation{CanCancel: true, WithinWindow: true, Deadline: time.Now()},
	}
}

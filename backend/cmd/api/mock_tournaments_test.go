package main

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// mockTournamentStore is an empty store of tournaments: it holds none, so a
// lookup answers not found and a list is empty. A test that needs one sets
// seeded, and every read and change then answers for it.
type mockTournamentStore struct {
	seeded *tournamentstore.Detail
}

func (m *mockTournamentStore) Create(context.Context, *tournamentstore.Tournament, []tournamentstore.NewCategory) error {
	return nil
}

func (m *mockTournamentStore) Get(context.Context, uuid.UUID) (*tournamentstore.Tournament, error) {
	if m.seeded != nil {
		t := m.seeded.Tournament
		return &t, nil
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockTournamentStore) List(context.Context, uuid.UUID, time.Time, time.Time, bool) ([]*tournamentstore.Tournament, error) {
	return nil, nil
}

func (m *mockTournamentStore) Categories(context.Context, uuid.UUID) ([]*tournamentstore.Category, error) {
	if m.seeded != nil {
		return m.seeded.Categories, nil
	}
	return nil, nil
}

func (m *mockTournamentStore) Category(_ context.Context, _, id uuid.UUID) (*tournamentstore.Category, error) {
	if m.seeded != nil {
		for _, c := range m.seeded.Categories {
			if c.ID == id {
				return c, nil
			}
		}
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockTournamentStore) Detail(context.Context, uuid.UUID) (*tournamentstore.Detail, error) {
	if m.seeded != nil {
		return m.seeded, nil
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockTournamentStore) CategoryOfEvent(context.Context, uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	return uuid.Nil, uuid.Nil, data.ErrRecordNotFound
}

func (m *mockTournamentStore) PairOf(context.Context, uuid.UUID) (*tournamentstore.Pair, error) {
	return nil, tournamentstore.ErrNotPair
}

func (m *mockTournamentStore) DrawGenerated(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

func (m *mockTournamentStore) Cancel(context.Context, uuid.UUID, uuid.UUID, *string) (*tournamentstore.Tournament, error) {
	if m.seeded != nil {
		t := m.seeded.Tournament
		return &t, nil
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockTournamentStore) GenerateDraw(context.Context, tournamentstore.DrawInput) error {
	if m.seeded != nil {
		return nil
	}
	return data.ErrRecordNotFound
}

func (m *mockTournamentStore) MovePair(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	if m.seeded != nil {
		return nil
	}
	return data.ErrRecordNotFound
}

func (m *mockTournamentStore) SetResult(context.Context, tournamentstore.ResultInput) (*tournamentstore.Match, error) {
	return m.firstMatch()
}

func (m *mockTournamentStore) firstMatch() (*tournamentstore.Match, error) {
	if m.seeded != nil && len(m.seeded.Matches) > 0 {
		return m.seeded.Matches[0], nil
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockTournamentStore) WithdrawPair(context.Context, uuid.UUID) error { return nil }

func (m *mockTournamentStore) Schedule(context.Context, tournamentstore.ScheduleInput) (*tournamentstore.Match, error) {
	return m.firstMatch()
}

func (m *mockTournamentStore) Unschedule(context.Context, uuid.UUID) (*tournamentstore.Match, error) {
	return m.firstMatch()
}

func (m *mockTournamentStore) PairHooks(string, tournamentstore.Player) *eventstore.SignUpHooks {
	return nil
}

func (m *mockTournamentStore) SetCourtFee(context.Context, uuid.UUID, int) error { return nil }

func (m *mockTournamentStore) ChargeCourtFee(context.Context, tournamentstore.ChargeInput) (*tournamentstore.MatchFee, error) {
	return &tournamentstore.MatchFee{}, nil
}

func (m *mockTournamentStore) UndoCourtFee(context.Context, uuid.UUID, int, uuid.UUID) error {
	return nil
}

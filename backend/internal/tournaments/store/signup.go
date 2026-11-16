package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
)

// PairHooks is what a pair's sign-up adds to the event sign-up, run inside its
// transaction and under the category container's row lock: the same lock that
// serializes capacity, so two pairs racing for the last spot cannot both fit.
//
// Admit refuses once the draw exists (ErrEventClosed) and when any of the two
// players already holds a live spot in the category, as the payer or as a
// partner (ErrAlreadyRegistered). Attach writes the pair beside the registration.
func (m *Store) PairHooks(payerPhone string, partner Player) *eventstore.SignUpHooks {
	return &eventstore.SignUpHooks{
		Admit: func(ctx context.Context, q *db.Queries, event db.OpenPlayEvent) error {
			cat, err := q.GetTournamentCategoryByEvent(ctx, event.ID)
			if err != nil {
				return fmt.Errorf("tournaments: read the category: %w", err)
			}
			t, err := q.GetTournament(ctx, db.GetTournamentParams{ID: cat.TournamentID})
			if err != nil {
				return fmt.Errorf("tournaments: read the tournament: %w", err)
			}
			if t.Status != StatusActive || cat.DrawGeneratedAt.Valid {
				return eventstore.ErrEventClosed
			}
			taken, err := q.PairPlayersTaken(ctx, db.PairPlayersTakenParams{
				CategoryID: cat.ID, Phones: []string{payerPhone, partner.Phone},
			})
			if err != nil {
				return fmt.Errorf("tournaments: check players: %w", err)
			}
			if taken {
				return eventstore.ErrAlreadyRegistered
			}
			return nil
		},
		Attach: func(ctx context.Context, q *db.Queries, reg db.OpenPlayRegistration) error {
			cat, err := q.GetTournamentCategoryByEvent(ctx, reg.EventID)
			if err != nil {
				return fmt.Errorf("tournaments: read the category: %w", err)
			}
			if _, err := q.InsertTournamentPair(ctx, db.InsertTournamentPairParams{
				CategoryID: cat.ID, ComplexID: cat.ComplexID, RegistrationID: reg.ID,
				PartnerFirstName: partner.FirstName, PartnerLastName: partner.LastName,
				PartnerPhone: partner.Phone, PartnerEmail: data.TextToPg(partner.Email),
			}); err != nil {
				return fmt.Errorf("tournaments: insert pair: %w", err)
			}
			return nil
		},
	}
}

// DrawGenerated reports whether the category behind a sign-up container has
// its draw. A container that is not a category has none.
func (m *Store) DrawGenerated(ctx context.Context, eventID uuid.UUID) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	cat, err := m.Q.GetTournamentCategoryByEvent(ctx, data.UUIDToPg(eventID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("tournaments: read the category: %w", err)
	}
	return cat.DrawGeneratedAt.Valid, nil
}

// PairOf returns the pair a registration belongs to, or ErrNotPair.
func (m *Store) PairOf(ctx context.Context, registrationID uuid.UUID) (*Pair, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetTournamentPairByRegistration(ctx, data.UUIDToPg(registrationID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotPair
		}
		return nil, fmt.Errorf("tournaments: read pair: %w", err)
	}
	p := pairFromDB(row)
	return &p, nil
}

// CategoryOfEvent returns the category a sign-up container belongs to, with the
// tournament id, or ErrRecordNotFound.
func (m *Store) CategoryOfEvent(ctx context.Context, eventID uuid.UUID) (tournamentID, categoryID uuid.UUID, err error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	cat, err := m.Q.GetTournamentCategoryByEvent(ctx, data.UUIDToPg(eventID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, uuid.Nil, data.ErrRecordNotFound
		}
		return uuid.Nil, uuid.Nil, fmt.Errorf("tournaments: read the category: %w", err)
	}
	return data.PgToUUID(cat.TournamentID), data.PgToUUID(cat.ID), nil
}

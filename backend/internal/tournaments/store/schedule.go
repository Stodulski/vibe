package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// ScheduleInput is an owner giving a match a court, a day and a time.
type ScheduleInput struct {
	MatchID   uuid.UUID
	CourtID   uuid.UUID
	Date      time.Time
	StartTime string
	Minutes   int
}

// Schedule gives a match a court and a time. From then on the match holds that
// court for its duration everywhere availability is decided, like an event
// does. It refuses with a *eventstore.ConflictError naming what already holds
// the hours (a booking, a blocked slot, a fixed series, an event or another
// match), with ErrOutOfDates outside the tournament's days, and with
// ErrMatchNotReady for a bye. Scheduling again moves the match.
func (m *Store) Schedule(ctx context.Context, in ScheduleInput) (*Match, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var out db.TournamentMatch
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		match, err := q.GetTournamentMatch(ctx, data.UUIDToPg(in.MatchID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("tournaments: read match: %w", err)
		}
		return m.lockedCategory(ctx, q, match.CategoryID, func(e *engine, cat db.TournamentCategory) error {
			if match.Bye {
				return ErrMatchNotReady
			}
			t, err := q.GetTournament(ctx, db.GetTournamentParams{ID: cat.TournamentID})
			if err != nil {
				return fmt.Errorf("tournaments: read tournament: %w", err)
			}
			day := timezone.Day(in.Date)
			if day.Before(data.PgToDate(t.StartDate)) || day.After(data.PgToDate(t.EndDate)) {
				return ErrOutOfDates
			}
			if err := m.checkCourt(ctx, tx, in.CourtID, cat.ComplexID); err != nil {
				return err
			}
			if err := eventstore.LockCourt(ctx, tx, in.CourtID); err != nil {
				return err
			}
			if err := eventstore.CheckCourtsFree(ctx, tx, []uuid.UUID{in.CourtID}, day, in.StartTime, in.Minutes, m.PaymentExpiry, in.MatchID); err != nil {
				return err
			}
			if err := q.ScheduleMatch(ctx, db.ScheduleMatchParams{
				ID: match.ID, CourtID: data.UUIDToPg(in.CourtID), ScheduledOn: data.DateToPg(day),
				StartTime: data.TimeStrToPg(in.StartTime), DurationMinutes: data.Int4ToPg(in.Minutes),
			}); err != nil {
				return fmt.Errorf("tournaments: schedule match: %w", err)
			}
			out, err = q.GetTournamentMatch(ctx, match.ID)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	return matchFromDB(out), nil
}

// checkCourt refuses a court that is not an active court of the complex.
func (m *Store) checkCourt(ctx context.Context, tx pgx.Tx, courtID uuid.UUID, complexID pgtype.UUID) error {
	var active bool
	err := tx.QueryRow(ctx, `SELECT is_active FROM courts WHERE id = $1 AND complex_id = $2 AND deleted_at IS NULL`,
		courtID, complexID).Scan(&active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return fmt.Errorf("tournaments: read court: %w", err)
	}
	if !active {
		return data.ErrRecordNotFound
	}
	return nil
}

// Unschedule takes a match off its court, releasing the hours.
func (m *Store) Unschedule(ctx context.Context, matchID uuid.UUID) (*Match, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var out db.TournamentMatch
	err := m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		match, err := q.GetTournamentMatch(ctx, data.UUIDToPg(matchID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("tournaments: read match: %w", err)
		}
		return m.lockedCategory(ctx, q, match.CategoryID, func(*engine, db.TournamentCategory) error {
			if err := q.UnscheduleMatch(ctx, match.ID); err != nil {
				return fmt.Errorf("tournaments: unschedule: %w", err)
			}
			out, err = q.GetTournamentMatch(ctx, match.ID)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	return matchFromDB(out), nil
}

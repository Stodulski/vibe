package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// PlaceStore is the sqlc-backed store of the place waitlist: the entries that
// wait for one spot of an event (open play or a tournament category) or one
// court slot. Its rows are tenant-scoped; the join runs on a context scoped to
// the complex it names and the sweep and cleanup on the cross-tenant bypass.
type PlaceStore struct {
	Q *db.Queries
}

// NewSlotEntry is one address waiting for one court slot on one grid day.
type NewSlotEntry struct {
	ComplexID       uuid.UUID
	Email           string
	CourtID         uuid.UUID
	Day             time.Time
	StartTime       string // "HH:MM"
	DurationMinutes int
}

// FreedEventEntry is an event entry claimed by the sweep because its event has
// a free spot, with what its email says. TournamentID and CategoryName are set
// when the event is a tournament category's sign-up container.
type FreedEventEntry struct {
	ID           uuid.UUID
	Email        string
	EventID      uuid.UUID
	EventName    string
	EventDay     time.Time
	EventStart   string // "HH:MM"
	TournamentID *uuid.UUID
	CategoryName string
	ComplexName  string
	ComplexSlug  string
}

// SlotGrid names one storefront grid that slot entries wait on: one complex,
// one day, one duration. It is also the sweep's cursor over those grids.
type SlotGrid struct {
	ComplexID       uuid.UUID
	ComplexSlug     string
	Day             time.Time
	DurationMinutes int
}

// FreedSlotEntry is a slot entry claimed because its slot is on sale again,
// with what its email says.
type FreedSlotEntry struct {
	ID          uuid.UUID
	Email       string
	CourtID     uuid.UUID
	CourtName   string
	Day         time.Time
	StartTime   string // "HH:MM"
	ComplexName string
	ComplexSlug string
}

// notFound turns "no row" into data.ErrRecordNotFound.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return data.ErrRecordNotFound
	}
	return err
}

// WaitlistableEvent reports whether eventID is an open-play event of complexID
// a person can still wait for (live, not started at now, the product's
// wall-clock) and how many spots it has left. Anything else answers
// data.ErrRecordNotFound.
func (s *PlaceStore) WaitlistableEvent(ctx context.Context, complexID, eventID uuid.UUID, now time.Time) (int, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := s.Q.GetWaitlistableOpenPlayEvent(ctx, db.GetWaitlistableOpenPlayEventParams{
		EventID: data.UUIDToPg(eventID), ComplexID: data.UUIDToPg(complexID),
		NowDate: data.DateToPg(now), NowTime: data.TimeStrToPg(now.Format("15:04")),
	})
	if err != nil {
		return 0, notFound(err)
	}
	return int(row.SpotsLeft), nil
}

// WaitlistableCategoryEvent returns the sign-up container of a tournament
// category of complexID a person can still wait for (live, sign-up open, not
// begun at now) and its pair spots left. Anything else answers
// data.ErrRecordNotFound.
func (s *PlaceStore) WaitlistableCategoryEvent(ctx context.Context, complexID, categoryID uuid.UUID, now time.Time) (uuid.UUID, int, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := s.Q.GetWaitlistableCategoryEvent(ctx, db.GetWaitlistableCategoryEventParams{
		CategoryID: data.UUIDToPg(categoryID), ComplexID: data.UUIDToPg(complexID),
		NowDate: data.DateToPg(now), NowTime: data.TimeStrToPg(now.Format("15:04")),
	})
	if err != nil {
		return uuid.Nil, 0, notFound(err)
	}
	return data.PgToUUID(row.ID), int(row.SpotsLeft), nil
}

// CountWaiting returns how many places email is still waiting for at complexID.
func (s *PlaceStore) CountWaiting(ctx context.Context, complexID uuid.UUID, email string) (int, error) {
	if err := data.AssertTenant(ctx, complexID); err != nil {
		return 0, err
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	n, err := s.Q.CountWaitingPlaceEntriesOfEmail(ctx, db.CountWaitingPlaceEntriesOfEmailParams{
		ComplexID: data.UUIDToPg(complexID), Email: email,
	})
	return int(n), err
}

// WaitlistableCourt reports whether courtID is an active court of complexID.
// Anything else answers data.ErrRecordNotFound.
func (s *PlaceStore) WaitlistableCourt(ctx context.Context, complexID, courtID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := s.Q.GetWaitlistableCourt(ctx, db.GetWaitlistableCourtParams{
		CourtID: data.UUIDToPg(courtID), ComplexID: data.UUIDToPg(complexID),
	})
	return notFound(err)
}

// InsertEvent records that email waits for a spot of eventID. A repeat is a no-op.
func (s *PlaceStore) InsertEvent(ctx context.Context, complexID, eventID uuid.UUID, email string) error {
	if err := data.AssertTenant(ctx, complexID); err != nil {
		return err
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.InsertPlaceWaitlistOpenPlay(ctx, db.InsertPlaceWaitlistOpenPlayParams{
		ComplexID: data.UUIDToPg(complexID), Email: email, EventID: data.UUIDToPg(eventID),
	})
}

// InsertSlot records that an address waits for one court slot. A repeat is a no-op.
func (s *PlaceStore) InsertSlot(ctx context.Context, e NewSlotEntry) error {
	if err := data.AssertTenant(ctx, e.ComplexID); err != nil {
		return err
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.InsertPlaceWaitlistSlot(ctx, db.InsertPlaceWaitlistSlotParams{
		ComplexID: data.UUIDToPg(e.ComplexID), Email: e.Email, CourtID: data.UUIDToPg(e.CourtID),
		Date: data.DateToPg(e.Day), StartTime: data.TimeStrToPg(e.StartTime),
		//nolint:gosec // G115: validated by the handler (1..720).
		DurationMinutes: int32(e.DurationMinutes),
	})
}

// ClaimFreedEvents marks notified, and returns, up to limit event entries whose
// event has a free spot and has not begun at now. A row is claimed at most once.
func (s *PlaceStore) ClaimFreedEvents(ctx context.Context, now time.Time, limit int) ([]FreedEventEntry, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.Q.ClaimFreedOpenPlayWaitlist(ctx, db.ClaimFreedOpenPlayWaitlistParams{
		NowDate: data.DateToPg(now), NowTime: data.TimeStrToPg(now.Format("15:04")),
		//nolint:gosec // G115: a small constant batch size.
		BatchLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]FreedEventEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, FreedEventEntry{
			ID: data.PgToUUID(r.ID), Email: r.Email, EventID: data.PgToUUID(r.EventID), EventName: r.EventName,
			EventDay: data.PgToDate(r.EventDate), EventStart: data.PgToTimeStr(r.EventStartTime),
			TournamentID: data.PgToUUIDPtr(r.TournamentID), CategoryName: r.CategoryName,
			ComplexName: r.ComplexName, ComplexSlug: r.ComplexSlug,
		})
	}
	return out, nil
}

// ListWaitingGrids returns up to limit of the distinct grids slot entries still
// wait on, from fromDay, in key order after the cursor after (nil: from the
// first). A page shorter than limit means the walk reached the end.
func (s *PlaceStore) ListWaitingGrids(ctx context.Context, fromDay time.Time, after *SlotGrid, limit int) ([]SlotGrid, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	params := db.ListWaitingSlotGridsParams{
		FromDate: data.DateToPg(fromDay),
		//nolint:gosec // G115: a small constant batch size.
		BatchLimit: int32(limit),
	}
	if after != nil {
		params.AfterDate = data.DateToPg(after.Day)
		params.AfterComplexID = data.UUIDToPg(after.ComplexID)
		params.AfterDuration = data.Int4ToPg(after.DurationMinutes)
	}
	rows, err := s.Q.ListWaitingSlotGrids(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]SlotGrid, 0, len(rows))
	for _, r := range rows {
		out = append(out, SlotGrid{
			ComplexID: data.PgToUUID(r.ComplexID), ComplexSlug: r.ComplexSlug,
			Day: data.PgToDate(r.Date), DurationMinutes: data.PgToInt(r.DurationMinutes),
		})
	}
	return out, nil
}

// ClaimFreeSlots marks notified, and returns, every entry still waiting on grid
// whose position is among positions, each "<court id> HH:MM". A row is claimed
// at most once.
func (s *PlaceStore) ClaimFreeSlots(ctx context.Context, grid SlotGrid, positions []string) ([]FreedSlotEntry, error) {
	if len(positions) == 0 {
		return nil, nil
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := s.Q.ClaimFreeSlotEntries(ctx, db.ClaimFreeSlotEntriesParams{
		ComplexID: data.UUIDToPg(grid.ComplexID), Date: data.DateToPg(grid.Day),
		//nolint:gosec // G115: validated by the handler (1..720).
		DurationMinutes: int32(grid.DurationMinutes), Positions: positions,
	})
	if err != nil {
		return nil, err
	}
	out := make([]FreedSlotEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, FreedSlotEntry{
			ID: data.PgToUUID(r.ID), Email: r.Email, CourtID: data.PgToUUID(r.CourtID), CourtName: r.CourtName,
			Day: data.PgToDate(r.Date), StartTime: data.PgToTimeStr(r.StartTime),
			ComplexName: r.ComplexName, ComplexSlug: r.ComplexSlug,
		})
	}
	return out, nil
}

// DeletePast removes the entries whose place has gone: slot entries of a grid
// day before yesterday, and event entries whose event was cancelled or whose day
// is before today. It is the daily cleanup and runs on the cross-tenant bypass.
func (s *PlaceStore) DeletePast(ctx context.Context, today time.Time) (int64, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return s.Q.DeletePastPlaceWaitlistEntries(ctx, db.DeletePastPlaceWaitlistEntriesParams{
		SlotBefore: data.DateToPg(today.AddDate(0, 0, -1)), Today: data.DateToPg(today),
	})
}

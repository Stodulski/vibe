package bookings

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// similarity scores how alike two courts are: the same floor counts most, then
// covered or open, then lighting.
func similarity(a, b *courtstore.Court) int {
	score := 0
	if a.Surface != nil && b.Surface != nil && *a.Surface == *b.Surface {
		score += 4
	}
	if a.CourtType == b.CourtType {
		score += 2
	}
	if a.HasLighting == b.HasLighting {
		score++
	}
	return score
}

// otherCourts are the active courts of the chosen court's complex and sport,
// the chosen one apart, the most alike to it first.
func (s *Service) otherCourts(ctx context.Context, chosen *Series) ([]*courtstore.Court, error) {
	all, err := s.courts.GetByComplex(ctx, chosen.ComplexID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(all, func(c *courtstore.Court) bool { return c.ID == chosen.CourtID })
	if i < 0 {
		return nil, nil
	}
	own := all[i]
	var out []*courtstore.Court
	for _, c := range all {
		if c.ID != own.ID && c.IsActive && c.Sport == own.Sport {
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b *courtstore.Court) int {
		return cmp.Compare(similarity(own, b), similarity(own, a))
	})
	return out, nil
}

// freeDates is the dates of conflicts the series would not collide on if it
// were on court c, using the same checks as creating it. ok is false when the
// series could not be on that court at all (no price rule covers its hours).
func (s *Service) freeDates(ctx context.Context, ser *Series, c *courtstore.Court, conflicts []SeriesConflict) (free map[time.Time]bool, ok bool) {
	candidate := *ser
	candidate.CourtID = c.ID
	if s.seriesWideChecks(ctx, &candidate) != nil {
		return nil, false
	}
	found, err := s.seriesConflicts(ctx, &candidate)
	if err != nil {
		return nil, false
	}
	busy := map[time.Time]bool{}
	for _, f := range found {
		busy[seriesDay(f.Date)] = true
	}
	free = map[time.Time]bool{}
	for _, d := range conflicts {
		if !busy[seriesDay(d.Date)] {
			free[seriesDay(d.Date)] = true
		}
	}
	return free, true
}

// seriesFreeCourts fills, on each conflict, the other courts free on that date
// at the series' hours. It is a hint: a read that fails offers nothing rather
// than failing the refusal.
func (s *Service) seriesFreeCourts(ctx context.Context, ser *Series, conflicts []SeriesConflict) {
	for i := range conflicts {
		conflicts[i].FreeCourts = []CourtRef{}
	}
	courts, err := s.otherCourts(ctx, ser)
	if err != nil {
		return
	}
	for _, c := range courts {
		free, ok := s.freeDates(ctx, ser, c, conflicts)
		if !ok {
			continue
		}
		for i := range conflicts {
			if free[seriesDay(conflicts[i].Date)] {
				conflicts[i].FreeCourts = append(conflicts[i].FreeCourts, CourtRef{
					CourtID: c.ID, CourtName: c.Name, CourtType: c.CourtType, Surface: c.Surface, HasLighting: c.HasLighting,
				})
			}
		}
	}
}

// checkDateCourts validates the dates the owner moved to other courts and
// returns them by date. Each must be a date the chosen court cannot hold, on
// another active court of the same sport, once; and the picked court must still
// be free that day (409 otherwise). Nothing is written.
func (s *Service) checkDateCourts(ctx context.Context, ser *Series, conflicts []SeriesConflict, picks []SeriesDateCourt) (map[time.Time]uuid.UUID, error) {
	moves := map[time.Time]uuid.UUID{}
	if len(picks) == 0 {
		return moves, nil
	}
	courts, err := s.otherCourts(ctx, ser)
	if err != nil {
		return nil, err
	}
	free := map[uuid.UUID]map[time.Time]bool{}
	for _, p := range picks {
		day := seriesDay(p.Date)
		switch {
		case !slices.ContainsFunc(conflicts, func(c SeriesConflict) bool { return seriesDay(c.Date).Equal(day) }):
			return nil, &FieldError{Field: "date_courts", Message: fmt.Sprintf("%s is not a date the chosen court holds", day.Format(time.DateOnly))}
		case moves[day] != uuid.Nil:
			return nil, &FieldError{Field: "date_courts", Message: fmt.Sprintf("%s is listed twice", day.Format(time.DateOnly))}
		}
		i := slices.IndexFunc(courts, func(c *courtstore.Court) bool { return c.ID == p.CourtID })
		if i < 0 {
			return nil, &FieldError{Field: "date_courts", Message: "must be another active court of the same sport"}
		}
		if _, seen := free[p.CourtID]; !seen {
			dates, _ := s.freeDates(ctx, ser, courts[i], conflicts)
			free[p.CourtID] = dates
		}
		if !free[p.CourtID][day] {
			return nil, &ConflictError{Message: fmt.Sprintf("%s is not free on %s", courts[i].Name, day.Format(time.DateOnly))}
		}
		moves[day] = p.CourtID
	}
	return moves, nil
}

// movedBookings prepares the normal booking of each moved date on its court,
// priced like the series' own occurrences. The store links them to the series
// when it writes them.
func (s *Service) movedBookings(
	ctx context.Context, complex *complexstore.Complex, ser *Series, client *clientstore.Client, moves map[time.Time]uuid.UUID,
) ([]*bookingstore.Booking, error) {
	dates := make([]time.Time, 0, len(moves))
	for d := range moves {
		dates = append(dates, d)
	}
	slices.SortFunc(dates, time.Time.Compare)
	out := make([]*bookingstore.Booking, 0, len(dates))
	for _, d := range dates {
		in := seriesOccurrence(ser, client, d)
		in.CourtID, in.seriesID = moves[d], nil
		createdBy := ser.CreatedBy
		p, err := s.prepareCreate(ctx, complex, Actor{UserID: &createdBy}, in)
		if err != nil {
			return nil, err
		}
		out = append(out, p.booking)
	}
	return out, nil
}

// skippedAndExceptions splits the conflicts into the dates that stay skipped,
// and the exceptions to record for them: a moved date is neither.
func skippedAndExceptions(conflicts []SeriesConflict, moves map[time.Time]uuid.UUID) ([]SeriesConflict, []bookingstore.SeriesException) {
	var skipped []SeriesConflict
	var exceptions []bookingstore.SeriesException
	for _, c := range conflicts {
		if _, moved := moves[seriesDay(c.Date)]; !moved {
			skipped = append(skipped, c)
			exceptions = append(exceptions, bookingstore.SeriesException{Date: c.Date, Reason: c.Reason})
		}
	}
	return skipped, exceptions
}

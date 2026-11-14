package bookings

import (
	"cmp"
	"context"
	"slices"

	"github.com/google/uuid"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// maxCourtAlternatives caps the courts offered when the chosen one is taken.
const maxCourtAlternatives = 3

// CourtAlternative is another court of the same complex and sport that holds the
// series' hours better than the one the owner chose: Conflicts is how many dates
// something else holds there, 0 when it is free every week.
type CourtAlternative struct {
	CourtID   uuid.UUID
	CourtName string
	Conflicts int
}

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

// rankAlternatives orders free courts first, then the ones with fewer clashes,
// the more similar first within each, and keeps the best few.
func rankAlternatives(chosen *courtstore.Court, courts []*courtstore.Court, clashes map[uuid.UUID]int) []CourtAlternative {
	slices.SortStableFunc(courts, func(a, b *courtstore.Court) int {
		return cmp.Or(
			cmp.Compare(clashes[a.ID], clashes[b.ID]),
			cmp.Compare(similarity(chosen, b), similarity(chosen, a)),
		)
	})
	out := make([]CourtAlternative, 0, maxCourtAlternatives)
	for _, c := range courts {
		if len(out) == maxCourtAlternatives {
			break
		}
		out = append(out, CourtAlternative{CourtID: c.ID, CourtName: c.Name, Conflicts: clashes[c.ID]})
	}
	return out
}

// seriesAlternatives looks for other courts of the chosen court's complex and
// sport that the same series fits better, using the same checks as creating it.
// It is a hint: a read that fails offers nothing rather than failing the refusal.
func (s *Service) seriesAlternatives(ctx context.Context, ser *Series, taken int) []CourtAlternative {
	all, err := s.courts.GetByComplex(ctx, ser.ComplexID)
	if err != nil {
		return nil
	}
	var chosen *courtstore.Court
	for _, c := range all {
		if c.ID == ser.CourtID {
			chosen = c
		}
	}
	if chosen == nil {
		return nil
	}
	clashes := map[uuid.UUID]int{}
	var fits []*courtstore.Court
	for _, c := range all {
		if c.ID == chosen.ID || !c.IsActive || c.Sport != chosen.Sport {
			continue
		}
		candidate := *ser
		candidate.CourtID = c.ID
		if s.seriesWideChecks(ctx, &candidate) != nil {
			continue
		}
		found, err := s.seriesConflicts(ctx, &candidate)
		if err != nil || len(found) >= taken {
			continue
		}
		clashes[c.ID] = len(found)
		fits = append(fits, c)
	}
	return rankAlternatives(chosen, fits, clashes)
}

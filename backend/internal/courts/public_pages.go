package courts

import (
	"context"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/indexnow"
)

// PageIndexer is told about every committed court write, so search engines hear
// that the complex's page and its city hub changed: a court's sport, count and
// price band are on both. indexnow.Service is the production implementation.
// A nil PageIndexer publishes nothing.
type PageIndexer interface {
	ComplexChanged(c indexnow.Change)
}

// pageChanged reports a committed court write. Callers invoke it only after the
// write has succeeded, so a refused write announces nothing.
//
// A court write never moves a complex's slug, city or switch, so the change
// reads the same before and after. If the complex cannot be read the write still
// stands and nothing is announced: the announcement is best effort and must not
// turn a committed write into an error.
func (s *Service) pageChanged(ctx context.Context, complexID uuid.UUID) {
	if s.pages == nil {
		return
	}
	complex, err := s.complexes.GetByID(ctx, complexID)
	if err != nil {
		return
	}
	snap := indexnow.SnapshotOf(complex)
	s.pages.ComplexChanged(indexnow.Change{
		ComplexID: complexID,
		Before:    snap,
		After:     snap,
		RequestID: httpx.RequestIDFromContext(ctx),
	})
}

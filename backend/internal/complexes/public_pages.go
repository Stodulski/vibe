package complexes

import (
	"context"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/indexnow"
)

// PageIndexer is told about every committed change to a complex's public page,
// so search engines hear about it. indexnow.Service is the production
// implementation. A nil PageIndexer publishes nothing.
type PageIndexer interface {
	ComplexChanged(c indexnow.Change)
}

// pageChanged reports a committed write. Callers invoke it only after the write
// has succeeded, so a refused or failed write announces nothing.
func (s *Service) pageChanged(ctx context.Context, c indexnow.Change) {
	if s.pages == nil {
		return
	}
	c.RequestID = httpx.RequestIDFromContext(ctx)
	s.pages.ComplexChanged(c)
}

// publicSnapshot is how a complex's public presence reads right now: the slug
// its page answers under, its city, and whether it is served (switched on).
// Deleted complexes never reach the service's readers, so they are not checked here.
func publicSnapshot(c *complexstore.Complex) indexnow.Snapshot {
	return indexnow.SnapshotOf(c)
}

//go:build integration

package complexes

import (
	"sync"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/data/datatest"
)

// The same race as TestConcurrentRefreshesOfOneVenueCallMercadoPagoOnce, with
// the real lease lock (job_locks) between the callers instead of the in-memory
// one: the refresh serializes through Postgres, so it holds across instances.
func TestConcurrentRefreshesSerializeThroughTheJobLocksTable(t *testing.T) {
	pool := datatest.SetupTestDB(t)
	w := newRefreshWorld(t)
	w.f.service.locks = &data.LockModel{DB: data.NewDB(pool)}
	w.mercado.delay = 50 * time.Millisecond

	const callers = 4
	tokens := make([]string, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			tokens[i], errs[i] = w.f.service.RefreshMPCredentials(t.Context(), w.id, "access-0")
		})
	}
	wg.Wait()

	if got := w.mercado.callCount(); got != 1 {
		t.Fatalf("want MercadoPago called once across %d callers; got %d", callers, got)
	}
	for i := range callers {
		if errs[i] != nil || tokens[i] != "access-1" {
			t.Errorf("caller %d: want access-1; got %q, %v", i, tokens[i], errs[i])
		}
	}
}

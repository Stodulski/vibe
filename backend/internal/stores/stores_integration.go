//go:build integration

package stores

import "github.com/stodulski/vibe-server/internal/data"

// NewOver builds the same set of stores New builds, over a database handle the
// caller already has rather than over a pool New would wrap one around.
//
// It is the composition root's counterpart to data.NewDBOver / data.NewDBOverTx,
// and it exists for one caller: the integration fixture
// (internal/data/datatest), which runs a test inside a transaction it rolls
// back and therefore needs every store bound to that transaction's handle.
// Duplicating newStores' nineteen lines in the fixture instead would mean a
// store added to the application is a store the integration suite silently
// never exercises.
//
// It is behind the integration build tag, so it is not part of the package the
// application compiles: no production binary can reach it.
func NewOver(pooled *data.DB, cfg Config) Stores {
	return newStores(pooled, cfg)
}

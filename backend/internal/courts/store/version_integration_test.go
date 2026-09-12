//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// API-08. Two staff members editing the same court from two browsers is
// last-write-wins without a version: both load the row, both PUT every column
// back, and the second silently overwrites the first with a value it read
// before that first write existed. These run against a real database because
// the counter is a trigger — the one place a stub would agree with the code and
// both would be wrong.

func TestTheVersionMovesOnEveryWriteWhetherTheWriterChecksItOrNot(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	court, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if court.Version < 1 {
		t.Fatalf("a fresh row starts at version %d, want at least 1", court.Version)
	}
	first := court.Version

	// A writer that sends no precondition still moves the counter: a version
	// that stands still while the row changes is worse than no version, because
	// it is a lock that reports success.
	court.Name = "Cancha Uno"
	if err := f.Stores.Courts.Update(f.Scoped(ctx), court, nil); err != nil {
		t.Fatalf("Update with no precondition: %v", err)
	}
	if court.Version != first+1 {
		t.Errorf("version = %d after an unconditional write, want %d", court.Version, first+1)
	}

	reread, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID after write: %v", err)
	}
	if reread.Version != court.Version {
		t.Errorf("the row reads version %d, the write reported %d", reread.Version, court.Version)
	}
}

func TestASecondTabWithAStaleVersionIsRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	firstTab, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID (first tab): %v", err)
	}
	secondTab, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID (second tab): %v", err)
	}
	staleVersion := secondTab.Version

	firstTab.Name = "Renamed By The First Tab"
	if err := f.Stores.Courts.Update(f.Scoped(ctx), firstTab, &firstTab.Version); err != nil {
		t.Fatalf("first Update (should win the race): %v", err)
	}

	secondTab.Name = "Renamed By The Second Tab"
	err = f.Stores.Courts.Update(f.Scoped(ctx), secondTab, &staleVersion)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("second Update with a stale version = %v, want ErrRecordNotFound", err)
	}

	final, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID (final): %v", err)
	}
	if final.Name != "Renamed By The First Tab" {
		t.Errorf("the winning writer's change was lost; Name = %q", final.Name)
	}

	// And the losing tab can retry against the row it can now re-read.
	secondTab.Version = final.Version
	if err := f.Stores.Courts.Update(f.Scoped(ctx), secondTab, &final.Version); err != nil {
		t.Fatalf("retry against the current version: %v", err)
	}
}

// Without a precondition the endpoint behaves exactly as it did before versions
// existed, which is what lets the frontend adopt this on its own schedule.
func TestAWriteWithNoVersionIsStillLastWriteWins(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	firstTab, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID (first tab): %v", err)
	}
	secondTab, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID (second tab): %v", err)
	}

	firstTab.Name = "First"
	if err := f.Stores.Courts.Update(f.Scoped(ctx), firstTab, nil); err != nil {
		t.Fatalf("first Update: %v", err)
	}
	secondTab.Name = "Second"
	if err := f.Stores.Courts.Update(f.Scoped(ctx), secondTab, nil); err != nil {
		t.Fatalf("second Update with no precondition should not be refused: %v", err)
	}

	final, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID (final): %v", err)
	}
	if final.Name != "Second" {
		t.Errorf("Name = %q, want the last writer's value", final.Name)
	}
}

// The price bands are deleted and reinserted on every save, so a band's own
// version cannot be anybody's precondition. The court's version is the price
// set's version, and replacing the set has to move it.
func TestReplacingThePricesMovesTheCourtsVersion(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	before, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	prices := []*courtstore.CourtPrice{
		{CourtID: f.CourtID, Price: 12_000, DayType: "monday", TimeFrom: "08:00", TimeTo: "20:00"},
	}
	if _, err := f.Stores.Courts.ReplacePrices(ctx, f.CourtID, prices, &before.Version); err != nil {
		t.Fatalf("ReplacePrices against the current version: %v", err)
	}

	after, err := f.Stores.Courts.GetByID(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetByID after ReplacePrices: %v", err)
	}
	if after.Version != before.Version+1 {
		t.Fatalf("court version = %d after replacing its prices, want %d", after.Version, before.Version+1)
	}

	// The version the caller held is now stale, and a second save with it must
	// be refused — otherwise the second tab's price table silently replaces the
	// first tab's.
	stale := before.Version
	_, err = f.Stores.Courts.ReplacePrices(ctx, f.CourtID, prices, &stale)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("ReplacePrices with a stale version = %v, want ErrRecordNotFound", err)
	}

	// And nothing was written: the bump is checked before the delete, so a
	// refused replace leaves the existing bands alone.
	stillThere, err := f.Stores.Courts.GetPrices(ctx, f.CourtID)
	if err != nil {
		t.Fatalf("GetPrices: %v", err)
	}
	if len(stillThere) != 1 {
		t.Errorf("a refused replace left %d bands, want the 1 that was already there", len(stillThere))
	}
}

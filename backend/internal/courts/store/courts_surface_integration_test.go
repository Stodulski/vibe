//go:build integration

package store_test

import (
	"context"
	"testing"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// has_lighting and surface are written by Insert and Update and read back
// through the active_courts view; courts_surface_known refuses a value outside
// the vocabulary even when a caller bypasses the handler.
func TestIntegration_CourtSurfaceAndLightingRoundTrip(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	court, err := f.Stores.Courts.GetByID(f.Scoped(ctx), f.CourtID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if court.HasLighting || court.Surface != nil {
		t.Fatalf("a court starts without lighting or surface; got %+v", court)
	}

	surface := "synthetic_grass"
	court.HasLighting = true
	court.Surface = &surface
	if err := f.Stores.Courts.Update(f.Scoped(ctx), court, nil); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := f.Stores.Courts.GetByID(f.Scoped(ctx), f.CourtID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.HasLighting || got.Surface == nil || *got.Surface != surface {
		t.Fatalf("fields not persisted: %+v", got)
	}

	all, err := f.Stores.Courts.GetByComplex(f.Scoped(ctx), f.ComplexID)
	if err != nil || len(all) == 0 || !all[0].HasLighting || all[0].Surface == nil {
		t.Fatalf("list does not carry the fields: %v %+v", err, all)
	}

	got.Surface = nil
	if err := f.Stores.Courts.Update(f.Scoped(ctx), got, nil); err != nil {
		t.Fatalf("clearing: %v", err)
	}
	cleared, _ := f.Stores.Courts.GetByID(f.Scoped(ctx), f.CourtID)
	if cleared.Surface != nil {
		t.Fatalf("surface not cleared: %v", *cleared.Surface)
	}
}

func TestIntegration_CourtSurfaceCheckRefusesUnknownValue(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	_, err := f.DB.Exec(ctx, `UPDATE courts SET surface = 'ice' WHERE id = $1`, f.CourtID)
	if err == nil {
		t.Fatal("the database accepted a surface outside the vocabulary")
	}
}

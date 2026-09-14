//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// These three pin courts_active_name_unique (db/migrations/001_init.sql)
// against the real constraint rather than a stub: the index is partial over
// active courts, so a court write must be refused with
// courtstore.ErrDuplicateCourtName when it collides with another LIVE
// court's name, and must succeed when the name it collides with belongs to a
// soft-deleted one. The frontend E2E that found the bug this guards against
// only ever exercised the first case (Create), which used to reach a bare
// 500 in the handler — see courts.TestCreateReportsADuplicateCourtNameAsAFieldErrorNotACrash
// for that shape; these three are the store's own half of the contract.

// TestIntegration_InsertRefusesADuplicateNameAmongLiveCourts covers Create.
func TestIntegration_InsertRefusesADuplicateNameAmongLiveCourts(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// The fixture already seeded "Court 1" as f.CourtID, live.
	err := f.Stores.Courts.Insert(ctx, &courtstore.Court{
		ComplexID: f.ComplexID,
		Name:      "Court 1",
		Sport:     "padel",
		CourtType: "indoor",
	})
	if !errors.Is(err, courtstore.ErrDuplicateCourtName) {
		t.Errorf("inserting a court named after a live one must answer ErrDuplicateCourtName; got %v", err)
	}
}

// TestIntegration_UpdateRefusesRenamingOntoAnotherLiveCourtsName covers
// Update, including the reactivation path (is_active: true): both go through
// this same store method, and there is no separate restore call.
func TestIntegration_UpdateRefusesRenamingOntoAnotherLiveCourtsName(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	var otherID uuid.UUID
	err := f.DB.QueryRow(ctx,
		`INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 2') RETURNING id`,
		f.ComplexID,
	).Scan(&otherID)
	if err != nil {
		t.Fatalf("seeding a second live court: %v", err)
	}

	other, err := f.Stores.Courts.GetByID(ctx, otherID)
	if err != nil {
		t.Fatalf("reading the second court back: %v", err)
	}

	other.Name = "Court 1"
	err = f.Stores.Courts.Update(ctx, other, nil)
	if !errors.Is(err, courtstore.ErrDuplicateCourtName) {
		t.Errorf("renaming onto a live court's name must answer ErrDuplicateCourtName; got %v", err)
	}
}

// TestIntegration_UpdateAllowsRenamingOntoADeletedCourtsName is the partial
// index's whole point: courts_active_name_unique excludes soft-deleted rows,
// so reusing a deleted court's name is an ordinary write, not a collision.
func TestIntegration_UpdateAllowsRenamingOntoADeletedCourtsName(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	var otherID uuid.UUID
	err := f.DB.QueryRow(ctx,
		`INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 2') RETURNING id`,
		f.ComplexID,
	).Scan(&otherID)
	if err != nil {
		t.Fatalf("seeding a second live court: %v", err)
	}

	// Free "Court 1" by soft-deleting the court that holds it.
	if err := f.Stores.Courts.SoftDelete(ctx, f.CourtID); err != nil {
		t.Fatalf("soft-deleting the first court: %v", err)
	}

	other, err := f.Stores.Courts.GetByID(ctx, otherID)
	if err != nil {
		t.Fatalf("reading the second court back: %v", err)
	}

	other.Name = "Court 1"
	if err := f.Stores.Courts.Update(ctx, other, nil); err != nil {
		t.Errorf("renaming onto a deleted court's name must be allowed; got %v", err)
	}
}

//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// The invariant the soft-delete cascade installs: no live court under a soft-deleted
// complex. These tests read the `courts` table directly, never `active_courts`,
// because the view's own join would hide a failure of the cascade — a view that
// filters what a trigger was supposed to have stamped answers the same either
// way. The view has its own test further down.

// TestSoftDeleteCascadeLeavesNoLiveCourt is the cascade itself: stamping the
// complex closes every one of its courts, in the same transaction, and the
// store reports how many.
func TestSoftDeleteCascadeLeavesNoLiveCourt(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	// The fixture ships one court; a second one makes the count meaningful —
	// a method that returned a hardcoded 1, or that closed only the first row
	// it found, would pass with one court and fail here.
	var secondCourt uuid.UUID
	err := f.DB.QueryRow(ctx,
		`INSERT INTO courts (complex_id, name) VALUES ($1, 'Court 2') RETURNING id`,
		f.ComplexID).Scan(&secondCourt)
	if err != nil {
		t.Fatalf("creating the second court: %v", err)
	}

	deactivated, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}
	if deactivated != 2 {
		t.Errorf("SoftDeleteCascade reported %d courts deactivated, want 2", deactivated)
	}

	var live int
	err = f.DB.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM courts WHERE complex_id = $1 AND deleted_at IS NULL`,
		f.ComplexID).Scan(&live)
	if err != nil {
		t.Fatalf("counting live courts: %v", err)
	}
	if live != 0 {
		t.Errorf("%d court(s) are still live under a soft-deleted complex; "+
			"they stay bookable through every query that filters only on the court's own deleted_at", live)
	}

	// The courts carry the venue's own deletion timestamp, not the moment the
	// cascade happened to run. They stopped being reachable when it did.
	var sameStamp bool
	err = f.DB.QueryRow(ctx, `
		SELECT bool_and(c.deleted_at = cx.deleted_at)
		FROM courts c JOIN complexes cx ON cx.id = c.complex_id
		WHERE cx.id = $1`, f.ComplexID).Scan(&sameStamp)
	if err != nil {
		t.Fatalf("comparing timestamps: %v", err)
	}
	if !sameStamp {
		t.Error("a court was stamped with a different deleted_at than its complex")
	}

	// And deactivated, not merely deleted: is_active is what the owner's own
	// screens read.
	var stillActive int
	err = f.DB.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM courts WHERE complex_id = $1 AND is_active`,
		f.ComplexID).Scan(&stillActive)
	if err != nil {
		t.Fatalf("counting active courts: %v", err)
	}
	if stillActive != 0 {
		t.Errorf("%d court(s) are still flagged active under a soft-deleted complex", stillActive)
	}
}

// TestSoftDeleteCascadeOnAnAlreadyDeletedComplexIsNotFound pins the second
// half of SoftDeleteCascade's contract: the UPDATE carries
// `AND deleted_at IS NULL`, so a repeat delete affects no row, and reporting
// that as a success would let a caller log a cascade that never happened.
func TestSoftDeleteCascadeOnAnAlreadyDeletedComplexIsNotFound(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("first SoftDeleteCascade: %v", err)
	}

	_, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID)
	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("deleting an already-deleted complex returned %v, want ErrRecordNotFound", err)
	}
}

// TestRevivingACourtUnderADeletedComplexIsRefused is the half a cascade cannot
// give you. The cascade fires when the parent moves; nothing about it stops a
// later statement from clearing a court's own deleted_at, and that statement is
// exactly what a half-written restore path, a support script or a psql session
// would run.
//
// The refusal is a trigger raising 23514 with an explicit constraint name, the
// same shape as bookings_forbid_status_reversal, so the test
// can assert *which* rule fired rather than settle for "something refused it".
func TestRevivingACourtUnderADeletedComplexIsRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}

	_, err := f.DB.Exec(ctx,
		`UPDATE courts SET deleted_at = NULL, is_active = true WHERE id = $1`, f.CourtID)
	if err == nil {
		t.Fatal("a court was revived under a soft-deleted complex and nothing refused it")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want a *pgconn.PgError, got %T: %v", err, err)
	}
	if pgErr.Code != "23514" {
		t.Errorf("SQLSTATE %s, want 23514", pgErr.Code)
	}
	if pgErr.ConstraintName != "courts_no_live_under_deleted_complex" {
		t.Errorf("constraint %q, want courts_no_live_under_deleted_complex", pgErr.ConstraintName)
	}

	// Refused, not silently reverted: the row is unchanged.
	var deleted bool
	if err := f.DB.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM courts WHERE id = $1`, f.CourtID).Scan(&deleted); err != nil {
		t.Fatalf("re-reading the court: %v", err)
	}
	if !deleted {
		t.Error("the court is live again after a refused UPDATE")
	}
}

// TestCreatingACourtUnderADeletedComplexIsRefused is the other direction the
// cascade misses: the parent was already deleted when the write arrived, so
// there is no transition to fire on.
func TestCreatingACourtUnderADeletedComplexIsRefused(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}

	_, err := f.DB.Exec(ctx,
		`INSERT INTO courts (complex_id, name) VALUES ($1, 'Court After The Fact')`, f.ComplexID)
	if err == nil {
		t.Fatal("a court was created under a soft-deleted complex and nothing refused it")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "courts_no_live_under_deleted_complex" {
		t.Errorf("want courts_no_live_under_deleted_complex (23514), got %v", err)
	}
}

// TestTheCascadeDoesNotFireOnAnOrdinaryEdit is the control. The trigger's WHEN
// clause restricts it to the NULL -> non-NULL transition; without that clause
// every write to a complex would walk its courts, and a test that only ever
// deletes cannot tell the difference.
func TestTheCascadeDoesNotFireOnAnOrdinaryEdit(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	if _, err := f.DB.Exec(ctx,
		`UPDATE complexes SET name = 'Renamed' WHERE id = $1`, f.ComplexID); err != nil {
		t.Fatalf("renaming the complex: %v", err)
	}

	var live int
	if err := f.DB.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM courts WHERE complex_id = $1 AND deleted_at IS NULL`,
		f.ComplexID).Scan(&live); err != nil {
		t.Fatalf("counting live courts: %v", err)
	}
	if live == 0 {
		t.Error("renaming a complex closed its courts")
	}
}

// TestPublicReadsSkipADeletedComplexAndItsCourts is the read side: the queries
// the storefront, the owner's dashboards and the availability grid run all go
// through active_complexes / active_courts, so none of them can serve a row
// under a deleted venue.
//
// GetCourtsByComplex and GetCourtByID are the two that changed meaning. They
// used to filter on the court's own deleted_at alone, which was true of every
// court whose complex had been deleted before this migration.
func TestPublicReadsSkipADeletedComplexAndItsCourts(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	var slug string
	if err := f.DB.QueryRow(ctx,
		`SELECT slug FROM complexes WHERE id = $1`, f.ComplexID).Scan(&slug); err != nil {
		t.Fatalf("reading the fixture's slug: %v", err)
	}

	// Before: everything is visible. Without this half the test would pass
	// against a fixture that never had a court in the first place.
	if courts, err := f.Stores.Courts.GetByComplex(ctx, f.ComplexID); err != nil || len(courts) == 0 {
		t.Fatalf("the fixture must start with a visible court; got %d courts, err %v", len(courts), err)
	}

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}

	if _, err := f.Stores.Complexes.GetBySlug(ctx, slug); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("the public page still resolves a deleted venue by slug: %v", err)
	}
	if _, err := f.Stores.Complexes.GetByID(ctx, f.ComplexID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("GetByID still resolves a deleted venue: %v", err)
	}

	courts, err := f.Stores.Courts.GetByComplex(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByComplex: %v", err)
	}
	if len(courts) != 0 {
		t.Errorf("the court listing returned %d court(s) of a deleted venue", len(courts))
	}

	if _, err := f.Stores.Courts.GetByID(ctx, f.CourtID); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("GetByID still resolves a court of a deleted venue: %v", err)
	}

	owned, err := f.Stores.Complexes.GetByOwner(ctx, f.UserID)
	if err != nil {
		t.Fatalf("GetByOwner: %v", err)
	}
	for _, c := range owned {
		if c.ID == f.ComplexID {
			t.Error("the owner's dashboard still lists a deleted venue")
		}
	}

	slugs, err := f.Stores.Complexes.GetAllSlugs(ctx)
	if err != nil {
		t.Fatalf("GetAllSlugs: %v", err)
	}
	for _, s := range slugs {
		if s.Slug == slug {
			t.Error("the sitemap still advertises a deleted venue")
		}
	}
}

// TestTheViewsHoldEvenWithTheCascadeUndone separates the two mechanisms. The
// tests above would all still pass if `active_courts` were the only thing doing
// the work, and `active_courts` would still answer correctly if the trigger
// were the only thing doing the work. This one takes the trigger's result away
// — stamping the complex directly with the cascade trigger disabled, which is
// the state every row created before the cascade existed was in — and requires the
// view to be right anyway.
func TestTheViewsHoldEvenWithTheCascadeUndone(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	if _, err := f.DB.Exec(ctx,
		`ALTER TABLE complexes DISABLE TRIGGER complexes_cascade_soft_delete_to_courts`); err != nil {
		t.Fatalf("disabling the cascade trigger: %v", err)
	}
	t.Cleanup(func() {
		if _, err := f.DB.Exec(context.Background(),
			`ALTER TABLE complexes ENABLE TRIGGER complexes_cascade_soft_delete_to_courts`); err != nil {
			t.Errorf("re-enabling the cascade trigger: %v", err)
		}
	})

	if _, err := f.DB.Exec(ctx,
		`UPDATE complexes SET deleted_at = NOW(), is_active = false WHERE id = $1`, f.ComplexID); err != nil {
		t.Fatalf("soft-deleting the complex: %v", err)
	}

	// The orphan this migration was written about really exists right now.
	var live int
	if err := f.DB.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM courts WHERE complex_id = $1 AND deleted_at IS NULL`,
		f.ComplexID).Scan(&live); err != nil {
		t.Fatalf("counting live courts: %v", err)
	}
	if live == 0 {
		t.Fatal("the cascade still ran with its trigger disabled, so this test proves nothing")
	}

	courts, err := f.Stores.Courts.GetByComplex(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByComplex: %v", err)
	}
	if len(courts) != 0 {
		t.Errorf("active_courts returned %d live court(s) of a deleted venue; the view's join is what "+
			"makes it true for rows the trigger never saw", len(courts))
	}
}

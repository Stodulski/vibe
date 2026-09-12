//go:build integration

package data_test

import (
	"context"
	"testing"
)

// FINDING 2. Court names are almost always "Cancha N", and a plain
// lexical ORDER BY name sorts "Cancha 10" and "Cancha 11" before "Cancha 2".
// GetByComplex must return them in natural (numeric) order instead, since
// this is the one query every public-facing court list (complex profile,
// availability grid, owner court list) shares.
func TestIntegration_CourtsByComplexSortNaturallyNotLexically(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	// The fixture already created "Court 1" for f.CourtID; add courts whose
	// names would come out wrong under a lexical sort.
	names := []string{"Cancha 10", "Cancha 2", "Cancha 3", "Cancha 1"}
	for _, name := range names {
		if _, err := f.Pool.Exec(ctx,
			`INSERT INTO courts (complex_id, name) VALUES ($1, $2)`, f.ComplexID, name,
		); err != nil {
			t.Fatalf("inserting court %q: %v", name, err)
		}
	}

	courts, err := f.Models.Courts.GetByComplex(ctx, f.ComplexID)
	if err != nil {
		t.Fatalf("GetByComplex: %v", err)
	}

	got := make([]string, len(courts))
	for i, c := range courts {
		got[i] = c.Name
	}

	// "Court 1" and "Cancha 1" both extract the numeric key "1", so within
	// that tie they fall back to plain name order ("Cancha 1" < "Court 1").
	want := []string{"Cancha 1", "Court 1", "Cancha 2", "Cancha 3", "Cancha 10"}
	if len(got) != len(want) {
		t.Fatalf("want %d courts; got %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want order %v; got %v", want, got)
		}
	}
}

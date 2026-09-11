package complexes

import (
	"reflect"
	"testing"
)

// The database CHECK constraint cannot express "these elements are distinct" —
// a CHECK may not contain a subquery — so this function is the ONLY thing
// standing between a duplicate and a badge rendered twice on the storefront.
// It is also the only layer that can turn an unknown value into a field error
// the owner can read, rather than the 500 the constraint would produce.
func TestCleanAmenities(t *testing.T) {
	tests := []struct {
		name        string
		in          []string
		wantOut     []string
		wantUnknown string
	}{
		{
			name:    "keeps known values in the order given",
			in:      []string{"bar", "parking", "wifi"},
			wantOut: []string{"bar", "parking", "wifi"},
		},
		{
			name:    "drops a repeat and keeps the first occurrence's position",
			in:      []string{"bar", "parking", "bar"},
			wantOut: []string{"bar", "parking"},
		},
		{
			name:    "an empty list stays an empty list, never nil",
			in:      []string{},
			wantOut: []string{},
		},
		{
			name:        "reports the first unknown value and keeps nothing",
			in:          []string{"parking", "swimming_pool", "bar"},
			wantOut:     nil,
			wantUnknown: "swimming_pool",
		},
		{
			// The empty string is not in the vocabulary. Without this it would
			// reach the CHECK constraint and fail the whole request with a 500.
			name:        "an empty string is unknown, not a blank amenity",
			in:          []string{""},
			wantOut:     nil,
			wantUnknown: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, unknown := cleanAmenities(tt.in)
			if unknown != tt.wantUnknown {
				t.Errorf("unknown = %q, want %q", unknown, tt.wantUnknown)
			}
			if tt.wantOut == nil {
				if out != nil {
					t.Errorf("out = %v, want nil", out)
				}
				return
			}
			if !reflect.DeepEqual(out, tt.wantOut) {
				t.Errorf("out = %v, want %v", out, tt.wantOut)
			}
		})
	}
}

// Every value the vocabulary holds must also be one the migration's CHECK
// accepts. If these drift, a value the API happily takes is refused by the
// database with a 500 that names no field.
func TestKnownAmenitiesMatchesMigration(t *testing.T) {
	// The exact list in complexes_amenities_known, db/migrations/001_init.sql.
	migration := []string{
		"parking", "changing_rooms", "showers", "bar",
		"racket_rental", "pro_shop", "wifi", "lockers",
		"lessons", "tournaments", "accessible", "match_recording",
	}

	if len(knownAmenities) != len(migration) {
		t.Fatalf("handler knows %d amenities, migration allows %d", len(knownAmenities), len(migration))
	}
	for _, a := range migration {
		if !knownAmenities[a] {
			t.Errorf("migration allows %q but the handler rejects it", a)
		}
	}
}

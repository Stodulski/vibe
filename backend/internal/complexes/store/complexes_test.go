package store

import (
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/db"
)

// A NULL description reads back as nil and a stored one as its text, the same
// way the other optional contact fields map.
func TestComplexFromDB_MapsTheDescription(t *testing.T) {
	set := complexFromDB(db.Complex{Description: pgtype.Text{String: "Dos canchas techadas.", Valid: true}}, nil)
	if set.Description == nil || *set.Description != "Dos canchas techadas." {
		t.Errorf("a stored description must map to its text; got %v", set.Description)
	}

	unset := complexFromDB(db.Complex{}, nil)
	if unset.Description != nil {
		t.Errorf("a NULL description must map to nil; got %q", *unset.Description)
	}
}

// fillNonZero sets every field of the struct v points at to a non-zero value,
// for the column types the complexes table uses.
func fillNonZero(t *testing.T, v reflect.Value) {
	t.Helper()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		switch x := f.Addr().Interface().(type) {
		case *string:
			*x = "x"
		case *int32:
			*x = 7
		case *int64:
			*x = 7
		case *bool:
			*x = true
		case *[]string:
			*x = []string{"x"}
		case *pgtype.UUID:
			*x = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
		case *pgtype.Text:
			*x = pgtype.Text{String: "x", Valid: true}
		case *pgtype.Float8:
			*x = pgtype.Float8{Float64: 1, Valid: true}
		case *pgtype.Timestamptz:
			*x = pgtype.Timestamptz{Time: time.Unix(1, 0), Valid: true}
		default:
			t.Fatalf("fillNonZero: no filler for field %s (%s); add one", v.Type().Field(i).Name, f.Type())
		}
	}
}

// GetByOwner's row is the complex plus an aggregate, copied field by field
// onto db.Complex. That copy dropped `version` (and mp_token_expires_at), so
// every complex in GET /complexes read version 0 and the panel's settings
// form sent version 0 back — refused by the OpenAPI minimum of 1. Every
// column of the row must survive the copy, including ones added later.
func TestComplexFromOwnerRow_KeepsEveryColumn(t *testing.T) {
	var row db.GetComplexesByOwnerRow
	fillNonZero(t, reflect.ValueOf(&row).Elem())

	got := reflect.ValueOf(complexFromOwnerRow(row))
	for i := 0; i < got.NumField(); i++ {
		if got.Field(i).IsZero() {
			t.Errorf("db.Complex.%s is zero after the copy: GetByOwner drops that column", got.Type().Field(i).Name)
		}
	}
}

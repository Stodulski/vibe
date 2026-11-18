package store

import (
	"testing"

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

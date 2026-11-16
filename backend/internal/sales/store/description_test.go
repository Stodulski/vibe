package store

import "testing"

// The cash register shows these descriptions as they are stored, so they are
// written in Spanish and never join the owner's note with an em dash.
func TestSaleCashDescription(t *testing.T) {
	note := "Mesa 3"
	empty := ""
	cases := []struct {
		lines int
		note  *string
		want  string
	}{
		{1, nil, "Venta de 1 producto"},
		{3, nil, "Venta de 3 productos"},
		{2, &empty, "Venta de 2 productos"},
		{2, &note, "Venta de 2 productos. Mesa 3"},
	}
	for _, c := range cases {
		if got := saleCashDescription(c.lines, c.note); got != c.want {
			t.Errorf("saleCashDescription(%d, %v) = %q, want %q", c.lines, c.note, got, c.want)
		}
	}
}

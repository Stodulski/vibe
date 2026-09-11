package spreadsheet

import "testing"

func TestEscapeFormulaCell(t *testing.T) {
	tests := map[string]string{
		"":                     "",
		"Ana Perez":            "Ana Perez",
		"=cmd|' /C calc'!A0":   "'=cmd|' /C calc'!A0",
		"+1+1":                 "'+1+1",
		"-1+1":                 "'-1+1",
		"@SUM(A1:A2)":          "'@SUM(A1:A2)",
		"' already apostrophe": "' already apostrophe",
		// H-24's own payloads: legal email addresses whose first character is a
		// formula trigger. The first is hostile, the next two are what real
		// people hold — all three must come out as text, and none of them may
		// be refused anywhere upstream on account of that character.
		"=1+1@example.com": "'=1+1@example.com",
		"+qa@example.com":  "'+qa@example.com",
		"-qa@example.com":  "'-qa@example.com",
		// Only the FIRST character matters. A trigger in the middle is not a
		// formula start, and prefixing those too would mangle ordinary values
		// (a phone number, a price range) for nothing.
		"Ana=Perez":          "Ana=Perez",
		"qa+tag@example.com": "qa+tag@example.com",
	}

	for in, want := range tests {
		if got := EscapeFormulaCell(in); got != want {
			t.Errorf("EscapeFormulaCell(%q) = %q; want %q", in, got, want)
		}
	}
}

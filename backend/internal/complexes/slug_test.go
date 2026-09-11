package complexes

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "Mi Complejo", "mi-complejo"},
		{"special chars", "Club Pádel #1!", "club-pdel-1"},
		{"extra spaces", "  hello  world  ", "hello--world"},
		{"already slug", "my-slug", "my-slug"},
		{"empty", "", ""},
		{"trim hyphens", "--hello--", "hello"},
		{"uppercase", "ABC", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestReservedSlugsMatchTheKnownClientRoutes is the tripwire reservedSlugs'
// own comment promises.
//
// It cannot read frontend from here — a Go test in one repository has no
// business reaching into another, and a path that worked on one machine would
// break in CI. So it does the next best thing: it re-asserts the exact list
// the comment documents. That converts a silent cross-repository drift into a
// visible one, because adding a top-level route to the client's router and
// then adding it here forces a deliberate edit in this file too, which a
// reviewer sees.
//
// What it therefore does NOT prove: that the list still matches the client's
// router. Nothing automated can, until the two repositories share a manifest.
// It proves only that nobody changed the list without meaning to.
func TestReservedSlugsMatchTheKnownClientRoutes(t *testing.T) {
	want := []string{
		// authRoutes
		"login", "register", "verify-email-sent", "verify-email",
		"forgot-password", "reset-password",
		// ownerStandaloneRoutes
		"complexes", "onboarding",
		// ownerDashboardRoutes
		"dashboard", "bookings", "courts", "clients", "reports", "settings", "profile",
		// adminRoutes
		"admin",
		// Platform paths from middleware.ts's SKIP_PREFIXES (H-20) — not
		// routes the SPA declares, but paths it never gets to see.
		"api", "assets", "fonts", "icons", "logo",
	}

	for _, slug := range want {
		if !reservedSlugs[slug] {
			t.Errorf("%q is a top-level client route but is not reserved; a complex could claim it and be unreachable at its own URL", slug)
		}
	}

	if len(reservedSlugs) != len(want) {
		t.Errorf("reservedSlugs has %d entries and this test knows %d: if a route was added to frontend's router, add it to both; if one was removed, remove it from both",
			len(reservedSlugs), len(want))
	}
}

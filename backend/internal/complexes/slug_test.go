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

// TestReservedSlugsMatchTheKnownClientRoutes checks that every top-level
// client route is still reserved, so no complex can claim one of them and be
// unreachable at its own URL.
//
// It cannot read frontend from here, so it re-asserts the list the
// reservedSlugs comment documents. It does NOT prove that the list still
// matches the client's router.
func TestReservedSlugsMatchTheKnownClientRoutes(t *testing.T) {
	want := []string{
		// authRoutes
		"login", "register", "verify-email-sent", "verify-email",
		"forgot-password", "reset-password", "confirm-email-change",
		// ownerStandaloneRoutes
		"complexes", "onboarding",
		// ownerDashboardRoutes
		"dashboard", "bookings", "cash", "courts", "clients", "reports", "settings", "profile",
		// adminRoutes
		"admin",
		// City hub, /canchas/<city>, reserved ahead of its route.
		"canchas",
		// Platform paths from middleware.ts's SKIP_PREFIXES (H-20) — not
		// routes the SPA declares, but paths it never gets to see.
		"api", "assets", "fonts", "icons", "logo",
	}

	for _, slug := range want {
		if !reservedSlugs[slug] {
			t.Errorf("%q is a top-level client route but is not reserved; a complex could claim it and be unreachable at its own URL", slug)
		}
	}
}

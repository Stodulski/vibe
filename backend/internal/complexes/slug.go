package complexes

import (
	"net/url"
	"regexp"
	"strings"
)

// slugRX matches everything that may not appear in a slug.
var slugRX = regexp.MustCompile(`[^a-z0-9-]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRX.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	return s
}

// maxSlugLength bounds the public slug.
//
// H-13: the `slug` column is unbounded TEXT and Create validated the length
// of `name` but never of `slug`, so a 300-character value was accepted with
// 201 and persisted straight into sitemap.xml and every public URL. A slug
// is a URL path segment, not prose — 60 is generous for a venue name turned
// into one and short of anything that could meaningfully bloat a sitemap or
// a shared link.
const maxSlugLength = 60

// reservedSlugs is every first-path-segment route frontend's router
// declares statically, ahead of its own catch-all `/{slug}` page — see
// frontend/src/app/router.tsx, which lists authRoutes, ownerStandaloneRoutes,
// ownerDashboardRoutes and adminRoutes all before publicRoutes, and
// react-router matches in declaration order. A complex created with any of
// these as its slug is permanently unreachable at its own public URL: the
// client's own static page always wins the match.
//
// H-13 (CPX-09): `slug: "../admin"` slugified to the bare "admin" and was
// accepted with 201 — the venue was created, paid for, and its storefront
// link led to the admin panel's route instead, forever. The API reported
// success and nothing else about the request looked wrong.
//
// This is a fact about a different repository (frontend) living in this
// one, because the backend is the only place that can refuse a slug before
// it is ever tried in a browser. Nothing keeps the two lists in sync
// automatically — there is no shared manifest today — so a new top-level
// route added to the client's router without a matching entry here silently
// reopens this finding. TestReservedSlugsMatchTheKnownClientRoutes (slug_test.go)
// is the best available tripwire, and it is worth being precise about what it
// does: it cannot read the frontend repository from here, so it re-asserts the
// exact list this comment documents. It catches an edit to this list that
// nobody meant to make. It does NOT catch a route added to the client's router
// and never mirrored here — nothing automated can, until the two repositories
// share a manifest.
//
// Routes with a second path segment (register/google, admin/users,
// admin/complexes, settings/mp/callback, ...) are not listed: a slug is
// exactly one path segment (slugValidRX forbids "/"), so it can only ever
// collide with another route's first segment, and every one of those first
// segments below already is.
var reservedSlugs = map[string]bool{
	// authRoutes.tsx
	"login":             true,
	"register":          true,
	"verify-email-sent": true,
	"verify-email":      true,
	"forgot-password":   true,
	"reset-password":    true,
	// ownerRoutes.tsx (ownerStandaloneRoutes)
	"complexes":  true,
	"onboarding": true,
	// ownerRoutes.tsx (ownerDashboardRoutes) — also covers /settings/mp/callback above
	"dashboard": true,
	"bookings":  true,
	"courts":    true,
	"clients":   true,
	"reports":   true,
	"settings":  true,
	"profile":   true,
	// adminRoutes.tsx — also covers /admin/users, /admin/complexes, ...
	"admin": true,

	// Platform paths, from frontend/middleware.ts's SKIP_PREFIXES rather
	// than from its router. H-20: the first version of this list was derived
	// from the React router alone and accepted "api" — a slug the storefront
	// can never serve, because Vercel's edge middleware hands /api/ to the
	// backend before the SPA is ever reached. The two lists are not subsets of
	// each other: the middleware omits profile, reports and admin, and the
	// router omits every one of these. The union is what a slug can collide
	// with, so the union is what is reserved.
	"api":    true,
	"assets": true,
	"fonts":  true,
	"icons":  true,
	"logo":   true,
}

// isValidImageURL checks that s is a valid HTTP or HTTPS URL.
// Rejects javascript:, data:, file: and other dangerous schemes.
func isValidImageURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

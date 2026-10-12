//go:build integration

package store_test

import (
	"context"
	"testing"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	sqldb "github.com/stodulski/vibe-server/internal/db"
)

// TestIntegration_GetAllSlugsCarriesTheCoverAndLogoOfEveryActiveComplex proves the
// sitemap read against a real database: a switched-on complex comes back with its
// photos, one without photos comes back with nil, and a switched-off one is absent.
func TestIntegration_GetAllSlugsCarriesTheCoverAndLogoOfEveryActiveComplex(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()

	store := &complexstore.Store{DB: f.DB, Q: sqldb.New(f.DB)}

	withPhotos := insertHubComplex(ctx, t, f, "Club Fotos", "Banfield", true)
	bare := insertHubComplex(ctx, t, f, "Club Sin Fotos", "Banfield", true)
	off := insertHubComplex(ctx, t, f, "Club Off", "Banfield", false)

	if _, err := f.DB.Exec(ctx,
		`UPDATE complexes SET slug = 'club-fotos', cover_url = $2, logo_url = $3 WHERE id = $1`,
		withPhotos, "https://cdn.example/cover.jpg", "https://cdn.example/logo.png"); err != nil {
		t.Fatalf("setting the photos of a complex: %v", err)
	}
	if _, err := f.DB.Exec(ctx, `UPDATE complexes SET slug = 'club-sin-fotos' WHERE id = $1`, bare); err != nil {
		t.Fatalf("setting the slug of a complex: %v", err)
	}
	if _, err := f.DB.Exec(ctx, `UPDATE complexes SET slug = 'club-off' WHERE id = $1`, off); err != nil {
		t.Fatalf("setting the slug of a complex: %v", err)
	}

	slugs, err := store.GetAllSlugs(ctx)
	if err != nil {
		t.Fatalf("GetAllSlugs: %v", err)
	}

	bySlug := make(map[string]complexstore.ComplexSlug, len(slugs))
	for _, s := range slugs {
		bySlug[s.Slug] = s
	}

	got, ok := bySlug["club-fotos"]
	if !ok {
		t.Fatal("the switched-on complex with photos is not listed")
	}
	if got.CoverURL == nil || *got.CoverURL != "https://cdn.example/cover.jpg" {
		t.Errorf("cover_url = %v; want https://cdn.example/cover.jpg", got.CoverURL)
	}
	if got.LogoURL == nil || *got.LogoURL != "https://cdn.example/logo.png" {
		t.Errorf("logo_url = %v; want https://cdn.example/logo.png", got.LogoURL)
	}

	plain, ok := bySlug["club-sin-fotos"]
	if !ok {
		t.Fatal("the switched-on complex without photos is not listed")
	}
	if plain.CoverURL != nil || plain.LogoURL != nil {
		t.Errorf("a complex without photos carries cover_url %v and logo_url %v; want nil", plain.CoverURL, plain.LogoURL)
	}

	if _, ok := bySlug["club-off"]; ok {
		t.Error("a switched-off complex is listed in the sitemap")
	}
}

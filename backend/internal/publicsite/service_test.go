package publicsite

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// TestServiceSitemap covers what the crawler-facing sitemap is allowed to list:
// one entry per published complex, each dated by the row's own last change, and
// nothing else. The app's own root is not in it — the host serves that with
// X-Robots-Tag: noindex, so listing it asks a crawler to fetch a URL it is then
// told to discard.
func TestServiceSitemap(t *testing.T) {
	updated := time.Date(2026, 3, 12, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name        string
		slugs       []complexstore.ComplexSlug
		frontendURL string
		err         error
		wantLocs    []string
		unwantedLoc string
	}{
		{
			name:        "with no complexes the sitemap lists nothing at all",
			frontendURL: "https://vibe.example",
			unwantedLoc: "<loc>https://vibe.example/</loc>",
		},
		{
			name:        "every complex is listed under the frontend origin, and only those",
			frontendURL: "https://vibe.example/",
			slugs: []complexstore.ComplexSlug{
				{Slug: "club-norte", UpdatedAt: updated},
				{Slug: "club-sur", UpdatedAt: updated},
			},
			wantLocs: []string{
				"<loc>https://vibe.example/club-norte</loc>",
				"<loc>https://vibe.example/club-sur</loc>",
				"<lastmod>2026-03-12</lastmod>",
			},
			unwantedLoc: "<loc>https://vibe.example/</loc>",
		},
		{
			name:        "a store failure is passed through, not published as an empty sitemap",
			frontendURL: "https://vibe.example",
			err:         errors.New("db down"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&stubStore{slugs: tt.slugs, slugsErr: tt.err}, tt.frontendURL)

			doc, err := svc.Sitemap(t.Context())

			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("got error %v, want %v", err, tt.err)
				}
				if doc != "" {
					t.Error("a failed read still produced a sitemap")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tt.wantLocs {
				if !strings.Contains(doc, want) {
					t.Errorf("sitemap is missing %q:\n%s", want, doc)
				}
			}
			if tt.unwantedLoc != "" && strings.Contains(doc, tt.unwantedLoc) {
				t.Errorf("sitemap still lists the noindex root %q:\n%s", tt.unwantedLoc, doc)
			}
		})
	}
}

// A venue that has switched itself off is not published: the sitemap already
// filters on is_active, so a live prerender would keep it indexable and
// shareable through a link a crawler had already seen.
func TestServicePrerenderHidesADeactivatedComplex(t *testing.T) {
	store := &stubStore{complex: &complexstore.Complex{ID: uuid.New(), Slug: "club-norte", IsActive: false}}
	svc := NewService(store, "https://vibe.example")

	if _, err := svc.Prerender(t.Context(), "club-norte"); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("got %v, want data.ErrRecordNotFound", err)
	}
}

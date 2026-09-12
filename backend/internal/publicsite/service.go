package publicsite

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/stodulski/vibe-server/internal/data"
)

// Service holds this module's rules: what a crawler is shown, and for which
// venues. There are two, and the second is the one that matters — a venue that
// has switched itself off is not published, on either page.
type Service struct {
	store       Store
	frontendURL string
	templates   *templateCache
}

// NewService returns a Service rendering against the given frontend origin.
func NewService(store Store, frontendURL string) *Service {
	return &Service{
		store:       store,
		frontendURL: frontendURL,
		templates:   newTemplateCache(templateTTL, fetchTimeout),
	}
}

// Sitemap builds the XML listing every published complex.
func (s *Service) Sitemap(ctx context.Context) (string, error) {
	slugs, err := s.store.GetAllSlugs(ctx)
	if err != nil {
		return "", err
	}

	baseURL := strings.TrimRight(s.frontendURL, "/")

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	// The app's own root is deliberately not listed. The host serves it with
	// `X-Robots-Tag: noindex` — it is the signed-in dashboard, not a page
	// anyone searches for — so listing it asks a crawler to fetch a URL it is
	// then told to throw away, and a sitemap that points at noindex pages is a
	// sitemap a crawler trusts less. The venue pages below are the ones that
	// exist to be found.
	for _, slug := range slugs {
		b.WriteString("  <url>\n")
		fmt.Fprintf(&b, "    <loc>%s/%s</loc>\n", baseURL, slug.Slug)
		fmt.Fprintf(&b, "    <lastmod>%s</lastmod>\n", slug.UpdatedAt.Format("2006-01-02"))
		b.WriteString("    <changefreq>daily</changefreq>\n")
		b.WriteString("    <priority>0.8</priority>\n")
		b.WriteString("  </url>\n")
	}

	b.WriteString("</urlset>\n")

	return b.String(), nil
}

// ErrComplexUnavailable reports that the venue exists as far as anyone knows
// but this request could not read it — a database blip, not an unknown or
// deactivated slug.
var ErrComplexUnavailable = errors.New("complex temporarily unavailable")

// Prerender fetches the frontend's index.html and substitutes the complex's own
// title, description, image and structured data, so a crawler that runs no
// JavaScript sees a fully described page rather than an empty app shell.
//
// A deactivated venue answers the same data.ErrRecordNotFound one that does not
// exist answers. The sitemap already filters on is_active, so leaving this page
// live meant a complex that switched itself off stayed indexable and shareable
// through a link a crawler had already seen — with its address, phone and
// opening hours in structured data.
func (s *Service) Prerender(ctx context.Context, slug string) (Prerendered, error) {
	complex, err := s.store.GetBySlug(ctx, slug)
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		return Prerendered{}, err
	case err != nil:
		// This used to answer 200 with the generic shell, on the theory that
		// a 500 costs the page its index entry — but a crawler treats 2xx as
		// final and 5xx as transient: the shell got indexed in the venue's
		// place permanently, where a 503 is simply retried once reads work.
		return Prerendered{}, fmt.Errorf("%w: %w", ErrComplexUnavailable, err)
	case !complex.IsActive:
		return Prerendered{}, data.ErrRecordNotFound
	}

	// A schedules read that fails costs the opening-hours block of the JSON-LD
	// and nothing else: the title, description, image and canonical URL are all
	// on the complex this already holds. Rendering without it beats both a 500
	// and the generic shell.
	schedules, schedulesErr := s.store.GetSchedules(ctx, complex.ID)
	if schedulesErr != nil {
		schedules = nil
	}

	// templateCache.get already prefers a stale copy to an error, so a failure
	// here means there has never been one — the first request after a deploy
	// while the frontend is down. fallbackTemplate carries the same
	// placeholders, so this complex's own tags still land on it.
	//
	// Neither this nor the schedules failure above returns an error: both are
	// reported through Degraded and the page is served anyway. That is the
	// whole point of this endpoint's error handling — see Prerendered.
	tmpl, templateErr := s.templates.get(ctx, s.frontendURL)
	if templateErr != nil {
		tmpl = fallbackTemplate
	}

	return Prerendered{
		Page:     s.render(tmpl, complex, schedules, slug),
		Degraded: errors.Join(schedulesErr, templateErr),
	}, nil
}

// Prerendered is a crawler-facing page and, when something went wrong on the
// way to building it, what that was.
//
// Degraded is never a reason to fail the request — it is what the handler
// logs while serving the page anyway, because the complex itself was read
// successfully and only the opening hours or the frontend's own template was
// lost. A failed complex read is not a Prerendered with Degraded set — see
// ErrComplexUnavailable — because that page has nothing of this venue on it.
type Prerendered struct {
	Page string
	// Degraded is why this page is less than it should be — the complex's own
	// page without its opening hours, or built from the fallback template
	// instead of the frontend's real one — or nil when nothing went wrong.
	Degraded error
}

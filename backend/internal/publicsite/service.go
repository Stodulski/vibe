package publicsite

import (
	"context"
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

// Sitemap builds the XML listing the homepage and every published complex.
func (s *Service) Sitemap(ctx context.Context) (string, error) {
	slugs, err := s.store.GetAllSlugs(ctx)
	if err != nil {
		return "", err
	}

	baseURL := strings.TrimRight(s.frontendURL, "/")

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	b.WriteString("  <url>\n")
	fmt.Fprintf(&b, "    <loc>%s/</loc>\n", baseURL)
	b.WriteString("    <changefreq>daily</changefreq>\n")
	b.WriteString("    <priority>1.0</priority>\n")
	b.WriteString("  </url>\n")

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

// Prerender fetches the frontend's index.html and substitutes the complex's own
// title, description, image and structured data, so a crawler that runs no
// JavaScript sees a fully described page rather than an empty app shell.
//
// A deactivated venue answers the same data.ErrRecordNotFound one that does not
// exist answers. The sitemap already filters on is_active, so leaving this page
// live meant a complex that switched itself off stayed indexable and shareable
// through a link a crawler had already seen — with its address, phone and
// opening hours in structured data.
func (s *Service) Prerender(ctx context.Context, slug string) (string, error) {
	complex, err := s.store.GetBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	if !complex.IsActive {
		return "", data.ErrRecordNotFound
	}

	schedules, err := s.store.GetSchedules(ctx, complex.ID)
	if err != nil {
		return "", err
	}

	tmpl, err := s.templates.get(ctx, s.frontendURL)
	if err != nil {
		return "", err
	}

	return s.render(tmpl, complex, schedules, slug), nil
}

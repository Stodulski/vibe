package publicsite

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Service holds this module's rules: which complex pages and city hubs the public
// site publishes. A venue that has switched itself off is not published on either.
type Service struct {
	store Store
	// publicBaseURL is the origin every public page address is built on: the
	// complex pages and city hubs listed in the sitemap, and the hub listings.
	publicBaseURL string
}

// NewService returns a Service that publishes every public page address under
// publicBaseURL.
func NewService(store Store, publicBaseURL string) *Service {
	return &Service{store: store, publicBaseURL: publicBaseURL}
}

// complexURL is the public address of one complex's page, under the public
// origin base. The /c/ prefix is part of the address: a bare single-segment slug
// collides with platform routes, so every URL this package publishes for a
// complex (the sitemap and the city hubs) is built here.
func complexURL(base, slug string) string {
	return base + "/c/" + slug
}

// Sitemap builds the XML listing every published complex.
func (s *Service) Sitemap(ctx context.Context) (string, error) {
	slugs, err := s.store.GetAllSlugs(ctx)
	if err != nil {
		return "", err
	}

	baseURL := strings.TrimRight(s.publicBaseURL, "/")

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
		fmt.Fprintf(&b, "    <loc>%s</loc>\n", complexURL(baseURL, slug.Slug))
		fmt.Fprintf(&b, "    <lastmod>%s</lastmod>\n", slug.UpdatedAt.Format("2006-01-02"))
		b.WriteString("    <changefreq>daily</changefreq>\n")
		b.WriteString("    <priority>0.8</priority>\n")
		b.WriteString("  </url>\n")
	}

	// Each city with a switched-on complex has a hub page, listed alongside the
	// complex pages it links to.
	hubs, err := s.cityHubs(ctx)
	if err != nil {
		return "", err
	}
	writeHubSitemapEntries(&b, hubs, baseURL)

	b.WriteString("</urlset>\n")

	return b.String(), nil
}

// ErrComplexUnavailable reports that the complexes this request needs exist as far
// as anyone knows but could not be read — a database blip, not an unknown city or
// a deactivated venue.
var ErrComplexUnavailable = errors.New("complex temporarily unavailable")

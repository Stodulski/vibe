package publicsite

import (
	"fmt"
	"net/http"
	"strings"
)

// sitemapCacheSeconds is how long crawlers may reuse the sitemap.
const sitemapCacheSeconds = 3600

// Sitemap handles GET /api/sitemap.xml, listing the homepage and every
// complex's public page.
func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) {
	slugs, err := h.store.GetAllSlugs(r.Context())
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	baseURL := strings.TrimRight(h.frontendURL, "/")

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	b.WriteString("  <url>\n")
	fmt.Fprintf(&b, "    <loc>%s/</loc>\n", baseURL)
	b.WriteString("    <changefreq>daily</changefreq>\n")
	b.WriteString("    <priority>1.0</priority>\n")
	b.WriteString("  </url>\n")

	for _, s := range slugs {
		b.WriteString("  <url>\n")
		fmt.Fprintf(&b, "    <loc>%s/%s</loc>\n", baseURL, s.Slug)
		fmt.Fprintf(&b, "    <lastmod>%s</lastmod>\n", s.UpdatedAt.Format("2006-01-02"))
		b.WriteString("    <changefreq>daily</changefreq>\n")
		b.WriteString("    <priority>0.8</priority>\n")
		b.WriteString("  </url>\n")
	}

	b.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", sitemapCacheSeconds))
	w.WriteHeader(http.StatusOK)
	// The response is committed; a write failure can no longer be reported.
	_, _ = w.Write([]byte(b.String()))
}

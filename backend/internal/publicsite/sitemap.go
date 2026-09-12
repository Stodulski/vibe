package publicsite

import (
	"fmt"
	"net/http"
)

// sitemapCacheSeconds is how long crawlers may reuse the sitemap.
const sitemapCacheSeconds = 3600

// SitemapPath is where the sitemap lives. It is a constant because the
// redirect below has to name the same address the route does.
const SitemapPath = "/api/v1/sitemap.xml"

// SitemapMoved handles GET /api/sitemap.xml, the pre-versioning address, and
// sends a crawler to the versioned one.
func (h *Handler) SitemapMoved(w http.ResponseWriter, r *http.Request) {
	h.respond.MovedPermanently(w, r, SitemapPath)
}

// Sitemap handles GET /api/v1/sitemap.xml, listing the homepage and every
// complex's public page.
func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) {
	doc, err := h.svc.Sitemap(r.Context())
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", sitemapCacheSeconds))
	w.WriteHeader(http.StatusOK)
	// The response is committed; a write failure can no longer be reported.
	_, _ = w.Write([]byte(doc))
}

package publicsite

import (
	"fmt"
	"net/http"
)

// sitemapCacheSeconds is how long crawlers may reuse the sitemap.
const sitemapCacheSeconds = 3600

// Sitemap handles GET /api/sitemap.xml, listing the homepage and every
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

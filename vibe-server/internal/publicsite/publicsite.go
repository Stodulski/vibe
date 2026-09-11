// Package publicsite serves the two endpoints crawlers hit rather than
// browsers: the sitemap, and the server-rendered version of a complex's public
// page.
//
// Both exist because the frontend is a single-page app. A crawler that runs no
// JavaScript would otherwise see an empty shell, so this package fills in the
// meta tags and structured data for it.
package publicsite

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// templateTTL is how long the fetched index.html is reused before refetching.
const templateTTL = 10 * time.Minute

// fetchTimeout bounds the fetch of the frontend's index.html.
const fetchTimeout = 10 * time.Second

// Store is the complex data these two pages need.
type Store interface {
	GetAllSlugs(ctx context.Context) ([]data.ComplexSlug, error)
	GetBySlug(ctx context.Context, slug string) (*data.Complex, error)
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*data.Schedule, error)
}

// Handler serves the crawler-facing routes.
type Handler struct {
	store       Store
	respond     *httpx.Responder
	frontendURL string
	templates   *templateCache
}

// NewHandler returns a Handler rendering against the given frontend origin.
func NewHandler(store Store, respond *httpx.Responder, frontendURL string) *Handler {
	return &Handler{
		store:       store,
		respond:     respond,
		frontendURL: frontendURL,
		templates:   newTemplateCache(templateTTL, fetchTimeout),
	}
}

// Routes registers the crawler-facing endpoints. Both are public by design:
// they exist to be fetched by search engines and social unfurlers.
func (h *Handler) Routes(router httpx.Router, _ httpx.Guards) {
	router.HandlerFunc(http.MethodGet, "/api/sitemap.xml", h.Sitemap)
	router.HandlerFunc(http.MethodGet, "/api/v1/public/prerender/:slug", h.Prerender)
}

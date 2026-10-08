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
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/complexes"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// templateTTL is how long the fetched index.html is reused before refetching.
const templateTTL = 10 * time.Minute

// fetchTimeout bounds the fetch of the frontend's index.html.
const fetchTimeout = 10 * time.Second

// Store is the complex data these pages need. GetPublic is the storefront
// profile the booking page already reads; its Courts are the active courts only,
// in the order the store lists them, and this package takes the courts from it
// without the prices it carries.
type Store interface {
	GetAllSlugs(ctx context.Context) ([]complexstore.ComplexSlug, error)
	GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error)
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
	GetPublic(ctx context.Context, slug string) (*complexes.PublicProfile, error)
}

// Handler serves the crawler-facing routes. It maps the service's errors onto
// HTTP and writes the two documents it builds; the rules live in the Service.
type Handler struct {
	svc     *Service
	respond *httpx.Responder
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder) *Handler {
	return &Handler{svc: svc, respond: respond}
}

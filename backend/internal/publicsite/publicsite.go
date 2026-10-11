// Package publicsite serves the crawler-facing sitemap and the city hub listings
// the public site renders from JSON.
//
// Every public page address this package publishes is built on the public site
// origin, the one config.PublicBaseURL names.
package publicsite

import (
	"context"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// Store is the complex data the sitemap and the city hubs read: every slug for the
// sitemap, and every switched-on complex with its city and court sports for the hubs.
type Store interface {
	GetAllSlugs(ctx context.Context) ([]complexstore.ComplexSlug, error)
	// ListActiveComplexesForHubs is every switched-on complex with its city and
	// court sports, for the city hubs and their sitemap entries.
	ListActiveComplexesForHubs(ctx context.Context) ([]complexstore.HubComplex, error)
}

// Handler serves the crawler-facing and hub routes. It maps the service's errors onto
// HTTP and writes the documents it builds; the rules live in the Service.
type Handler struct {
	svc     *Service
	respond *httpx.Responder
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder) *Handler {
	return &Handler{svc: svc, respond: respond}
}

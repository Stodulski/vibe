// Package complexes owns the venue itself: its profile, its slug, its opening
// hours, its images, and its MercadoPago connection.
//
// It is the tenant boundary of the product. A complex owns courts, which own
// bookings, which own payments — so deleting one is the one operation here that
// cascades, and it is guarded accordingly.
package complexes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
)

// The venue domain persists five distinguishable things, and no rule here
// touches more than one or two of them: the venue rows, the slug namespace,
// the opening hours, and the MercadoPago credentials a venue is connected
// with. They are separate ports so a rule reads through the one it needs, and
// so the credential surface — the only one holding a seller's OAuth tokens —
// is nameable on its own.

// VenueReader is every read of a venue row.
type VenueReader interface {
	GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error)
	GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error)
	// GetByID and GetAllSlugs serve the cross-domain reads on Service; every
	// other module enters this domain through them rather than through the
	// complex store.
	GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error)
	GetAllSlugs(ctx context.Context) ([]complexstore.ComplexSlug, error)
}

// VenueWriter creates, edits and closes a venue.
type VenueWriter interface {
	Insert(ctx context.Context, c *complexstore.Complex) error
	Update(ctx context.Context, c *complexstore.Complex) error
	// SoftDeleteCascade deletes the venue and its courts in one transaction,
	// returning how many courts it closed. See the soft-delete cascade in db/migrations/001_init.sql.
	SoftDeleteCascade(ctx context.Context, id uuid.UUID) (int, error)
}

// SlugStore is the public-URL namespace, which is shared across every tenant.
type SlugStore interface {
	SlugExists(ctx context.Context, slug string) (bool, error)
	SlugsWithPrefix(ctx context.Context, base string) ([]string, error)
}

// ScheduleStore is a venue's opening hours.
type ScheduleStore interface {
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
	UpsertSchedule(ctx context.Context, s *complexstore.Schedule) error
}

// CredentialStore is the venue's MercadoPago connection: the only port here
// that touches a seller's OAuth tokens.
type CredentialStore interface {
	UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error
	ClearMPCredentials(ctx context.Context, complexID uuid.UUID) error
	// ListComplexesNeedingMPRefresh drives Service.RefreshMPTokens, the OAuth
	// sweep cmd/api schedules every 12 hours.
	ListComplexesNeedingMPRefresh(ctx context.Context) ([]*complexstore.Complex, error)
}

// Store is all five together: one concrete store implements them, and the
// composition is what Dependencies takes, so a caller still passes one value.
type Store interface {
	VenueReader
	VenueWriter
	SlugStore
	ScheduleStore
	CredentialStore
}

// CourtStore is the court side of the public profile. Deletion is no longer
// here: the courts of a deleted venue are closed by the same transaction that
// closes the venue (Store.SoftDeleteCascade), not by a second call this handler
// had to remember to make.
type CourtStore interface {
	GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error)
	GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*courtstore.CourtPrice, error)
}

// BookingStore is what deletion needs: whether anything is still live, and the
// cancellation that follows if the owner goes ahead.
type BookingStore interface {
	HasActiveBookings(ctx context.Context, complexID uuid.UUID) (bool, error)
	CancelFutureByComplex(ctx context.Context, complexID uuid.UUID) error
}

// PaymentConnector is the MercadoPago OAuth exchange behind connecting an
// owner's own account.
type PaymentConnector interface {
	ExchangeOAuthCode(ctx context.Context, code, redirectURI, codeVerifier string) (*mp.OAuthTokens, error)
}

// Recorder writes the audit trail.
type Recorder interface {
	Record(e audit.Entry)
}

// Config is what this module needs from application configuration.
type Config struct {
	// MaxComplexes caps how many venues one account may own.
	MaxComplexes int
	// FrontendURL is the origin the MercadoPago OAuth redirect returns to.
	FrontendURL string
	// TrustProxies decides which address the audit trail records.
	TrustProxies bool
	// MPAppID is the MercadoPago application the OAuth connect flow runs
	// against. The client builds the authorize URL, so it must use the same
	// id the API exchanges the code with; serving it from mp/status keeps one
	// source of truth instead of a second env var in the client build.
	MPAppID string
}

// Handler serves the complex routes. It decodes, validates, and maps the
// service's domain errors onto HTTP; every rule lives in the Service.
type Handler struct {
	svc     *Service
	respond *httpx.Refuser
	cfg     Config
}

// refusals is this module's whole error-to-status table: every domain error of
// its own that is a refusal rather than a fault, and the status and message it
// earns. Everything absent from it — data.ErrRecordNotFound above all — is
// answered by internal/httpx.
//
// It is a function rather than a variable because the ceiling in the
// ErrMaxComplexes message is a configured number, not a constant.
//
// ErrActiveBookings is the one entry whose message is not the whole story: the
// MercadoPago disconnect refuses on the same sentinel and has to say so in its
// own words, so that handler overrides the message through DomainErrorWith and
// keeps this status.
func refusals(cfg Config) httpx.Refusals {
	return httpx.Refusals{
		ErrMaxComplexes: httpx.Forbidden(
			fmt.Sprintf("maximum of %d complexes per account reached", cfg.MaxComplexes)),
		ErrActiveBookings: httpx.Conflict(
			"cannot delete complex while it has active bookings, cancel them first"),
		ErrUploadsNotConfigured: httpx.NotImplemented("image uploads are not configured"),
		ErrForeignObject:        httpx.BadRequest("URL does not belong to this storage"),
	}
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder, cfg Config) *Handler {
	return &Handler{
		svc:     svc,
		respond: respond.WithRefusals(refusals(cfg)),
		cfg:     cfg,
	}
}

// Routes registers this module's endpoints.
//
// The public profile is the page a client lands on from a shared link, so it
// takes no guard. Listing and creating need only a session, because neither is
// scoped to a complex the caller already owns. Everything else is the owner's.
func (h *Handler) Routes(router httpx.Router, guards httpx.Guards) {
	owner := func(next http.HandlerFunc) http.HandlerFunc {
		return guards.RequireAuth(guards.RequireComplexOwner(next))
	}

	router.HandlerFunc(http.MethodGet, "/api/v1/public/complexes/:slug", h.GetPublic)

	// NOT under /api/v1/complexes/: httprouter refuses a static segment in the
	// position a wildcard already owns, and ":id" owns everything after
	// /complexes/. Registration order does not help — it is rejected at build
	// time, not matched at request time. So it sits at the root, and the query
	// parameter says what it is about.
	router.HandlerFunc(http.MethodGet, "/api/v1/slug-available", guards.RequireAuth(h.SlugAvailable))

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes", guards.RequireAuth(h.List))
	router.HandlerFunc(http.MethodPost, "/api/v1/complexes", guards.RequireAuth(h.Create))

	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id", owner(h.Get))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/:id", owner(h.Update))
	router.HandlerFunc(http.MethodDelete, "/api/v1/complexes/:id", owner(h.Delete))
	router.HandlerFunc(http.MethodPut, "/api/v1/complexes/:id/schedules", owner(h.UpdateSchedules))

	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/uploads/presign", owner(h.PresignUpload))
	router.HandlerFunc(http.MethodDelete, "/api/v1/complexes/:id/uploads", owner(h.DeleteUpload))

	router.HandlerFunc(http.MethodPost, "/api/v1/complexes/:id/mp/connect", owner(h.ConnectMercadoPago))
	router.HandlerFunc(http.MethodDelete, "/api/v1/complexes/:id/mp/connect", owner(h.DisconnectMercadoPago))
	router.HandlerFunc(http.MethodGet, "/api/v1/complexes/:id/mp/status", owner(h.MercadoPagoStatus))
}

// actor reads who is making the change, and from where, off the request. It is
// the only thing the audit trail needs that lives on the HTTP side.
func (h *Handler) actor(r *http.Request) Actor {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}
	return Actor{UserID: userID, IP: httpx.ClientIP(r, h.cfg.TrustProxies)}
}

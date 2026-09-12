// Package complexes owns the venue itself: its profile, its slug, its opening
// hours, its images, and its MercadoPago connection.
//
// It is the tenant boundary of the product. A complex owns courts, which own
// bookings, which own payments — so deleting one is the one operation here that
// cascades, and it is guarded accordingly.
package complexes

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/storage"
)

// Store is the complex and schedule persistence this module uses.
type Store interface {
	GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error)
	GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error)
	GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
	Insert(ctx context.Context, c *complexstore.Complex) error
	Update(ctx context.Context, c *complexstore.Complex) error
	// SoftDeleteCascade deletes the venue and its courts in one transaction,
	// returning how many courts it closed. See the soft-delete cascade in db/migrations/001_init.sql.
	SoftDeleteCascade(ctx context.Context, id uuid.UUID) (int, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	SlugsWithPrefix(ctx context.Context, base string) ([]string, error)
	UpsertSchedule(ctx context.Context, s *complexstore.Schedule) error
	UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error
	ClearMPCredentials(ctx context.Context, complexID uuid.UUID) error
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

// Handler serves the complex routes.
type Handler struct {
	store    Store
	courts   CourtStore
	bookings BookingStore
	payments PaymentConnector
	storage  storage.ObjectStorage
	audit    Recorder
	logger   *slog.Logger
	// run schedules background work on the application's tracked goroutines,
	// so cleanup started here still completes during a graceful shutdown.
	run     func(func())
	respond *httpx.Responder
	cfg     Config
}

// NewHandler returns a Handler backed by the given stores and services.
func NewHandler(store Store, courts CourtStore, bookings BookingStore, payments PaymentConnector,
	objectStorage storage.ObjectStorage, recorder Recorder, respond *httpx.Responder,
	logger *slog.Logger, run func(func()), cfg Config,
) *Handler {
	return &Handler{
		store:    store,
		courts:   courts,
		bookings: bookings,
		payments: payments,
		storage:  objectStorage,
		audit:    recorder,
		logger:   logger,
		run:      run,
		respond:  respond,
		cfg:      cfg,
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

// record writes an audit entry for a change to a complex. Every write in this
// module acts on the complex itself, so the entity type is fixed.
func (h *Handler) record(r *http.Request, complexID uuid.UUID, action string, entityID *uuid.UUID, oldVal, newVal any) {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}

	h.audit.Record(audit.Entry{
		UserID:     userID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: "complex",
		EntityID:   entityID,
		OldValue:   oldVal,
		NewValue:   newVal,
		IPAddress:  httpx.ClientIP(r, h.cfg.TrustProxies),
	})
}

package complexes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/mpcred"
	"github.com/stodulski/vibe-server/internal/storage"
)

// The refusals this module raises that are not a plain "not found". Each one
// carries a decision the handler turns into a status and a sentence.
var (
	// ErrSlugTaken reports that the public URL an owner asked for is already
	// serving another venue, or is one of the client's own reserved routes.
	ErrSlugTaken = errors.New("slug is already taken")
	// ErrMaxComplexes reports that the account already owns as many venues as
	// its plan allows.
	ErrMaxComplexes = errors.New("maximum complexes per account reached")
	// ErrActiveBookings reports that an operation was refused because the venue
	// still has live bookings. Both deletion and disconnecting MercadoPago
	// raise it; they answer with different sentences because they are different
	// requests.
	ErrActiveBookings = errors.New("complex still has active bookings")
	// ErrUploadsNotConfigured reports that no object storage is wired, so the
	// image endpoints have nothing to sign against.
	ErrUploadsNotConfigured = errors.New("image uploads are not configured")
	// ErrForeignObject reports that a URL names no object of this storage, or
	// names one through a path this service never mints.
	ErrForeignObject = errors.New("url does not belong to this storage")
)

// Actor is who a change is attributed to, as the handler read it off the
// request. The service needs it for the audit trail and for nothing else.
type Actor struct {
	// UserID is the authenticated owner, or nil for a system action.
	UserID *uuid.UUID
	// IP is the address the request arrived from.
	IP string
}

// Dependencies is everything the service needs from the outside world.
type Dependencies struct {
	Store    Store
	Courts   CourtStore
	Bookings BookingStore
	// Payments exchanges an OAuth code for an owner's own credentials.
	Payments PaymentConnector
	// OAuth refreshes those credentials before they expire. It is a separate
	// client from Payments in cmd/api, on its own circuit breaker: a sweep over
	// other people's expired credentials must not be able to take checkout down
	// for every tenant at once.
	OAuth   TokenRefresher
	Storage storage.ObjectStorage
	Audit   Recorder
	Logger  *slog.Logger
	// Run schedules background work on the application's tracked goroutines, so
	// cleanup started here still completes during a graceful shutdown.
	Run func(func())
}

// TokenRefresher renews a seller's MercadoPago OAuth credentials.
type TokenRefresher interface {
	RefreshOAuthToken(ctx context.Context, refreshToken string) (*mp.OAuthTokens, error)
}

// Service holds this module's rules: what a venue may be called, what a public
// URL may be, when a venue may be deleted or disconnected from MercadoPago, and
// which objects belong to which tenant.
type Service struct {
	store    Store
	courts   CourtStore
	bookings BookingStore
	payments PaymentConnector
	oauth    TokenRefresher
	storage  storage.ObjectStorage
	audit    Recorder
	logger   *slog.Logger
	run      func(func())
	cfg      Config
}

// NewService returns a Service backed by the given dependencies.
func NewService(deps Dependencies, cfg Config) *Service {
	return &Service{
		store:    deps.Store,
		courts:   deps.Courts,
		bookings: deps.Bookings,
		payments: deps.Payments,
		oauth:    deps.OAuth,
		storage:  deps.Storage,
		audit:    deps.Audit,
		logger:   deps.Logger,
		run:      deps.Run,
		cfg:      cfg,
	}
}

// record writes an audit entry for a change to a complex. Every write in this
// module acts on the complex itself, so the entity type is fixed.
func (s *Service) record(complexID uuid.UUID, actor Actor, action string, entityID *uuid.UUID, oldVal, newVal any) {
	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: "complex",
		EntityID:   entityID,
		OldValue:   oldVal,
		NewValue:   newVal,
		IPAddress:  actor.IP,
	})
}

// List returns every venue an account owns.
func (s *Service) List(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error) {
	return s.store.GetByOwner(ctx, ownerID)
}

// CreateInput is a validated request to open a venue. Slug is already
// normalised and checked for shape by the handler.
type CreateInput struct {
	Name              string
	Slug              string
	Address           string
	City              string
	Province          string
	Phone             string
	Email             *string
	DepositPercentage int
	CancellationHours int
	Latitude          *float64
	Longitude         *float64
}

// Create opens a venue for an account, refusing a slug another venue already
// holds and an account that is already at its cap, and gives the new venue the
// default 08:00-23:00 week.
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, actor Actor, in CreateInput) (*complexstore.Complex, error) {
	// Ensure slug uniqueness.
	exists, err := s.store.SlugExists(ctx, in.Slug)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSlugTaken
	}

	// Check complex limit per account.
	owned, err := s.store.GetByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if len(owned) >= s.cfg.MaxComplexes {
		return nil, ErrMaxComplexes
	}

	complex := &complexstore.Complex{
		OwnerID:           ownerID,
		Name:              in.Name,
		Slug:              in.Slug,
		Address:           in.Address,
		City:              in.City,
		Province:          in.Province,
		CountryCode:       "AR",
		Currency:          "ARS",
		Phone:             in.Phone,
		Email:             in.Email,
		DepositPercentage: in.DepositPercentage,
		CancellationHours: in.CancellationHours,
		Latitude:          in.Latitude,
		Longitude:         in.Longitude,
		// A new venue lists nothing yet. This must be an empty list, not nil:
		// the column is NOT NULL and an explicit NULL parameter does not fall
		// back to the column default, so a nil slice failed every creation
		// with a 500.
		Amenities: []string{},
	}

	err = s.store.Insert(ctx, complex)
	if err != nil {
		// The check above said the slug was free and the constraint disagreed:
		// either another request took it in between, or the two are answering
		// different questions. Either way the owner is told which field to
		// change instead of being handed a 500.
		if errors.Is(err, complexstore.ErrDuplicateSlug) {
			return nil, ErrSlugTaken
		}
		return nil, err
	}

	s.record(complex.ID, actor, "create", &complex.ID, nil, complex)

	// Create default schedules (Mon-Sun, 08:00-23:00).
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		schedule := &complexstore.Schedule{
			ComplexID: complex.ID,
			Day:       day,
			OpenTime:  "08:00",
			CloseTime: "23:00",
			IsClosed:  false,
		}
		if err := s.store.UpsertSchedule(ctx, schedule); err != nil {
			return nil, err
		}
	}

	return complex, nil
}

// SlugStatus answers whether a public URL can be claimed, and what to use
// instead when it cannot.
type SlugStatus struct {
	// Slug is the normalised form of what the caller asked about.
	Slug string
	// Valid reports whether the input is a slug at all. An empty or malformed
	// one is not "taken"; the form's own validation says so, and reporting it
	// as unavailable would make the field name the wrong reason.
	Valid bool
	// Available reports whether the slug is free to claim.
	Available bool
	// Suggestion is the first free "slug-N", empty when none is within reach or
	// when the slug is free.
	Suggestion string
}

// SlugAvailable reports whether a public URL is free.
//
// H-13: a reserved slug (see reservedSlugs) is unavailable even though no
// complex has ever claimed it in the database — this is the suggestion path
// too, and it must not tell an owner "admin" is free just because nobody has
// raced them to it. Folding it into the taken set also makes suggestSlug skip
// it for free, the same way it already skips a slug another complex holds.
func (s *Service) SlugAvailable(ctx context.Context, raw string) (SlugStatus, error) {
	slug := slugify(raw)

	if slug == "" || !slugValidRX.MatchString(slug) || len(slug) > maxSlugLength {
		return SlugStatus{Slug: slug, Valid: false, Available: false}, nil
	}

	existing, err := s.store.SlugsWithPrefix(ctx, slug)
	if err != nil {
		return SlugStatus{}, err
	}

	taken := make(map[string]bool, len(existing))
	for _, e := range existing {
		taken[e] = true
	}
	taken[slug] = taken[slug] || reservedSlugs[slug]

	status := SlugStatus{Slug: slug, Valid: true, Available: !taken[slug]}
	if taken[slug] {
		status.Suggestion = suggestSlug(slug, taken)
	}
	return status, nil
}

// UpdateInput is a validated partial update. A nil field keeps its current
// value. Slug is already normalised and checked for shape by the handler.
type UpdateInput struct {
	Name              *string
	Slug              *string
	Address           *string
	City              *string
	Province          *string
	Phone             *string
	Email             *string
	LogoURL           *string
	CoverURL          *string
	DepositPercentage *int
	CancellationHours *int
	IsActive          *bool
	Latitude          *float64
	Longitude         *float64
	// Amenities is already deduplicated and checked against the known
	// vocabulary by the handler, which is the only layer that can name the
	// offending value in a field error.
	Amenities *[]string
}

// Update applies a partial change to a venue and schedules the cleanup of any
// image it replaced.
//
// Slug uniqueness is checked only when the slug actually changes: comparing a
// venue against itself would report every save of an unrelated field as "slug
// taken".
//
// It is one cohesive write — apply, persist, record, clean up — and splitting
// it would relocate sequential steps into helpers without reducing what a
// reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) Update(ctx context.Context, complex *complexstore.Complex, actor Actor, in UpdateInput) (*complexstore.Complex, error) {
	oldLogoURL := complex.LogoURL
	oldCoverURL := complex.CoverURL

	if in.Slug != nil && *in.Slug != complex.Slug {
		taken, err := s.store.SlugExists(ctx, *in.Slug)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, ErrSlugTaken
		}
	}

	if in.Name != nil {
		complex.Name = *in.Name
	}
	if in.Slug != nil {
		complex.Slug = *in.Slug
	}
	if in.Address != nil {
		complex.Address = *in.Address
	}
	if in.City != nil {
		complex.City = *in.City
	}
	if in.Province != nil {
		complex.Province = *in.Province
	}
	if in.Latitude != nil {
		complex.Latitude = in.Latitude
	}
	if in.Amenities != nil {
		complex.Amenities = *in.Amenities
	}
	if in.Longitude != nil {
		complex.Longitude = in.Longitude
	}
	if in.Phone != nil {
		complex.Phone = *in.Phone
	}
	if in.Email != nil {
		complex.Email = in.Email
	}
	if in.LogoURL != nil {
		complex.LogoURL = in.LogoURL
	}
	if in.CoverURL != nil {
		complex.CoverURL = in.CoverURL
	}
	if in.DepositPercentage != nil {
		complex.DepositPercentage = *in.DepositPercentage
	}
	if in.CancellationHours != nil {
		complex.CancellationHours = *in.CancellationHours
	}
	if in.IsActive != nil {
		complex.IsActive = *in.IsActive
	}

	if err := s.store.Update(ctx, complex); err != nil {
		return nil, err
	}

	s.record(complex.ID, actor, "update", &complex.ID, nil, complex)

	// Clean up old images from R2 when URLs change.
	//
	//nolint:contextcheck // intentionally detached, see cleanupReplacedImage.
	s.cleanupReplacedImage(in.LogoURL, oldLogoURL, "logo")
	//nolint:contextcheck // see the identical justification above.
	s.cleanupReplacedImage(in.CoverURL, oldCoverURL, "cover")

	return complex, nil
}

// cleanupReplacedImage deletes the object a changed image URL left behind.
//
// It runs on the application's tracked background goroutines rather than in the
// request: the owner's save is already done, and a storage outage must not fail
// it.
func (s *Service) cleanupReplacedImage(newURL, oldURL *string, kind string) {
	if s.storage == nil || newURL == nil || oldURL == nil || *oldURL == "" || *newURL == *oldURL {
		return
	}

	stale := *oldURL
	s.run(func() {
		if key, ok := s.storage.KeyFromPublicURL(stale); ok {
			if err := s.storage.DeleteObject(context.Background(), key); err != nil {
				s.logger.Error("storage: failed to delete old "+kind, "error", err, "key", key)
			}
		}
	})
}

// Delete closes a venue and everything under it.
//
// This is the one cascading operation in the module: it soft-deletes the
// venue's courts and cancels its future bookings, so it is refused outright
// while any booking is still live.
//
// The venue and its courts go down in one transaction (SoftDeleteCascade, which
// leans on the soft-delete cascade trigger and verifies the result), so there is
// no window in which the venue is deleted and its courts are not. Cancelling
// future bookings stays outside it: those are a separate table with its own
// status machine, a cancellation that fails is recoverable by hand, and the
// guard above has already established there are none live.
func (s *Service) Delete(ctx context.Context, complex *complexstore.Complex, actor Actor) (courtsDeactivated int, err error) {
	hasActive, err := s.bookings.HasActiveBookings(ctx, complex.ID)
	if err != nil {
		return 0, err
	}
	if hasActive {
		return 0, ErrActiveBookings
	}

	courtsDeactivated, err = s.store.SoftDeleteCascade(ctx, complex.ID)
	if err != nil {
		return 0, err
	}

	s.record(complex.ID, actor, "delete", &complex.ID, complex, deletionOutcome{CourtsDeactivated: courtsDeactivated})

	if err := s.bookings.CancelFutureByComplex(ctx, complex.ID); err != nil {
		s.logger.Error("failed to cancel future bookings for complex", "error", err, "complex_id", complex.ID)
	}

	return courtsDeactivated, nil
}

// PublicProfile is everything the storefront page of a venue is allowed to
// know.
type PublicProfile struct {
	Complex   PublicComplex
	Courts    []CourtWithPrices
	Schedules []*complexstore.Schedule
}

// GetPublic returns the page a client lands on from a shared link.
//
// A deactivated venue is closed to the public and answers the same
// data.ErrRecordNotFound a venue that does not exist answers. Only the booking
// write used to check this, so a venue that switched itself off kept serving
// its address, phone, courts and prices, and answered every booking attempt
// with a bare 404 the page could not explain.
func (s *Service) GetPublic(ctx context.Context, slug string) (*PublicProfile, error) {
	complex, err := s.store.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !complex.IsActive {
		return nil, data.ErrRecordNotFound
	}

	courtsWithPrices, err := s.publicCourts(ctx, complex.ID)
	if err != nil {
		return nil, err
	}

	schedules, err := s.store.GetSchedules(ctx, complex.ID)
	if err != nil {
		return nil, err
	}

	return &PublicProfile{
		Complex:   newPublicComplex(complex),
		Courts:    courtsWithPrices,
		Schedules: schedules,
	}, nil
}

// publicCourts returns the complex's bookable courts with their price bands.
//
// Only active courts, matching the availability grid. This endpoint used to
// return every court the complex has ever had, so a retired court appeared on
// the page with a price list and no slots to book it in.
func (s *Service) publicCourts(ctx context.Context, complexID uuid.UUID) ([]CourtWithPrices, error) {
	courts, err := s.courts.GetByComplex(ctx, complexID)
	if err != nil {
		return nil, err
	}

	active := make([]*courtstore.Court, 0, len(courts))
	for _, c := range courts {
		if c.IsActive {
			active = append(active, c)
		}
	}

	// Batch-fetch all prices in a single query (instead of N queries).
	courtIDs := make([]uuid.UUID, len(active))
	for i, c := range active {
		courtIDs[i] = c.ID
	}
	allPrices, err := s.courts.GetPricesByCourtIDs(ctx, courtIDs)
	if err != nil {
		return nil, err
	}
	pricesByCourtID := make(map[uuid.UUID][]*courtstore.CourtPrice, len(active))
	for _, p := range allPrices {
		pricesByCourtID[p.CourtID] = append(pricesByCourtID[p.CourtID], p)
	}

	out := make([]CourtWithPrices, len(active))
	for i, c := range active {
		out[i] = CourtWithPrices{Court: c, Prices: pricesByCourtID[c.ID]}
	}
	return out, nil
}

// ScheduleInput is one validated day of a venue's week.
type ScheduleInput struct {
	Day       string
	OpenTime  string
	CloseTime string
	IsClosed  bool
}

// UpdateSchedules replaces a venue's weekly opening hours.
func (s *Service) UpdateSchedules(ctx context.Context, complexID uuid.UUID, actor Actor, in []ScheduleInput) ([]*complexstore.Schedule, error) {
	var schedules []*complexstore.Schedule
	for _, day := range in {
		schedule := &complexstore.Schedule{
			ComplexID: complexID,
			Day:       day.Day,
			OpenTime:  day.OpenTime,
			CloseTime: day.CloseTime,
			IsClosed:  day.IsClosed,
		}
		if err := s.store.UpsertSchedule(ctx, schedule); err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}

	s.record(complexID, actor, "update_schedules", &complexID, nil, schedules)

	return schedules, nil
}

// ConnectMercadoPago exchanges the OAuth code for the owner's own MercadoPago
// credentials, so payments settle into their account rather than the platform's.
// It returns the seller's MercadoPago user id.
func (s *Service) ConnectMercadoPago(ctx context.Context, complexID uuid.UUID, actor Actor, code, redirectURI, codeVerifier string) (string, error) {
	tokens, err := s.payments.ExchangeOAuthCode(ctx, code, redirectURI, codeVerifier)
	if err != nil {
		s.logger.Error("mp connect: oauth exchange failed", "error", err, "complex_id", complexID)
		return "", fmt.Errorf("failed to connect MercadoPago: %w", err)
	}

	mpUserID := fmt.Sprintf("%d", tokens.UserID)

	err = s.store.UpdateMPCredentials(ctx, complexID, tokens.AccessToken, tokens.RefreshToken, mpUserID, tokens.ExpiresIn)
	if err != nil {
		return "", err
	}

	s.record(complexID, actor, "mp_connect", &complexID, nil, nil)

	return mpUserID, nil
}

// DisconnectMercadoPago drops the owner's stored credentials. It is refused
// while the venue still has live bookings: those are money that still has to be
// collected or returned through that connection.
func (s *Service) DisconnectMercadoPago(ctx context.Context, complexID uuid.UUID, actor Actor) error {
	hasActive, err := s.bookings.HasActiveBookings(ctx, complexID)
	if err != nil {
		return err
	}
	if hasActive {
		return ErrActiveBookings
	}

	if err := s.store.ClearMPCredentials(ctx, complexID); err != nil {
		return err
	}

	s.record(complexID, actor, "mp_disconnect", &complexID, nil, nil)
	return nil
}

// PresignedUpload is a short-lived URL the browser uploads straight to, so
// image bytes never pass through this service.
type PresignedUpload struct {
	UploadURL string
	PublicURL string
	Key       string
}

// PresignUpload signs an upload into the calling complex's own namespace.
//
// The validated content type is the one that gets signed, and its extension is
// the one the key carries. Both were once hardcoded to webp while three types
// were accepted, so a client legitimately declaring image/jpeg got a URL that
// only accepts image/webp — the upload either failed at R2 or succeeded by
// lying about itself.
func (s *Service) PresignUpload(ctx context.Context, complexID uuid.UUID, kind, contentType string, fileSize int64) (*PresignedUpload, error) {
	if s.storage == nil {
		return nil, ErrUploadsNotConfigured
	}

	ext := allowedContentTypes[contentType]
	key := fmt.Sprintf("%s%s/%s.%s", uploadKeyPrefix(complexID), kind, uuid.New(), ext)

	uploadURL, publicURL, err := s.storage.GeneratePresignedPUT(ctx, key, contentType, fileSize, 10*time.Minute)
	if err != nil {
		return nil, err
	}

	return &PresignedUpload{UploadURL: uploadURL, PublicURL: publicURL, Key: key}, nil
}

// DeleteUpload removes an image, and only one inside the calling complex's own
// namespace.
//
// The route guard proved the caller owns the complex in the *path*; nothing
// else proves they own the object in the *body*. Every venue's logo_url and
// cover_url are public (the sitemap enumerates the slugs and the public complex
// endpoint hands out both URLs), so without the comparison below one
// authenticated owner could delete every rival venue's images.
//
// H-12: strings.HasPrefix compares raw strings, and a key of
// complexes/{A}/../{B}/logo/x.jpg lexically starts with A's prefix — the
// comparison passed a delete that targets B's object straight through. CPX-04
// reproduced this against production: no cross-tenant object was actually
// removed, but only because the storage SDK normalizes ".." out of the path it
// sends to R2 while the request's signature was computed over the raw key, so
// R2 answered SignatureDoesNotMatch (403) and it surfaced as a
// caller-triggered 500 — an accident of URL normalization stood in for the
// check, not the check itself.
//
// A ".." segment is rejected outright rather than cleaned and allowed:
// PresignUpload only ever builds keys shaped complexes/{id}/{type}/{uuid}.{ext},
// so a legitimate key never contains one. Resolving it with path.Clean and
// letting a passing comparison decide would silently rewrite what the caller
// asked for into a different key entirely, which is not a decision this
// endpoint should make quietly.
//
// A prefix mismatch answers data.ErrRecordNotFound rather than a refusal,
// matching the convention documented in internal/clients: telling the caller
// the object exists but is not theirs turns this endpoint into a probe for
// another tenant's storage keys.
func (s *Service) DeleteUpload(ctx context.Context, complexID uuid.UUID, publicURL string) error {
	if s.storage == nil {
		return ErrUploadsNotConfigured
	}

	key, ok := s.storage.KeyFromPublicURL(publicURL)
	if !ok {
		return ErrForeignObject
	}
	if strings.Contains(key, "..") {
		return ErrForeignObject
	}

	// Normalize before comparing, not after — path.Clean the key (collapsing
	// redundant slashes and "." segments), compare that against the prefix, and
	// delete the cleaned key rather than the raw one.
	cleanKey := path.Clean(key)
	if !strings.HasPrefix(cleanKey, uploadKeyPrefix(complexID)) {
		return data.ErrRecordNotFound
	}

	return s.storage.DeleteObject(ctx, cleanKey)
}

// ---------------------------------------------------------------------------
// Background work
// ---------------------------------------------------------------------------

// mpRefreshResult is what happened to one complex's refresh attempt.
type mpRefreshResult int

const (
	mpRefreshOK mpRefreshResult = iota
	mpRefreshFailed
	// mpRefreshSkipped means there was nothing to refresh — the credential read
	// as not-connected, which ListComplexesNeedingMPRefresh's own filter should
	// already have excluded. Defensive, not expected.
	mpRefreshSkipped
)

// RefreshMPTokens proactively refreshes the OAuth tokens of every connected
// venue. MercadoPago tokens expire after ~6 months; the caller's 12h cadence
// keeps them fresh.
//
// It is called by cmd/api's scheduler rather than by a route, and it is here
// rather than there because it is the same credential lifecycle
// ConnectMercadoPago opens and DisconnectMercadoPago closes.
func (s *Service) RefreshMPTokens(ctx context.Context) {
	// Narrowed to complexes whose token has no known expiry or expires within
	// 30 days, instead of GetWithMPConnected's every-connected-complex: MP
	// tokens live ~180 days, and refreshing all of them on every 12h tick was
	// pure waste once the expiry was actually tracked (mp_token_expires_at).
	complexes, err := s.store.ListComplexesNeedingMPRefresh(ctx)
	if err != nil {
		s.logger.Error("cron_refresh_mp_tokens: failed to get complexes", "error", err)
		sentry.CaptureMessage(fmt.Sprintf("cron_refresh_mp_tokens: CRITICAL - cannot fetch complexes: %v", err))
		return
	}

	refreshed := 0
	failed := 0
	for _, c := range complexes {
		switch s.refreshOneMPToken(ctx, c) {
		case mpRefreshOK:
			refreshed++
		case mpRefreshFailed:
			failed++
		case mpRefreshSkipped:
			// Not connected (ErrMPNotConnected): ListComplexesNeedingMPRefresh
			// already filters on mp_refresh_token IS NOT NULL, so this is
			// defensive rather than expected — neither a success nor a failure
			// of this run.
		}
	}

	if failed > 0 {
		sentry.CaptureMessage(fmt.Sprintf("cron_refresh_mp_tokens: %d/%d complexes failed to refresh", failed, len(complexes)))
	}

	s.logger.Info("cron_refresh_mp_tokens: completed",
		"total", len(complexes),
		"refreshed", refreshed,
		"failed", failed,
	)
}

// refreshOneMPToken refreshes a single complex's MercadoPago OAuth token and
// persists the result. Every failure branch alerts Sentry itself, so the caller
// only has to count.
func (s *Service) refreshOneMPToken(ctx context.Context, c *complexstore.Complex) mpRefreshResult {
	refreshTok, refreshErr := c.SellerRefreshToken()
	if refreshErr != nil {
		if errors.Is(refreshErr, mpcred.ErrMPCredentialUnreadable) {
			sentry.CaptureMessage(fmt.Sprintf("MP refresh token UNREADABLE (skipping refresh): complex=%s (%s) error=%v", c.Name, c.ID, refreshErr))
			return mpRefreshFailed
		}
		return mpRefreshSkipped
	}

	newTokens, err := s.oauth.RefreshOAuthToken(ctx, refreshTok)
	if err != nil {
		// A 4xx here is ordinarily MercadoPago answering invalid_grant — the
		// seller revoked access, or the refresh token itself expired — which is
		// an expected, unactionable-by-retry outcome, not an operational
		// failure of this job. Anything else (5xx, network, decode) is.
		if refreshTokenWasRejected(err) {
			s.logger.Warn("cron_refresh_mp_tokens: MercadoPago rejected the refresh token",
				"error", err, "complex_id", c.ID, "complex_name", c.Name)
		} else {
			s.logger.Error("cron_refresh_mp_tokens: failed to refresh token",
				"error", err, "complex_id", c.ID, "complex_name", c.Name)
		}
		sentry.CaptureMessage(fmt.Sprintf("MP OAuth refresh FAILED: complex=%s (%s) error=%v", c.Name, c.ID, err))
		return mpRefreshFailed
	}

	mpUserID := fmt.Sprintf("%d", newTokens.UserID)
	if updateErr := s.store.UpdateMPCredentials(ctx, c.ID, newTokens.AccessToken, newTokens.RefreshToken, mpUserID, newTokens.ExpiresIn); updateErr != nil {
		s.logger.Error("cron_refresh_mp_tokens: failed to save new tokens", "error", updateErr, "complex_id", c.ID)
		sentry.CaptureMessage(fmt.Sprintf("MP OAuth token save FAILED: complex=%s (%s) error=%v", c.Name, c.ID, updateErr))
		return mpRefreshFailed
	}

	s.logger.Info("cron_refresh_mp_tokens: refreshed token", "complex_id", c.ID, "complex_name", c.Name)
	return mpRefreshOK
}

// refreshTokenWasRejected reports whether a RefreshOAuthToken failure was
// MercadoPago's 4xx invalid_grant-style answer — the seller revoked the grant,
// or the refresh token itself expired — rather than an outage (5xx, network,
// decode failure) worth an Error-level page.
func refreshTokenWasRejected(err error) bool {
	var apiErr *mp.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
}

// ---------------------------------------------------------------------------
// Cross-domain reads
//
// These proxy the store with its exact signature. They exist so another
// domain's entry point into complexes is this service rather than the complex
// store, which is what lets a rule added later (authorization, caching) land in
// one place.
// ---------------------------------------------------------------------------

// GetByID returns one venue. Exported for bookings and payments.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error) {
	return s.store.GetByID(ctx, id)
}

// GetBySlug returns the venue serving a public URL. Exported for courts and
// publicsite.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error) {
	return s.store.GetBySlug(ctx, slug)
}

// GetSchedules returns a venue's opening hours. Exported for courts, reporting
// and publicsite.
func (s *Service) GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error) {
	return s.store.GetSchedules(ctx, complexID)
}

// GetByOwner returns the venues an account owns. Exported for auth, which
// refuses to delete an account that still owns one.
func (s *Service) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error) {
	return s.store.GetByOwner(ctx, ownerID)
}

// GetAllSlugs returns every public URL the product serves. Exported for
// publicsite's sitemap.
func (s *Service) GetAllSlugs(ctx context.Context) ([]complexstore.ComplexSlug, error) {
	return s.store.GetAllSlugs(ctx)
}

// UpdateMPCredentials stores a seller's MercadoPago credentials. Exported for
// bookings, which refreshes them on the booking path when it finds them stale.
func (s *Service) UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error {
	return s.store.UpdateMPCredentials(ctx, complexID, accessToken, refreshToken, userID, expiresIn)
}

package main

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	adminstore "github.com/stodulski/vibe-server/internal/admin/store"
	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	cashboxstore "github.com/stodulski/vibe-server/internal/cashbox/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	productstore "github.com/stodulski/vibe-server/internal/products/store"
	reportstore "github.com/stodulski/vibe-server/internal/reporting/store"
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// ---------------------------------------------------------------------------
// mockUserStore
// ---------------------------------------------------------------------------

// mockUserStore is an in-memory users table, not a set of canned answers.
//
// It used to accept an Insert and forget it, so GetByEmail still reported
// ErrRecordNotFound afterwards and no test could register an account and then
// log into it. That is why the suite had no authenticated caller at all, and
// why TestEveryRouteIsGuarded — which can only send anonymous requests — was
// the whole of the authorization coverage: swapping RequireSuperAdmin for
// RequireAuth left it green, because both reject an anonymous caller.
//
// The default behaviour here is therefore *correct*, not *successful*: an
// unknown id or address is still ErrRecordNotFound, a store that was never
// written to behaves exactly as the old stub did, and a duplicate address is
// still ErrDuplicateEmail. Tests that need a specific failure keep overriding
// the matching Fn field.
type mockUserStore struct {
	InsertFn         func(ctx context.Context, user *authstore.User) error
	GetByEmailFn     func(ctx context.Context, email string) (*authstore.User, error)
	GetByIDFn        func(ctx context.Context, id uuid.UUID) (*authstore.User, error)
	UpdateFn         func(ctx context.Context, user *authstore.User) error
	UpdatePasswordFn func(ctx context.Context, userID uuid.UUID, newHash []byte) error

	// mu guards the rows below. Handlers touch the store from app.background
	// goroutines as well as from the request goroutine, so the map is shared
	// even in a single-request test.
	mu   sync.Mutex
	rows map[uuid.UUID]authstore.User
}

// seed writes user into the store, bypassing the Fn overrides, and returns a
// copy. Test fixtures use it to establish an account that already exists.
func (m *mockUserStore) seed(user authstore.User) *authstore.User {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.put(user)
	stored := m.rows[user.ID]
	return &stored
}

// put stores user. The caller holds mu.
func (m *mockUserStore) put(user authstore.User) {
	if m.rows == nil {
		m.rows = make(map[uuid.UUID]authstore.User)
	}
	m.rows[user.ID] = user
}

func (m *mockUserStore) Insert(ctx context.Context, user *authstore.User) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, user)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.rows {
		if strings.EqualFold(existing.Email, user.Email) {
			return authstore.ErrDuplicateEmail
		}
	}

	// The real store lets PostgreSQL assign the id and the timestamps and
	// reads them back onto the struct the caller passed. A mock that skips
	// that hands the handler a zero id, which then travels into a token, a
	// URL or an audit row and turns a wiring bug into a puzzle.
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	m.put(*user)
	return nil
}

func (m *mockUserStore) GetByEmail(ctx context.Context, email string) (*authstore.User, error) {
	if m.GetByEmailFn != nil {
		return m.GetByEmailFn(ctx, email)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, user := range m.rows {
		if strings.EqualFold(user.Email, email) {
			stored := user
			return &stored, nil
		}
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockUserStore) GetByID(ctx context.Context, id uuid.UUID) (*authstore.User, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.rows[id]
	if !ok {
		return nil, data.ErrRecordNotFound
	}
	return &user, nil
}

func (m *mockUserStore) Update(ctx context.Context, user *authstore.User) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, user)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rows[user.ID]; !ok {
		return data.ErrRecordNotFound
	}
	user.UpdatedAt = time.Now()
	m.put(*user)
	return nil
}

func (m *mockUserStore) UpdatePassword(ctx context.Context, userID uuid.UUID, newHash []byte) error {
	if m.UpdatePasswordFn != nil {
		return m.UpdatePasswordFn(ctx, userID, newHash)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.rows[userID]
	if !ok {
		return data.ErrRecordNotFound
	}
	user.PasswordHash = newHash
	m.put(user)
	return nil
}

func (m *mockUserStore) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.rows[userID]
	if !ok {
		return nil
	}
	user.EmailVerified = true
	m.put(user)
	return nil
}

func (m *mockUserStore) DeleteUnverifiedStale(ctx context.Context) error {
	return nil
}

func (m *mockUserStore) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.rows[userID]
	if !ok {
		return nil
	}
	user.FailedLoginAttempts++
	m.put(user)
	return nil
}

func (m *mockUserStore) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.rows[userID]
	if !ok {
		return nil
	}
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	m.put(user)
	return nil
}

func (m *mockUserStore) Delete(ctx context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rows, userID)
	return nil
}

// ---------------------------------------------------------------------------
// mockEmailVerificationStore
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// mockUserIdentityStore
// ---------------------------------------------------------------------------

// mockUserIdentityStore is a no-op double: nothing in cmd/api's route-level
// suite (surface, audit, authz, exemptions, OpenAPI sync) drives the Google
// sign-in flow far enough to observe a link being written or read — that
// behavior is covered in internal/auth's own handler tests, against
// stubIdentities. This satisfies stores.Stores.UserIdentities so the harness
// wires a complete Models without a nil store panicking a handler that does
// reach it.
type mockUserIdentityStore struct{}

func (m *mockUserIdentityStore) Insert(ctx context.Context, identity *authstore.UserIdentity) error {
	return nil
}

func (m *mockUserIdentityStore) GetByProviderSubject(ctx context.Context, provider, subject string) (*authstore.UserIdentity, error) {
	return nil, data.ErrRecordNotFound
}

func (m *mockUserIdentityStore) GetByUser(ctx context.Context, userID uuid.UUID) ([]*authstore.UserIdentity, error) {
	return nil, nil
}

type mockEmailVerificationStore struct{}

func (m *mockEmailVerificationStore) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	return nil
}

func (m *mockEmailVerificationStore) InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	return nil
}

func (m *mockEmailVerificationStore) GetByHash(ctx context.Context, tokenHash []byte) (*authstore.EmailVerificationToken, error) {
	return nil, data.ErrRecordNotFound
}

func (m *mockEmailVerificationStore) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (m *mockEmailVerificationStore) DeleteExpired(ctx context.Context) error {
	return nil
}

// ---------------------------------------------------------------------------
// mockTokenStore
// ---------------------------------------------------------------------------

type mockTokenStore struct {
	InsertRefreshTokenFn   func(ctx context.Context, userID uuid.UUID, tokenHash []byte, ttl time.Duration) error
	GetRefreshTokenFn      func(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error)
	MarkRefreshTokenUsedFn func(ctx context.Context, tokenHash []byte) error
	GetUsedRefreshTokenFn  func(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error)
	DeleteRefreshTokenFn   func(ctx context.Context, tokenHash []byte) error
	DeleteAllForUserFn     func(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredFn        func(ctx context.Context) error
}

func (m *mockTokenStore) InsertRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, ttl time.Duration) error {
	if m.InsertRefreshTokenFn != nil {
		return m.InsertRefreshTokenFn(ctx, userID, tokenHash, ttl)
	}
	return nil
}

func (m *mockTokenStore) GetRefreshToken(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error) {
	if m.GetRefreshTokenFn != nil {
		return m.GetRefreshTokenFn(ctx, tokenHash)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockTokenStore) MarkRefreshTokenUsed(ctx context.Context, tokenHash []byte) error {
	if m.MarkRefreshTokenUsedFn != nil {
		return m.MarkRefreshTokenUsedFn(ctx, tokenHash)
	}
	return nil
}

func (m *mockTokenStore) GetUsedRefreshToken(ctx context.Context, tokenHash []byte) (*authstore.RefreshToken, error) {
	if m.GetUsedRefreshTokenFn != nil {
		return m.GetUsedRefreshTokenFn(ctx, tokenHash)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockTokenStore) DeleteRefreshToken(ctx context.Context, tokenHash []byte) error {
	if m.DeleteRefreshTokenFn != nil {
		return m.DeleteRefreshTokenFn(ctx, tokenHash)
	}
	return nil
}

func (m *mockTokenStore) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	if m.DeleteAllForUserFn != nil {
		return m.DeleteAllForUserFn(ctx, userID)
	}
	return nil
}

func (m *mockTokenStore) DeleteExpired(ctx context.Context) error {
	if m.DeleteExpiredFn != nil {
		return m.DeleteExpiredFn(ctx)
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockComplexStore
// ---------------------------------------------------------------------------

// mockComplexStore is an in-memory complexes table.
//
// RequireComplexOwner loads the complex named in the route and compares its
// owner to the caller, so a store that answers ErrRecordNotFound to everything
// can only ever produce a 404 from that guard — the ownership comparison is
// never reached. TestRouteAuthorizationMatrix needs the comparison to run for
// an owner, a stranger and a superadmin alike, which needs a complex that
// actually exists.
//
// Empty, it behaves exactly as the previous stub did: every lookup is
// ErrRecordNotFound. The Fn fields still take precedence, so a test that wants
// a specific failure keeps setting one.
type mockComplexStore struct {
	InsertFn                        func(ctx context.Context, c *complexstore.Complex) error
	GetByIDFn                       func(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error)
	GetBySlugFn                     func(ctx context.Context, slug string) (*complexstore.Complex, error)
	GetByOwnerFn                    func(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error)
	UpdateFn                        func(ctx context.Context, c *complexstore.Complex) error
	SoftDeleteCascadeFn             func(ctx context.Context, id uuid.UUID) (int, error)
	UpsertScheduleFn                func(ctx context.Context, schedule *complexstore.Schedule) error
	GetSchedulesFn                  func(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error)
	UpdateMPCredentialsFn           func(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error
	ClearMPCredentialsFn            func(ctx context.Context, complexID uuid.UUID) error
	GetWithMPConnectedFn            func(ctx context.Context) ([]*complexstore.Complex, error)
	ListComplexesNeedingMPRefreshFn func(ctx context.Context) ([]*complexstore.Complex, error)
	SlugExistsFn                    func(ctx context.Context, slug string) (bool, error)
	SlugsWithPrefixFn               func(ctx context.Context, base string) ([]string, error)
	GetAllSlugsFn                   func(ctx context.Context) ([]complexstore.ComplexSlug, error)

	mu   sync.Mutex
	rows map[uuid.UUID]complexstore.Complex
}

// seed writes c into the store, bypassing the Fn overrides, and returns a copy.
func (m *mockComplexStore) seed(c complexstore.Complex) *complexstore.Complex {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.put(c)
	stored := m.rows[c.ID]
	return &stored
}

// put stores c. The caller holds mu.
func (m *mockComplexStore) put(c complexstore.Complex) {
	if m.rows == nil {
		m.rows = make(map[uuid.UUID]complexstore.Complex)
	}
	m.rows[c.ID] = c
}

func (m *mockComplexStore) Insert(ctx context.Context, c *complexstore.Complex) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, c)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now

	m.put(*c)
	return nil
}

func (m *mockComplexStore) GetByID(ctx context.Context, id uuid.UUID) (*complexstore.Complex, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.rows[id]
	if !ok {
		return nil, data.ErrRecordNotFound
	}
	return &c, nil
}

func (m *mockComplexStore) GetBySlug(ctx context.Context, slug string) (*complexstore.Complex, error) {
	if m.GetBySlugFn != nil {
		return m.GetBySlugFn(ctx, slug)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.rows {
		if c.Slug == slug {
			stored := c
			return &stored, nil
		}
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockComplexStore) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*complexstore.Complex, error) {
	if m.GetByOwnerFn != nil {
		return m.GetByOwnerFn(ctx, ownerID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var out []*complexstore.Complex
	for _, c := range m.rows {
		if c.OwnerID == ownerID {
			stored := c
			out = append(out, &stored)
		}
	}
	return out, nil
}

func (m *mockComplexStore) Update(ctx context.Context, c *complexstore.Complex, _ *int) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, c)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rows[c.ID]; !ok {
		return data.ErrRecordNotFound
	}
	c.UpdatedAt = time.Now()
	m.put(*c)
	return nil
}

func (m *mockComplexStore) SoftDeleteCascade(ctx context.Context, id uuid.UUID) (int, error) {
	if m.SoftDeleteCascadeFn != nil {
		return m.SoftDeleteCascadeFn(ctx, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.rows[id]
	if !ok {
		return 0, data.ErrRecordNotFound
	}
	c.IsActive = false
	m.put(c)
	// This mock holds complexes only, so it can report no courts rather than
	// invent a number. Tests that care about the cascade count set
	// SoftDeleteCascadeFn.
	return 0, nil
}

func (m *mockComplexStore) UpsertSchedule(ctx context.Context, schedule *complexstore.Schedule) error {
	if m.UpsertScheduleFn != nil {
		return m.UpsertScheduleFn(ctx, schedule)
	}
	return nil
}

func (m *mockComplexStore) GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*complexstore.Schedule, error) {
	if m.GetSchedulesFn != nil {
		return m.GetSchedulesFn(ctx, complexID)
	}
	return nil, nil
}

func (m *mockComplexStore) UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error {
	if m.UpdateMPCredentialsFn != nil {
		return m.UpdateMPCredentialsFn(ctx, complexID, accessToken, refreshToken, userID, expiresIn)
	}
	return nil
}

func (m *mockComplexStore) ClearMPCredentials(ctx context.Context, complexID uuid.UUID) error {
	if m.ClearMPCredentialsFn != nil {
		return m.ClearMPCredentialsFn(ctx, complexID)
	}
	return nil
}

func (m *mockComplexStore) GetWithMPConnected(ctx context.Context) ([]*complexstore.Complex, error) {
	if m.GetWithMPConnectedFn != nil {
		return m.GetWithMPConnectedFn(ctx)
	}
	return nil, nil
}

func (m *mockComplexStore) ListComplexesNeedingMPRefresh(ctx context.Context) ([]*complexstore.Complex, error) {
	if m.ListComplexesNeedingMPRefreshFn != nil {
		return m.ListComplexesNeedingMPRefreshFn(ctx)
	}
	return nil, nil
}

func (m *mockComplexStore) SlugExists(ctx context.Context, slug string) (bool, error) {
	if m.SlugExistsFn != nil {
		return m.SlugExistsFn(ctx, slug)
	}
	return false, nil
}

func (m *mockComplexStore) SlugsWithPrefix(ctx context.Context, base string) ([]string, error) {
	if m.SlugsWithPrefixFn != nil {
		return m.SlugsWithPrefixFn(ctx, base)
	}
	return nil, nil
}

func (m *mockComplexStore) GetAllSlugs(ctx context.Context) ([]complexstore.ComplexSlug, error) {
	if m.GetAllSlugsFn != nil {
		return m.GetAllSlugsFn(ctx)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// mockCourtStore
// ---------------------------------------------------------------------------

type mockCourtStore struct {
	InsertFn                   func(ctx context.Context, court *courtstore.Court) error
	GetByIDFn                  func(ctx context.Context, id uuid.UUID) (*courtstore.Court, error)
	GetByComplexFn             func(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error)
	UpdateFn                   func(ctx context.Context, court *courtstore.Court) error
	SoftDeleteFn               func(ctx context.Context, id uuid.UUID) error
	InsertPriceFn              func(ctx context.Context, price *courtstore.CourtPrice) error
	GetPricesFn                func(ctx context.Context, courtID uuid.UUID) ([]*courtstore.CourtPrice, error)
	UpdatePriceFn              func(ctx context.Context, price *courtstore.CourtPrice) error
	DeletePriceFn              func(ctx context.Context, id uuid.UUID) error
	DeletePricesByCourtFn      func(ctx context.Context, courtID uuid.UUID) error
	ReplacePricesFn            func(ctx context.Context, courtID uuid.UUID, prices []*courtstore.CourtPrice) (int, error)
	InsertBlockedSlotFn        func(ctx context.Context, slot *courtstore.BlockedSlot) error
	GetBlockedSlotsFn          func(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error)
	GetBlockedSlotByIDFn       func(ctx context.Context, id uuid.UUID) (*courtstore.BlockedSlot, error)
	GetBlockedSlotsByComplexFn func(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time) ([]*courtstore.BlockedSlot, error)
	DeleteBlockedSlotFn        func(ctx context.Context, id uuid.UUID) error
}

func (m *mockCourtStore) Insert(ctx context.Context, court *courtstore.Court) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, court)
	}
	return nil
}

func (m *mockCourtStore) GetByID(ctx context.Context, id uuid.UUID) (*courtstore.Court, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockCourtStore) GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error) {
	if m.GetByComplexFn != nil {
		return m.GetByComplexFn(ctx, complexID)
	}
	return nil, nil
}

func (m *mockCourtStore) Update(ctx context.Context, court *courtstore.Court, _ *int) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, court)
	}
	return nil
}

func (m *mockCourtStore) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.SoftDeleteFn != nil {
		return m.SoftDeleteFn(ctx, id)
	}
	return nil
}

func (m *mockCourtStore) InsertPrice(ctx context.Context, price *courtstore.CourtPrice) error {
	if m.InsertPriceFn != nil {
		return m.InsertPriceFn(ctx, price)
	}
	return nil
}

func (m *mockCourtStore) GetPrices(ctx context.Context, courtID uuid.UUID) ([]*courtstore.CourtPrice, error) {
	if m.GetPricesFn != nil {
		return m.GetPricesFn(ctx, courtID)
	}
	return nil, nil
}

func (m *mockCourtStore) UpdatePrice(ctx context.Context, price *courtstore.CourtPrice) error {
	if m.UpdatePriceFn != nil {
		return m.UpdatePriceFn(ctx, price)
	}
	return nil
}

func (m *mockCourtStore) DeletePrice(ctx context.Context, id uuid.UUID) error {
	if m.DeletePriceFn != nil {
		return m.DeletePriceFn(ctx, id)
	}
	return nil
}

func (m *mockCourtStore) DeletePricesByCourtID(ctx context.Context, courtID uuid.UUID) error {
	if m.DeletePricesByCourtFn != nil {
		return m.DeletePricesByCourtFn(ctx, courtID)
	}
	return nil
}

// ReplacePrices is the transactional replacement for the delete-then-insert
// pair above (H-07): a refused insert used to leave the court with no prices at
// all, because the delete had already committed on its own.
//
// The default stands in for a successful replacement, and returns the
// failedIndex the real store returns in that case: -1, meaning no price was the
// one that failed. A test that needs the failure path sets ReplacePricesFn.
func (m *mockCourtStore) ReplacePrices(ctx context.Context, courtID uuid.UUID, prices []*courtstore.CourtPrice, _ *int) (int, error) {
	if m.ReplacePricesFn != nil {
		return m.ReplacePricesFn(ctx, courtID, prices)
	}
	return -1, nil
}

func (m *mockCourtStore) InsertBlockedSlot(ctx context.Context, slot *courtstore.BlockedSlot) error {
	if m.InsertBlockedSlotFn != nil {
		return m.InsertBlockedSlotFn(ctx, slot)
	}
	return nil
}

func (m *mockCourtStore) GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error) {
	if m.GetBlockedSlotsFn != nil {
		return m.GetBlockedSlotsFn(ctx, courtID, date)
	}
	return nil, nil
}

func (m *mockCourtStore) GetBlockedSlotByID(ctx context.Context, id uuid.UUID) (*courtstore.BlockedSlot, error) {
	if m.GetBlockedSlotByIDFn != nil {
		return m.GetBlockedSlotByIDFn(ctx, id)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockCourtStore) GetBlockedSlotsByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time) ([]*courtstore.BlockedSlot, error) {
	if m.GetBlockedSlotsByComplexFn != nil {
		return m.GetBlockedSlotsByComplexFn(ctx, complexID, dateFrom, dateTo)
	}
	return nil, nil
}

func (m *mockCourtStore) GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*courtstore.CourtPrice, error) {
	return nil, nil
}

func (m *mockCourtStore) GetBlockedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error) {
	return nil, nil
}

func (m *mockCourtStore) DeleteBlockedSlot(ctx context.Context, id uuid.UUID) error {
	if m.DeleteBlockedSlotFn != nil {
		return m.DeleteBlockedSlotFn(ctx, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockBookingStore
// ---------------------------------------------------------------------------

type mockBookingStore struct {
	InsertFn                    func(ctx context.Context, booking *bookingstore.Booking) error
	InsertSafeFn                func(ctx context.Context, booking *bookingstore.Booking) error
	GetByIDFn                   func(ctx context.Context, id uuid.UUID) (*bookingstore.Booking, error)
	GetByComplexFn              func(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters) ([]*bookingstore.Booking, data.Metadata, error)
	UpdateFn                    func(ctx context.Context, booking *bookingstore.Booking) error
	GetForReminder2hFn          func(ctx context.Context, now time.Time) ([]*bookingstore.Booking, error)
	MarkReminderSent2hFn        func(ctx context.Context, id uuid.UUID) error
	GetDashboardStatsFn         func(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DashboardStats, error)
	GetUpcomingTodayFn          func(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*bookingstore.Booking, error)
	GetRevenueByDayFn           func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.RevenueDataPoint, error)
	GetOccupancyByHourDayFn     func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.OccupancyDataPoint, error)
	GetByClientFn               func(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*bookingstore.Booking, error)
	HasActiveBookingsFn         func(ctx context.Context, complexID uuid.UUID) (bool, error)
	HasActiveBookingsByCourtFn  func(ctx context.Context, courtID uuid.UUID) (bool, error)
	CompletePastBookingsFn      func(ctx context.Context) (int64, error)
	GetForReminder2hEnrichedFn  func(ctx context.Context, now time.Time) ([]*bookingstore.CronBooking, error)
	GetExpiredPendingEnrichedFn func(ctx context.Context, expiry time.Duration) ([]*bookingstore.CronBooking, error)
}

func (m *mockBookingStore) Insert(ctx context.Context, booking *bookingstore.Booking) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, booking)
	}
	return nil
}

func (m *mockBookingStore) InsertSafe(ctx context.Context, booking *bookingstore.Booking) error {
	if m.InsertSafeFn != nil {
		return m.InsertSafeFn(ctx, booking)
	}
	// Fall back to InsertFn so existing tests keep working.
	if m.InsertFn != nil {
		return m.InsertFn(ctx, booking)
	}
	return nil
}

func (m *mockBookingStore) GetByID(ctx context.Context, id uuid.UUID) (*bookingstore.Booking, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockBookingStore) GetByComplex(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters) ([]*bookingstore.Booking, data.Metadata, error) {
	if m.GetByComplexFn != nil {
		return m.GetByComplexFn(ctx, complexID, dateFrom, dateTo, filters)
	}
	return nil, data.Metadata{}, nil
}

func (m *mockBookingStore) Update(ctx context.Context, booking *bookingstore.Booking) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, booking)
	}
	return nil
}

func (m *mockBookingStore) GetBookedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]bookingstore.BookedSpan, error) {
	return nil, nil
}

func (m *mockBookingStore) GetForReminder2h(ctx context.Context, now time.Time) ([]*bookingstore.Booking, error) {
	if m.GetForReminder2hFn != nil {
		return m.GetForReminder2hFn(ctx, now)
	}
	return nil, nil
}

func (m *mockBookingStore) MarkReminderSent2h(ctx context.Context, id uuid.UUID) error {
	if m.MarkReminderSent2hFn != nil {
		return m.MarkReminderSent2hFn(ctx, id)
	}
	return nil
}

func (m *mockBookingStore) GetForReminder2hEnriched(ctx context.Context, now time.Time) ([]*bookingstore.CronBooking, error) {
	if m.GetForReminder2hEnrichedFn != nil {
		return m.GetForReminder2hEnrichedFn(ctx, now)
	}
	return nil, nil
}

func (m *mockBookingStore) GetExpiredPendingEnriched(ctx context.Context, expiry time.Duration) ([]*bookingstore.CronBooking, error) {
	if m.GetExpiredPendingEnrichedFn != nil {
		return m.GetExpiredPendingEnrichedFn(ctx, expiry)
	}
	return nil, nil
}

func (m *mockBookingStore) GetDashboardStats(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.DashboardStats, error) {
	if m.GetDashboardStatsFn != nil {
		return m.GetDashboardStatsFn(ctx, complexID, today)
	}
	return &bookingstore.DashboardStats{}, nil
}

func (m *mockBookingStore) GetUpcomingToday(ctx context.Context, complexID uuid.UUID, today time.Time, nowTime string, limit int) ([]*bookingstore.Booking, error) {
	if m.GetUpcomingTodayFn != nil {
		return m.GetUpcomingTodayFn(ctx, complexID, today, nowTime, limit)
	}
	return nil, nil
}

func (m *mockBookingStore) GetRevenueByDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.RevenueDataPoint, error) {
	if m.GetRevenueByDayFn != nil {
		return m.GetRevenueByDayFn(ctx, complexID, from, to)
	}
	return nil, nil
}

func (m *mockBookingStore) GetOccupancyByHourDay(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]bookingstore.OccupancyDataPoint, error) {
	if m.GetOccupancyByHourDayFn != nil {
		return m.GetOccupancyByHourDayFn(ctx, complexID, from, to)
	}
	return nil, nil
}

func (m *mockBookingStore) GetByClient(ctx context.Context, complexID, clientID uuid.UUID, limit int) ([]*bookingstore.Booking, error) {
	if m.GetByClientFn != nil {
		return m.GetByClientFn(ctx, complexID, clientID, limit)
	}
	return nil, nil
}

func (m *mockBookingStore) HasActiveBookings(ctx context.Context, complexID uuid.UUID) (bool, error) {
	if m.HasActiveBookingsFn != nil {
		return m.HasActiveBookingsFn(ctx, complexID)
	}
	return false, nil
}

func (m *mockBookingStore) CompletePastBookings(ctx context.Context) (int64, error) {
	if m.CompletePastBookingsFn != nil {
		return m.CompletePastBookingsFn(ctx)
	}
	return 0, nil
}

func (m *mockBookingStore) GetPaymentSummary(ctx context.Context, complexID uuid.UUID, today time.Time) (*bookingstore.PaymentSummary, error) {
	return &bookingstore.PaymentSummary{}, nil
}

func (m *mockBookingStore) HasActiveBookingsByCourt(ctx context.Context, courtID uuid.UUID) (bool, error) {
	if m.HasActiveBookingsByCourtFn != nil {
		return m.HasActiveBookingsByCourtFn(ctx, courtID)
	}
	return false, nil
}

func (m *mockBookingStore) CancelFutureByComplex(ctx context.Context, complexID uuid.UUID) error {
	return nil
}

// GetRefundIntentOrphans, ClaimRefundIntent and ClearRefundIntent
// (BookingRefundIntentManager, refund-intent-durability spec) have no *Fn
// field: nothing in cmd/api's own test suite exercises the reconciliation
// sweep — that behavior is covered in internal/data and internal/payments,
// against the real store — so these only exist to keep mockBookingStore
// satisfying stores.BookingStore.
func (m *mockBookingStore) GetRefundIntentOrphans(ctx context.Context, olderThan time.Duration, limit int) ([]*bookingstore.Booking, error) {
	return nil, nil
}

func (m *mockBookingStore) ClaimRefundIntent(ctx context.Context, id uuid.UUID, seen time.Time) error {
	return nil
}

func (m *mockBookingStore) ClearRefundIntent(ctx context.Context, id uuid.UUID) error {
	return nil
}

// ---------------------------------------------------------------------------
// mockBookingLinkTokenStore
// ---------------------------------------------------------------------------

type mockBookingLinkTokenStore struct {
	MintFn                  func(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (string, error)
	ResolveBookingFn        func(ctx context.Context, plaintext string) (*bookingstore.Booking, time.Time, error)
	DeleteExpiredTerminalFn func(ctx context.Context, retention time.Duration) error
}

func (m *mockBookingLinkTokenStore) Mint(ctx context.Context, bookingID uuid.UUID, expiresAt time.Time) (string, error) {
	if m.MintFn != nil {
		return m.MintFn(ctx, bookingID, expiresAt)
	}
	return "mock-link-token-" + bookingID.String(), nil
}

func (m *mockBookingLinkTokenStore) ResolveBooking(ctx context.Context, plaintext string) (*bookingstore.Booking, time.Time, error) {
	if m.ResolveBookingFn != nil {
		return m.ResolveBookingFn(ctx, plaintext)
	}
	return nil, time.Time{}, data.ErrRecordNotFound
}

func (m *mockBookingLinkTokenStore) DeleteExpiredTerminal(ctx context.Context, retention time.Duration) error {
	if m.DeleteExpiredTerminalFn != nil {
		return m.DeleteExpiredTerminalFn(ctx, retention)
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockClientStore
// ---------------------------------------------------------------------------

type mockClientStore struct {
	InsertFn           func(ctx context.Context, client *clientstore.Client) error
	GetByIDFn          func(ctx context.Context, id uuid.UUID) (*clientstore.Client, error)
	GetByPhoneFn       func(ctx context.Context, complexID uuid.UUID, phone string) (*clientstore.Client, error)
	GetByComplexFn     func(ctx context.Context, complexID uuid.UUID, search string, filters data.Filters) ([]*clientstore.Client, data.Metadata, error)
	UpdateFn           func(ctx context.Context, client *clientstore.Client) error
	GetOrCreateFn      func(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*clientstore.Client, error)
	IncrementNoShowsFn func(ctx context.Context, clientID uuid.UUID) error
	CountByComplexFn   func(ctx context.Context, complexID uuid.UUID) (int, error)
}

func (m *mockClientStore) Insert(ctx context.Context, client *clientstore.Client) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, client)
	}
	return nil
}

func (m *mockClientStore) GetByID(ctx context.Context, id uuid.UUID) (*clientstore.Client, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockClientStore) GetByPhone(ctx context.Context, complexID uuid.UUID, phone string) (*clientstore.Client, error) {
	if m.GetByPhoneFn != nil {
		return m.GetByPhoneFn(ctx, complexID, phone)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockClientStore) GetByComplex(ctx context.Context, complexID uuid.UUID, search string, filters data.Filters) ([]*clientstore.Client, data.Metadata, error) {
	if m.GetByComplexFn != nil {
		return m.GetByComplexFn(ctx, complexID, search, filters)
	}
	return nil, data.Metadata{}, nil
}

func (m *mockClientStore) Update(ctx context.Context, client *clientstore.Client) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, client)
	}
	return nil
}

func (m *mockClientStore) GetOrCreate(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*clientstore.Client, error) {
	if m.GetOrCreateFn != nil {
		return m.GetOrCreateFn(ctx, complexID, firstName, lastName, phone, email, allowNameUpdate)
	}
	return &clientstore.Client{
		ID:        uuid.New(),
		ComplexID: complexID,
		FirstName: firstName,
		LastName:  lastName,
		Phone:     phone,
	}, nil
}

func (m *mockClientStore) IncrementNoShows(ctx context.Context, clientID uuid.UUID) error {
	if m.IncrementNoShowsFn != nil {
		return m.IncrementNoShowsFn(ctx, clientID)
	}
	return nil
}

func (m *mockClientStore) CountByComplex(ctx context.Context, complexID uuid.UUID) (int, error) {
	if m.CountByComplexFn != nil {
		return m.CountByComplexFn(ctx, complexID)
	}
	return 0, nil
}

func (m *mockClientStore) GetInsights(ctx context.Context, complexID uuid.UUID, today time.Time) (*clientstore.ClientInsights, error) {
	return &clientstore.ClientInsights{}, nil
}

// ---------------------------------------------------------------------------
// mockPaymentStore
// ---------------------------------------------------------------------------

type mockPaymentStore struct {
	InsertFn                  func(ctx context.Context, payment *paymentstore.Payment) error
	InsertAndConfirmBookingFn func(ctx context.Context, payment *paymentstore.Payment, booking *bookingstore.Booking) error
	ConfirmWebhookPaymentFn   func(ctx context.Context, payment *paymentstore.Payment, booking *bookingstore.Booking) error
	GetByBookingIDFn          func(ctx context.Context, bookingID uuid.UUID) (*paymentstore.Payment, error)
	ListByBookingIDFn         func(ctx context.Context, bookingID uuid.UUID) ([]*paymentstore.Payment, error)
	GetByMPPaymentFn          func(ctx context.Context, mpPaymentID string) (*paymentstore.Payment, error)
	UpdateFn                  func(ctx context.Context, payment *paymentstore.Payment) error
	ClaimRefundFn             func(ctx context.Context, paymentID uuid.UUID) (*paymentstore.RefundClaim, error)
	RecordRefundSuccessFn     func(ctx context.Context, claim paymentstore.RefundClaim, manualOwedCentavos int) (int, error)
	RecordRefundFailureFn     func(ctx context.Context, claim paymentstore.RefundClaim, cause string) (bool, error)
	RecordManualRefundFn      func(ctx context.Context, bookingID uuid.UUID) (int, error)
}

func (m *mockPaymentStore) Insert(ctx context.Context, payment *paymentstore.Payment) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, payment)
	}
	return nil
}

func (m *mockPaymentStore) InsertAndConfirmBooking(ctx context.Context, payment *paymentstore.Payment, booking *bookingstore.Booking) error {
	if m.InsertAndConfirmBookingFn != nil {
		return m.InsertAndConfirmBookingFn(ctx, payment, booking)
	}
	return nil
}

func (m *mockPaymentStore) ConfirmWebhookPayment(ctx context.Context, payment *paymentstore.Payment, booking *bookingstore.Booking) error {
	if m.ConfirmWebhookPaymentFn != nil {
		return m.ConfirmWebhookPaymentFn(ctx, payment, booking)
	}
	return nil
}

func (m *mockPaymentStore) GetByBookingID(ctx context.Context, bookingID uuid.UUID) (*paymentstore.Payment, error) {
	if m.GetByBookingIDFn != nil {
		return m.GetByBookingIDFn(ctx, bookingID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockPaymentStore) ListByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*paymentstore.Payment, error) {
	if m.ListByBookingIDFn != nil {
		return m.ListByBookingIDFn(ctx, bookingID)
	}
	return nil, nil
}

func (m *mockPaymentStore) GetByMPPaymentID(ctx context.Context, mpPaymentID string) (*paymentstore.Payment, error) {
	if m.GetByMPPaymentFn != nil {
		return m.GetByMPPaymentFn(ctx, mpPaymentID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockPaymentStore) Update(ctx context.Context, payment *paymentstore.Payment) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, payment)
	}
	return nil
}

// The refund lifecycle. Each default refuses rather than succeeding: a mock that
// silently reports a refund done is how a broken money path passes its tests.
func (m *mockPaymentStore) ClaimRefund(ctx context.Context, paymentID uuid.UUID) (*paymentstore.RefundClaim, error) {
	if m.ClaimRefundFn != nil {
		return m.ClaimRefundFn(ctx, paymentID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockPaymentStore) RecordRefundSuccess(ctx context.Context, claim paymentstore.RefundClaim, manualOwedCentavos int) (int, error) {
	if m.RecordRefundSuccessFn != nil {
		return m.RecordRefundSuccessFn(ctx, claim, manualOwedCentavos)
	}
	return 0, data.ErrRecordNotFound
}

func (m *mockPaymentStore) RecordRefundFailure(ctx context.Context, claim paymentstore.RefundClaim, cause string) (bool, error) {
	if m.RecordRefundFailureFn != nil {
		return m.RecordRefundFailureFn(ctx, claim, cause)
	}
	return false, data.ErrRecordNotFound
}

func (m *mockPaymentStore) RecordManualRefund(ctx context.Context, bookingID uuid.UUID) (int, error) {
	if m.RecordManualRefundFn != nil {
		return m.RecordManualRefundFn(ctx, bookingID)
	}
	return 0, data.ErrRecordNotFound
}

// ---------------------------------------------------------------------------
// mockFailedRefundStore
// ---------------------------------------------------------------------------

type mockFailedRefundStore struct {
	DeleteResolvedFn func(ctx context.Context, olderThan time.Duration) (int64, error)
}

func (m *mockFailedRefundStore) Insert(ctx context.Context, fr *paymentstore.FailedRefund) error {
	return nil
}

func (m *mockFailedRefundStore) GetPendingDue(ctx context.Context) ([]*paymentstore.FailedRefund, error) {
	return nil, nil
}

func (m *mockFailedRefundStore) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockFailedRefundStore) MarkResolved(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockFailedRefundStore) MarkExhausted(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockFailedRefundStore) IncrementRetry(ctx context.Context, id uuid.UUID, retryCount int, errMsg string) error {
	return nil
}

func (m *mockFailedRefundStore) DeleteResolved(ctx context.Context, olderThan time.Duration) (int64, error) {
	if m.DeleteResolvedFn != nil {
		return m.DeleteResolvedFn(ctx, olderThan)
	}
	return 0, nil
}

// ---------------------------------------------------------------------------
// mockWebhookEventStore
// ---------------------------------------------------------------------------

// mockWebhookEventStore is the durable webhook inbox. The behavioural tests for
// it live in internal/payments and internal/data, where the stubs can be made to
// fail; this one only keeps the wiring in newTestApplication honest.
type mockWebhookEventStore struct{}

func (m *mockWebhookEventStore) Insert(ctx context.Context, e *paymentstore.WebhookEvent) error {
	return nil
}

func (m *mockWebhookEventStore) GetPendingDue(ctx context.Context) ([]*paymentstore.WebhookEvent, error) {
	return nil, nil
}

func (m *mockWebhookEventStore) Claim(ctx context.Context, id uuid.UUID) (bool, error) {
	return true, nil
}

func (m *mockWebhookEventStore) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockWebhookEventStore) MarkFailed(ctx context.Context, id uuid.UUID, cause string) (bool, error) {
	return false, nil
}

func (m *mockWebhookEventStore) DeleteProcessed(ctx context.Context, olderThan time.Duration) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// mockAdminStore
// ---------------------------------------------------------------------------

type mockAdminStore struct {
	GetPlatformStatsFn func(ctx context.Context) (*adminstore.PlatformStats, error)
	ListUsersFn        func(ctx context.Context, search, roleFilter string, filters data.Filters) ([]*adminstore.AdminUserRow, data.Metadata, error)
	GetUserDetailFn    func(ctx context.Context, userID uuid.UUID) (*adminstore.AdminUserDetail, error)
	ListComplexesFn    func(ctx context.Context, search string, filters data.Filters) ([]*adminstore.AdminComplexRow, data.Metadata, error)
	GetComplexDetailFn func(ctx context.Context, complexID uuid.UUID) (*adminstore.AdminComplexDetail, error)
	ToggleUserActiveFn func(ctx context.Context, userID uuid.UUID, isActive bool) error
}

func (m *mockAdminStore) GetPlatformStats(ctx context.Context) (*adminstore.PlatformStats, error) {
	if m.GetPlatformStatsFn != nil {
		return m.GetPlatformStatsFn(ctx)
	}
	return &adminstore.PlatformStats{}, nil
}

func (m *mockAdminStore) ListUsers(ctx context.Context, search, roleFilter string, filters data.Filters) ([]*adminstore.AdminUserRow, data.Metadata, error) {
	if m.ListUsersFn != nil {
		return m.ListUsersFn(ctx, search, roleFilter, filters)
	}
	return nil, data.Metadata{}, nil
}

func (m *mockAdminStore) GetUserDetail(ctx context.Context, userID uuid.UUID) (*adminstore.AdminUserDetail, error) {
	if m.GetUserDetailFn != nil {
		return m.GetUserDetailFn(ctx, userID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockAdminStore) ListComplexes(ctx context.Context, search string, filters data.Filters) ([]*adminstore.AdminComplexRow, data.Metadata, error) {
	if m.ListComplexesFn != nil {
		return m.ListComplexesFn(ctx, search, filters)
	}
	return nil, data.Metadata{}, nil
}

func (m *mockAdminStore) GetComplexDetail(ctx context.Context, complexID uuid.UUID) (*adminstore.AdminComplexDetail, error) {
	if m.GetComplexDetailFn != nil {
		return m.GetComplexDetailFn(ctx, complexID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockAdminStore) ToggleUserActive(ctx context.Context, userID uuid.UUID, isActive bool) error {
	if m.ToggleUserActiveFn != nil {
		return m.ToggleUserActiveFn(ctx, userID, isActive)
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockAuditStore — the trail, split out of the admin store
// ---------------------------------------------------------------------------

type mockAuditStore struct{}

func (m *mockAuditStore) ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters data.Filters) ([]*auditstore.AuditLogRow, data.Metadata, error) {
	return nil, data.Metadata{}, nil
}

func (m *mockAuditStore) InsertAuditLog(ctx context.Context, userID, complexID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldJSON, newJSON []byte, ipAddr string) error {
	return nil
}

// ---------------------------------------------------------------------------
// mockSlotLockStore
// ---------------------------------------------------------------------------

type mockSlotLockStore struct{}

func (m *mockSlotLockStore) AcquireLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime, endTime string, bookingID *uuid.UUID, ttl time.Duration) error {
	return nil
}

func (m *mockSlotLockStore) ReleaseLock(ctx context.Context, courtID uuid.UUID, date time.Time, startTime string) error {
	return nil
}

func (m *mockSlotLockStore) ReleaseByBooking(ctx context.Context, bookingID uuid.UUID) error {
	return nil
}

func (m *mockSlotLockStore) CleanExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------------------
// mockReportStore
// ---------------------------------------------------------------------------

// mockReportStore stands in for stores.ReportStore.
//
// It exists because the harness left Models.Reports nil, so the two reporting
// endpoints panicked on the first request that reached them and answered 500.
// The boot-time checklist that stood here then covered services, not stores, so
// nothing reported it — TestRouteAuthorizationMatrix is what surfaced it, by
// being the first test to call those routes as a caller entitled to them.
type mockReportStore struct {
	PaymentSummaryByMethodFn       func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error)
	PaymentSummaryByMethodWindowFn func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error)
	PaymentSummaryByCourtFn        func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentCourtSummary, error)
	PaymentDetailsFn               func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentDetail, error)
	CashSalesByMethodFn            func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashSalesSummary, error)
	CashMovementsByCategoryFn      func(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashCategorySummary, error)
}

func (m *mockReportStore) PaymentSummaryByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error) {
	if m.PaymentSummaryByMethodFn != nil {
		return m.PaymentSummaryByMethodFn(ctx, complexID, from, to)
	}
	return nil, nil
}

func (m *mockReportStore) PaymentSummaryByMethodWindow(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentMethodSummary, error) {
	if m.PaymentSummaryByMethodWindowFn != nil {
		return m.PaymentSummaryByMethodWindowFn(ctx, complexID, from, to)
	}
	return nil, nil
}

func (m *mockReportStore) PaymentSummaryByCourt(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentCourtSummary, error) {
	if m.PaymentSummaryByCourtFn != nil {
		return m.PaymentSummaryByCourtFn(ctx, complexID, from, to)
	}
	return nil, nil
}

func (m *mockReportStore) PaymentDetails(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.PaymentDetail, error) {
	if m.PaymentDetailsFn != nil {
		return m.PaymentDetailsFn(ctx, complexID, from, to)
	}
	return nil, nil
}

func (m *mockReportStore) CashSalesByMethod(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashSalesSummary, error) {
	if m.CashSalesByMethodFn != nil {
		return m.CashSalesByMethodFn(ctx, complexID, from, to)
	}
	return nil, nil
}

func (m *mockReportStore) CashMovementsByCategory(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]reportstore.CashCategorySummary, error) {
	if m.CashMovementsByCategoryFn != nil {
		return m.CashMovementsByCategoryFn(ctx, complexID, from, to)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// mockCashboxStore
// ---------------------------------------------------------------------------

// mockCashboxStore stands in for stores.CashboxStore (a cash session store
// and a cash movement store in one, the same shape internal/cashbox/store's
// real one is).
type mockCashboxStore struct {
	OpenSessionFn         func(ctx context.Context, s *cashboxstore.CashSession) error
	GetOpenByComplexFn    func(ctx context.Context, complexID uuid.UUID) (*cashboxstore.CashSession, error)
	GetByIDFn             func(ctx context.Context, complexID, sessionID uuid.UUID) (*cashboxstore.CashSession, error)
	ListByComplexFn       func(ctx context.Context, complexID uuid.UUID, filters data.Filters) ([]*cashboxstore.CashSession, data.Metadata, error)
	CloseFn               func(ctx context.Context, complexID, sessionID, closedBy uuid.UUID, countedCash, cashBookingPaymentsInWindow int64, closedAt time.Time, note *string) (*cashboxstore.CashSession, error)
	InsertMovementFn      func(ctx context.Context, m *cashboxstore.CashMovement) error
	GetMovementByIDFn     func(ctx context.Context, complexID, movementID uuid.UUID) (*cashboxstore.CashMovement, error)
	ListMovementsBySessFn func(ctx context.Context, complexID, sessionID uuid.UUID) ([]*cashboxstore.CashMovement, error)
	SumBySessionFn        func(ctx context.Context, complexID, sessionID uuid.UUID) ([]cashboxstore.MovementTotal, error)
}

func (m *mockCashboxStore) OpenSession(ctx context.Context, s *cashboxstore.CashSession) error {
	if m.OpenSessionFn != nil {
		return m.OpenSessionFn(ctx, s)
	}
	s.ID = uuid.New()
	return nil
}

func (m *mockCashboxStore) GetOpenByComplex(ctx context.Context, complexID uuid.UUID) (*cashboxstore.CashSession, error) {
	if m.GetOpenByComplexFn != nil {
		return m.GetOpenByComplexFn(ctx, complexID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockCashboxStore) GetByID(ctx context.Context, complexID, sessionID uuid.UUID) (*cashboxstore.CashSession, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, complexID, sessionID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockCashboxStore) ListByComplex(ctx context.Context, complexID uuid.UUID, filters data.Filters) ([]*cashboxstore.CashSession, data.Metadata, error) {
	if m.ListByComplexFn != nil {
		return m.ListByComplexFn(ctx, complexID, filters)
	}
	return nil, data.Metadata{}, nil
}

func (m *mockCashboxStore) Close(ctx context.Context, complexID, sessionID, closedBy uuid.UUID, countedCash, cashBookingPaymentsInWindow int64, closedAt time.Time, note *string) (*cashboxstore.CashSession, error) {
	if m.CloseFn != nil {
		return m.CloseFn(ctx, complexID, sessionID, closedBy, countedCash, cashBookingPaymentsInWindow, closedAt, note)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockCashboxStore) InsertMovement(ctx context.Context, movement *cashboxstore.CashMovement) error {
	if m.InsertMovementFn != nil {
		return m.InsertMovementFn(ctx, movement)
	}
	movement.ID = uuid.New()
	return nil
}

func (m *mockCashboxStore) GetMovementByID(ctx context.Context, complexID, movementID uuid.UUID) (*cashboxstore.CashMovement, error) {
	if m.GetMovementByIDFn != nil {
		return m.GetMovementByIDFn(ctx, complexID, movementID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockCashboxStore) ListMovementsBySession(ctx context.Context, complexID, sessionID uuid.UUID) ([]*cashboxstore.CashMovement, error) {
	if m.ListMovementsBySessFn != nil {
		return m.ListMovementsBySessFn(ctx, complexID, sessionID)
	}
	return nil, nil
}

func (m *mockCashboxStore) SumBySession(ctx context.Context, complexID, sessionID uuid.UUID) ([]cashboxstore.MovementTotal, error) {
	if m.SumBySessionFn != nil {
		return m.SumBySessionFn(ctx, complexID, sessionID)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// mockProductStore
// ---------------------------------------------------------------------------

// mockProductStore stands in for stores.ProductStore (a product-catalog store
// and a stock-ledger store in one, the same shape mockCashboxStore is for
// stores.CashboxStore).
type mockProductStore struct {
	InsertFn             func(ctx context.Context, p *productstore.Product) error
	GetByIDFn            func(ctx context.Context, complexID, productID uuid.UUID) (*productstore.Product, error)
	ListByComplexFn      func(ctx context.Context, complexID uuid.UUID, activeFilter *bool) ([]*productstore.Product, error)
	UpdateFn             func(ctx context.Context, p *productstore.Product, expectedVersion *int) error
	RestockFn            func(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity, totalCost int, method string, note *string) (*productstore.Product, *productstore.StockMovement, error)
	AdjustFn             func(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity int, reason string, note *string) (*productstore.Product, *productstore.StockMovement, error)
	ListStockMovementsFn func(ctx context.Context, complexID, productID uuid.UUID, filters data.Filters) ([]*productstore.StockMovement, data.Metadata, error)
}

func (m *mockProductStore) Insert(ctx context.Context, p *productstore.Product) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, p)
	}
	p.ID = uuid.New()
	return nil
}

func (m *mockProductStore) GetByID(ctx context.Context, complexID, productID uuid.UUID) (*productstore.Product, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, complexID, productID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockProductStore) ListByComplex(ctx context.Context, complexID uuid.UUID, activeFilter *bool) ([]*productstore.Product, error) {
	if m.ListByComplexFn != nil {
		return m.ListByComplexFn(ctx, complexID, activeFilter)
	}
	return nil, nil
}

func (m *mockProductStore) Update(ctx context.Context, p *productstore.Product, expectedVersion *int) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, p, expectedVersion)
	}
	return nil
}

func (m *mockProductStore) Restock(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity, totalCost int, method string, note *string) (*productstore.Product, *productstore.StockMovement, error) {
	if m.RestockFn != nil {
		return m.RestockFn(ctx, complexID, productID, actorID, quantity, totalCost, method, note)
	}
	return nil, nil, data.ErrRecordNotFound
}

func (m *mockProductStore) Adjust(ctx context.Context, complexID, productID, actorID uuid.UUID, quantity int, reason string, note *string) (*productstore.Product, *productstore.StockMovement, error) {
	if m.AdjustFn != nil {
		return m.AdjustFn(ctx, complexID, productID, actorID, quantity, reason, note)
	}
	return nil, nil, data.ErrRecordNotFound
}

func (m *mockProductStore) ListStockMovements(ctx context.Context, complexID, productID uuid.UUID, filters data.Filters) ([]*productstore.StockMovement, data.Metadata, error) {
	if m.ListStockMovementsFn != nil {
		return m.ListStockMovementsFn(ctx, complexID, productID, filters)
	}
	return nil, data.Metadata{}, nil
}

// ---------------------------------------------------------------------------
// mockSalesStore
// ---------------------------------------------------------------------------

// mockSalesStore stands in for stores.SalesStore, the same shape
// mockProductStore is for stores.ProductStore.
type mockSalesStore struct {
	CreateFn             func(ctx context.Context, complexID, actorID uuid.UUID, items []salestore.ItemInput, method string, note *string) (*salestore.Sale, []*salestore.SaleItem, []salestore.StockWarning, error)
	GetByIDFn            func(ctx context.Context, complexID, saleID uuid.UUID) (*salestore.Sale, error)
	ListItemsBySaleFn    func(ctx context.Context, complexID, saleID uuid.UUID) ([]*salestore.SaleItem, error)
	ListItemsBySaleIDsFn func(ctx context.Context, complexID uuid.UUID, saleIDs []uuid.UUID) ([]*salestore.SaleItem, error)
	ListByComplexFn      func(ctx context.Context, complexID uuid.UUID, sessionID *uuid.UUID, filters data.Filters) ([]*salestore.Sale, data.Metadata, error)
	VoidFn               func(ctx context.Context, complexID, saleID, actorID uuid.UUID, note *string) (*salestore.Sale, error)
}

func (m *mockSalesStore) Create(ctx context.Context, complexID, actorID uuid.UUID, items []salestore.ItemInput, method string, note *string) (*salestore.Sale, []*salestore.SaleItem, []salestore.StockWarning, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, complexID, actorID, items, method, note)
	}
	return nil, nil, nil, data.ErrRecordNotFound
}

func (m *mockSalesStore) GetByID(ctx context.Context, complexID, saleID uuid.UUID) (*salestore.Sale, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, complexID, saleID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockSalesStore) ListItemsBySale(ctx context.Context, complexID, saleID uuid.UUID) ([]*salestore.SaleItem, error) {
	if m.ListItemsBySaleFn != nil {
		return m.ListItemsBySaleFn(ctx, complexID, saleID)
	}
	return nil, nil
}

func (m *mockSalesStore) ListItemsBySaleIDs(ctx context.Context, complexID uuid.UUID, saleIDs []uuid.UUID) ([]*salestore.SaleItem, error) {
	if m.ListItemsBySaleIDsFn != nil {
		return m.ListItemsBySaleIDsFn(ctx, complexID, saleIDs)
	}
	return nil, nil
}

func (m *mockSalesStore) ListByComplex(ctx context.Context, complexID uuid.UUID, sessionID *uuid.UUID, filters data.Filters) ([]*salestore.Sale, data.Metadata, error) {
	if m.ListByComplexFn != nil {
		return m.ListByComplexFn(ctx, complexID, sessionID, filters)
	}
	return nil, data.Metadata{}, nil
}

func (m *mockSalesStore) Void(ctx context.Context, complexID, saleID, actorID uuid.UUID, note *string) (*salestore.Sale, error) {
	if m.VoidFn != nil {
		return m.VoidFn(ctx, complexID, saleID, actorID, note)
	}
	return nil, data.ErrRecordNotFound
}

// ---------------------------------------------------------------------------
// mockPasswordResetStore
// ---------------------------------------------------------------------------

// mockPasswordResetStore stands in for stores.PasswordResetStore.
//
// It exists because newApplication's validateDeps rejects a nil store for
// every field stores.Stores composes — this one and mockLockStore were the two
// the harness left out, invisible to the boot-time checklist that preceded it
// because that list covered services, not the stores a service is built from.
type mockPasswordResetStore struct {
	InsertWithCooldownFn func(ctx context.Context, userID uuid.UUID, tokenHash []byte) error
	GetByHashFn          func(ctx context.Context, tokenHash []byte) (*authstore.PasswordResetToken, error)
	DeleteByUserFn       func(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredFn      func(ctx context.Context) error
}

func (m *mockPasswordResetStore) InsertWithCooldown(ctx context.Context, userID uuid.UUID, tokenHash []byte) error {
	if m.InsertWithCooldownFn != nil {
		return m.InsertWithCooldownFn(ctx, userID, tokenHash)
	}
	return nil
}

func (m *mockPasswordResetStore) GetByHash(ctx context.Context, tokenHash []byte) (*authstore.PasswordResetToken, error) {
	if m.GetByHashFn != nil {
		return m.GetByHashFn(ctx, tokenHash)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockPasswordResetStore) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	if m.DeleteByUserFn != nil {
		return m.DeleteByUserFn(ctx, userID)
	}
	return nil
}

func (m *mockPasswordResetStore) DeleteExpired(ctx context.Context) error {
	if m.DeleteExpiredFn != nil {
		return m.DeleteExpiredFn(ctx)
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockEmailChangeStore
// ---------------------------------------------------------------------------

// mockEmailChangeStore stands in for stores.EmailChangeStore, the same shape
// and the same reason as mockPasswordResetStore above.
type mockEmailChangeStore struct {
	PutFn              func(ctx context.Context, userID uuid.UUID, newEmail string, tokenHash []byte) error
	PeekFn             func(ctx context.Context, tokenHash []byte) (*authstore.EmailChangeRequest, error)
	ConsumeFn          func(ctx context.Context, tokenHash []byte) error
	GetPendingByUserFn func(ctx context.Context, userID uuid.UUID) (*authstore.EmailChangeRequest, error)
	DeleteExpiredFn    func(ctx context.Context) error
}

func (m *mockEmailChangeStore) Put(ctx context.Context, userID uuid.UUID, newEmail string, tokenHash []byte) error {
	if m.PutFn != nil {
		return m.PutFn(ctx, userID, newEmail, tokenHash)
	}
	return nil
}

func (m *mockEmailChangeStore) Peek(ctx context.Context, tokenHash []byte) (*authstore.EmailChangeRequest, error) {
	if m.PeekFn != nil {
		return m.PeekFn(ctx, tokenHash)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockEmailChangeStore) Consume(ctx context.Context, tokenHash []byte) error {
	if m.ConsumeFn != nil {
		return m.ConsumeFn(ctx, tokenHash)
	}
	return nil
}

func (m *mockEmailChangeStore) GetPendingByUser(ctx context.Context, userID uuid.UUID) (*authstore.EmailChangeRequest, error) {
	if m.GetPendingByUserFn != nil {
		return m.GetPendingByUserFn(ctx, userID)
	}
	return nil, data.ErrRecordNotFound
}

func (m *mockEmailChangeStore) DeleteExpired(ctx context.Context) error {
	if m.DeleteExpiredFn != nil {
		return m.DeleteExpiredFn(ctx)
	}
	return nil
}

// ---------------------------------------------------------------------------
// mockLockStore
// ---------------------------------------------------------------------------

// mockLockStore stands in for data.LockStore. The default grants the
// advisory lock unconditionally, which is correct for every test that does
// not care about cross-instance locking — payments.RetryFailedRefunds and the
// scheduler are the two callers, and both are exercised with a real fake
// elsewhere when the lock's behaviour itself is under test.
type mockLockStore struct {
	TryAdvisoryFn func(ctx context.Context, key string) (bool, func(), error)
}

func (m *mockLockStore) TryAdvisory(ctx context.Context, key string) (bool, func(), error) {
	if m.TryAdvisoryFn != nil {
		return m.TryAdvisoryFn(ctx, key)
	}
	return true, func() {}, nil
}

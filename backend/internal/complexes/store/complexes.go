package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

// Complex represents a padel venue owned by a user, including its MercadoPago integration state.
type Complex struct {
	ID                uuid.UUID `json:"id"`
	OwnerID           uuid.UUID `json:"owner_id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	Address           string    `json:"address"`
	City              string    `json:"city"`
	Province          string    `json:"province"`
	CountryCode       string    `json:"country_code"`
	Currency          string    `json:"currency"`
	Phone             string    `json:"phone"`
	Email             *string   `json:"email"`
	LogoURL           *string   `json:"logo_url"`
	CoverURL          *string   `json:"cover_url"`
	DepositPercentage int       `json:"deposit_percentage"`
	CancellationHours int       `json:"cancellation_hours"`
	Latitude          *float64  `json:"latitude"`
	Longitude         *float64  `json:"longitude"`
	IsActive          bool      `json:"is_active"`
	// What the venue offers, from a closed vocabulary (complexes_amenities_known). Always
	// a slice, never nil, so a complex with nothing ticked serialises as [] and
	// a client never has to tell "no amenities" apart from "field missing".
	Amenities []string `json:"amenities"`
	// How many courts this complex has, when the query that loaded it counted
	// them. Nil — and omitted from JSON — when it did not: only the owner's
	// list asks. Zero is a fact about the venue; nil is a fact about the query.
	CourtCount *int `json:"court_count,omitempty"`
	// Whether this venue can take an online payment.
	//
	// The same fact `mp_user_id` carries, named for what it enables instead of
	// for the credential behind it — and the difference is not cosmetic.
	// `mp_user_id` answers "can we charge online?" honestly, and three separate
	// places read it as "is this complex OK?": the selector card warned about
	// it, the onboarding fallback called such a complex incomplete, and the
	// manual booking form refused to save without it. None of those asked the
	// same question, and a club that takes cash lost its booking form to the
	// confusion.
	//
	// The public projection has carried this name since it was written, and
	// the public side never grew the bug. Naming it here closes the mould.
	PaymentsEnabled   bool `json:"payments_enabled"`
	mpAccessToken     *string
	mpAccessTokenErr  error
	mpRefreshToken    *string
	mpRefreshTokenErr error
	MPUserID          *string `json:"mp_user_id,omitempty"`
	// Version is the row's optimistic-concurrency counter, bumped by a trigger
	// on every UPDATE (db/migrations/003_optimistic_concurrency.sql). A client
	// that echoes it back on a PUT is told, with 409, when somebody else wrote
	// the row since it was read; a client that does not is not (API-08).
	Version int `json:"version"`
	// MPTokenExpiresAt is when the current OAuth access/refresh token pair
	// expires, per MercadoPago's own expires_in. Nil for a complex never
	// connected, or connected before this column existed — cronRefreshMPTokens
	// treats that the same as "needs a refresh".
	MPTokenExpiresAt *time.Time `json:"mp_token_expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Schedule defines the opening hours of a complex for one day of the week.
type Schedule struct {
	ID        uuid.UUID `json:"id"`
	ComplexID uuid.UUID `json:"complex_id"`
	Day       string    `json:"day"`
	OpenTime  string    `json:"open_time"`
	CloseTime string    `json:"close_time"`
	IsClosed  bool      `json:"is_closed"`
}

// ComplexSlug is the minimal projection of a complex needed to build sitemap entries.
type ComplexSlug struct {
	Slug      string    `json:"slug"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store implements ComplexStore against PostgreSQL.
type Store struct {
	DB *data.DB
	Q  *db.Queries
	// Keys opens and seals mp_access_token/mp_refresh_token. See
	// Config.Keys and crypto.Keyring — a nil Keys errors rather than
	// passing values through as plaintext.
	Keys *crypto.Keyring
}

// Insert creates a new complex and populates c with its generated ID and
// defaults, returning ErrDuplicateSlug when the public slug is already taken.
//
// The unique constraint is the authority on whether a slug is free, not the
// SlugExists check a handler runs first: that check is a fast path, and
// between it and this insert another request can take the name. Translating
// the violation here is what makes the losing request a 422 naming the field
// rather than a 500.
func (m *Store) Insert(ctx context.Context, c *Complex) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbComplex, err := m.Q.InsertComplex(ctx, db.InsertComplexParams{
		OwnerID:           data.UUIDToPg(c.OwnerID),
		Name:              c.Name,
		Slug:              c.Slug,
		Address:           c.Address,
		City:              c.City,
		Province:          c.Province,
		CountryCode:       c.CountryCode,
		Currency:          c.Currency,
		Phone:             c.Phone,
		Email:             data.TextToPg(c.Email),
		LogoUrl:           data.TextToPg(c.LogoURL),
		CoverUrl:          data.TextToPg(c.CoverURL),
		DepositPercentage: int32(c.DepositPercentage), //nolint:gosec // G115: validated 0..100 at complexes.go handler.
		CancellationHours: int32(c.CancellationHours), //nolint:gosec // G115: validated 0..168 at complexes.go handler.
		Latitude:          data.Float8ToPg(c.Latitude),
		Longitude:         data.Float8ToPg(c.Longitude),
		// The column is NOT NULL and a nil slice binds as NULL, which Postgres
		// takes as the value rather than as "use the default". Never send nil.
		Amenities: orEmpty(c.Amenities),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateSlug
		}
		return err
	}

	c.ID = data.PgToUUID(dbComplex.ID)
	c.IsActive = dbComplex.IsActive
	c.CreatedAt = data.PgToTime(dbComplex.CreatedAt)
	c.UpdatedAt = data.PgToTime(dbComplex.UpdatedAt)
	return nil
}

// GetByID returns the complex with the given ID, or ErrRecordNotFound if none exists.
func (m *Store) GetByID(ctx context.Context, id uuid.UUID) (*Complex, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbComplex, err := m.Q.GetComplexByID(ctx, data.UUIDToPg(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return complexFromDB(complexRowFromActive(dbComplex), m.Keys), nil
}

// GetBySlug returns the complex with the given public slug, or ErrRecordNotFound if none exists.
func (m *Store) GetBySlug(ctx context.Context, slug string) (*Complex, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbComplex, err := m.Q.GetComplexBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return complexFromDB(complexRowFromActive(dbComplex), m.Keys), nil
}

// GetByOwner returns every complex owned by the given user.
func (m *Store) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*Complex, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbComplexes, err := m.Q.GetComplexesByOwner(ctx, data.UUIDToPg(ownerID))
	if err != nil {
		return nil, err
	}

	result := make([]*Complex, len(dbComplexes))
	for i, c := range dbComplexes {
		// The row is the complex plus the aggregate; the shared mapper takes
		// the complex, and the count is attached after.
		complex := complexFromDB(db.Complex{
			ID: c.ID, OwnerID: c.OwnerID, Name: c.Name, Slug: c.Slug,
			Address: c.Address, City: c.City, Province: c.Province,
			CountryCode: c.CountryCode, Currency: c.Currency,
			Phone: c.Phone, Email: c.Email, LogoUrl: c.LogoUrl, CoverUrl: c.CoverUrl,
			DepositPercentage: c.DepositPercentage, CancellationHours: c.CancellationHours,
			IsActive: c.IsActive, MpAccessToken: c.MpAccessToken,
			MpRefreshToken: c.MpRefreshToken, MpUserID: c.MpUserID,
			Latitude: c.Latitude, Longitude: c.Longitude, Amenities: c.Amenities,
			DeletedAt: c.DeletedAt,
			CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		}, m.Keys)
		count := int(c.CourtCount)
		complex.CourtCount = &count
		result[i] = complex
	}
	return result, nil
}

// Update persists changes to an existing complex, returning ErrRecordNotFound
// if it no longer exists — or if it was changed by someone else first.
//
// H-14: the UPDATE carries an optimistic-concurrency precondition,
// `updated_at = c.UpdatedAt`, comparing against the timestamp the caller read
// the row with. c is always a row this package itself loaded a moment earlier
// (RequireComplexOwner's loadComplex, or the handler's own GetByID) and then
// mutated in place field by field, so c.UpdatedAt still holds that read-time
// value at this point — never a value this call is about to write. A second
// writer racing in between moves the row's real updated_at, the precondition
// no longer matches, and pgx.ErrNoRows becomes ErrRecordNotFound: a lost
// update is refused instead of silently overwriting the other request's
// change, and the handler already maps that error to a 409 conflict a caller
// can retry against the now-current row.
//
// expectedVersion is the caller's own precondition on top of that one: the
// version the client read before it filled in the form. Nil means it sent none,
// and the write is the last-write-wins it always was (API-08).
func (m *Store) Update(ctx context.Context, c *Complex, expectedVersion *int) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbComplex, err := m.Q.UpdateComplex(ctx, db.UpdateComplexParams{
		ExpectedVersion:   data.Int4PtrToPg(expectedVersion),
		Slug:              c.Slug,
		Name:              c.Name,
		Address:           c.Address,
		City:              c.City,
		Province:          c.Province,
		Phone:             c.Phone,
		Email:             data.TextToPg(c.Email),
		LogoUrl:           data.TextToPg(c.LogoURL),
		CoverUrl:          data.TextToPg(c.CoverURL),
		DepositPercentage: int32(c.DepositPercentage), //nolint:gosec // G115: validated 0..100 at complexes.go handler.
		CancellationHours: int32(c.CancellationHours), //nolint:gosec // G115: validated 0..168 at complexes.go handler.
		IsActive:          c.IsActive,
		Latitude:          data.Float8ToPg(c.Latitude),
		Longitude:         data.Float8ToPg(c.Longitude),
		Amenities:         c.Amenities,
		ID:                data.UUIDToPg(c.ID),
		UpdatedAt:         data.TimeToPg(c.UpdatedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		return err
	}

	c.UpdatedAt = data.PgToTime(dbComplex.UpdatedAt)
	c.Version = int(dbComplex.Version)
	return nil
}

// SoftDeleteCascade marks the complex as deleted without removing its row, and
// returns how many of its courts were deactivated with it.
//
// One transaction, one statement of intent. Stamping `complexes.deleted_at`
// fires the soft-delete cascade trigger, which stamps every live court of
// that complex; this method counts them before and verifies after, so the
// caller can say what happened and so a cascade that silently did not run
// becomes a failed request instead of an orphaned court.
//
// It replaced two calls in the handler — SoftDelete followed by the courts
// model's own SoftDeleteByComplex — that were separate statements outside any
// transaction, with the second one logged-and-ignored on failure. A crash
// between them left the venue deleted and its courts live.
//
// ErrRecordNotFound means the row is not there or was already deleted: the
// UPDATE carries `AND deleted_at IS NULL`, so a second delete is not an error
// to report as a success.
func (m *Store) SoftDeleteCascade(ctx context.Context, id uuid.UUID) (int, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var deactivated int
	err := m.DB.WithTx(ctx, func(tx pgx.Tx, q *db.Queries) error {
		return m.softDeleteCascade(ctx, tx, q, id, &deactivated)
	})
	if err != nil {
		return 0, err
	}
	return deactivated, nil
}

// softDeleteCascade is SoftDeleteCascade's body, inside the transaction.
func (m *Store) softDeleteCascade(
	ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID, deactivated *int,
) error {
	// Counted before the UPDATE, because after it there are none left to count.
	// FOR UPDATE on nothing: the count is taken inside the same transaction as
	// the write, and the trigger's own UPDATE takes the row locks, so a court
	// created concurrently is either already visible here (and closed) or
	// refused outright by courts_forbid_live_under_deleted_complex once this
	// transaction commits.
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM courts WHERE complex_id = $1 AND deleted_at IS NULL`,
		id).Scan(deactivated)
	if err != nil {
		return fmt.Errorf("count live courts: %w", err)
	}

	rows, err := q.SoftDeleteComplex(ctx, data.UUIDToPg(id))
	if err != nil {
		return fmt.Errorf("soft-delete complex: %w", err)
	}
	if rows == 0 {
		return data.ErrRecordNotFound
	}

	// The verification the trigger exists to make unnecessary, asserted anyway:
	// this is the one place that knows a cascade was supposed to happen, so it
	// is the one place that can tell the difference between "there were no
	// courts" and "the trigger did not fire".
	var stillLive int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM courts WHERE complex_id = $1 AND deleted_at IS NULL`,
		id).Scan(&stillLive)
	if err != nil {
		return fmt.Errorf("verify court cascade: %w", err)
	}
	if stillLive != 0 {
		return fmt.Errorf("soft-deleting complex %s left %d live court(s): the cascade trigger did not run", id, stillLive)
	}
	return nil
}

// UpsertSchedule creates or replaces the opening hours for one day of a complex's schedule.
func (m *Store) UpsertSchedule(ctx context.Context, s *Schedule) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbSchedule, err := m.Q.UpsertSchedule(ctx, db.UpsertScheduleParams{
		ComplexID: data.UUIDToPg(s.ComplexID),
		Day:       db.DayOfWeek(s.Day),
		OpenTime:  data.TimeStrToPg(s.OpenTime),
		CloseTime: data.TimeStrToPg(s.CloseTime),
		IsClosed:  s.IsClosed,
	})
	if err != nil {
		return err
	}

	s.ID = data.PgToUUID(dbSchedule.ID)
	return nil
}

// GetSchedules returns the complex's full weekly opening schedule.
func (m *Store) GetSchedules(ctx context.Context, complexID uuid.UUID) ([]*Schedule, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbSchedules, err := m.Q.GetSchedulesByComplex(ctx, data.UUIDToPg(complexID))
	if err != nil {
		return nil, err
	}

	result := make([]*Schedule, len(dbSchedules))
	for i, s := range dbSchedules {
		result[i] = &Schedule{
			ID:        data.PgToUUID(s.ID),
			ComplexID: data.PgToUUID(s.ComplexID),
			Day:       string(s.Day),
			OpenTime:  data.PgToTimeStr(s.OpenTime),
			CloseTime: data.PgToTimeStr(s.CloseTime),
			IsClosed:  s.IsClosed,
		}
	}
	return result, nil
}

// UpdateMPCredentials stores the complex's MercadoPago OAuth access token, refresh token and user ID.
//
// It refuses an empty accessToken, refreshToken or userID with
// mpcred.ErrMPCredentialEmpty before any sealing is attempted — the
// CHECK (mp_access_token <> the empty string, etc.) is a floor under this refusal, not the
// guard itself; nothing downstream may ever store an empty value that later
// reads as "connected".
//
// expiresIn is MercadoPago's own expires_in from the OAuth exchange/refresh
// response, in seconds. It is stored as now()+expiresIn so
// ListComplexesNeedingMPRefresh can tell a freshly issued token from one
// close to expiring, rather than refreshing every connected complex on every
// cron tick. Zero (an OAuth response that carried no expires_in) stores NULL,
// which ListComplexesNeedingMPRefresh also treats as due for a refresh.
func (m *Store) UpdateMPCredentials(ctx context.Context, complexID uuid.UUID, accessToken, refreshToken, userID string, expiresIn int) error {
	if accessToken == "" || refreshToken == "" || userID == "" {
		return mpcred.ErrMPCredentialEmpty
	}

	sealedAccess, err := m.Keys.Seal(mpcred.AAD(complexID, mpcred.AccessTokenColumn), accessToken)
	if err != nil {
		return fmt.Errorf("data: sealing mp_access_token: %w", err)
	}
	sealedRefresh, err := m.Keys.Seal(mpcred.AAD(complexID, mpcred.RefreshTokenColumn), refreshToken)
	if err != nil {
		return fmt.Errorf("data: sealing mp_refresh_token: %w", err)
	}

	var expiresAt pgtype.Timestamptz
	if expiresIn > 0 {
		expiresAt = data.TimeToPg(time.Now().Add(time.Duration(expiresIn) * time.Second))
	}

	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err = m.DB.Exec(ctx,
		`UPDATE complexes SET mp_access_token = $1, mp_refresh_token = $2, mp_user_id = $3, mp_token_expires_at = $4 WHERE id = $5`,
		sealedAccess, sealedRefresh, userID, expiresAt, data.UUIDToPg(complexID))
	return err
}

// ClearMPCredentials disconnects the complex's MercadoPago OAuth integration.
func (m *Store) ClearMPCredentials(ctx context.Context, complexID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx,
		`UPDATE complexes SET mp_access_token = NULL, mp_refresh_token = NULL, mp_user_id = NULL, mp_token_expires_at = NULL WHERE id = $1`,
		data.UUIDToPg(complexID))
	return err
}

// ListComplexesNeedingMPRefresh returns connected complexes whose OAuth token
// has no known expiry or expires within 30 days — see the query's own
// comment in db/queries/complexes.sql for why.
func (m *Store) ListComplexesNeedingMPRefresh(ctx context.Context) ([]*Complex, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbComplexes, err := m.Q.ListComplexesNeedingMPRefresh(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*Complex, len(dbComplexes))
	for i, c := range dbComplexes {
		result[i] = complexFromDB(complexRowFromActive(c), m.Keys)
	}
	return result, nil
}

// GetWithMPConnected returns all active complexes that have MercadoPago OAuth connected.
func (m *Store) GetWithMPConnected(ctx context.Context) ([]*Complex, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx,
		`SELECT id, owner_id, name, slug, address, city, province, country_code, currency,
		        phone, email, logo_url, cover_url, deposit_percentage, cancellation_hours, is_active,
		        mp_access_token, mp_refresh_token, mp_user_id, latitude, longitude,
		        deleted_at, created_at, updated_at
		 FROM active_complexes
		 WHERE mp_refresh_token IS NOT NULL
		   AND is_active = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*Complex, 0, 16)
	for rows.Next() {
		var c db.Complex
		if err := rows.Scan(
			&c.ID, &c.OwnerID, &c.Name, &c.Slug,
			&c.Address, &c.City, &c.Province, &c.CountryCode, &c.Currency,
			&c.Phone, &c.Email, &c.LogoUrl, &c.CoverUrl,
			&c.DepositPercentage, &c.CancellationHours, &c.IsActive,
			&c.MpAccessToken, &c.MpRefreshToken, &c.MpUserID,
			&c.Latitude, &c.Longitude,
			&c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("complexes: scan mp-connected row: %w", err)
		}
		result = append(result, complexFromDB(c, m.Keys))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("complexes: get mp-connected complexes: %w", err)
	}
	return result, nil
}

// SlugsWithPrefix returns every slug equal to base or starting with "base-".
//
// One query instead of a probe per candidate: suggesting a free variant of
// "club-norte" means knowing which of club-norte-2, -3, -4 are gone, and
// asking that one at a time is N round trips whose answers can each go stale
// before the next one lands.
//
// Like SlugExists it counts soft-deleted rows, for the same reason: the UNIQUE
// constraint spans the whole table, so a suggestion that ignored them would be
// refused by the insert it was made for.
func (m *Store) SlugsWithPrefix(ctx context.Context, base string) ([]string, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx,
		`SELECT slug FROM complexes WHERE slug = $1 OR slug LIKE $1 || '-%'`, base)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("complexes: scan slug row: %w", err)
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("complexes: list slugs with prefix: %w", err)
	}
	return slugs, nil
}

// SlugExists reports whether the given slug is already taken.
//
// It counts soft-deleted complexes, because complexes.slug is UNIQUE across
// the whole table: excluding them made this the only place in the codebase
// that answered "free" about a name the database would refuse, so creating a
// venue with a deleted one's slug passed validation and then failed on the
// insert with a 500 the owner could do nothing about.
//
// Keeping the name reserved is the product decision behind that constraint.
// The slug is the venue's public URL — it lives in shared links, WhatsApp
// messages, QR codes and the sitemap (GetAllSlugs) — so releasing it would
// point somebody else's traffic at a different business, and would make
// restoring a soft-deleted complex impossible without renaming it.
func (m *Store) SlugExists(ctx context.Context, slug string) (bool, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	var exists bool
	err := m.DB.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM complexes WHERE slug = $1)`, slug).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("complexes: slug exists: %w", err)
	}
	return exists, nil
}

// GetAllSlugs returns the slug and last-updated time of every active complex, for sitemap generation.
func (m *Store) GetAllSlugs(ctx context.Context) ([]ComplexSlug, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.DB.Query(ctx,
		`SELECT slug, updated_at FROM active_complexes WHERE is_active = true ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []ComplexSlug
	for rows.Next() {
		var s ComplexSlug
		if err := rows.Scan(&s.Slug, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("complexes: scan slug entry: %w", err)
		}
		slugs = append(slugs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("complexes: get all slugs: %w", err)
	}
	return slugs, nil
}

// complexFromDB is the single decode point for the mp_access_token /
// mp_refresh_token columns: it is where ciphertext becomes plaintext (or a
// recorded open failure) and nowhere else. keys may be nil — Keyring.Open
// on a nil *Keyring errors rather than passing the raw column through.
// complexRowFromActive re-labels a row of the active_complexes view as a row of
// the complexes table.
//
// sqlc treats a view as its own relation, so `SELECT * FROM active_complexes`
// yields db.ActiveComplex: the same columns, in the same order, with the same
// types, under a different name. Rather than a second copy of complexFromDB
// that would have to be kept in step with the first, the row is relabelled once
// here. GetByOwner does the same thing for its own aggregate row struct.
//
// A plain conversion, not a field-by-field copy, and that is the guarantee:
// Go permits it only while the two structs have identical field names, types
// and order. Add a column to `complexes` without replacing the view in the same
// migration and this line stops compiling, which is where that mistake should
// surface rather than in a struct silently missing a field.
func complexRowFromActive(c db.ActiveComplex) db.Complex {
	return db.Complex(c)
}

func complexFromDB(c db.Complex, keys *crypto.Keyring) *Complex {
	complex := &Complex{
		ID:                data.PgToUUID(c.ID),
		OwnerID:           data.PgToUUID(c.OwnerID),
		Name:              c.Name,
		Slug:              c.Slug,
		Address:           c.Address,
		City:              c.City,
		Province:          c.Province,
		CountryCode:       c.CountryCode,
		Currency:          c.Currency,
		Phone:             c.Phone,
		Email:             data.PgToTextPtr(c.Email),
		LogoURL:           data.PgToTextPtr(c.LogoUrl),
		CoverURL:          data.PgToTextPtr(c.CoverUrl),
		DepositPercentage: int(c.DepositPercentage),
		CancellationHours: int(c.CancellationHours),
		Latitude:          data.PgToFloat8Ptr(c.Latitude),
		Longitude:         data.PgToFloat8Ptr(c.Longitude),
		Amenities:         orEmpty(c.Amenities),
		IsActive:          c.IsActive,
		MPUserID:          data.PgToTextPtr(c.MpUserID),
		MPTokenExpiresAt:  data.PgToTimePtr(c.MpTokenExpiresAt),
		CreatedAt:         data.PgToTime(c.CreatedAt),
		UpdatedAt:         data.PgToTime(c.UpdatedAt),
		Version:           int(c.Version),
	}

	complex.mpAccessToken, complex.mpAccessTokenErr = mpcred.Open(keys, complex.ID, mpcred.AccessTokenColumn, data.PgToTextPtr(c.MpAccessToken))
	complex.mpRefreshToken, complex.mpRefreshTokenErr = mpcred.Open(keys, complex.ID, mpcred.RefreshTokenColumn, data.PgToTextPtr(c.MpRefreshToken))

	// Derived here, once, so no caller has to remember to compute it — and so
	// nobody has to reach for `mp_user_id` to answer a question this already
	// answers.
	complex.PaymentsEnabled = complex.MPConnected()

	return complex
}

// orEmpty turns a nil slice into an empty one.
//
// Postgres hands back a nil []string for '{}', and a nil slice marshals to
// JSON `null` while an empty one marshals to `[]`. A client reading amenities
// should never have to handle both for the same fact: "this venue lists
// nothing" is a list of length zero, not an absent field.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

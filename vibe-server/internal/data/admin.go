package data

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PlatformStats contains platform-wide aggregate statistics.
type PlatformStats struct {
	TotalUsers        int `json:"total_users"`
	ActiveUsers       int `json:"active_users"`
	NewUsersMonth     int `json:"new_users_month"`
	TotalComplexes    int `json:"total_complexes"`
	NewComplexesMonth int `json:"new_complexes_month"`
	TotalCourts       int `json:"total_courts"`
	TotalBookings     int `json:"total_bookings"`
	TotalRevenue      int `json:"total_revenue"`
}

// AdminUserRow represents a user with aggregate counts for admin listing.
type AdminUserRow struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Phone         string    `json:"phone"`
	Role          string    `json:"role"`
	IsActive      bool      `json:"is_active"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	ComplexCount  int       `json:"complex_count"`
}

// CursorKey returns the (created-at, ID) pair used to build a pagination cursor for this row.
func (u *AdminUserRow) CursorKey() (time.Time, uuid.UUID) { return u.CreatedAt, u.ID }

// AdminUserDetail contains a user along with their owned complexes.
type AdminUserDetail struct {
	User      *User      `json:"user"`
	Complexes []*Complex `json:"complexes"`
}

// AdminComplexRow represents a complex with owner info for admin listing.
type AdminComplexRow struct {
	ID          uuid.UUID `json:"id"`
	OwnerID     uuid.UUID `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	OwnerEmail  string    `json:"owner_email"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	City        string    `json:"city"`
	IsActive    bool      `json:"is_active"`
	CourtsCount int       `json:"courts_count"`
	MPConnected bool      `json:"mp_connected"`
	CreatedAt   time.Time `json:"created_at"`
}

// CursorKey returns the (created-at, ID) pair used to build a pagination cursor for this row.
func (c *AdminComplexRow) CursorKey() (time.Time, uuid.UUID) { return c.CreatedAt, c.ID }

// AdminComplexDetail contains a complex with aggregated statistics.
type AdminComplexDetail struct {
	Complex       *Complex `json:"complex"`
	OwnerName     string   `json:"owner_name"`
	OwnerEmail    string   `json:"owner_email"`
	CourtsCount   int      `json:"courts_count"`
	ClientsCount  int      `json:"clients_count"`
	BookingsCount int      `json:"bookings_count"`
	TotalRevenue  int      `json:"total_revenue"`
}

// AuditLogRow represents an audit log entry for admin listing.
type AuditLogRow struct {
	ID         uuid.UUID  `json:"id"`
	UserID     *uuid.UUID `json:"user_id"`
	UserEmail  *string    `json:"user_email"`
	ComplexID  *uuid.UUID `json:"complex_id"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   *uuid.UUID `json:"entity_id"`
	IPAddress  *string    `json:"ip_address"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CursorKey returns the (created-at, ID) pair used to build a pagination cursor for this row.
func (l *AuditLogRow) CursorKey() (time.Time, uuid.UUID) { return l.CreatedAt, l.ID }

// AdminReader provides read-only platform admin queries.
type AdminReader interface {
	GetPlatformStats(ctx context.Context) (*PlatformStats, error)
	ListUsers(ctx context.Context, search, roleFilter string, filters Filters) ([]*AdminUserRow, Metadata, error)
	GetUserDetail(ctx context.Context, userID uuid.UUID) (*AdminUserDetail, error)
	ListComplexes(ctx context.Context, search string, filters Filters) ([]*AdminComplexRow, Metadata, error)
	GetComplexDetail(ctx context.Context, complexID uuid.UUID) (*AdminComplexDetail, error)
	ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters Filters) ([]*AuditLogRow, Metadata, error)
}

// AdminWriter provides state-changing admin operations.
type AdminWriter interface {
	ToggleUserActive(ctx context.Context, userID uuid.UUID, isActive bool) error
	// InsertAuditLog takes its two values as already-encoded JSON: the caller
	// encodes on its own goroutine so the background write never reads a struct
	// the caller still owns.
	InsertAuditLog(ctx context.Context, userID, complexID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldJSON, newJSON []byte, ipAddr string) error
}

// AdminStore defines the full interface for platform admin operations.
type AdminStore interface {
	AdminReader
	AdminWriter
}

// AdminModel implements AdminStore using raw SQL queries.
type AdminModel struct {
	DB *DB
}

// GetPlatformStats computes platform-wide user, complex, court, booking and revenue counters.
func (m *AdminModel) GetPlatformStats(ctx context.Context) (*PlatformStats, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	var stats PlatformStats
	err := m.DB.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users)::int,
			(SELECT COUNT(*) FROM users WHERE is_active = true)::int,
			(SELECT COUNT(*) FROM users WHERE created_at >= NOW() - INTERVAL '30 days')::int,
			(SELECT COUNT(*) FROM complexes WHERE deleted_at IS NULL)::int,
			(SELECT COUNT(*) FROM complexes WHERE deleted_at IS NULL AND created_at >= NOW() - INTERVAL '30 days')::int,
			(SELECT COUNT(*) FROM courts WHERE deleted_at IS NULL)::int,
			(SELECT COUNT(*) FROM bookings WHERE status != 'cancelled')::int,
			(SELECT COALESCE(SUM(amount), 0) FROM payments WHERE status != 'refunded')::bigint
	`).Scan(
		&stats.TotalUsers,
		&stats.ActiveUsers,
		&stats.NewUsersMonth,
		&stats.TotalComplexes,
		&stats.NewComplexesMonth,
		&stats.TotalCourts,
		&stats.TotalBookings,
		&stats.TotalRevenue,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// ListUsers returns a paginated, optionally search- and role-filtered list of platform users.
func (m *AdminModel) ListUsers(ctx context.Context, search, roleFilter string, filters Filters) ([]*AdminUserRow, Metadata, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, Metadata{}, err
	}

	hasCursor := !cursorTime.IsZero()
	fetchLimit := filters.Limit + 1

	// H-04: escaped so a literal '%' or '_' in the search box matches only
	// itself instead of widening the search — see escapeLikeTerm's comment.
	// The `$1 = ''` "no search" guard above stays correct against the escaped
	// value: escaping an empty string still yields an empty string.
	escapedSearch := escapeLikeTerm(search)

	rows, err := m.DB.Query(ctx, `
		SELECT u.id, u.email, u.first_name, u.last_name, u.phone, u.role,
		       u.is_active, u.email_verified, u.created_at,
		       COUNT(c.id)::int AS complex_count
		FROM users u
		LEFT JOIN complexes c ON c.owner_id = u.id AND c.deleted_at IS NULL
		WHERE ($1 = '' OR u.first_name ILIKE '%' || $1 || '%' ESCAPE '\'
		       OR u.last_name ILIKE '%' || $1 || '%' ESCAPE '\'
		       OR u.email::text ILIKE '%' || $1 || '%' ESCAPE '\')
		  AND ($2 = '' OR u.role::text = $2)
		  AND (NOT $3 OR (u.created_at, u.id) < ($4, $5))
		GROUP BY u.id
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT $6
	`, escapedSearch, roleFilter, hasCursor, cursorTime, cursorID, fetchLimit)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	users := make([]*AdminUserRow, 0, filters.Limit)
	for rows.Next() {
		var u AdminUserRow
		err := rows.Scan(
			&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Phone, &u.Role,
			&u.IsActive, &u.EmailVerified, &u.CreatedAt, &u.ComplexCount,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		users = append(users, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	users, meta := TrimPage(users, filters.Limit, BuildTimestampCursor)
	return users, meta, nil
}

// GetUserDetail returns the user along with the complexes they own, or ErrRecordNotFound if the user does not exist.
//
//nolint:funlen // single cohesive fetch-user-then-fetch-owned-complexes flow for one admin query
func (m *AdminModel) GetUserDetail(ctx context.Context, userID uuid.UUID) (*AdminUserDetail, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	var user User
	err := m.DB.QueryRow(ctx, `
		SELECT id, email, first_name, last_name, phone, role,
		       is_active, email_verified, created_at, updated_at
		FROM users
		WHERE id = $1`, userID).Scan(
		&user.ID, &user.Email,
		&user.FirstName, &user.LastName, &user.Phone, &user.Role,
		&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	rows, err := m.DB.Query(ctx, `
		SELECT id, owner_id, name, slug, address, city, province,
		       country_code, currency, phone, email, logo_url, cover_url,
		       deposit_percentage, cancellation_hours, latitude, longitude,
		       is_active, mp_user_id, created_at, updated_at
		FROM complexes
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	complexes := make([]*Complex, 0, 4)
	for rows.Next() {
		var c Complex
		var email, logoURL, coverURL, mpUserID *string
		var lat, lng *float64
		err := rows.Scan(
			&c.ID, &c.OwnerID, &c.Name, &c.Slug, &c.Address, &c.City, &c.Province,
			&c.CountryCode, &c.Currency, &c.Phone, &email, &logoURL, &coverURL,
			&c.DepositPercentage, &c.CancellationHours, &lat, &lng,
			&c.IsActive, &mpUserID, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		c.Email = email
		c.LogoURL = logoURL
		c.CoverURL = coverURL
		c.Latitude = lat
		c.Longitude = lng
		c.MPUserID = mpUserID
		complexes = append(complexes, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &AdminUserDetail{User: &user, Complexes: complexes}, nil
}

// ListComplexes returns a paginated, optionally search-filtered list of complexes with owner info.
func (m *AdminModel) ListComplexes(ctx context.Context, search string, filters Filters) ([]*AdminComplexRow, Metadata, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, Metadata{}, err
	}

	hasCursor := !cursorTime.IsZero()
	fetchLimit := filters.Limit + 1

	// H-04: same escaping as ListUsers above, and for the same reason — see
	// escapeLikeTerm's comment.
	escapedSearch := escapeLikeTerm(search)

	rows, err := m.DB.Query(ctx, `
		SELECT c.id, c.owner_id,
		       u.first_name || ' ' || u.last_name AS owner_name,
		       u.email AS owner_email,
		       c.name, c.slug, c.city, c.is_active,
		       (SELECT COUNT(*)::int FROM courts WHERE complex_id = c.id AND deleted_at IS NULL) AS courts_count,
		       c.mp_user_id IS NOT NULL AS mp_connected,
		       c.created_at
		FROM complexes c
		JOIN users u ON u.id = c.owner_id
		WHERE c.deleted_at IS NULL
		  AND ($1 = '' OR c.name ILIKE '%' || $1 || '%' ESCAPE '\' OR c.city ILIKE '%' || $1 || '%' ESCAPE '\')
		  AND (NOT $2 OR (c.created_at, c.id) < ($3, $4))
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT $5
	`, escapedSearch, hasCursor, cursorTime, cursorID, fetchLimit)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	complexes := make([]*AdminComplexRow, 0, filters.Limit)
	for rows.Next() {
		var c AdminComplexRow
		err := rows.Scan(
			&c.ID, &c.OwnerID, &c.OwnerName, &c.OwnerEmail,
			&c.Name, &c.Slug, &c.City, &c.IsActive,
			&c.CourtsCount, &c.MPConnected, &c.CreatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		complexes = append(complexes, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	complexes, meta := TrimPage(complexes, filters.Limit, BuildTimestampCursor)
	return complexes, meta, nil
}

// GetComplexDetail returns the complex with owner info and aggregate statistics, or ErrRecordNotFound if it does not exist.
func (m *AdminModel) GetComplexDetail(ctx context.Context, complexID uuid.UUID) (*AdminComplexDetail, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	var c Complex
	var email, logoURL, coverURL, mpUserID *string
	var lat, lng *float64
	var ownerName, ownerEmail string
	var courtsCount, clientsCount, bookingsCount, totalRevenue int

	err := m.DB.QueryRow(ctx, `
		SELECT c.id, c.owner_id, c.name, c.slug, c.address, c.city, c.province,
		       c.country_code, c.currency, c.phone, c.email, c.logo_url, c.cover_url,
		       c.deposit_percentage, c.cancellation_hours, c.latitude, c.longitude,
		       c.is_active, c.mp_user_id, c.created_at, c.updated_at,
		       u.first_name || ' ' || u.last_name AS owner_name,
		       u.email AS owner_email,
		       (SELECT COUNT(*)::int FROM courts WHERE complex_id = c.id AND deleted_at IS NULL),
		       (SELECT COUNT(DISTINCT client_id)::int FROM bookings WHERE complex_id = c.id AND status != 'cancelled'),
		       (SELECT COUNT(*)::int FROM bookings WHERE complex_id = c.id AND status != 'cancelled'),
		       (SELECT COALESCE(SUM(amount), 0)::bigint FROM payments WHERE complex_id = c.id AND status != 'refunded')
		FROM complexes c
		JOIN users u ON u.id = c.owner_id
		WHERE c.id = $1 AND c.deleted_at IS NULL`, complexID).Scan(
		&c.ID, &c.OwnerID, &c.Name, &c.Slug, &c.Address, &c.City, &c.Province,
		&c.CountryCode, &c.Currency, &c.Phone, &email, &logoURL, &coverURL,
		&c.DepositPercentage, &c.CancellationHours, &lat, &lng,
		&c.IsActive, &mpUserID, &c.CreatedAt, &c.UpdatedAt,
		&ownerName, &ownerEmail,
		&courtsCount, &clientsCount, &bookingsCount, &totalRevenue,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	c.Email = email
	c.LogoURL = logoURL
	c.CoverURL = coverURL
	c.Latitude = lat
	c.Longitude = lng
	c.MPUserID = mpUserID

	return &AdminComplexDetail{
		Complex:       &c,
		OwnerName:     ownerName,
		OwnerEmail:    ownerEmail,
		CourtsCount:   courtsCount,
		ClientsCount:  clientsCount,
		BookingsCount: bookingsCount,
		TotalRevenue:  totalRevenue,
	}, nil
}

// ToggleUserActive sets a user's active status, returning ErrRecordNotFound if the user does not exist.
func (m *AdminModel) ToggleUserActive(ctx context.Context, userID uuid.UUID, isActive bool) error {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	result, err := m.DB.Exec(ctx, `UPDATE users SET is_active = $1 WHERE id = $2`, isActive, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// listAuditLogsSQL is the audit-log page query.
//
// It is a constant rather than a literal inside the method so the plan assertion
// in admin_integration_test.go can EXPLAIN the exact statement this store issues.
// Its unscoped form — complex_id NULL, the default superadmin view — is served by
// idx_audit_log_created_at; before that index existed it seq-scanned
// the whole table and blew past the queryContext budget at roughly 372,000 rows.
const listAuditLogsSQL = `
	SELECT a.id, a.user_id, u.email, a.complex_id, a.action, a.entity_type,
	       a.entity_id, a.ip_address::text, a.created_at
	FROM audit_log a
	LEFT JOIN users u ON u.id = a.user_id
	WHERE ($1::uuid IS NULL OR a.complex_id = $1)
	  AND ($2 = '' OR a.entity_type = $2)
	  AND (NOT $3 OR (a.created_at, a.id) < ($4, $5))
	ORDER BY a.created_at DESC, a.id DESC
	LIMIT $6
`

// ListAuditLogs returns a paginated, optionally complex- and entity-type-filtered list of audit log entries.
func (m *AdminModel) ListAuditLogs(ctx context.Context, complexID *uuid.UUID, entityType string, filters Filters) ([]*AuditLogRow, Metadata, error) {
	ctx, cancel := queryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, Metadata{}, err
	}

	hasCursor := !cursorTime.IsZero()
	fetchLimit := filters.Limit + 1

	var cID *uuid.UUID
	if complexID != nil {
		cID = complexID
	}

	rows, err := m.DB.Query(ctx, listAuditLogsSQL, cID, entityType, hasCursor, cursorTime, cursorID, fetchLimit)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	logs := make([]*AuditLogRow, 0, filters.Limit)
	for rows.Next() {
		var l AuditLogRow
		err := rows.Scan(
			&l.ID, &l.UserID, &l.UserEmail, &l.ComplexID, &l.Action, &l.EntityType,
			&l.EntityID, &l.IPAddress, &l.CreatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		logs = append(logs, &l)
	}
	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	logs, meta := TrimPage(logs, filters.Limit, BuildTimestampCursor)
	return logs, meta, nil
}

// InsertAuditLog records an admin or system action.
//
// oldJSON and newJSON are already-encoded JSON, or nil for "no value", which is
// stored as SQL NULL. This method used to take `any` and marshal here, but it
// runs on a background goroutine while the caller still owns the struct it
// handed over: encoding moved to the caller's goroutine (internal/audit.Record)
// so the snapshot is taken before anything can be scheduled against it.
func (m *AdminModel) InsertAuditLog(ctx context.Context, userID, complexID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldJSON, newJSON []byte, ipAddr string) error {
	var ip *netip.Addr
	if ipAddr != "" {
		if parsed, parseErr := netip.ParseAddr(ipAddr); parseErr == nil {
			ip = &parsed
		}
	}

	// The pool runs in pgx's QueryExecModeExec (cmd/api/main.go), which never
	// asks the server for parameter types and instead infers them from the Go
	// values: a []byte is sent as bytea, and Postgres refuses to read bytea as
	// json ("invalid input syntax for type json"). Every audit row used to fail
	// that way — silently but for one log line per action — so the trail was
	// empty. The payloads travel as text with an explicit cast, and "no value"
	// as an untyped nil, which is the one thing that reaches the driver as NULL.
	_, err := m.DB.Exec(ctx, `
		INSERT INTO audit_log (user_id, complex_id, action, entity_type, entity_id, old_value, new_value, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8)
	`, userID, complexID, action, entityType, entityID, jsonTextOrNull(oldJSON), jsonTextOrNull(newJSON), ip)
	return err
}

// jsonTextOrNull hands an encoded JSON payload to the driver as text (cast to
// jsonb in the statement), or as NULL when there is no value.
func jsonTextOrNull(encoded []byte) any {
	if len(encoded) == 0 {
		return nil
	}
	return string(encoded)
}

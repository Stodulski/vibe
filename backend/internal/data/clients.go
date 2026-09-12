package data

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/db"
)

// Client represents a customer who books courts within a complex.
type Client struct {
	ID            uuid.UUID `json:"id"`
	ComplexID     uuid.UUID `json:"complex_id"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Phone         string    `json:"phone"`
	Email         *string   `json:"email,omitempty"`
	Notes         *string   `json:"notes,omitempty"`
	IsBlocked     bool      `json:"is_blocked"`
	TotalBookings int       `json:"total_bookings"`
	NoShows       int       `json:"no_shows"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CursorKey returns the (created-at, ID) pair used to build a pagination cursor for this client.
func (c *Client) CursorKey() (time.Time, uuid.UUID) { return c.CreatedAt, c.ID }

// ClientModel implements ClientStore against PostgreSQL.
type ClientModel struct {
	DB *DB
	Q  *db.Queries
}

// Insert creates a new client and populates c with its generated ID and defaults.
func (m *ClientModel) Insert(ctx context.Context, c *Client) error {
	dbClient, err := m.Q.InsertClient(ctx, db.InsertClientParams{
		ComplexID: UUIDToPg(c.ComplexID),
		FirstName: c.FirstName,
		LastName:  c.LastName,
		Phone:     c.Phone,
		Email:     TextToPg(c.Email),
		Notes:     TextToPg(c.Notes),
	})
	if err != nil {
		return err
	}

	c.ID = PgToUUID(dbClient.ID)
	c.IsBlocked = dbClient.IsBlocked
	c.TotalBookings = int(dbClient.TotalBookings)
	c.NoShows = int(dbClient.NoShows)
	c.CreatedAt = PgToTime(dbClient.CreatedAt)
	c.UpdatedAt = PgToTime(dbClient.UpdatedAt)
	return nil
}

// GetByID returns the client with the given ID, or ErrRecordNotFound if none exists.
//
// total_bookings is computed live from the bookings table (confirmed/completed/
// no_show), not read off the clients.total_bookings column: that counter was
// only ever incremented from the MercadoPago webhook, so it silently stayed
// at 0 for every booking the owner confirmed manually or by cash/transfer.
func (m *ClientModel) GetByID(ctx context.Context, id uuid.UUID) (*Client, error) {
	row, err := m.Q.GetClientByID(ctx, UUIDToPg(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &Client{
		ID:            PgToUUID(row.ID),
		ComplexID:     PgToUUID(row.ComplexID),
		FirstName:     row.FirstName,
		LastName:      row.LastName,
		Phone:         row.Phone,
		Email:         PgToTextPtr(row.Email),
		Notes:         PgToTextPtr(row.Notes),
		IsBlocked:     row.IsBlocked,
		TotalBookings: int(row.TotalBookings),
		NoShows:       int(row.NoShows),
		CreatedAt:     PgToTime(row.CreatedAt),
		UpdatedAt:     PgToTime(row.UpdatedAt),
	}, nil
}

// GetByPhone returns the client with the given phone number within a complex, or ErrRecordNotFound if none exists.
// total_bookings is a live count — see GetByID's comment for why.
func (m *ClientModel) GetByPhone(ctx context.Context, complexID uuid.UUID, phone string) (*Client, error) {
	row, err := m.Q.GetClientByPhone(ctx, db.GetClientByPhoneParams{
		ComplexID: UUIDToPg(complexID),
		Phone:     phone,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &Client{
		ID:            PgToUUID(row.ID),
		ComplexID:     PgToUUID(row.ComplexID),
		FirstName:     row.FirstName,
		LastName:      row.LastName,
		Phone:         row.Phone,
		Email:         PgToTextPtr(row.Email),
		Notes:         PgToTextPtr(row.Notes),
		IsBlocked:     row.IsBlocked,
		TotalBookings: int(row.TotalBookings),
		NoShows:       int(row.NoShows),
		CreatedAt:     PgToTime(row.CreatedAt),
		UpdatedAt:     PgToTime(row.UpdatedAt),
	}, nil
}

// GetByComplex returns paginated clients for a complex with optional search.
// Uses raw SQL because the SQLC-generated params have incorrect types for
// the cursor-based (created_at, id) row comparison.
// (build filtered/searched/paginated query, execute, scan rows, build metadata); matches this
// codebase's store method conventions (wrap sqlc queries, map to domain structs).
//
//nolint:funlen // single cohesive SQL-building-and-execution flow for one store operation
func (m *ClientModel) GetByComplex(ctx context.Context, complexID uuid.UUID, search string, filters Filters) ([]*Client, Metadata, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	cursorTime, cursorID, err := filters.ParseCursor()
	if err != nil {
		return nil, Metadata{}, err
	}

	if cursorTime.IsZero() {
		cursorTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}

	// total_bookings is a live count, not the clients.total_bookings column:
	// see GetClientByID's comment for why the stored counter can't be trusted.
	var rows pgx.Rows
	if search != "" {
		// H-04: escaped so a literal '%' or '_' the owner typed matches only
		// itself — see EscapeLikeTerm's comment. ESCAPE '\' on all three
		// predicates is what makes the escaping below take effect.
		pattern := "%" + EscapeLikeTerm(search) + "%"
		rows, err = m.DB.Query(ctx, `
			SELECT c.id, c.complex_id, c.first_name, c.last_name, c.phone, c.email, c.notes,
			       c.is_blocked, c.no_shows, c.created_at, c.updated_at,
			       COALESCE((
			         SELECT COUNT(*)::int FROM bookings b
			         WHERE b.client_id = c.id AND b.status IN ('confirmed', 'completed', 'no_show')
			       ), 0)::int AS total_bookings
			FROM clients c
			WHERE c.complex_id = $1
			  AND (c.created_at, c.id) > ($2, $3)
			  AND (c.first_name ILIKE $5 ESCAPE '\' OR c.last_name ILIKE $5 ESCAPE '\' OR c.phone ILIKE $5 ESCAPE '\')
			ORDER BY c.created_at ASC, c.id ASC
			LIMIT $4`,
			UUIDToPg(complexID),
			TimeToPg(cursorTime),
			UUIDToPg(cursorID),
			int32(limit+1),
			pattern,
		)
	} else {
		rows, err = m.DB.Query(ctx, `
			SELECT c.id, c.complex_id, c.first_name, c.last_name, c.phone, c.email, c.notes,
			       c.is_blocked, c.no_shows, c.created_at, c.updated_at,
			       COALESCE((
			         SELECT COUNT(*)::int FROM bookings b
			         WHERE b.client_id = c.id AND b.status IN ('confirmed', 'completed', 'no_show')
			       ), 0)::int AS total_bookings
			FROM clients c
			WHERE c.complex_id = $1
			  AND (c.created_at, c.id) > ($2, $3)
			ORDER BY c.created_at ASC, c.id ASC
			LIMIT $4`,
			UUIDToPg(complexID),
			TimeToPg(cursorTime),
			UUIDToPg(cursorID),
			int32(limit+1),
		)
	}
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	clients := make([]*Client, 0, limit+1)
	for rows.Next() {
		var c db.Client
		err := rows.Scan(
			&c.ID, &c.ComplexID, &c.FirstName, &c.LastName, &c.Phone,
			&c.Email, &c.Notes, &c.IsBlocked, &c.NoShows,
			&c.CreatedAt, &c.UpdatedAt, &c.TotalBookings,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		clients = append(clients, clientFromDB(c))
	}
	if err := rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	clients, meta := TrimPage(clients, limit, BuildTimestampCursor)
	return clients, meta, nil
}

// Update persists changes to an existing client, returning ErrRecordNotFound if it no longer exists.
func (m *ClientModel) Update(ctx context.Context, c *Client) error {
	dbClient, err := m.Q.UpdateClient(ctx, db.UpdateClientParams{
		FirstName: c.FirstName,
		LastName:  c.LastName,
		Phone:     c.Phone,
		Email:     TextToPg(c.Email),
		Notes:     TextToPg(c.Notes),
		IsBlocked: c.IsBlocked,
		ID:        UUIDToPg(c.ID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRecordNotFound
		}
		return err
	}

	c.UpdatedAt = PgToTime(dbClient.UpdatedAt)
	return nil
}

// GetOrCreate returns the client matching phone within the complex, creating
// one when none exists.
//
// H-01: this used to be a check-then-insert — GetByPhone, and on a miss,
// Insert — with nothing serializing the two. Two concurrent first-time
// bookings for the same phone both miss the SELECT before either commits,
// both attempt the INSERT, and the loser violated
// clients_complex_id_phone_key with nothing catching it: an unhandled 500 on
// POST /api/v1/book, after the slot lock was already taken. The collision key
// is (complex_id, phone), so this did not need two different people — a
// client double-clicking "Reservar" hit it, on what for a product that had
// not launched yet was essentially every client's first booking.
//
// ON CONFLICT is what makes the code agree the constraint is reachable,
// rather than catching 23505 after the fact: the INSERT and the conflict
// resolution are one statement, so there is no gap between "check" and "act"
// left for a second request to land in. The unique constraint itself is
// correct and unchanged — see clients_complex_id_phone_key in
// db/migrations/001_init.sql.
//
// On a match, the owner's decision (2026-09-10) is that the latest name
// submitted wins, including over one mistyped on an earlier visit — a
// returning client identified by phone had no other way to correct it. The
// trade this accepts: two different people who happen to share one phone
// number will overwrite each other's name on every booking. Email is treated
// differently — it is backfilled only when the client does not already have
// one, exactly as before this fix — because nothing else in the product lets
// a client correct a mistyped email either, and overwriting a real address
// with a placeholder used on a later booking would be worse than leaving it
// alone.
//
// That decision holds only for the caller it was made for. GetOrCreate is
// reached from two places: the authenticated owner-booking path
// (internal/bookings/create.go, behind requireComplexOwner), and the public
// POST /api/v1/book endpoint (internal/bookings/public.go), which needs no
// account at all. Applying the name overwrite unconditionally handed anyone
// who knew a complex id and an existing client's phone number a write
// primitive over that client's stored record — the one the owner's dashboard
// renders — with nothing to stop it: submit a booking with that phone and any
// name, and the row is rewritten. allowNameUpdate is what tells the two
// callers apart: the owner path passes true and keeps the correction the
// owner asked for, the public path passes false and keeps only H-01's
// atomicity — the ON CONFLICT still resolves the double-insert race without a
// 500, it just never lets an unauthenticated request rename an existing
// client. A brand-new row is unaffected either way, since there is nothing
// yet to overwrite.
func (m *ClientModel) GetOrCreate(ctx context.Context, complexID uuid.UUID, firstName, lastName, phone, email string, allowNameUpdate bool) (*Client, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	var id pgtype.UUID
	err := m.DB.QueryRow(ctx, `
		INSERT INTO clients (complex_id, first_name, last_name, phone, email)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (complex_id, phone) DO UPDATE SET
			first_name = CASE WHEN $6 THEN EXCLUDED.first_name ELSE clients.first_name END,
			last_name = CASE WHEN $6 THEN EXCLUDED.last_name ELSE clients.last_name END,
			email = COALESCE(NULLIF(clients.email, ''), EXCLUDED.email)
		RETURNING id`,
		UUIDToPg(complexID), firstName, lastName, phone, TextToPg(emailPtr), allowNameUpdate,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	// Read back through GetByID rather than building the Client from the
	// RETURNING clause above, so this keeps computing total_bookings live —
	// see GetByID's comment for why the stored column cannot be trusted — the
	// same way the check-then-insert version did by way of GetByPhone.
	return m.GetByID(ctx, PgToUUID(id))
}

// IncrementNoShows increments the client's no-show counter by one.
func (m *ClientModel) IncrementNoShows(ctx context.Context, clientID uuid.UUID) error {
	return m.Q.IncrementNoShowCount(ctx, UUIDToPg(clientID))
}

// CountByComplex returns the total number of clients registered in the complex.
func (m *ClientModel) CountByComplex(ctx context.Context, complexID uuid.UUID) (int, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	var count int
	err := m.DB.QueryRow(ctx, `SELECT COUNT(*) FROM clients WHERE complex_id = $1`, UUIDToPg(complexID)).Scan(&count)
	return count, err
}

// ─── Dashboard: Client Insights ─────────────────────────────────────────

// TopClient summarizes one of a complex's highest-spending clients for the insights dashboard.
type TopClient struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Phone        string    `json:"phone"`
	BookingCount int       `json:"booking_count"`
	TotalSpent   int       `json:"total_spent"`
}

// ClientInsights aggregates client engagement metrics for a complex's dashboard.
type ClientInsights struct {
	TopClients     []TopClient `json:"top"`
	NoShowRate     int         `json:"no_show_rate"`
	NoShowCount    int         `json:"no_show_count"`
	CompletedCount int         `json:"resolved_count"`
	NewClients     int         `json:"new_clients_30d"`
	Recurring      int         `json:"recurring_30d"`
	TotalActive    int         `json:"total_active_30d"`
}

// GetInsights computes top clients, no-show rate and new/recurring client counts for the trailing 30 days.
//
//nolint:funlen // single cohesive sequence of aggregate queries feeding one insights struct
func (m *ClientModel) GetInsights(ctx context.Context, complexID uuid.UUID, today time.Time) (*ClientInsights, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	insights := &ClientInsights{}
	from := today.AddDate(0, 0, -29)

	rows, err := m.DB.Query(ctx, `
		SELECT cl.id, cl.first_name || ' ' || cl.last_name, cl.phone,
		       COUNT(*)::int, COALESCE(SUM(b.price), 0)::bigint
		FROM bookings b
		JOIN clients cl ON cl.id = b.client_id
		WHERE b.complex_id = $1 AND b.date BETWEEN $2 AND $3 AND b.status != 'cancelled'
		GROUP BY cl.id, cl.first_name, cl.last_name, cl.phone
		ORDER BY COUNT(*) DESC
		LIMIT 10`,
		UUIDToPg(complexID), DateToPg(from), DateToPg(today),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tc TopClient
		var id pgtype.UUID
		if err := rows.Scan(&id, &tc.Name, &tc.Phone, &tc.BookingCount, &tc.TotalSpent); err != nil {
			return nil, err
		}
		tc.ID = PgToUUID(id)
		insights.TopClients = append(insights.TopClients, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if insights.TopClients == nil {
		insights.TopClients = []TopClient{}
	}

	err = m.DB.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'no_show')::int,
			COUNT(*) FILTER (WHERE status IN ('completed', 'no_show'))::int
		FROM bookings
		WHERE complex_id = $1 AND date BETWEEN $2 AND $3`,
		UUIDToPg(complexID), DateToPg(from), DateToPg(today),
	).Scan(&insights.NoShowCount, &insights.CompletedCount)
	if err != nil {
		return nil, err
	}
	if insights.CompletedCount > 0 {
		insights.NoShowRate = (insights.NoShowCount * 100) / insights.CompletedCount
	}

	err = m.DB.QueryRow(ctx, `
		SELECT
			COUNT(DISTINCT b.client_id) FILTER (WHERE cl.created_at >= $2)::int,
			COUNT(DISTINCT b.client_id)::int
		FROM bookings b
		JOIN clients cl ON cl.id = b.client_id
		WHERE b.complex_id = $1 AND b.date BETWEEN $3 AND $4 AND b.status != 'cancelled'`,
		UUIDToPg(complexID), TimeToPg(from), DateToPg(from), DateToPg(today),
	).Scan(&insights.NewClients, &insights.TotalActive)
	if err != nil {
		return nil, err
	}
	insights.Recurring = insights.TotalActive - insights.NewClients

	return insights, nil
}

func clientFromDB(c db.Client) *Client {
	return &Client{
		ID:            PgToUUID(c.ID),
		ComplexID:     PgToUUID(c.ComplexID),
		FirstName:     c.FirstName,
		LastName:      c.LastName,
		Phone:         c.Phone,
		Email:         PgToTextPtr(c.Email),
		Notes:         PgToTextPtr(c.Notes),
		IsBlocked:     c.IsBlocked,
		TotalBookings: int(c.TotalBookings),
		NoShows:       int(c.NoShows),
		CreatedAt:     PgToTime(c.CreatedAt),
		UpdatedAt:     PgToTime(c.UpdatedAt),
	}
}

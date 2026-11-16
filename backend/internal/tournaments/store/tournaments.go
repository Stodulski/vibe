package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// MaxList bounds one page of a tournaments list.
const MaxList = 200

// Store is the sqlc-backed tournaments store.
type Store struct {
	DB *data.DB
	Q  *db.Queries
	// PaymentExpiry is the window an unpaid storefront booking holds its slot,
	// which the court-free check honours when scheduling.
	PaymentExpiry time.Duration
}

// NewCategory is a category to create with its tournament. Name is composed by
// the service from the traits below.
type NewCategory struct {
	Name     string
	Modality string
	// Level (1 to 9) or Suma (2 to 18) is set, the other zero. Prizes is nil for
	// none. StartsOn and StartsAt are when the category begins.
	Level              int
	Suma               int
	Prizes             *string
	StartsOn           time.Time
	StartsAt           string
	Format             string
	Capacity           int
	Price              int
	GroupSize          int
	QualifiersPerGroup int
	// SetsToWin (1 or 2), GamesPerSet (4..9), SuperTiebreakDecider and
	// MatchDurationMinutes (30..480) are the match format; the service fills the
	// defaults, the store writes what it is given.
	SetsToWin            int
	GamesPerSet          int
	SuperTiebreakDecider bool
	MatchDurationMinutes int
	// CourtFee is centavos each pair pays per match; 0 is none.
	CourtFee int
}

// Create writes a tournament and its categories, each with its sign-up
// container, in one transaction.
func (m *Store) Create(ctx context.Context, t *Tournament, cats []NewCategory) error {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	if err := data.AssertTenant(ctx, t.ComplexID); err != nil {
		return err
	}
	var created db.Tournament
	err := m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		var err error
		created, err = q.InsertTournament(ctx, db.InsertTournamentParams{
			ComplexID: data.UUIDToPg(t.ComplexID), Name: t.Name, Sport: db.SportType(t.Sport),
			StartDate: data.DateToPg(t.StartDate), EndDate: data.DateToPg(t.EndDate), CreatedBy: data.UUIDToPg(t.CreatedBy),
		})
		if err != nil {
			return fmt.Errorf("tournaments: insert tournament: %w", err)
		}
		for _, c := range cats {
			if err := insertCategory(ctx, q, created, c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	*t = *tournamentFromDB(created)
	return nil
}

// insertCategory writes a category and its sign-up container. The container's
// name is "tournament · category", which is what notices and the checkout say;
// it starts on the tournament's first day and holds no courts.
func insertCategory(ctx context.Context, q *db.Queries, t db.Tournament, c NewCategory) error {
	ev, err := q.InsertOpenPlayEvent(ctx, db.InsertOpenPlayEventParams{
		ComplexID: t.ComplexID, Name: t.Name + " · " + c.Name, Sport: t.Sport, Date: t.StartDate,
		StartTime: data.TimeStrToPg("00:00"), DurationMinutes: 30,
		//nolint:gosec // G115: validated by the service (2..128).
		Capacity: int32(c.Capacity),
		//nolint:gosec // G115: a price in centavos validated by the service.
		Price: int32(c.Price), CreatedBy: t.CreatedBy, TournamentID: t.ID,
	})
	if err != nil {
		return fmt.Errorf("tournaments: insert category event: %w", err)
	}
	if _, err := q.InsertTournamentCategory(ctx, db.InsertTournamentCategoryParams{
		TournamentID: t.ID, ComplexID: t.ComplexID, EventID: ev.ID, Name: c.Name, Format: c.Format,
		Modality: data.TextToPg(nullableText(c.Modality)), Level: data.Int4PtrToPg(optionalInt(c.Level)),
		Suma: data.Int4PtrToPg(optionalInt(c.Suma)), Prizes: data.TextToPg(c.Prizes),
		StartsOn: data.DateToPg(c.StartsOn), StartsAt: data.TimeStrToPg(c.StartsAt),
		//nolint:gosec // G115: validated by the service (2..12).
		GroupSize: int32(c.GroupSize),
		//nolint:gosec // G115: validated by the service (1..11).
		QualifiersPerGroup: int32(c.QualifiersPerGroup),
		//nolint:gosec // G115: validated by the service (1..2, 4..9).
		SetsToWin: int32(c.SetsToWin), GamesPerSet: int32(c.GamesPerSet),
		SuperTiebreakDecider: c.SuperTiebreakDecider,
		//nolint:gosec // G115: validated by the service (30..480).
		MatchDurationMinutes: int32(c.MatchDurationMinutes),
		//nolint:gosec // G115: validated by the service (0..MaxPrice).
		CourtFee: int32(c.CourtFee),
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == data.SQLStateUniqueViolation && pgErr.ConstraintName == "tournament_categories_name_key" {
			return ErrCategoryTaken
		}
		return fmt.Errorf("tournaments: insert category: %w", err)
	}
	return nil
}

// nullableText is nil for an empty trait: a category without a modality stores
// NULL, never an empty string that would fail the CHECK.
func nullableText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalInt is nil for a trait the category does not have: the zero value is
// stored as NULL, never as 0.
func optionalInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

// Get returns one tournament, or ErrRecordNotFound. A scoped caller only ever
// sees its own tenant's.
func (m *Store) Get(ctx context.Context, id uuid.UUID) (*Tournament, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	row, err := m.Q.GetTournament(ctx, db.GetTournamentParams{ID: data.UUIDToPg(id), ComplexID: data.TenantParam(ctx)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, fmt.Errorf("tournaments: get: %w", err)
	}
	return tournamentFromDB(row), nil
}

// List returns the tournaments of a complex that overlap [from, to], soonest
// first.
func (m *Store) List(ctx context.Context, complexID uuid.UUID, from, to time.Time, includeCancelled bool) ([]*Tournament, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	rows, err := m.Q.ListTournaments(ctx, db.ListTournamentsParams{
		ComplexID: data.UUIDToPg(complexID), DateFrom: data.DateToPg(from), DateTo: data.DateToPg(to),
		IncludeCancelled: includeCancelled, PageLimit: MaxList,
	})
	if err != nil {
		return nil, fmt.Errorf("tournaments: list: %w", err)
	}
	out := make([]*Tournament, len(rows))
	for i, r := range rows {
		out[i] = tournamentFromDB(r)
	}
	return out, nil
}

// Categories returns the categories of a tournament with their counts.
func (m *Store) Categories(ctx context.Context, tournamentID uuid.UUID) ([]*Category, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	return categories(ctx, m.Q, tournamentID)
}

func categories(ctx context.Context, q *db.Queries, tournamentID uuid.UUID) ([]*Category, error) {
	rows, err := q.ListTournamentCategories(ctx, data.UUIDToPg(tournamentID))
	if err != nil {
		return nil, fmt.Errorf("tournaments: list categories: %w", err)
	}
	out := make([]*Category, len(rows))
	for i, r := range rows {
		out[i] = categoryFromRow(r)
	}
	return out, nil
}

// Matches returns every match of a tournament, in the order the draw made them.
func (m *Store) Matches(ctx context.Context, tournamentID uuid.UUID) ([]*Match, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()
	rows, err := m.Q.ListTournamentMatches(ctx, data.UUIDToPg(tournamentID))
	if err != nil {
		return nil, fmt.Errorf("tournaments: list matches: %w", err)
	}
	out := make([]*Match, len(rows))
	for i, r := range rows {
		out[i] = matchFromDB(r)
	}
	return out, nil
}

// Category returns one category of the tournament by id, or ErrRecordNotFound.
func (m *Store) Category(ctx context.Context, tournamentID, categoryID uuid.UUID) (*Category, error) {
	cats, err := m.Categories(ctx, tournamentID)
	if err != nil {
		return nil, err
	}
	for _, c := range cats {
		if c.ID == categoryID {
			return c, nil
		}
	}
	return nil, data.ErrRecordNotFound
}

// Detail returns a tournament with its categories, pairs, groups and matches.
func (m *Store) Detail(ctx context.Context, id uuid.UUID) (*Detail, error) {
	t, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	d := &Detail{Tournament: *t}
	if d.Categories, err = categories(ctx, m.Q, id); err != nil {
		return nil, err
	}
	pairs, err := m.Q.ListTournamentPairs(ctx, data.UUIDToPg(id))
	if err != nil {
		return nil, fmt.Errorf("tournaments: list pairs: %w", err)
	}
	for _, p := range pairs {
		d.Pairs = append(d.Pairs, pairRowFromDB(p))
	}
	groups, err := m.Q.ListTournamentGroupsByTournament(ctx, data.UUIDToPg(id))
	if err != nil {
		return nil, fmt.Errorf("tournaments: list groups: %w", err)
	}
	for _, g := range groups {
		d.Groups = append(d.Groups, groupFromDB(g))
	}
	matches, err := m.Q.ListTournamentMatches(ctx, data.UUIDToPg(id))
	if err != nil {
		return nil, fmt.Errorf("tournaments: list matches: %w", err)
	}
	for _, mt := range matches {
		d.Matches = append(d.Matches, matchFromDB(mt))
	}
	fees, err := m.Q.ListTournamentMatchFees(ctx, data.UUIDToPg(id))
	if err != nil {
		return nil, fmt.Errorf("tournaments: list court fees: %w", err)
	}
	for _, f := range fees {
		d.Fees = append(d.Fees, matchFeeFromDB(f))
	}
	return d, nil
}

// Cancel marks an active tournament cancelled, which releases every court its
// matches hold. The categories' sign-ups are cancelled and refunded by the
// caller, through the events domain. A second cancel answers ErrNotActive.
func (m *Store) Cancel(ctx context.Context, id, by uuid.UUID, reason *string) (*Tournament, error) {
	ctx, cancel := data.TxContext(ctx)
	defer cancel()

	var out db.Tournament
	err := m.DB.WithTx(ctx, func(_ pgx.Tx, q *db.Queries) error {
		locked, err := q.LockTournament(ctx, data.UUIDToPg(id))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return data.ErrRecordNotFound
			}
			return fmt.Errorf("tournaments: lock: %w", err)
		}
		if err := data.AssertTenant(ctx, data.PgToUUID(locked.ComplexID)); err != nil {
			return err
		}
		if out, err = q.CancelTournament(ctx, db.CancelTournamentParams{
			ID: locked.ID, CancelledBy: data.UUIDToPg(by), CancelReason: data.TextToPg(reason),
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotActive
			}
			return fmt.Errorf("tournaments: cancel: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tournamentFromDB(out), nil
}

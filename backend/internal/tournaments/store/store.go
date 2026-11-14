// Package store persists tournaments: the tournament and its categories, the
// pairs that signed up, the groups and matches a draw makes, and the schedule
// and results of those matches.
//
// A category's sign-up container is an open-play event row, and a pair's sign-up
// an open-play registration (see migration 029); this package owns what a pair
// adds and every rule that changes a draw, a result or a schedule, each under
// the category's row lock so two writers never interleave on one bracket.
package store

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
)

// Statuses and formats.
const (
	StatusActive    = "active"
	StatusCancelled = "cancelled"

	FormatElimination = "elimination"
	FormatGroups      = "groups"

	StageGroup   = "group"
	StageBracket = "bracket"

	MatchPending   = "pending"
	MatchCompleted = "completed"

	PairActive    = "active"
	PairWithdrawn = "withdrawn"
)

// Sentinel errors returned by this store.
var (
	// ErrNotEnoughPairs is returned by a draw that has too few paid pairs.
	ErrNotEnoughPairs = errors.New("not enough pairs to draw")
	// ErrHasResults is returned when a draw is changed after a result exists.
	ErrHasResults = errors.New("a result was already loaded")
	// ErrNoDraw is returned by an operation that needs the draw to exist.
	ErrNoDraw = errors.New("the draw was not generated yet")
	// ErrCancelled is returned for a tournament or category that was cancelled.
	ErrCancelled = errors.New("the tournament or category was cancelled")
	// ErrUnknownPair is returned for a pair id that is not a drawable pair of the category.
	ErrUnknownPair = errors.New("unknown pair")
	// ErrInvalidMove is returned for a move the draw does not allow.
	ErrInvalidMove = errors.New("invalid move")
	// ErrMatchNotReady is returned when a result is loaded for a match that does
	// not have both pairs yet, or never plays (a bye).
	ErrMatchNotReady = errors.New("the match does not have both pairs yet")
	// ErrCorrectionLocked is returned when a result cannot change because the
	// next match already has a result.
	ErrCorrectionLocked = errors.New("the next match already has a result")
	// ErrOutOfDates is returned for a match scheduled outside the tournament's days.
	ErrOutOfDates = errors.New("outside the tournament's dates")
	// ErrNotPair is returned when a registration does not belong to a tournament pair.
	ErrNotPair = errors.New("the registration is not a tournament pair")
	// ErrNotActive is returned when cancelling something already cancelled.
	ErrNotActive = errors.New("already cancelled")
)

// Tournament is a tournament of a complex.
type Tournament struct {
	ID           uuid.UUID
	ComplexID    uuid.UUID
	Name         string
	Sport        string
	StartDate    time.Time
	EndDate      time.Time
	Status       string
	CancelReason *string
	CancelledAt  *time.Time
	CreatedBy    uuid.UUID
	CreatedAt    time.Time
}

// Category is one category of a tournament with what its sign-up container says.
type Category struct {
	ID                 uuid.UUID
	TournamentID       uuid.UUID
	ComplexID          uuid.UUID
	EventID            uuid.UUID
	Name               string
	Format             string
	GroupSize          int
	QualifiersPerGroup int
	// SetsToWin, GamesPerSet and SuperTiebreakDecider are how the category's
	// matches are played; MatchDurationMinutes is the court time a match gets
	// when it is scheduled without one.
	SetsToWin            int
	GamesPerSet          int
	SuperTiebreakDecider bool
	MatchDurationMinutes int
	DrawGeneratedAt      *time.Time
	// Capacity is pairs; Price is centavos per pair.
	Capacity       int
	Price          int
	Cancelled      bool
	ConfirmedCount int
	HeldCount      int
}

// MatchFormat is the format its results are validated and counted by.
func (c *Category) MatchFormat() draw.Format {
	return draw.Format{SetsToWin: c.SetsToWin, GamesPerSet: c.GamesPerSet, SuperTiebreak: c.SuperTiebreakDecider && c.SetsToWin == 2}
}

// categoryFormat is the same format read from a category row.
func categoryFormat(c db.TournamentCategory) draw.Format {
	return draw.Format{SetsToWin: int(c.SetsToWin), GamesPerSet: int(c.GamesPerSet), SuperTiebreak: c.SuperTiebreakDecider && c.SetsToWin == 2}
}

// SpotsLeft is the pairs that can still sign up.
func (c *Category) SpotsLeft() int { return max(0, c.Capacity-c.ConfirmedCount-c.HeldCount) }

// Pair is two players signed up together. The payer is the registration's
// person.
type Pair struct {
	ID             uuid.UUID
	CategoryID     uuid.UUID
	RegistrationID uuid.UUID
	Seed           *int
	GroupID        *uuid.UUID
	Status         string
	Partner        Player
}

// Player is a person in a pair.
type Player struct {
	FirstName string
	LastName  string
	Phone     string
	Email     *string
}

// PairRow is a pair with its payer, the state of its spot and its payment, as the
// owner's pair list reads it.
type PairRow struct {
	Pair
	Payer              Player
	RegistrationStatus string
	Price              int
	ServiceFee         int
	HeldUntil          time.Time
	ConfirmedAt        *time.Time
	CancelledBy        *string
	SignedUpAt         time.Time
	Payment            *Payment
}

// Payment is the money of a pair's registration.
type Payment struct {
	ID           uuid.UUID
	Status       string
	Amount       int
	ServiceFee   int
	RefundAmount int
}

// Group is a group of a groups-format draw.
type Group struct {
	ID         uuid.UUID
	CategoryID uuid.UUID
	Name       string
	Position   int
}

// Slot says where a bracket slot's pair comes from when it is not known yet:
// the Rank-th of a group.
type Slot struct {
	GroupID uuid.UUID
	Rank    int
}

// Match is a match of a group or of the bracket.
type Match struct {
	ID         uuid.UUID
	CategoryID uuid.UUID
	Stage      string
	GroupID    *uuid.UUID
	Round      int
	Position   int
	Pair1ID    *uuid.UUID
	Pair2ID    *uuid.UUID
	Source1    *Slot
	Source2    *Slot
	Bye        bool
	NextMatch  *uuid.UUID
	NextSlot   int
	Status     string
	WinnerID   *uuid.UUID
	Sets       []draw.Set
	// WalkoverLoser is 1 or 2 for a walkover, 0 otherwise.
	WalkoverLoser int
	CompletedAt   *time.Time
	CourtID       *uuid.UUID
	ScheduledOn   *time.Time
	StartTime     *string
	Duration      *int
}

// Scheduled reports whether the match has a court and a time.
func (m *Match) Scheduled() bool { return m.CourtID != nil }

// StartsAt is the instant a scheduled match starts.
func (m *Match) StartsAt() time.Time {
	t, _ := time.Parse("15:04", *m.StartTime) //nolint:errcheck // written by the store from a validated HH:MM
	d := *m.ScheduledOn
	return time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), 0, 0, timezone.Argentina)
}

// Detail is a tournament with everything under it.
type Detail struct {
	Tournament Tournament
	Categories []*Category
	Pairs      []*PairRow
	Groups     []*Group
	Matches    []*Match
}

// Outcome is the figures of a played match of the given format, from its stored
// sets.
func (m *Match) Outcome(f draw.Format) (draw.Outcome, bool) {
	switch {
	case m.Status != MatchCompleted || m.Bye:
		return draw.Outcome{}, false
	case m.WalkoverLoser != 0:
		return f.Walkover(m.WalkoverLoser), true
	}
	out, err := f.ParseResult(m.Sets)
	return out, err == nil
}

func tournamentFromDB(t db.Tournament) *Tournament {
	return &Tournament{
		ID: data.PgToUUID(t.ID), ComplexID: data.PgToUUID(t.ComplexID), Name: t.Name, Sport: string(t.Sport),
		StartDate: data.PgToDate(t.StartDate), EndDate: data.PgToDate(t.EndDate), Status: t.Status,
		CancelReason: data.PgToTextPtr(t.CancelReason), CancelledAt: data.PgToTimePtr(t.CancelledAt),
		CreatedBy: data.PgToUUID(t.CreatedBy), CreatedAt: data.PgToTime(t.CreatedAt),
	}
}

func categoryFromRow(r db.ListTournamentCategoriesRow) *Category {
	return &Category{
		ID: data.PgToUUID(r.ID), TournamentID: data.PgToUUID(r.TournamentID), ComplexID: data.PgToUUID(r.ComplexID),
		EventID: data.PgToUUID(r.EventID), Name: r.Name, Format: r.Format,
		GroupSize: int(r.GroupSize), QualifiersPerGroup: int(r.QualifiersPerGroup),
		SetsToWin: int(r.SetsToWin), GamesPerSet: int(r.GamesPerSet), SuperTiebreakDecider: r.SuperTiebreakDecider,
		MatchDurationMinutes: int(r.MatchDurationMinutes),
		DrawGeneratedAt:      data.PgToTimePtr(r.DrawGeneratedAt), Capacity: int(r.Capacity), Price: int(r.Price),
		Cancelled: r.EventStatus != "active", ConfirmedCount: int(r.ConfirmedCount), HeldCount: int(r.HeldCount),
	}
}

func pairFromDB(p db.TournamentPair) Pair {
	return Pair{
		ID: data.PgToUUID(p.ID), CategoryID: data.PgToUUID(p.CategoryID), RegistrationID: data.PgToUUID(p.RegistrationID),
		Seed: data.PgToInt4Ptr(p.Seed), GroupID: data.PgToUUIDPtr(p.GroupID), Status: p.Status,
		Partner: Player{
			FirstName: p.PartnerFirstName, LastName: p.PartnerLastName, Phone: p.PartnerPhone,
			Email: data.PgToTextPtr(p.PartnerEmail),
		},
	}
}

func pairRowFromDB(r db.ListTournamentPairsRow) *PairRow {
	row := &PairRow{
		Pair: pairFromDB(db.TournamentPair{
			ID: r.ID, CategoryID: r.CategoryID, ComplexID: r.ComplexID, RegistrationID: r.RegistrationID,
			PartnerFirstName: r.PartnerFirstName, PartnerLastName: r.PartnerLastName, PartnerPhone: r.PartnerPhone,
			PartnerEmail: r.PartnerEmail, Seed: r.Seed, GroupID: r.GroupID, Status: r.Status,
		}),
		Payer:              Player{FirstName: r.FirstName, LastName: r.LastName, Phone: r.Phone, Email: data.PgToTextPtr(r.Email)},
		RegistrationStatus: r.RegistrationStatus, Price: int(r.RegistrationPrice), ServiceFee: int(r.RegistrationServiceFee),
		HeldUntil: data.PgToTime(r.HeldUntil), ConfirmedAt: data.PgToTimePtr(r.ConfirmedAt),
		CancelledBy: data.PgToTextPtr(r.CancelledBy), SignedUpAt: data.PgToTime(r.SignedUpAt),
	}
	if r.PaymentID.Valid {
		row.Payment = &Payment{
			ID: data.PgToUUID(r.PaymentID), Status: r.PaymentStatus, Amount: int(r.PaymentAmount),
			ServiceFee: int(r.PaymentServiceFee), RefundAmount: int(r.PaymentRefundAmount),
		}
	}
	return row
}

func groupFromDB(g db.TournamentGroup) *Group {
	return &Group{ID: data.PgToUUID(g.ID), CategoryID: data.PgToUUID(g.CategoryID), Name: g.Name, Position: int(g.Position)}
}

type wireSet struct {
	A     int  `json:"a"`
	B     int  `json:"b"`
	Super bool `json:"super,omitempty"`
}

func encodeSets(sets []draw.Set) pgtype.Text {
	if sets == nil {
		return pgtype.Text{}
	}
	w := make([]wireSet, len(sets))
	for i, s := range sets {
		w[i] = wireSet{A: s.A, B: s.B, Super: s.Super}
	}
	b, _ := json.Marshal(w) //nolint:errcheck // plain ints and bools always marshal
	return pgtype.Text{String: string(b), Valid: true}
}

func decodeSets(raw []byte) []draw.Set {
	if len(raw) == 0 {
		return nil
	}
	var w []wireSet
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil
	}
	out := make([]draw.Set, len(w))
	for i, s := range w {
		out[i] = draw.Set{A: s.A, B: s.B, Super: s.Super}
	}
	return out
}

func matchFromDB(m db.TournamentMatch) *Match {
	out := &Match{
		ID: data.PgToUUID(m.ID), CategoryID: data.PgToUUID(m.CategoryID), Stage: m.Stage,
		GroupID: data.PgToUUIDPtr(m.GroupID), Round: int(m.Round), Position: int(m.Position),
		Pair1ID: data.PgToUUIDPtr(m.Pair1ID), Pair2ID: data.PgToUUIDPtr(m.Pair2ID), Bye: m.Bye,
		NextMatch: data.PgToUUIDPtr(m.NextMatchID), NextSlot: data.PgToInt(m.NextSlot), Status: m.Status,
		WinnerID: data.PgToUUIDPtr(m.WinnerPairID), Sets: decodeSets(m.Sets), WalkoverLoser: data.PgToInt(m.WalkoverLoser),
		CompletedAt: data.PgToTimePtr(m.CompletedAt), CourtID: data.PgToUUIDPtr(m.CourtID),
		Duration: data.PgToInt4Ptr(m.DurationMinutes),
	}
	if m.Slot1GroupID.Valid {
		out.Source1 = &Slot{GroupID: data.PgToUUID(m.Slot1GroupID), Rank: data.PgToInt(m.Slot1Rank)}
	}
	if m.Slot2GroupID.Valid {
		out.Source2 = &Slot{GroupID: data.PgToUUID(m.Slot2GroupID), Rank: data.PgToInt(m.Slot2Rank)}
	}
	if m.ScheduledOn.Valid {
		d := data.PgToDate(m.ScheduledOn)
		out.ScheduledOn = &d
		st := data.PgToTimeStr(m.StartTime)
		out.StartTime = &st
	}
	return out
}

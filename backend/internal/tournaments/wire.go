package tournaments

import (
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// The wire types are written by hand rather than generated so the owner's and the
// visitor's views can share their parts; internal/openapi/openapi.yaml is their
// document, and wire_test.go keeps the two from drifting apart.

type wireTournament struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Sport        string     `json:"sport"`
	StartDate    string     `json:"start_date"`
	EndDate      string     `json:"end_date"`
	Status       string     `json:"status"`
	CancelReason *string    `json:"cancel_reason"`
	CancelledAt  *time.Time `json:"cancelled_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type wireCategory struct {
	ID                   uuid.UUID `json:"id"`
	TournamentID         uuid.UUID `json:"tournament_id"`
	Name                 string    `json:"name"`
	Modality             *string   `json:"modality"`
	Level                *int      `json:"level"`
	Suma                 *int      `json:"suma"`
	Prizes               *string   `json:"prizes"`
	StartsOn             *string   `json:"starts_on"`
	StartsAt             *string   `json:"starts_at"`
	Format               string    `json:"format"`
	Capacity             int       `json:"capacity"`
	Price                int       `json:"price"`
	ServiceFee           *int      `json:"service_fee,omitempty"`
	TotalPrice           *int      `json:"total_price,omitempty"`
	SpotsLeft            int       `json:"spots_left"`
	ConfirmedCount       int       `json:"confirmed_count"`
	HeldCount            int       `json:"held_count"`
	GroupSize            int       `json:"group_size"`
	QualifiersPerGroup   int       `json:"qualifiers_per_group"`
	SetsToWin            int       `json:"sets_to_win"`
	GamesPerSet          int       `json:"games_per_set"`
	SuperTiebreakDecider bool      `json:"super_tiebreak_decider"`
	MatchDurationMinutes int       `json:"match_duration_minutes"`
	Status               string    `json:"status"`
	DrawGenerated        bool      `json:"draw_generated"`
}

type wirePlayer struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type wireOwnerPlayer struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Phone     string  `json:"phone"`
	Email     *string `json:"email"`
	// Gender and Level are what the player declared at sign-up, null when none was given.
	Gender *string `json:"gender"`
	Level  *int    `json:"level"`
}

type wirePair struct {
	ID         uuid.UUID    `json:"id"`
	CategoryID uuid.UUID    `json:"category_id"`
	Players    []wirePlayer `json:"players"`
	Seed       *int         `json:"seed"`
	GroupID    *uuid.UUID   `json:"group_id"`
	Status     string       `json:"status"`
}

type wireRegistration struct {
	// ID is what the owner names the pair by to withdraw it.
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	Price       int        `json:"price"`
	ServiceFee  int        `json:"service_fee"`
	HeldUntil   time.Time  `json:"held_until"`
	ConfirmedAt *time.Time `json:"confirmed_at"`
	CancelledBy *string    `json:"cancelled_by"`
	SignedUpAt  time.Time  `json:"signed_up_at"`
}

type wirePayment struct {
	Status       string `json:"status"`
	Amount       int    `json:"amount"`
	ServiceFee   int    `json:"service_fee"`
	RefundAmount int    `json:"refund_amount"`
}

type wireOwnerPair struct {
	wirePair
	Payer        wireOwnerPlayer  `json:"payer"`
	Partner      wireOwnerPlayer  `json:"partner"`
	Registration wireRegistration `json:"registration"`
	Payment      *wirePayment     `json:"payment"`
}

type wireStanding struct {
	PairID       uuid.UUID `json:"pair_id"`
	Rank         int       `json:"rank"`
	Played       int       `json:"played"`
	Won          int       `json:"won"`
	Lost         int       `json:"lost"`
	SetsFor      int       `json:"sets_for"`
	SetsAgainst  int       `json:"sets_against"`
	GamesFor     int       `json:"games_for"`
	GamesAgainst int       `json:"games_against"`
}

type wireGroup struct {
	ID         uuid.UUID      `json:"id"`
	CategoryID uuid.UUID      `json:"category_id"`
	Name       string         `json:"name"`
	Position   int            `json:"position"`
	Standings  []wireStanding `json:"standings"`
}

type wireSet struct {
	A     int  `json:"a"`
	B     int  `json:"b"`
	Super bool `json:"super"`
}

type wireSource struct {
	GroupID uuid.UUID `json:"group_id"`
	Rank    int       `json:"rank"`
}

type wireSchedule struct {
	CourtID         uuid.UUID `json:"court_id"`
	CourtName       string    `json:"court_name"`
	Date            string    `json:"date"`
	StartTime       string    `json:"start_time"`
	EndTime         string    `json:"end_time"`
	DurationMinutes int       `json:"duration_minutes"`
}

type wireMatch struct {
	ID            uuid.UUID     `json:"id"`
	CategoryID    uuid.UUID     `json:"category_id"`
	Stage         string        `json:"stage"`
	GroupID       *uuid.UUID    `json:"group_id"`
	Round         int           `json:"round"`
	Position      int           `json:"position"`
	Pair1ID       *uuid.UUID    `json:"pair1_id"`
	Pair2ID       *uuid.UUID    `json:"pair2_id"`
	Source1       *wireSource   `json:"source1"`
	Source2       *wireSource   `json:"source2"`
	Bye           bool          `json:"bye"`
	Status        string        `json:"status"`
	WinnerPairID  *uuid.UUID    `json:"winner_pair_id"`
	Sets          []wireSet     `json:"sets"`
	WalkoverLoser int           `json:"walkover_loser"`
	Schedule      *wireSchedule `json:"schedule"`
}

// Detail is a tournament as the owner or a visitor reads it.
type wireDetail struct {
	Tournament wireTournament `json:"tournament"`
	Categories []wireCategory `json:"categories"`
	Pairs      any            `json:"pairs"`
	Groups     []wireGroup    `json:"groups"`
	Matches    []wireMatch    `json:"matches"`
}

type wireSummary struct {
	wireTournament
	Categories []wireCategory `json:"categories"`
}

func toWireTournament(t *tournamentstore.Tournament) wireTournament {
	return wireTournament{
		ID: t.ID, Name: t.Name, Sport: t.Sport, StartDate: t.StartDate.Format(time.DateOnly), EndDate: t.EndDate.Format(time.DateOnly),
		Status: t.Status, CancelReason: t.CancelReason, CancelledAt: t.CancelledAt, CreatedAt: t.CreatedAt,
	}
}

// toWireCategory maps a category; serviceFee, when given, is the flat fee a
// visitor pays on top of the entry.
func toWireCategory(c *tournamentstore.Category, serviceFee *int) wireCategory {
	status := "open"
	switch {
	case c.Cancelled:
		status = "cancelled"
	case c.DrawGeneratedAt != nil, c.SignUpClosedByDate(time.Now()):
		status = "closed"
	}
	out := wireCategory{
		ID: c.ID, TournamentID: c.TournamentID, Name: c.Name, Format: c.Format, Capacity: c.Capacity, Price: c.Price,
		Modality: c.Modality, Level: c.Level, Suma: c.Suma, Prizes: c.Prizes, StartsAt: c.StartsAt,
		SpotsLeft: c.SpotsLeft(), ConfirmedCount: c.ConfirmedCount, HeldCount: c.HeldCount,
		GroupSize: c.GroupSize, QualifiersPerGroup: c.QualifiersPerGroup, Status: status,
		SetsToWin: c.SetsToWin, GamesPerSet: c.GamesPerSet, SuperTiebreakDecider: c.SuperTiebreakDecider,
		MatchDurationMinutes: c.MatchDurationMinutes, DrawGenerated: c.DrawGeneratedAt != nil,
	}
	if c.StartsOn != nil {
		day := c.StartsOn.Format(time.DateOnly)
		out.StartsOn = &day
	}
	if serviceFee != nil {
		total := c.Price + *serviceFee
		out.ServiceFee, out.TotalPrice = serviceFee, &total
	}
	return out
}

func toWireCategories(cats []*tournamentstore.Category, serviceFee func(price int) *int) []wireCategory {
	out := make([]wireCategory, len(cats))
	for i, c := range cats {
		var fee *int
		if serviceFee != nil {
			fee = serviceFee(c.Price)
		}
		out[i] = toWireCategory(c, fee)
	}
	return out
}

func toWirePair(p *tournamentstore.PairRow) wirePair {
	return wirePair{
		ID: p.ID, CategoryID: p.CategoryID, Seed: p.Seed, GroupID: p.GroupID, Status: p.Status,
		Players: []wirePlayer{
			{FirstName: p.Payer.FirstName, LastName: p.Payer.LastName},
			{FirstName: p.Partner.FirstName, LastName: p.Partner.LastName},
		},
	}
}

func toWireOwnerPair(p *tournamentstore.PairRow) wireOwnerPair {
	out := wireOwnerPair{
		wirePair: toWirePair(p),
		Payer:    wireOwnerPlayer{FirstName: p.Payer.FirstName, LastName: p.Payer.LastName, Phone: p.Payer.Phone, Email: p.Payer.Email, Gender: p.Payer.Gender, Level: p.Payer.Level},
		Partner:  wireOwnerPlayer{FirstName: p.Partner.FirstName, LastName: p.Partner.LastName, Phone: p.Partner.Phone, Email: p.Partner.Email, Gender: p.Partner.Gender, Level: p.Partner.Level},
		Registration: wireRegistration{
			ID: p.RegistrationID, Status: p.RegistrationStatus, Price: p.Price, ServiceFee: p.ServiceFee, HeldUntil: p.HeldUntil,
			ConfirmedAt: p.ConfirmedAt, CancelledBy: p.CancelledBy, SignedUpAt: p.SignedUpAt,
		},
	}
	if p.Payment != nil {
		out.Payment = &wirePayment{Status: p.Payment.Status, Amount: p.Payment.Amount, ServiceFee: p.Payment.ServiceFee, RefundAmount: p.Payment.RefundAmount}
	}
	return out
}

func toWireStandings(rows []draw.Row) []wireStanding {
	out := make([]wireStanding, len(rows))
	for i, r := range rows {
		out[i] = wireStanding{
			PairID: r.Pair, Rank: r.Rank, Played: r.Played, Won: r.Won, Lost: r.Lost, SetsFor: r.SetsFor,
			SetsAgainst: r.SetsAgainst, GamesFor: r.GamesFor, GamesAgainst: r.GamesAgainst,
		}
	}
	return out
}

func toWireGroups(d *tournamentstore.Detail) []wireGroup {
	out := []wireGroup{}
	for _, c := range d.Categories {
		for _, t := range tournamentstore.GroupTables(d, c.ID) {
			out = append(out, wireGroup{
				ID: t.Group.ID, CategoryID: t.Group.CategoryID, Name: t.Group.Name, Position: t.Group.Position,
				Standings: toWireStandings(t.Rows),
			})
		}
	}
	return out
}

func toWireMatch(m *tournamentstore.Match, courts map[uuid.UUID]string) wireMatch {
	out := wireMatch{
		ID: m.ID, CategoryID: m.CategoryID, Stage: m.Stage, GroupID: m.GroupID, Round: m.Round, Position: m.Position,
		Pair1ID: m.Pair1ID, Pair2ID: m.Pair2ID, Bye: m.Bye, Status: m.Status, WinnerPairID: m.WinnerID,
		WalkoverLoser: m.WalkoverLoser, Sets: []wireSet{},
	}
	for _, s := range m.Sets {
		out.Sets = append(out.Sets, wireSet{A: s.A, B: s.B, Super: s.Super})
	}
	if m.Source1 != nil {
		out.Source1 = &wireSource{GroupID: m.Source1.GroupID, Rank: m.Source1.Rank}
	}
	if m.Source2 != nil {
		out.Source2 = &wireSource{GroupID: m.Source2.GroupID, Rank: m.Source2.Rank}
	}
	if m.Scheduled() {
		starts := m.StartsAt()
		out.Schedule = &wireSchedule{
			CourtID: *m.CourtID, CourtName: courts[*m.CourtID], Date: m.ScheduledOn.Format(time.DateOnly),
			StartTime: *m.StartTime, EndTime: starts.Add(time.Duration(*m.Duration) * time.Minute).In(timezone.Argentina).Format("15:04"),
			DurationMinutes: *m.Duration,
		}
	}
	return out
}

func toWireMatches(ms []*tournamentstore.Match, courts map[uuid.UUID]string) []wireMatch {
	out := make([]wireMatch, len(ms))
	for i, m := range ms {
		out[i] = toWireMatch(m, courts)
	}
	return out
}

// The owner's views carry the court fee, which a visitor never sees: they wrap
// the shared category and match so the public wire stays as it was.

type wireOwnerCategory struct {
	wireCategory
	// CourtFee is centavos each pair pays per match at the complex; 0 is none.
	CourtFee int `json:"court_fee"`
}

// wireCourtFeeSide is what one pair of a match owes or paid for the court. While
// uncharged Amount is the category's current fee; once charged it is what the
// charge was made at.
type wireCourtFeeSide struct {
	Side       int        `json:"side"`
	Chargeable bool       `json:"chargeable"`
	Amount     int        `json:"amount"`
	Charged    bool       `json:"charged"`
	Method     *string    `json:"method"`
	ChargedAt  *time.Time `json:"charged_at"`
	ChargedBy  *uuid.UUID `json:"charged_by"`
}

type wireOwnerMatch struct {
	wireMatch
	CourtFee []wireCourtFeeSide `json:"court_fee"`
}

type wireOwnerDetail struct {
	Tournament wireTournament      `json:"tournament"`
	Categories []wireOwnerCategory `json:"categories"`
	Pairs      any                 `json:"pairs"`
	Groups     []wireGroup         `json:"groups"`
	Matches    []wireOwnerMatch    `json:"matches"`
}

type wireOwnerSummary struct {
	wireTournament
	Categories []wireOwnerCategory `json:"categories"`
	// UnscheduledCount is how many matches have both pairs and no court yet.
	UnscheduledCount int `json:"unscheduled_count"`
}

func toWireOwnerCategories(cats []*tournamentstore.Category) []wireOwnerCategory {
	out := make([]wireOwnerCategory, len(cats))
	for i, c := range cats {
		out[i] = wireOwnerCategory{wireCategory: toWireCategory(c, nil), CourtFee: c.CourtFee}
	}
	return out
}

// toWireOwnerMatches maps the matches with what each pair owes for the court.
func toWireOwnerMatches(d *tournamentstore.Detail, courts map[uuid.UUID]string) []wireOwnerMatch {
	fees := map[uuid.UUID]map[int]*tournamentstore.MatchFee{}
	for _, f := range d.Fees {
		if fees[f.MatchID] == nil {
			fees[f.MatchID] = map[int]*tournamentstore.MatchFee{}
		}
		fees[f.MatchID][f.Side] = f
	}
	current := map[uuid.UUID]*tournamentstore.Category{}
	for _, c := range d.Categories {
		current[c.ID] = c
	}
	out := make([]wireOwnerMatch, len(d.Matches))
	for i, m := range d.Matches {
		out[i] = wireOwnerMatch{wireMatch: toWireMatch(m, courts), CourtFee: courtFeeSides(m, current[m.CategoryID], fees[m.ID], d.Tournament.Status)}
	}
	return out
}

// courtFeeSides is the court fee of both sides of a match. A side is chargeable
// while its pair is known, the match is played (not a bye, not a walkover), the
// category has a fee and neither it nor the tournament was cancelled.
func courtFeeSides(m *tournamentstore.Match, cat *tournamentstore.Category, charged map[int]*tournamentstore.MatchFee, tournamentStatus string) []wireCourtFeeSide {
	due := 0
	live := tournamentStatus == tournamentstore.StatusActive
	if cat != nil {
		due, live = cat.CourtFee, live && !cat.Cancelled
	}
	played := !m.Bye && m.WalkoverLoser == 0
	pairs := [2]*uuid.UUID{m.Pair1ID, m.Pair2ID}
	out := make([]wireCourtFeeSide, 2)
	for i := range out {
		side := i + 1
		out[i] = wireCourtFeeSide{Side: side, Amount: due, Chargeable: live && played && due > 0 && pairs[i] != nil}
		if f := charged[side]; f != nil {
			method, at, by := f.Method, f.ChargedAt, f.ChargedBy
			out[i].Charged, out[i].Amount, out[i].Chargeable = true, f.Amount, false
			out[i].Method, out[i].ChargedAt, out[i].ChargedBy = &method, &at, &by
		}
	}
	return out
}

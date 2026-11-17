package tournaments

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/events"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// Limits of a tournament, mirrored by the schema's CHECK constraints where it
// has them. The names are short because a category's sign-up container is named
// "tournament · category" and event names stop at 100 characters.
const (
	MaxNameLength = 60
	MaxCategories = 20
	MinCapacity   = 2
	MaxCapacity   = 128
	MaxPrice      = events.MaxPrice
	// HorizonDays is how far ahead a tournament may start, the booking horizon.
	HorizonDays = 365
	// MaxDays is the longest a tournament may last.
	MaxDays = 60
	// Defaults of a groups-format category: groups of 3, the top 2 advance.
	DefaultGroupSize          = 3
	DefaultQualifiersPerGroup = 2
	MaxGroupSize              = 12
	// MinMatchMinutes and MaxMatchMinutes bound a scheduled match's duration;
	// DefaultMatchMinutes is what a padel match is given when none is asked for.
	MinMatchMinutes     = 30
	MaxMatchMinutes     = 480
	DefaultMatchMinutes = 90

	// The match format a category plays unless its owner says otherwise: best
	// of three sets of six games, the third a match tiebreak.
	DefaultSetsToWin   = 2
	DefaultGamesPerSet = 6
	MinGamesPerSet     = 4
	MaxGamesPerSet     = 9
)

var allowedSports = []string{"padel", "tennis", "soccer", "basketball", "volleyball", "hockey", "pickleball"}

// sportPadel is the one sport whose categories take a level or a suma. Every
// sport's category takes a modality.
const sportPadel = "padel"

// onlyPadelCategory is the refusal of a level or a suma on a category of another sport.
const onlyPadelCategory = "only padel categories take a level or a suma; other sports take a modality alone"

// CategoryInput is a category an owner creates with a tournament. Every category
// takes a modality; padel also takes a level or a suma. The name is composed from
// those traits and never typed.
type CategoryInput struct {
	// Modality is ModalityMen, ModalityWomen or ModalityMixed.
	Modality string
	// Level (MinLevel to MaxLevel) or Suma (MinSuma to MaxSuma) is set on padel, the
	// other zero: a category is a level or a suma, never both. Other sports take neither.
	Level int
	Suma  int
	// Prizes is optional free text, at most MaxPrizesLength characters.
	Prizes string
	// StartsOn is the day the category starts, within the tournament's dates, and
	// StartsAt its time as HH:MM.
	StartsOn time.Time
	StartsAt string
	Format   string
	Capacity int
	// Price is centavos per pair.
	Price int
	// GroupSize and QualifiersPerGroup apply to the groups format; zero means the
	// default (3 and 2).
	GroupSize          int
	QualifiersPerGroup int
	// SetsToWin is 1 (a single set) or 2 (best of three), GamesPerSet 4..9,
	// SuperTiebreakDecider makes the deciding set a match tiebreak (only when
	// SetsToWin is 2) and MatchDurationMinutes is the court time a match gets
	// when scheduled without one. Zero, or nil for the bool, means the default
	// (2, 6, true and 90).
	// CourtFee is centavos each pair pays per match at the complex; 0 is none.
	SetsToWin            int
	GamesPerSet          int
	SuperTiebreakDecider *bool
	MatchDurationMinutes int
	CourtFee             int
}

// CreateInput is an owner creating a tournament.
type CreateInput struct {
	Name       string
	Sport      string
	StartDate  time.Time
	EndDate    time.Time
	Categories []CategoryInput
}

func (in *CreateInput) validate(now time.Time) map[string]string {
	errs := map[string]string{}
	if in.Sport == "" {
		in.Sport = sportPadel
	}
	// The name is optional: a tournament given none is named after its sport.
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case in.Name == "":
		in.Name = defaultTournamentName(in.Sport)
	case len([]rune(in.Name)) > MaxNameLength:
		errs["name"] = fmt.Sprintf("must be between 1 and %d characters", MaxNameLength)
	}
	if !slices.Contains(allowedSports, in.Sport) {
		errs["sport"] = "must be a supported sport"
	}
	today, start, end := timezone.Day(now), timezone.Day(in.StartDate), timezone.Day(in.EndDate)
	switch {
	case start.Before(today):
		errs["start_date"] = "must not be in the past"
	case start.After(today.AddDate(0, 0, HorizonDays)):
		errs["start_date"] = fmt.Sprintf("must be within %d days", HorizonDays)
	}
	switch {
	case end.Before(start):
		errs["end_date"] = "must be on or after start_date"
	case end.After(start.AddDate(0, 0, MaxDays)):
		errs["end_date"] = fmt.Sprintf("must be within %d days of start_date", MaxDays)
	}
	if len(in.Categories) < 1 || len(in.Categories) > MaxCategories {
		errs["categories"] = fmt.Sprintf("must have between 1 and %d categories", MaxCategories)
	}
	seen := map[string]bool{}
	for i := range in.Categories {
		in.Categories[i].validate(in.Sport, i, start, end, seen, errs)
	}
	return errs
}

// validate checks one category of a tournament that runs from start to end. Its
// traits decide its composed name, which must be new in the tournament, so a
// repeated one is refused.
func (c *CategoryInput) validate(sport string, i int, start, end time.Time, seen map[string]bool, errs map[string]string) {
	field := func(name string) string { return fmt.Sprintf("categories[%d].%s", i, name) }
	c.validateTraits(sport, field, seen, errs)
	c.validatePrizes(field, errs)
	c.validateStart(field, start, end, errs)
	if c.Format != tournamentstore.FormatElimination && c.Format != tournamentstore.FormatGroups {
		errs[field("format")] = "must be elimination or groups"
	}
	if c.Capacity < MinCapacity || c.Capacity > MaxCapacity {
		errs[field("capacity")] = fmt.Sprintf("must be between %d and %d pairs", MinCapacity, MaxCapacity)
	}
	if c.Price <= 0 || c.Price > MaxPrice {
		errs[field("price")] = "must be a positive amount in centavos"
	}
	if c.GroupSize == 0 {
		c.GroupSize = DefaultGroupSize
	}
	if c.QualifiersPerGroup == 0 {
		c.QualifiersPerGroup = DefaultQualifiersPerGroup
		// The default qualifiers never exceed what a small group leaves out.
		c.QualifiersPerGroup = min(c.QualifiersPerGroup, max(1, c.GroupSize-1))
	}
	if c.GroupSize < 2 || c.GroupSize > MaxGroupSize {
		errs[field("group_size")] = fmt.Sprintf("must be between 2 and %d pairs", MaxGroupSize)
	} else if c.QualifiersPerGroup < 1 || c.QualifiersPerGroup >= c.GroupSize {
		errs[field("qualifiers_per_group")] = "must be at least 1 and less than the group size"
	}
	c.validateMatchFormat(field, errs)
	if c.CourtFee < 0 || c.CourtFee > MaxPrice {
		errs[field("court_fee")] = "must be an amount in centavos, 0 for none"
	}
}

// validateTraits checks a category's modality, which every sport's category takes,
// and its level or suma, which only padel takes: any other sport refuses both. The
// composed name must be new in the tournament; a repeat is reported under the field
// the category is identified by, with the stable code.
func (c *CategoryInput) validateTraits(sport string, field func(string) string, seen map[string]bool, errs map[string]string) {
	ok := true
	if c.Modality != ModalityMen && c.Modality != ModalityWomen && c.Modality != ModalityMixed {
		errs[field("modality")] = "must be men, women or mixed"
		ok = false
	}
	trait := field("modality")
	if sport == sportPadel {
		trait = field("level")
		if c.Suma != 0 {
			trait = field("suma")
		}
		ok = c.validateLevelOrSuma(field, errs) && ok
	} else {
		ok = c.refuseLevelAndSuma(field, errs) && ok
	}
	if !ok {
		return
	}
	if key := strings.ToLower(c.composedName(sport)); seen[key] {
		errs[trait] = httpx.CodeCategoryDuplicate
	} else {
		seen[key] = true
	}
}

// validateLevelOrSuma checks a padel category's level or suma: exactly one, in range.
func (c *CategoryInput) validateLevelOrSuma(field func(string) string, errs map[string]string) bool {
	switch {
	case c.Level != 0 && c.Suma != 0:
		errs[field("level")] = "must not be set together with suma: a category is a level or a suma"
	case c.Level == 0 && c.Suma == 0:
		errs[field("level")] = fmt.Sprintf("must be set (%d to %d), or suma (%d to %d) instead", MinLevel, MaxLevel, MinSuma, MaxSuma)
	case c.Level != 0 && (c.Level < MinLevel || c.Level > MaxLevel):
		errs[field("level")] = fmt.Sprintf("must be between %d and %d", MinLevel, MaxLevel)
	case c.Suma != 0 && (c.Suma < MinSuma || c.Suma > MaxSuma):
		errs[field("suma")] = fmt.Sprintf("must be between %d and %d", MinSuma, MaxSuma)
	default:
		return true
	}
	return false
}

// refuseLevelAndSuma refuses a level or a suma on a category of a sport other than
// padel, which takes neither.
func (c *CategoryInput) refuseLevelAndSuma(field func(string) string, errs map[string]string) bool {
	if c.Level != 0 {
		errs[field("level")] = onlyPadelCategory
	}
	if c.Suma != 0 {
		errs[field("suma")] = onlyPadelCategory
	}
	return c.Level == 0 && c.Suma == 0
}

// validatePrizes trims a category's prizes and refuses a text over MaxPrizesLength.
func (c *CategoryInput) validatePrizes(field func(string) string, errs map[string]string) {
	c.Prizes = strings.TrimSpace(c.Prizes)
	if len([]rune(c.Prizes)) > MaxPrizesLength {
		errs[field("prizes")] = fmt.Sprintf("must be at most %d characters", MaxPrizesLength)
	}
}

// validateStart checks the day and the time a category starts. Both are required:
// the day has to fall within the tournament's dates.
func (c *CategoryInput) validateStart(field func(string) string, start, end time.Time, errs map[string]string) {
	switch day := timezone.Day(c.StartsOn); {
	case c.StartsOn.IsZero():
		errs[field("starts_on")] = "must be provided"
	case day.Before(start) || day.After(end):
		errs[field("starts_on")] = "must be within the tournament's dates"
	}
	t, err := time.Parse("15:04", c.StartsAt)
	if err != nil {
		errs[field("starts_at")] = "must be a time in HH:MM format"
		return
	}
	c.StartsAt = t.Format("15:04")
}

// composedName is the name a category is stored under, composed from its traits: the
// modality and the level or suma on padel, the modality's label alone on any other sport.
func (c *CategoryInput) composedName(sport string) string {
	return ComposeCategoryName(sport, c.Modality, c.Level, c.Suma)
}

// validateMatchFormat fills the match format's defaults and refuses a value out
// of range.
func (c *CategoryInput) validateMatchFormat(field func(string) string, errs map[string]string) {
	if c.SetsToWin == 0 {
		c.SetsToWin = DefaultSetsToWin
	}
	if c.GamesPerSet == 0 {
		c.GamesPerSet = DefaultGamesPerSet
	}
	if c.MatchDurationMinutes == 0 {
		c.MatchDurationMinutes = DefaultMatchMinutes
	}
	if c.SuperTiebreakDecider == nil {
		yes := true
		c.SuperTiebreakDecider = &yes
	}
	if c.SetsToWin != 1 && c.SetsToWin != 2 {
		errs[field("sets_to_win")] = "must be 1 (a single set) or 2 (best of three)"
	}
	if c.GamesPerSet < MinGamesPerSet || c.GamesPerSet > MaxGamesPerSet {
		errs[field("games_per_set")] = fmt.Sprintf("must be between %d and %d", MinGamesPerSet, MaxGamesPerSet)
	}
	if c.MatchDurationMinutes < MinMatchMinutes || c.MatchDurationMinutes > MaxMatchMinutes {
		errs[field("match_duration_minutes")] = fmt.Sprintf("must be between %d and %d", MinMatchMinutes, MaxMatchMinutes)
	}
}

// Create publishes a tournament with its categories.
func (s *Service) Create(ctx context.Context, complex *complexstore.Complex, actor Actor, in CreateInput) (*tournamentstore.Detail, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	if errs := in.validate(time.Now()); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	t := &tournamentstore.Tournament{
		ComplexID: complex.ID, Name: in.Name, Sport: in.Sport, StartDate: timezone.Day(in.StartDate),
		EndDate: timezone.Day(in.EndDate), CreatedBy: *actor.UserID,
	}
	cats := make([]tournamentstore.NewCategory, len(in.Categories))
	for i, c := range in.Categories {
		cats[i] = tournamentstore.NewCategory{
			Name: c.composedName(in.Sport), Modality: c.Modality, Level: c.Level, Suma: c.Suma,
			Prizes: optionalText(c.Prizes), StartsOn: timezone.Day(c.StartsOn), StartsAt: c.StartsAt,
			Format: c.Format, Capacity: c.Capacity, Price: c.Price,
			GroupSize: c.GroupSize, QualifiersPerGroup: c.QualifiersPerGroup,
			SetsToWin: c.SetsToWin, GamesPerSet: c.GamesPerSet, SuperTiebreakDecider: *c.SuperTiebreakDecider,
			MatchDurationMinutes: c.MatchDurationMinutes, CourtFee: c.CourtFee,
		}
	}
	if err := s.store.Create(ctx, t, cats); err != nil {
		return nil, err
	}
	detail, err := s.store.Detail(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	s.record(actor, complex.ID, "create", t.ID, detail.Tournament)
	return detail, nil
}

const entityTournament = "tournament"

// List returns the complex's tournaments overlapping [from, to], with their
// categories, cancelled ones included.
func (s *Service) List(ctx context.Context, complexID uuid.UUID, from, to time.Time) ([]*Summary, error) {
	ts, err := s.store.List(ctx, complexID, from, to, true)
	if err != nil {
		return nil, err
	}
	sums, err := s.summaries(ctx, ts)
	if err != nil {
		return nil, err
	}
	for _, sum := range sums {
		if sum.Tournament.Status != tournamentstore.StatusActive {
			continue
		}
		matches, err := s.store.Matches(ctx, sum.Tournament.ID)
		if err != nil {
			return nil, err
		}
		sum.UnscheduledCount = countToSchedule(matches)
	}
	return sums, nil
}

// countToSchedule is how many matches still have no court: not a bye, not
// played. A later-round match still waiting for its pairs counts too, since it
// can be given its court and hours before they are known.
func countToSchedule(matches []*tournamentstore.Match) int {
	n := 0
	for _, m := range matches {
		if !m.Bye && m.Status != tournamentstore.MatchCompleted && !m.Scheduled() {
			n++
		}
	}
	return n
}

// Summary is a tournament with its categories. UnscheduledCount is filled by
// the owner's List only.
type Summary struct {
	Tournament       *tournamentstore.Tournament
	Categories       []*tournamentstore.Category
	UnscheduledCount int
}

func (s *Service) summaries(ctx context.Context, ts []*tournamentstore.Tournament) ([]*Summary, error) {
	out := make([]*Summary, len(ts))
	for i, t := range ts {
		cats, err := s.store.Categories(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		out[i] = &Summary{Tournament: t, Categories: cats}
	}
	return out, nil
}

// Get returns one tournament of the complex with everything under it. One of
// another complex is not found.
func (s *Service) Get(ctx context.Context, complexID, id uuid.UUID) (*tournamentstore.Detail, error) {
	d, err := s.store.Detail(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.Tournament.ComplexID != complexID {
		return nil, data.ErrRecordNotFound
	}
	return d, nil
}

// category loads a category of a tournament of the complex.
func (s *Service) category(ctx context.Context, complexID, tournamentID, categoryID uuid.UUID) (*tournamentstore.Tournament, *tournamentstore.Category, error) {
	t, err := s.store.Get(ctx, tournamentID)
	if err != nil {
		return nil, nil, err
	}
	if t.ComplexID != complexID {
		return nil, nil, data.ErrRecordNotFound
	}
	cat, err := s.store.Category(ctx, tournamentID, categoryID)
	if err != nil {
		return nil, nil, err
	}
	return t, cat, nil
}

// Refund is what became of one paid pair's money when a tournament or category
// was cancelled.
type Refund = events.Refund

// CancelResult is what a cancellation did: every paid pair refunded.
type CancelResult struct {
	Tournament *tournamentstore.Tournament
	Refunds    []Refund
}

// Cancel cancels a tournament: every category is cancelled (each paid pair is
// refunded through MercadoPago and told, as for an event) and every court its
// matches held goes back on sale.
func (s *Service) Cancel(ctx context.Context, complex *complexstore.Complex, actor Actor, id uuid.UUID, reason *string) (*CancelResult, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	t, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if t.ComplexID != complex.ID {
		return nil, data.ErrRecordNotFound
	}
	if t.Status != tournamentstore.StatusActive {
		return nil, &StateError{Message: "the tournament is already cancelled"}
	}
	cats, err := s.store.Categories(ctx, id)
	if err != nil {
		return nil, err
	}
	result := &CancelResult{}
	for _, c := range cats {
		if c.Cancelled {
			continue
		}
		res, err := s.events.CancelAny(ctx, complex, actor, c.EventID, reason)
		if err != nil {
			var state *StateError
			if errors.As(err, &state) {
				continue
			}
			return nil, err
		}
		result.Refunds = append(result.Refunds, res.Refunds...)
	}
	cancelled, err := s.store.Cancel(ctx, id, *actor.UserID, reason)
	if err != nil {
		if errors.Is(err, tournamentstore.ErrNotActive) {
			return nil, &StateError{Message: "the tournament is already cancelled"}
		}
		return nil, err
	}
	result.Tournament = cancelled
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "cancel", id, cancelled)
	return result, nil
}

// CancelCategory cancels one category: its paid pairs are refunded and told, its
// scheduled matches release their courts. The tournament goes on without it.
func (s *Service) CancelCategory(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, categoryID uuid.UUID, reason *string) (*CancelResult, error) {
	t, cat, err := s.category(ctx, complex.ID, tournamentID, categoryID)
	if err != nil {
		return nil, err
	}
	if cat.Cancelled || t.Status != tournamentstore.StatusActive {
		return nil, &StateError{Message: "the category is already cancelled"}
	}
	res, err := s.events.CancelAny(ctx, complex, actor, cat.EventID, reason)
	if err != nil {
		return nil, err
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "cancel_category", categoryID, cat)
	return &CancelResult{Tournament: t, Refunds: res.Refunds}, nil
}

// DrawInput is an owner generating a category's draw.
type DrawInput struct {
	// SeedOrder names the pairs, best seed first; the rest follow in random order.
	SeedOrder []uuid.UUID
}

// GenerateDraw makes (or remakes) a category's draw from its paid pairs. It
// closes the category's sign-up, and is refused once a result exists.
func (s *Service) GenerateDraw(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, categoryID uuid.UUID, in DrawInput) (*tournamentstore.Detail, error) {
	if _, _, err := s.category(ctx, complex.ID, tournamentID, categoryID); err != nil {
		return nil, err
	}
	err := s.store.GenerateDraw(ctx, tournamentstore.DrawInput{
		CategoryID: categoryID, SeedOrder: in.SeedOrder,
		//nolint:gosec // G404: orders the unseeded pairs of a draw the owner can remake; not a secret.
		Shuffle: func(ids []uuid.UUID) { rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] }) },
	})
	if err != nil {
		return nil, drawError(err)
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "generate_draw", categoryID, map[string]int{"seeds": len(in.SeedOrder)})
	return s.Get(ctx, complex.ID, tournamentID)
}

// drawError turns what the store refused a draw change with into the error the
// handler maps.
func drawError(err error) error {
	switch {
	case errors.Is(err, tournamentstore.ErrNotEnoughPairs):
		return &StateError{Message: "the category needs at least two paid pairs to be drawn"}
	case errors.Is(err, draw.ErrGroupCount):
		return &ValidationError{Errors: map[string]string{"group_size": "the paid pairs cannot be split into groups with the category's group size and qualifiers"}}
	case errors.Is(err, tournamentstore.ErrHasResults):
		return &ConflictError{Message: "a result was already loaded, the draw can no longer change"}
	case errors.Is(err, tournamentstore.ErrHasCharges):
		return &ConflictError{Message: "a court fee was already charged, undo it before changing the draw"}
	case errors.Is(err, tournamentstore.ErrNoDraw):
		return &StateError{Message: "the draw was not generated yet"}
	case errors.Is(err, tournamentstore.ErrUnknownPair):
		return &ValidationError{Errors: map[string]string{"pair_id": "must be a paid pair of the category"}}
	case errors.Is(err, tournamentstore.ErrInvalidMove):
		return &ValidationError{Errors: map[string]string{"with_pair_id": "must be another pair, in another group for a groups draw"}}
	case errors.Is(err, tournamentstore.ErrCancelled):
		return &StateError{Message: "the tournament or category was cancelled"}
	}
	return err
}

// MovePair swaps two pairs of a drawn category by hand: their places in the
// bracket or their groups. Allowed only before any result.
func (s *Service) MovePair(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, categoryID, pairID, withID uuid.UUID) (*tournamentstore.Detail, error) {
	if _, _, err := s.category(ctx, complex.ID, tournamentID, categoryID); err != nil {
		return nil, err
	}
	if err := s.store.MovePair(ctx, categoryID, pairID, withID); err != nil {
		return nil, drawError(err)
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "move_pair", categoryID, map[string]uuid.UUID{"pair_id": pairID, "with_pair_id": withID})
	return s.Get(ctx, complex.ID, tournamentID)
}

// match loads a match of a tournament of the complex.
func (s *Service) match(ctx context.Context, complexID, tournamentID, matchID uuid.UUID) (*tournamentstore.Match, error) {
	d, err := s.Get(ctx, complexID, tournamentID)
	if err != nil {
		return nil, err
	}
	for _, m := range d.Matches {
		if m.ID == matchID {
			return m, nil
		}
	}
	return nil, data.ErrRecordNotFound
}

// matchCategory is the category a match of the tournament belongs to.
func (s *Service) matchCategory(ctx context.Context, complexID, tournamentID, matchID uuid.UUID) (*tournamentstore.Category, error) {
	m, err := s.match(ctx, complexID, tournamentID, matchID)
	if err != nil {
		return nil, err
	}
	cats, err := s.store.Categories(ctx, tournamentID)
	if err != nil {
		return nil, err
	}
	for _, c := range cats {
		if c.ID == m.CategoryID {
			return c, nil
		}
	}
	return nil, data.ErrRecordNotFound
}

// ScheduleInput is an owner giving a match a court, a day and a time.
type ScheduleInput struct {
	CourtID   uuid.UUID
	Date      time.Time
	StartTime string
	// Minutes is the match's duration; zero means the category's
	// match_duration_minutes.
	Minutes int
}

// Schedule gives a match a court and a time, holding the court for the match's
// duration. It refuses with a ConflictError naming what already holds the hours.
func (s *Service) Schedule(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, matchID uuid.UUID, in ScheduleInput) (*tournamentstore.Match, error) {
	errs := map[string]string{}
	if !slots.ValidFormat(in.StartTime) {
		errs["start_time"] = "must be in HH:MM format (00:00-23:59)"
	}
	if in.Minutes != 0 && (in.Minutes < MinMatchMinutes || in.Minutes > MaxMatchMinutes) {
		errs["duration_minutes"] = fmt.Sprintf("must be between %d and %d", MinMatchMinutes, MaxMatchMinutes)
	}
	if len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	cat, err := s.matchCategory(ctx, complex.ID, tournamentID, matchID)
	if err != nil {
		return nil, err
	}
	if in.Minutes == 0 {
		in.Minutes = cat.MatchDurationMinutes
	}
	m, err := s.store.Schedule(ctx, tournamentstore.ScheduleInput{
		MatchID: matchID, CourtID: in.CourtID, Date: in.Date, StartTime: in.StartTime, Minutes: in.Minutes,
	})
	if err != nil {
		return nil, s.scheduleError(ctx, complex.ID, err)
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "schedule_match", matchID, m)
	return m, nil
}

func (s *Service) scheduleError(ctx context.Context, complexID uuid.UUID, err error) error {
	var held *eventstore.ConflictError
	switch {
	case errors.As(err, &held):
		return &ConflictError{Message: s.conflictMessage(ctx, complexID, held)}
	case errors.Is(err, tournamentstore.ErrOutOfDates):
		return &ValidationError{Errors: map[string]string{"date": "must be within the tournament's dates"}}
	case errors.Is(err, tournamentstore.ErrMatchNotReady):
		return &StateError{Message: "a bye is never played, so it has no schedule"}
	case errors.Is(err, tournamentstore.ErrCancelled):
		return &StateError{Message: "the tournament or category was cancelled"}
	case errors.Is(err, data.ErrRecordNotFound):
		return &ValidationError{Errors: map[string]string{"court_id": "must be an active court of this complex"}}
	}
	return err
}

// conflictMessage names what holds the match's hours, and where.
func (s *Service) conflictMessage(ctx context.Context, complexID uuid.UUID, c *eventstore.ConflictError) string {
	court := c.CourtID.String()
	if all, err := s.courts.GetByComplex(ctx, complexID); err == nil {
		for _, ct := range all {
			if ct.ID == c.CourtID {
				court = ct.Name
			}
		}
	}
	switch c.Reason {
	case eventstore.ConflictBooking:
		return fmt.Sprintf("a booking already holds those hours on %s", court)
	case eventstore.ConflictBlocked:
		return fmt.Sprintf("those hours are blocked on %s", court)
	case eventstore.ConflictFixed:
		return fmt.Sprintf("a fixed weekly booking (turno fijo) holds those hours on %s", court)
	case eventstore.ConflictMatch:
		return fmt.Sprintf("another tournament match already holds those hours on %s", court)
	default:
		return fmt.Sprintf("an event already holds those hours on %s", court)
	}
}

// Unschedule takes a match off its court.
func (s *Service) Unschedule(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, matchID uuid.UUID) (*tournamentstore.Match, error) {
	if _, err := s.match(ctx, complex.ID, tournamentID, matchID); err != nil {
		return nil, err
	}
	m, err := s.store.Unschedule(ctx, matchID)
	if err != nil {
		return nil, s.scheduleError(ctx, complex.ID, err)
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "unschedule_match", matchID, m)
	return m, nil
}

// ResultInput is a result the owner loads, in exactly one shape: the sets, only
// the winning side (1 or 2), or the side (1 or 2) that did not show up.
type ResultInput struct {
	Sets          []draw.Set
	Winner        int
	WalkoverLoser int
}

// SetResult loads or corrects the result of a match: the winner advances, group
// tables update and a finished group fills the bracket. A correction is allowed
// while the next match has no result.
func (s *Service) SetResult(ctx context.Context, complex *complexstore.Complex, actor Actor, tournamentID, matchID uuid.UUID, in ResultInput) (*tournamentstore.Detail, error) {
	if _, err := s.match(ctx, complex.ID, tournamentID, matchID); err != nil {
		return nil, err
	}
	_, err := s.store.SetResult(ctx, tournamentstore.ResultInput{MatchID: matchID, Sets: in.Sets, Winner: in.Winner, WalkoverLoser: in.WalkoverLoser})
	switch {
	case err == nil:
	case errors.Is(err, draw.ErrInvalidScore):
		return nil, &ValidationError{Errors: map[string]string{"sets": err.Error()}}
	case errors.Is(err, tournamentstore.ErrMatchNotReady):
		return nil, &StateError{Message: "the match does not have both pairs yet"}
	case errors.Is(err, tournamentstore.ErrCorrectionLocked):
		return nil, &ConflictError{Message: "the next match already has a result, so this one can no longer change"}
	case errors.Is(err, tournamentstore.ErrCancelled):
		return nil, &StateError{Message: "the tournament or category was cancelled"}
	default:
		return nil, err
	}
	s.realtime.PublishBookingChanged(complex.ID)
	s.record(actor, complex.ID, "match_result", matchID, in)
	return s.Get(ctx, complex.ID, tournamentID)
}

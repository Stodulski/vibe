package tournaments

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/events"
	"github.com/stodulski/vibe-server/internal/timezone"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// day counts from Argentina's today, the date validate checks against: the UTC
// date is a day ahead from 21:00 to midnight there.
func day(offset int) time.Time {
	d := timezone.Today().AddDate(0, 0, offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func validCreate() CreateInput {
	return CreateInput{
		Name: "Torneo Vibe", StartDate: day(10), EndDate: day(11),
		Categories: []CategoryInput{
			{Modality: ModalityMen, Level: 6, Format: tournamentstore.FormatElimination, Capacity: 16, Price: 1_000_000, StartsOn: day(10), StartsAt: "18:00"},
			{Modality: ModalityMixed, Suma: 15, Format: tournamentstore.FormatGroups, Capacity: 12, Price: 800_000, StartsOn: day(11), StartsAt: "21:30"},
		},
	}
}

func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("want a validation error; got %v", err)
	}
	return invalid.Errors
}

// S1: an owner creates a tournament with categories, each with its own capacity,
// price and format, and S3b's group settings default to 3 and 2.
func TestCreateWritesTheTournamentWithItsCategories(t *testing.T) {
	f := newFixture(t)
	d, err := f.svc.Create(t.Context(), f.complex, f.actor(), validCreate())
	if err != nil {
		t.Fatal(err)
	}
	if f.store.created.ComplexID != f.complex.ID || f.store.created.CreatedBy != f.owner || f.store.created.Sport != "padel" {
		t.Errorf("tournament = %+v", f.store.created)
	}
	if len(f.store.createdCats) != 2 || f.store.createdCats[1].Format != tournamentstore.FormatGroups ||
		f.store.createdCats[1].GroupSize != 3 || f.store.createdCats[1].QualifiersPerGroup != 2 {
		t.Errorf("categories = %+v; want the groups defaults 3 and 2", f.store.createdCats)
	}
	if len(d.Categories) != 2 || len(f.audit.entries) != 1 {
		t.Errorf("detail = %+v audit = %d", d, len(f.audit.entries))
	}
}

// S3b: group size and qualifiers per group are the owner's to set per category.
func TestCreateTakesTheGroupSettingsOfEachCategory(t *testing.T) {
	f := newFixture(t)
	in := validCreate()
	in.Categories = []CategoryInput{{
		Modality: ModalityWomen, Level: 2, Format: tournamentstore.FormatGroups, Capacity: 20, Price: 100, GroupSize: 5, QualifiersPerGroup: 3,
		StartsOn: day(10), StartsAt: "10:00",
	}}
	if _, err := f.svc.Create(t.Context(), f.complex, f.actor(), in); err != nil {
		t.Fatal(err)
	}
	if c := f.store.createdCats[0]; c.GroupSize != 5 || c.QualifiersPerGroup != 3 {
		t.Errorf("category = %+v", c)
	}
}

func TestCreateRefusesWhatTheRulesRefuse(t *testing.T) {
	cases := map[string]struct {
		change func(*CreateInput)
		field  string
	}{
		"long name":              {func(in *CreateInput) { in.Name = strings.Repeat("x", 61) }, "name"},
		"unknown sport":          {func(in *CreateInput) { in.Sport = "chess" }, "sport"},
		"start in the past":      {func(in *CreateInput) { in.StartDate = day(-1) }, "start_date"},
		"start too far":          {func(in *CreateInput) { in.StartDate = day(400); in.EndDate = day(401) }, "start_date"},
		"end before start":       {func(in *CreateInput) { in.EndDate = day(9) }, "end_date"},
		"too many days":          {func(in *CreateInput) { in.EndDate = day(10 + 61) }, "end_date"},
		"no categories":          {func(in *CreateInput) { in.Categories = nil }, "categories"},
		"unknown modality":       {func(in *CreateInput) { in.Categories[0].Modality = "boys" }, "categories[0].modality"},
		"level and suma":         {func(in *CreateInput) { in.Categories[0].Suma = 10 }, "categories[0].level"},
		"neither level nor suma": {func(in *CreateInput) { in.Categories[0].Level = 0 }, "categories[0].level"},
		"level of ten":           {func(in *CreateInput) { in.Categories[0].Level = 10 }, "categories[0].level"},
		"suma of one":            {func(in *CreateInput) { in.Categories[1].Suma = 1 }, "categories[1].suma"},
		"suma of nineteen":       {func(in *CreateInput) { in.Categories[1].Suma = 19 }, "categories[1].suma"},
		"duplicate combination":  {func(in *CreateInput) { in.Categories[1] = traitsCategory(ModalityMen, 6, 0) }, "categories[1].level"},
		"no start day":           {func(in *CreateInput) { in.Categories[0].StartsOn = time.Time{} }, "categories[0].starts_on"},
		"start day before":       {func(in *CreateInput) { in.Categories[0].StartsOn = day(9) }, "categories[0].starts_on"},
		"start day after":        {func(in *CreateInput) { in.Categories[0].StartsOn = day(12) }, "categories[0].starts_on"},
		"malformed start time":   {func(in *CreateInput) { in.Categories[0].StartsAt = "7pm" }, "categories[0].starts_at"},
		"prizes over 200":        {func(in *CreateInput) { in.Categories[0].Prizes = strings.Repeat("x", 201) }, "categories[0].prizes"},
		"unknown format":         {func(in *CreateInput) { in.Categories[0].Format = "league" }, "categories[0].format"},
		"capacity of one":        {func(in *CreateInput) { in.Categories[0].Capacity = 1 }, "categories[0].capacity"},
		"capacity too big":       {func(in *CreateInput) { in.Categories[0].Capacity = 129 }, "categories[0].capacity"},
		"free entry":             {func(in *CreateInput) { in.Categories[0].Price = 0 }, "categories[0].price"},
		"group of one":           {func(in *CreateInput) { in.Categories[1].GroupSize = 1 }, "categories[1].group_size"},
		"group too big":          {func(in *CreateInput) { in.Categories[1].GroupSize = 13 }, "categories[1].group_size"},
		"everybody qualifies":    {func(in *CreateInput) { in.Categories[1].GroupSize, in.Categories[1].QualifiersPerGroup = 4, 4 }, "categories[1].qualifiers_per_group"},
		"negative qualifiers":    {func(in *CreateInput) { in.Categories[1].QualifiersPerGroup = -1 }, "categories[1].qualifiers_per_group"},
		"more than 20 category":  {func(in *CreateInput) { in.Categories = make([]CategoryInput, 21) }, "categories"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			in := validCreate()
			tc.change(&in)
			_, err := f.svc.Create(t.Context(), f.complex, f.actor(), in)
			if _, ok := fieldErrors(t, err)[tc.field]; !ok {
				t.Errorf("want an error on %s; got %v", tc.field, err)
			}
			if f.store.created != nil {
				t.Error("nothing may be written for a refused tournament")
			}
		})
	}
	f := newFixture(t)
	if _, err := f.svc.Create(t.Context(), f.complex, Actor{}, validCreate()); !errors.Is(err, ErrNoActor) {
		t.Errorf("no actor: %v", err)
	}
}

// With two groups of one... the default qualifiers never exceed what a small group leaves out.
func TestDefaultQualifiersFitASmallGroup(t *testing.T) {
	f := newFixture(t)
	in := validCreate()
	in.Categories = []CategoryInput{{Modality: ModalityMen, Level: 1, Format: tournamentstore.FormatGroups, Capacity: 8, Price: 100, GroupSize: 2, StartsOn: day(10), StartsAt: "09:00"}}
	if _, err := f.svc.Create(t.Context(), f.complex, f.actor(), in); err != nil {
		t.Fatalf("groups of 2 with the default qualifiers: %v", err)
	}
	if f.store.createdCats[0].QualifiersPerGroup != 1 {
		t.Errorf("qualifiers = %d, want 1", f.store.createdCats[0].QualifiersPerGroup)
	}
}

// S8: owner cancels a tournament: every live category is cancelled through the
// events domain (which refunds and notifies each paid pair), the ones already
// cancelled are skipped, and the tournament is marked cancelled.
func TestCancellingATournamentCancelsEveryCategoryAndCollectsTheRefunds(t *testing.T) {
	f := newFixture(t)
	second := &tournamentstore.Category{ID: uuid.New(), TournamentID: f.store.detail.Tournament.ID, ComplexID: f.complex.ID, EventID: uuid.New(), Name: "7ma"}
	gone := &tournamentstore.Category{ID: uuid.New(), TournamentID: f.store.detail.Tournament.ID, ComplexID: f.complex.ID, EventID: uuid.New(), Name: "8va", Cancelled: true}
	f.store.detail.Categories = append(f.store.detail.Categories, second, gone)
	paid, other := uuid.New(), uuid.New()
	f.events.refunds[f.category().EventID] = []events.Refund{{RegistrationID: paid, Outcome: paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 1_020_000}}}
	f.events.refunds[second.EventID] = []events.Refund{{RegistrationID: other, Outcome: paymentstore.RefundOutcome{Result: paymentstore.RefundQueued}}}

	res, err := f.svc.Cancel(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.events.cancelled) != 2 || f.events.cancelled[0] != f.category().EventID || f.events.cancelled[1] != second.EventID {
		t.Errorf("cancelled events = %v; want the two live categories", f.events.cancelled)
	}
	if len(res.Refunds) != 2 || res.Refunds[0].RegistrationID != paid || res.Refunds[1].RegistrationID != other {
		t.Errorf("refunds = %+v", res.Refunds)
	}
	if f.store.cancelled != 1 || res.Tournament.Status != tournamentstore.StatusCancelled {
		t.Errorf("the tournament must be marked cancelled: %+v", res.Tournament)
	}
}

func TestCancellingTwiceOrSomeoneElsesTournamentIsRefused(t *testing.T) {
	f := newFixture(t)
	f.store.detail.Tournament.Status = tournamentstore.StatusCancelled
	var state *StateError
	if _, err := f.svc.Cancel(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, nil); !errors.As(err, &state) {
		t.Errorf("a second cancel: %v", err)
	}
	f.store.detail.Tournament.Status = tournamentstore.StatusActive
	f.store.detail.Tournament.ComplexID = uuid.New()
	if _, err := f.svc.Cancel(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, nil); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("another complex's tournament: %v", err)
	}
	if len(f.events.cancelled) != 0 {
		t.Error("nothing may be refunded for a refused cancel")
	}
}

// S8: cancelling one category refunds only its pairs.
func TestCancellingACategoryRefundsOnlyItsPairs(t *testing.T) {
	f := newFixture(t)
	paid := uuid.New()
	f.events.refunds[f.category().EventID] = []events.Refund{{RegistrationID: paid}}
	res, err := f.svc.CancelCategory(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, nil)
	if err != nil || len(res.Refunds) != 1 || len(f.events.cancelled) != 1 {
		t.Fatalf("res = %+v err = %v cancelled = %v", res, err, f.events.cancelled)
	}
	if f.store.cancelled != 0 {
		t.Error("the tournament goes on")
	}
	f.category().Cancelled = true
	var state *StateError
	if _, err := f.svc.CancelCategory(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, nil); !errors.As(err, &state) {
		t.Errorf("a second cancel: %v", err)
	}
}

// S4: what the store refuses a draw change with is answered with the right class
// of error, and the owner's seeds reach the store.
func TestDrawErrorsAreMappedAndSeedsPassedOn(t *testing.T) {
	cases := []struct {
		err   error
		check func(error) bool
	}{
		{tournamentstore.ErrNotEnoughPairs, func(e error) bool { var s *StateError; return errors.As(e, &s) }},
		{tournamentstore.ErrCancelled, func(e error) bool { var s *StateError; return errors.As(e, &s) }},
		{tournamentstore.ErrHasResults, func(e error) bool { var c *ConflictError; return errors.As(e, &c) }},
		{draw.ErrGroupCount, func(e error) bool { var v *ValidationError; return errors.As(e, &v) }},
		{tournamentstore.ErrUnknownPair, func(e error) bool { var v *ValidationError; return errors.As(e, &v) }},
	}
	seeds := []uuid.UUID{uuid.New(), uuid.New()}
	for _, tc := range cases {
		f := newFixture(t)
		f.store.drawErr = tc.err
		_, err := f.svc.GenerateDraw(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, DrawInput{SeedOrder: seeds})
		if err == nil || !tc.check(err) {
			t.Errorf("%v mapped to %T %v", tc.err, err, err)
		}
		if len(f.store.draws) != 1 || len(f.store.draws[0].SeedOrder) != 2 || f.store.draws[0].Shuffle == nil {
			t.Errorf("the draw must carry the seeds and a shuffle for the rest: %+v", f.store.draws)
		}
	}
	f := newFixture(t)
	if _, err := f.svc.GenerateDraw(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, uuid.New(), DrawInput{}); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a category that is not the tournament's: %v", err)
	}
	f.store.moveErr = tournamentstore.ErrHasResults
	var c *ConflictError
	if _, err := f.svc.MovePair(t.Context(), f.complex, f.actor(), f.store.detail.Tournament.ID, f.category().ID, uuid.New(), uuid.New()); !errors.As(err, &c) {
		t.Errorf("a move after a result: %v", err)
	}
}

// S5: scheduling refuses what already holds the hours, naming it and the court.
func TestSchedulingConflictsAreNamedAndValidated(t *testing.T) {
	f := newFixture(t)
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New(), CategoryID: f.category().ID}}
	matchID := f.store.detail.Matches[0].ID
	tid := f.store.detail.Tournament.ID
	in := ScheduleInput{CourtID: f.court.ID, Date: day(3), StartTime: "18:00"}

	got, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in)
	if err != nil || got.ID != matchID || f.store.scheduled[0].Minutes != DefaultMatchMinutes {
		t.Fatalf("schedule = %+v (%v) input = %+v; want the default 90 minutes", got, err, f.store.scheduled)
	}

	reasons := map[string]string{
		eventstore.ConflictBooking: "booking", eventstore.ConflictBlocked: "blocked", eventstore.ConflictFixed: "fixed",
		eventstore.ConflictEvent: "event", eventstore.ConflictMatch: "match",
	}
	for reason, word := range reasons {
		f.store.scheduleErr = &eventstore.ConflictError{Reason: reason, CourtID: f.court.ID}
		_, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in)
		var conflict *ConflictError
		if !errors.As(err, &conflict) || !strings.Contains(conflict.Message, "Cancha 1") || !strings.Contains(strings.ToLower(conflict.Message), word[:3]) {
			t.Errorf("%s: %v", reason, err)
		}
	}

	f.store.scheduleErr = nil
	for name, bad := range map[string]ScheduleInput{
		"time":     {CourtID: f.court.ID, Date: day(3), StartTime: "25:00"},
		"duration": {CourtID: f.court.ID, Date: day(3), StartTime: "18:00", Minutes: 10},
	} {
		if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	f.store.scheduleErr = tournamentstore.ErrOutOfDates
	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in); fieldErrors(t, err)["date"] == "" {
		t.Errorf("out of the tournament's dates: %v", err)
	}
	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, uuid.New(), in); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a match that is not the tournament's: %v", err)
	}
}

// S6: results are validated by the rules and the store's refusals are mapped.
func TestResultErrorsAreMapped(t *testing.T) {
	f := newFixture(t)
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New()}}
	mid, tid := f.store.detail.Matches[0].ID, f.store.detail.Tournament.ID
	in := ResultInput{Sets: []draw.Set{{A: 6, B: 4}, {A: 6, B: 3}}}

	cases := []struct {
		err   error
		check func(error) bool
	}{
		{draw.ErrInvalidScore, func(e error) bool { return fieldErrors(t, e)["sets"] != "" }},
		{tournamentstore.ErrMatchNotReady, func(e error) bool { var s *StateError; return errors.As(e, &s) }},
		{tournamentstore.ErrCorrectionLocked, func(e error) bool { var c *ConflictError; return errors.As(e, &c) }},
		{tournamentstore.ErrCancelled, func(e error) bool { var s *StateError; return errors.As(e, &s) }},
	}
	for _, tc := range cases {
		f.store.resultErr = tc.err
		if _, err := f.svc.SetResult(t.Context(), f.complex, f.actor(), tid, mid, in); err == nil || !tc.check(err) {
			t.Errorf("%v mapped to %T %v", tc.err, err, err)
		}
	}
	f.store.resultErr = nil
	if _, err := f.svc.SetResult(t.Context(), f.complex, f.actor(), tid, mid, ResultInput{WalkoverLoser: 2}); err != nil {
		t.Fatal(err)
	}
	if last := f.store.results[len(f.store.results)-1]; last.WalkoverLoser != 2 || last.MatchID != mid {
		t.Errorf("walkover input = %+v", last)
	}
}

// S6b: a category created without a format plays best of three, six games, a
// match tiebreak, 90 minutes; what the owner sets is kept.
func TestCreateDefaultsAndKeepsTheMatchFormat(t *testing.T) {
	f := newFixture(t)
	no := false
	in := validCreate()
	in.Categories[1].SetsToWin, in.Categories[1].GamesPerSet = 1, 4
	in.Categories[1].SuperTiebreakDecider, in.Categories[1].MatchDurationMinutes = &no, 60
	if _, err := f.svc.Create(t.Context(), f.complex, f.actor(), in); err != nil {
		t.Fatal(err)
	}
	def, set := f.store.createdCats[0], f.store.createdCats[1]
	if def.SetsToWin != 2 || def.GamesPerSet != 6 || !def.SuperTiebreakDecider || def.MatchDurationMinutes != 90 {
		t.Errorf("defaults = %+v", def)
	}
	if set.SetsToWin != 1 || set.GamesPerSet != 4 || set.SuperTiebreakDecider || set.MatchDurationMinutes != 60 {
		t.Errorf("set = %+v", set)
	}
}

func TestCreateRefusesAMatchFormatOutOfRange(t *testing.T) {
	cases := map[string]struct {
		change func(*CategoryInput)
		field  string
	}{
		"three sets to win":    {func(c *CategoryInput) { c.SetsToWin = 3 }, "categories[0].sets_to_win"},
		"negative sets":        {func(c *CategoryInput) { c.SetsToWin = -1 }, "categories[0].sets_to_win"},
		"three game sets":      {func(c *CategoryInput) { c.GamesPerSet = 3 }, "categories[0].games_per_set"},
		"ten game sets":        {func(c *CategoryInput) { c.GamesPerSet = 10 }, "categories[0].games_per_set"},
		"match of 29 minutes":  {func(c *CategoryInput) { c.MatchDurationMinutes = 29 }, "categories[0].match_duration_minutes"},
		"match of 481 minutes": {func(c *CategoryInput) { c.MatchDurationMinutes = 481 }, "categories[0].match_duration_minutes"},
	}
	for name, c := range cases {
		f := newFixture(t)
		in := validCreate()
		c.change(&in.Categories[0])
		_, err := f.svc.Create(t.Context(), f.complex, f.actor(), in)
		if _, ok := fieldErrors(t, err)[c.field]; !ok {
			t.Errorf("%s: want an error on %s, got %v", name, c.field, err)
		}
	}
	f := newFixture(t)
	in := validCreate()
	in.Categories[0].SetsToWin, in.Categories[0].GamesPerSet, in.Categories[0].MatchDurationMinutes = 2, 9, 480
	if _, err := f.svc.Create(t.Context(), f.complex, f.actor(), in); err != nil {
		t.Errorf("the edges are valid: %v", err)
	}
}

// S6b: scheduling without a duration uses the category's, and a duration in
// the request still wins.
func TestSchedulingUsesTheCategoryDuration(t *testing.T) {
	f := newFixture(t)
	f.category().MatchDurationMinutes = 75
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New(), CategoryID: f.category().ID}}
	matchID, tid := f.store.detail.Matches[0].ID, f.store.detail.Tournament.ID
	in := ScheduleInput{CourtID: f.court.ID, Date: day(3), StartTime: "18:00"}
	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in); err != nil {
		t.Fatal(err)
	}
	in.Minutes = 120
	if _, err := f.svc.Schedule(t.Context(), f.complex, f.actor(), tid, matchID, in); err != nil {
		t.Fatal(err)
	}
	if got := f.store.scheduled; len(got) != 2 || got[0].Minutes != 75 || got[1].Minutes != 120 {
		t.Errorf("scheduled minutes = %+v, want 75 then 120", got)
	}
}

// S6d: the owner can send only the winner; it reaches the store as such.
func TestAWinnerOnlyResultReachesTheStore(t *testing.T) {
	f := newFixture(t)
	f.store.detail.Matches = []*tournamentstore.Match{{ID: uuid.New()}}
	mid, tid := f.store.detail.Matches[0].ID, f.store.detail.Tournament.ID
	if _, err := f.svc.SetResult(t.Context(), f.complex, f.actor(), tid, mid, ResultInput{Winner: 2}); err != nil {
		t.Fatal(err)
	}
	if last := f.store.results[len(f.store.results)-1]; last.Winner != 2 || last.MatchID != mid || len(last.Sets) != 0 {
		t.Errorf("winner input = %+v", last)
	}
}

// S19: "today" is Argentina's date, whatever zone the process runs in. At 22:00
// in Buenos Aires the UTC date is already tomorrow; a tournament that starts
// today is not in the past.
func TestCreateReadsTodayOnArgentinasClock(t *testing.T) {
	now := time.Date(2026, 11, 21, 1, 0, 0, 0, time.UTC) // 22:00 on the 20th in Buenos Aires
	today := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	in := validCreate()
	in.StartDate, in.EndDate = today, today.AddDate(0, 0, 1)
	in.Categories[0].StartsOn, in.Categories[1].StartsOn = today, today
	if errs := in.validate(now); errs["start_date"] != "" {
		t.Fatalf("a tournament starting today must be accepted at 22:00; got %v", errs)
	}
}

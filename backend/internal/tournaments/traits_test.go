package tournaments

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/events"
	"github.com/stodulski/vibe-server/internal/httpx"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// traitsCategory is a category that passes every rule on its own, with the given
// modality and either a level or a suma (the other is zero).
func traitsCategory(modality string, level, suma int) CategoryInput {
	return CategoryInput{
		Modality: modality, Level: level, Suma: suma, Format: tournamentstore.FormatElimination,
		Capacity: 16, Price: 1_000_000, StartsOn: day(10), StartsAt: "19:30",
	}
}

// The name is composed from the two traits, with es-AR labels for the modality and
// the ordinal level: 1ra to 9na.
func TestTheComposedNameReadsModalityAndLevelOrSuma(t *testing.T) {
	cases := []struct {
		modality    string
		level, suma int
		want        string
	}{
		{ModalityMen, 8, 0, "Caballeros 8va"},
		{ModalityWomen, 5, 0, "Damas 5ta"},
		{ModalityMixed, 6, 0, "Mixto 6ta"},
		{ModalityMixed, 1, 0, "Mixto 1ra"},
		{ModalityWomen, 2, 0, "Damas 2da"},
		{ModalityMen, 3, 0, "Caballeros 3ra"},
		{ModalityMen, 4, 0, "Caballeros 4ta"},
		{ModalityMen, 7, 0, "Caballeros 7ma"},
		{ModalityMen, 9, 0, "Caballeros 9na"},
		{ModalityMixed, 0, 15, "Mixto Suma 15"},
		{ModalityWomen, 0, 2, "Damas Suma 2"},
	}
	for _, tc := range cases {
		if got := ComposeCategoryName(tc.modality, tc.level, tc.suma); got != tc.want {
			t.Errorf("ComposeCategoryName(%s, %d, %d) = %q; want %q", tc.modality, tc.level, tc.suma, got, tc.want)
		}
	}
}

// The owner picks the traits; the server stores the composed name with them, and
// the prizes are trimmed while a blank prize is stored as none.
func TestCreateStoresTheComposedNameAndTheTraits(t *testing.T) {
	f := newFixture(t)
	in := validCreate()
	in.Categories = []CategoryInput{traitsCategory(ModalityWomen, 5, 0), traitsCategory(ModalityMixed, 0, 15)}
	in.Categories[0].Prizes = "  Trofeo y 2 pelotas  "
	in.Categories[1].Prizes = "   "
	if _, err := f.svc.Create(t.Context(), f.complex, f.actor(), in); err != nil {
		t.Fatal(err)
	}
	damas, mixto := f.store.createdCats[0], f.store.createdCats[1]
	if damas.Name != "Damas 5ta" || damas.Modality != ModalityWomen || damas.Level != 5 || damas.Suma != 0 {
		t.Errorf("level category = %+v", damas)
	}
	if damas.Prizes == nil || *damas.Prizes != "Trofeo y 2 pelotas" {
		t.Errorf("prizes = %v; want the trimmed text", damas.Prizes)
	}
	if mixto.Name != "Mixto Suma 15" || mixto.Level != 0 || mixto.Suma != 15 || mixto.Prizes != nil {
		t.Errorf("suma category = %+v; want the suma and no prizes", mixto)
	}
	if damas.StartsAt != "19:30" || damas.StartsOn.Format("2006-01-02") != day(10).Format("2006-01-02") {
		t.Errorf("start = %s %s", damas.StartsOn, damas.StartsAt)
	}
}

// A blank tournament name is not refused: the stored name is composed from the sport.
func TestAnOmittedTournamentNameGetsTheComposedDefault(t *testing.T) {
	cases := map[string]struct{ name, sport, want string }{
		"blank name":  {"   ", "padel", "Torneo de pádel"},
		"empty name":  {"", "tennis", "Torneo de tenis"},
		"soccer name": {"", "soccer", "Torneo de fútbol"},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			f := newFixture(t)
			in := validCreate()
			in.Name, in.Sport = tc.name, tc.sport
			if _, err := f.svc.Create(t.Context(), f.complex, f.actor(), in); err != nil {
				t.Fatal(err)
			}
			if f.store.created.Name != tc.want {
				t.Errorf("name = %q; want %q", f.store.created.Name, tc.want)
			}
		})
	}
}

// A repeated modality and level, or modality and suma, is one combination: the
// tournament refuses it before anything is written.
func TestARepeatedCombinationIsRefusedWithItsStableCode(t *testing.T) {
	f := newFixture(t)
	in := validCreate()
	in.Categories[1] = traitsCategory(ModalityMen, 6, 0)
	_, err := f.svc.Create(t.Context(), f.complex, f.actor(), in)
	var invalid *ValidationError
	if !errors.As(err, &invalid) || invalid.Errors["categories[1].level"] != httpx.CodeCategoryDuplicate {
		t.Fatalf("err = %v; want the duplicate code on categories[1].level", err)
	}
	if f.store.created != nil {
		t.Error("nothing may be written for a repeated combination")
	}
}

// setTraits gives the fixture category its traits: a modality and either a level
// or a suma (the other zero), as a category created with the current rules has.
func setTraits(f *fixture, modality string, level, suma int) {
	c := f.category()
	c.Modality = ptr(modality)
	if level != 0 {
		c.Level = ptr(level)
	}
	if suma != 0 {
		c.Suma = ptr(suma)
	}
}

// player is a person signing up as the given gender and level.
func player(gender string, level int) events.SignUpInput {
	return events.SignUpInput{Gender: ptr(gender), Level: ptr(level)}
}

// signUpPair signs the pair up to the fixture category and returns the refusal, if
// any. The payer and the partner carry the declarations given.
func signUpPair(t *testing.T, f *fixture, payer, partner events.SignUpInput) error {
	t.Helper()
	in := pairInput()
	in.Payer.Gender, in.Payer.Level = payer.Gender, payer.Level
	in.Partner.Gender, in.Partner.Level = partner.Gender, partner.Level
	_, err := f.svc.SignUp(t.Context(), events.Actor{IP: "198.51.100.9"}, "vibe", f.store.detail.Tournament.ID, f.category().ID, in)
	return err
}

// assertHeld fails when the refused pair still reached the hold or the checkout:
// the events domain's sign-up is the one that holds the spot and asks for payment.
func assertHeld(t *testing.T, f *fixture, want int) {
	t.Helper()
	if len(f.events.signUps) != want || len(f.store.pairHooks) != want {
		t.Errorf("holds = %d, hooks = %d; want %d: a refused pair must hold nothing", len(f.events.signUps), len(f.store.pairHooks), want)
	}
}

func assertRefused(t *testing.T, err error, field, code string) {
	t.Helper()
	var invalid *ValidationError
	if !errors.As(err, &invalid) || invalid.Errors[field] != code {
		t.Fatalf("err = %v; want %s on %s", err, code, field)
	}
}

// A men's category refuses a woman before anything is held.
func TestAModalityMismatchIsRefusedBeforeAnyPayment(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityMen, 6, 0)
	err := signUpPair(t, f, player(GenderMen, 7), player(GenderWomen, 7))
	assertRefused(t, err, "modality", httpx.CodeModalityMismatch)
	assertHeld(t, f, 0)
}

// A mixed category takes one man and one woman, with the genders stored on the pair.
func TestAMixedPairOfOneManAndOneWomanIsAccepted(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityMixed, 6, 0)
	if err := signUpPair(t, f, player(GenderMen, 7), player(GenderWomen, 8)); err != nil {
		t.Fatal(err)
	}
	assertHeld(t, f, 1)
	got := f.store.pairHooks[0].partner
	if got.Gender == nil || *got.Gender != GenderWomen || got.Level == nil || *got.Level != 8 {
		t.Errorf("partner declared = %v / %v; want women and 8", got.Gender, got.Level)
	}
}

// A women's category takes two women: a matching declaration goes through.
func TestAMatchingGenderIsAccepted(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityWomen, 6, 0)
	if err := signUpPair(t, f, player(GenderWomen, 6), player(GenderWomen, 8)); err != nil {
		t.Fatal(err)
	}
	assertHeld(t, f, 1)
}

// A 6ta player cannot enter an 8va category, whatever the partner.
func TestALevelTooStrongIsRefusedBeforeAnyPayment(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityMen, 8, 0)
	err := signUpPair(t, f, player(GenderMen, 6), player(GenderMen, 9))
	assertRefused(t, err, "level", httpx.CodeLevelTooStrong)
	assertHeld(t, f, 0)
}

// A player of the category's own level, or a weaker one, is accepted.
func TestAnEqualOrWeakerLevelIsAccepted(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityMen, 8, 0)
	if err := signUpPair(t, f, player(GenderMen, 8), player(GenderMen, 9)); err != nil {
		t.Fatal(err)
	}
	assertHeld(t, f, 1)
}

// A Suma below its suma is refused before anything is held.
func TestASumaBelowTheSumaIsRefusedBeforeAnyPayment(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityMixed, 0, 15)
	err := signUpPair(t, f, player(GenderMen, 6), player(GenderWomen, 7))
	assertRefused(t, err, "levels", httpx.CodeSumaTooLow)
	assertHeld(t, f, 0)
}

// A Suma at its suma is accepted, and the declared levels are stored.
func TestASumaAtTheSumaIsAccepted(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityMixed, 0, 15)
	if err := signUpPair(t, f, player(GenderMen, 6), player(GenderWomen, 9)); err != nil {
		t.Fatal(err)
	}
	assertHeld(t, f, 1)
	if got := f.store.pairHooks[0].partner; got.Level == nil || *got.Level != 9 {
		t.Errorf("partner level = %v; want 9", got.Level)
	}
}

// A category with traits needs both players' gender and level, and names the
// missing one.
func TestAModalityCategoryNeedsEveryPlayersDeclaration(t *testing.T) {
	f := newFixture(t)
	setTraits(f, ModalityMixed, 6, 0)
	err := signUpPair(t, f, events.SignUpInput{Level: ptr(7)}, player(GenderWomen, 8))
	assertRefused(t, err, "gender", "must be provided")
	assertHeld(t, f, 0)
}

// A category created before the traits keeps today's behaviour: the fields are
// optional and nothing is checked.
func TestALegacyCategoryTakesTheSignUpWithoutTheFields(t *testing.T) {
	f := newFixture(t)
	if err := signUpPair(t, f, events.SignUpInput{}, events.SignUpInput{}); err != nil {
		t.Fatal(err)
	}
	assertHeld(t, f, 1)
}

// An old category row has no traits; its wire says so with nulls, and a new one
// carries them. Both match the document.
func TestTheCategoryWireCarriesTheTraitsAndNullsForOldRows(t *testing.T) {
	prizes, startsAt, level := "Trofeo", "20:00", 4
	startsOn := day(10)
	fresh := &tournamentstore.Category{
		ID: uuid.New(), TournamentID: uuid.New(), Name: "Caballeros 4ta", Format: tournamentstore.FormatElimination,
		Capacity: 8, Price: 100_000, SetsToWin: 2, GamesPerSet: 6, MatchDurationMinutes: 90,
		Modality: ptr(ModalityMen), Level: &level, Prizes: &prizes, StartsOn: &startsOn, StartsAt: &startsAt,
	}
	validate(t, "TournamentCategory", toWireCategory(fresh, nil))

	old := &tournamentstore.Category{
		ID: uuid.New(), TournamentID: uuid.New(), Name: "6ta", Format: tournamentstore.FormatElimination,
		Capacity: 8, Price: 100_000, SetsToWin: 2, GamesPerSet: 6, MatchDurationMinutes: 90,
	}
	validate(t, "TournamentCategory", toWireCategory(old, nil))
	raw, err := json.Marshal(toWireCategory(old, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"modality":null`, `"level":null`, `"suma":null`, `"prizes":null`, `"starts_on":null`, `"starts_at":null`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("old category wire lacks %s: %s", want, raw)
		}
	}
}

// The owner's pair list shows what each player declared.
func TestTheOwnerPairWireShowsTheDeclaredLevels(t *testing.T) {
	payer, partner := 5, 8
	row := &tournamentstore.PairRow{
		Pair: tournamentstore.Pair{
			ID: uuid.New(), CategoryID: uuid.New(), RegistrationID: uuid.New(), Status: tournamentstore.PairActive,
			Partner: tournamentstore.Player{FirstName: "Socio", LastName: "Dos", Phone: "+5491100000002", Level: &partner},
		},
		Payer:              tournamentstore.Player{FirstName: "Ana", LastName: "Uno", Phone: "+5491100000001", Level: &payer},
		RegistrationStatus: "confirmed", Price: 100_000, ServiceFee: 5_000,
	}
	validate(t, "TournamentOwnerPair", toWireOwnerPair(row))
	wire := toWireOwnerPair(row)
	if wire.Payer.Level == nil || *wire.Payer.Level != 5 || wire.Partner.Level == nil || *wire.Partner.Level != 8 {
		t.Errorf("owner pair levels = %+v / %+v; want 5 and 8", wire.Payer.Level, wire.Partner.Level)
	}
}

func ptr[T any](v T) *T { return &v }

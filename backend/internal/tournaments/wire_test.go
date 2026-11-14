package tournaments

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// richDetail is a tournament with every field the wire carries set somewhere: a
// groups category with a table and a knockout waiting for it, an elimination
// category with a walkover, a super tiebreak and a scheduled match.
func richDetail(f *fixture) *tournamentstore.Detail {
	d := f.store.detail
	tid, now := d.Tournament.ID, time.Now()
	groupCat := &tournamentstore.Category{
		ID: uuid.New(), TournamentID: tid, ComplexID: f.complex.ID, EventID: uuid.New(), Name: "Mixto",
		Format: tournamentstore.FormatGroups, Capacity: 12, Price: 800_000, GroupSize: 4, QualifiersPerGroup: 2,
		DrawGeneratedAt: &now, ConfirmedCount: 3,
	}
	d.Categories = append(d.Categories, groupCat)
	email, by := "ana@example.com", "participant"
	group := &tournamentstore.Group{ID: uuid.New(), CategoryID: groupCat.ID, Name: "A", Position: 1}
	d.Groups = []*tournamentstore.Group{group}
	mkPair := func(seed int, name string, inGroup bool) *tournamentstore.PairRow {
		p := &tournamentstore.PairRow{
			Pair: tournamentstore.Pair{
				ID: uuid.New(), CategoryID: groupCat.ID, RegistrationID: uuid.New(), Seed: &seed, Status: tournamentstore.PairActive,
				Partner: tournamentstore.Player{FirstName: "Socio", LastName: name, Phone: "+549220000000"},
			},
			Payer:              tournamentstore.Player{FirstName: "Ana", LastName: name, Phone: "+549110000000", Email: &email},
			RegistrationStatus: "confirmed", Price: 800_000, ServiceFee: 20_000, HeldUntil: now, ConfirmedAt: &now,
			CancelledBy: &by, SignedUpAt: now,
			Payment: &tournamentstore.Payment{ID: uuid.New(), Status: "deposit_paid", Amount: 800_000, ServiceFee: 20_000},
		}
		if inGroup {
			p.GroupID = &group.ID
		}
		return p
	}
	a, b, c := mkPair(1, "Uno", true), mkPair(2, "Dos", true), mkPair(3, "Tres", true)
	held := mkPair(4, "Cuatro", false)
	held.RegistrationStatus, held.Payment, held.Seed = "held", nil, nil
	d.Pairs = []*tournamentstore.PairRow{a, b, c, held}

	court := f.court.ID
	start, minutes := "18:00", 90
	date := d.Tournament.StartDate
	play := &tournamentstore.Match{
		ID: uuid.New(), CategoryID: groupCat.ID, Stage: tournamentstore.StageGroup, GroupID: &group.ID, Round: 1, Position: 0,
		Pair1ID: &a.ID, Pair2ID: &b.ID, Status: tournamentstore.MatchCompleted, WinnerID: &a.ID,
		Sets:    []draw.Set{{A: 6, B: 4}, {A: 3, B: 6}, {A: 10, B: 8, Super: true}},
		CourtID: &court, ScheduledOn: &date, StartTime: &start, Duration: &minutes,
	}
	walk := &tournamentstore.Match{
		ID: uuid.New(), CategoryID: groupCat.ID, Stage: tournamentstore.StageGroup, GroupID: &group.ID, Round: 2, Position: 1,
		Pair1ID: &a.ID, Pair2ID: &c.ID, Status: tournamentstore.MatchCompleted, WinnerID: &a.ID, WalkoverLoser: 2,
	}
	final := &tournamentstore.Match{
		ID: uuid.New(), CategoryID: groupCat.ID, Stage: tournamentstore.StageBracket, Round: 1, Position: 0, Bye: true,
		Source1: &tournamentstore.Slot{GroupID: group.ID, Rank: 1}, Status: tournamentstore.MatchPending,
	}
	d.Matches = []*tournamentstore.Match{play, walk, final}
	return d
}

func (f *fixture) handler() *Handler {
	return NewHandler(f.svc, httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil))), httpx.TrustedProxies{})
}

// validate checks a JSON body against a schema of the OpenAPI document, so the
// hand-written wire types and the document cannot drift apart.
func validate(t *testing.T, schema string, body any) {
	t.Helper()
	h, err := openapi.NewHandler(httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := h.Document().Components.Schemas[schema]
	if !ok {
		t.Fatalf("schema %s is not in the document", schema)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := ref.Value.VisitJSON(decoded); err != nil {
		t.Errorf("%s does not match its schema: %v\n%s", schema, err, raw)
	}
}

func TestTheOwnersWireMatchesTheDocument(t *testing.T) {
	f := newFixture(t)
	d := richDetail(f)
	h := f.handler()
	validate(t, "TournamentDetail", envelopeOf(h.ownerDetail(t.Context(), f.complex.ID, d)))
	validate(t, "TournamentSummary", wireSummary{toWireTournament(&d.Tournament), toWireCategories(d.Categories, nil)})
	validate(t, "TournamentMatch", toWireMatch(d.Matches[0], map[uuid.UUID]string{f.court.ID: "Cancha 1"}))
}

func TestThePublicWireMatchesTheDocumentAndShowsNoContactData(t *testing.T) {
	f := newFixture(t)
	d := richDetail(f)
	body := envelopeOf(f.handler().publicDetail(t.Context(), f.complex.ID, d))
	body["complex_name"], body["cancellation_hours"] = "Vibe Padel", 24
	validate(t, "PublicTournamentDetail", body)

	raw, _ := json.Marshal(body)
	for _, secret := range []string{"+549110000000", "+549220000000", "ana@example.com", "deposit_paid", "phone", "email"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("the public view must not carry %q", secret)
		}
	}
	pairs := body["pairs"].([]any)
	if len(pairs) != 3 {
		t.Errorf("only the pairs that paid are listed; got %d", len(pairs))
	}
}

func TestThePairViewsMatchTheDocument(t *testing.T) {
	f := newFixture(t)
	view := &PairView{
		View:       eventsView(f),
		Tournament: &f.store.detail.Tournament, Category: f.category(),
		Pair: &tournamentstore.Pair{ID: uuid.New(), Status: tournamentstore.PairActive, Partner: tournamentstore.Player{FirstName: "Beto", LastName: "Socio"}},
	}
	view.ServiceFee = 20_000
	validate(t, "PublicPairRegistration", pairViewBody(view))
}

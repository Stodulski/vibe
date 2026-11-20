package tournaments

import (
	"context"
	"strings"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/events"
	"github.com/stodulski/vibe-server/internal/httpx"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/timezone"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
)

// Page is a complex with some of its tournaments, as a visitor sees them.
type Page struct {
	Complex     *complexstore.Complex
	Tournaments []*Summary
}

// PublicList returns the tournaments a visitor can still follow or sign up for:
// active and not yet over, soonest first.
func (s *Service) PublicList(ctx context.Context, slug string) (*Page, error) {
	complex, err := s.events.PublicComplex(ctx, slug)
	if err != nil {
		return nil, err
	}
	today := timezone.Today()
	ts, err := s.store.List(ctx, complex.ID, today, today.AddDate(0, 0, HorizonDays), false)
	if err != nil {
		return nil, err
	}
	sums, err := s.summaries(ctx, ts)
	return &Page{Complex: complex, Tournaments: sums}, err
}

// PublicGet returns one tournament of the complex a slug names, with its draw
// and schedule. A cancelled tournament is still shown, as cancelled, so a shared
// link says so; one of another complex is not found.
func (s *Service) PublicGet(ctx context.Context, slug string, id uuid.UUID) (*complexstore.Complex, *tournamentstore.Detail, error) {
	complex, err := s.events.PublicComplex(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	d, err := s.Get(ctx, complex.ID, id)
	return complex, d, err
}

// PairSignUpInput is a person signing up a pair: the payer, who is whoever fills
// the form, and the partner. Phone is required for both; email is optional.
type PairSignUpInput struct {
	Payer   events.SignUpInput
	Partner events.SignUpInput
}

// PairSignUpResult is a held spot for a pair and the checkout to pay for it.
type PairSignUpResult struct {
	*events.SignUpResult
	Tournament *tournamentstore.Tournament
	Category   *tournamentstore.Category
	// Pair is nil until read back.
}

// SignUp holds a spot for a pair and opens the MercadoPago checkout that pays
// the whole entry: one payer for both players. The hold, the capacity, the
// checkout and the confirmation are the events domain's; the pair, the refusal
// when either player is already in the category, and the closing of sign-up at
// the draw are this domain's, run in the same transaction.
func (s *Service) SignUp(ctx context.Context, actor Actor, slug string, tournamentID, categoryID uuid.UUID, in PairSignUpInput) (*PairSignUpResult, error) {
	if in.Payer.Phone != "" && in.Payer.Phone == in.Partner.Phone {
		return nil, &ValidationError{Errors: map[string]string{"partner.phone": "must be different from the payer's phone"}}
	}
	complex, err := s.events.PublicComplex(ctx, slug)
	if err != nil {
		return nil, err
	}
	t, cat, err := s.category(ctx, complex.ID, tournamentID, categoryID)
	if err != nil {
		return nil, err
	}
	if t.Status != tournamentstore.StatusActive || cat.Cancelled || cat.DrawGeneratedAt != nil {
		return nil, events.ErrEventClosed
	}
	// Refused before AnyEvent, the hold and the checkout: a pair that does not fit
	// its category never costs anything.
	if err := checkDeclaredPlayers(cat, in.Payer, in.Partner); err != nil {
		return nil, err
	}
	ev, err := s.events.AnyEvent(ctx, complex.ID, cat.EventID)
	if err != nil {
		return nil, err
	}
	var partnerEmail *string
	if in.Partner.Email != "" {
		partnerEmail = &in.Partner.Email
	}
	hooks := s.store.PairHooks(
		tournamentstore.Player{Phone: in.Payer.Phone},
		tournamentstore.Player{
			FirstName: in.Partner.FirstName, LastName: in.Partner.LastName, Phone: in.Partner.Phone, Email: partnerEmail,
			Gender: in.Partner.Gender, Level: in.Partner.Level,
		},
	)
	res, err := s.events.SignUpWith(ctx, actor, complex, ev, in.Payer, hooks)
	if err != nil {
		return nil, err
	}
	return &PairSignUpResult{SignUpResult: res, Tournament: t, Category: cat}, nil
}

// noLevelDeclared refuses a level declared on a category that has no level or suma.
const noLevelDeclared = "must not be sent: this category has no level or suma"

// checkDeclaredPlayers holds a pair to its category before anything is held. A
// category with a modality needs both players to declare a gender, whatever the
// sport; the genders must fit the modality. A category with a level or a suma (padel
// only) also needs each player's level: a level category takes no player stronger
// than its level, and a Suma takes a pair whose levels add up to at least its suma.
// A category without a modality (created before it) takes the pair as it is.
func checkDeclaredPlayers(cat *tournamentstore.Category, payer, partner events.SignUpInput) error {
	if cat.Modality == nil {
		return nil
	}
	errs := map[string]string{}
	if payer.Gender == nil {
		errs["gender"] = "must be provided"
	}
	if partner.Gender == nil {
		errs["partner.gender"] = "must be provided"
	}
	if cat.Level != nil || cat.Suma != nil {
		if payer.Level == nil {
			errs["level"] = "must be provided"
		}
		if partner.Level == nil {
			errs["partner.level"] = "must be provided"
		}
	} else {
		if payer.Level != nil {
			errs["level"] = noLevelDeclared
		}
		if partner.Level != nil {
			errs["partner.level"] = noLevelDeclared
		}
	}
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	if !pairFits(*cat.Modality, *payer.Gender, *partner.Gender) {
		errs["modality"] = httpx.CodeModalityMismatch
	}
	if cat.Level != nil {
		if *payer.Level < *cat.Level {
			errs["level"] = httpx.CodeLevelTooStrong
		}
		if *partner.Level < *cat.Level {
			errs["partner.level"] = httpx.CodeLevelTooStrong
		}
	}
	if cat.Suma != nil && *payer.Level+*partner.Level < *cat.Suma {
		errs["levels"] = httpx.CodeSumaTooLow
	}
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// pairFits is whether two players of these genders may play a category of this
// modality: men's both men, women's both women, mixed one of each.
func pairFits(modality, payer, partner string) bool {
	switch modality {
	case ModalityMen:
		return payer == GenderMen && partner == GenderMen
	case ModalityWomen:
		return payer == GenderWomen && partner == GenderWomen
	case ModalityMixed:
		return (payer == GenderMen && partner == GenderWomen) || (payer == GenderWomen && partner == GenderMen)
	default:
		return false
	}
}

// PairView is a pair's own spot with everything the status page renders.
type PairView struct {
	*events.View
	Tournament *tournamentstore.Tournament
	Category   *tournamentstore.Category
	Pair       *tournamentstore.Pair
	ServiceFee int
}

// Status answers a registration link with the pair's current state.
func (s *Service) Status(ctx context.Context, token string) (*PairView, error) {
	v, err := s.events.Status(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.pairView(ctx, v)
}

func (s *Service) pairView(ctx context.Context, v *events.View) (*PairView, error) {
	if v.Event.TournamentID == nil {
		return nil, data.ErrRecordNotFound
	}
	tournamentID, categoryID, err := s.store.CategoryOfEvent(ctx, v.Event.ID)
	if err != nil {
		return nil, err
	}
	t, err := s.store.Get(ctx, tournamentID)
	if err != nil {
		return nil, err
	}
	cat, err := s.store.Category(ctx, tournamentID, categoryID)
	if err != nil {
		return nil, err
	}
	pair, err := s.store.PairOf(ctx, v.Registration.ID)
	if err != nil {
		return nil, err
	}
	return &PairView{View: v, Tournament: t, Category: cat, Pair: pair, ServiceFee: pricing.ServiceFee(v.Event.Price)}, nil
}

// PairCancel is a cancelled pair and what became of its money.
type PairCancel struct {
	View *PairView
	// Outcome is the refund: issued, queued, none (nothing was paid) or not
	// eligible (cancelled outside the window, or after the draw).
	Outcome paymentstore.RefundOutcome
}

// CancelRegistration ends a pair's spot from its link. Before the draw the
// complex's cancellation policy decides the money exactly as it does for an
// event; after it the pair leaves the bracket as a walkover and the payment is
// kept.
func (s *Service) CancelRegistration(ctx context.Context, actor Actor, token string) (*PairCancel, error) {
	// Only a tournament pair's token belongs here.
	v, err := s.events.Status(ctx, token)
	if err != nil {
		return nil, err
	}
	if v.Event.TournamentID == nil {
		return nil, data.ErrRecordNotFound
	}
	res, err := s.events.CancelRegistration(ctx, actor, token)
	if err != nil {
		return nil, err
	}
	view, err := s.pairView(ctx, res.View)
	if err != nil {
		return nil, err
	}
	return &PairCancel{View: view, Outcome: res.Outcome}, nil
}

// PlayerName is how a pair is named on a bracket: "Ana Diaz / Beto Socio".
func PlayerName(p tournamentstore.Player) string {
	return strings.TrimSpace(p.FirstName + " " + p.LastName)
}

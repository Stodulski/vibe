package events

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	"github.com/stodulski/vibe-server/internal/checkout"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/events/notice"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Page is a complex with some of its events, as a visitor sees them.
type Page struct {
	Complex *complexstore.Complex
	Events  []*eventstore.Event
	// ServiceFee is the flat fee a person pays on top of an event's price.
	ServiceFee int
}

// publicComplex loads the active complex a public slug names.
func (s *Service) publicComplex(ctx context.Context, slug string) (*complexstore.Complex, error) {
	complex, err := s.complexes.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !complex.IsActive {
		return nil, data.ErrRecordNotFound
	}
	return complex, nil
}

// PublicList returns the events a visitor can still sign up for on a complex's
// page: active and not yet begun, soonest first.
func (s *Service) PublicList(ctx context.Context, slug string) (*Page, error) {
	complex, err := s.publicComplex(ctx, slug)
	if err != nil {
		return nil, err
	}
	today := timezone.Today()
	all, err := s.store.ListEvents(ctx, complex.ID, today, today.AddDate(0, 0, HorizonDays), false)
	if err != nil {
		return nil, err
	}
	var upcoming []*eventstore.Event
	for _, ev := range all {
		if ev.StartsAt().After(time.Now()) {
			upcoming = append(upcoming, ev)
		}
	}
	return &Page{Complex: complex, Events: upcoming, ServiceFee: pricing.ServiceFee(0)}, nil
}

// PublicGet returns one event of the complex a slug names. A cancelled event is
// still shown, as cancelled, so a shared link says so instead of a bare 404; an
// event of another complex is not found.
func (s *Service) PublicGet(ctx context.Context, slug string, eventID uuid.UUID) (*Page, error) {
	complex, err := s.publicComplex(ctx, slug)
	if err != nil {
		return nil, err
	}
	ev, err := s.ownedEvent(ctx, complex.ID, eventID)
	if err != nil {
		return nil, err
	}
	return &Page{Complex: complex, Events: []*eventstore.Event{ev}, ServiceFee: pricing.ServiceFee(ev.Price)}, nil
}

// SignUpInput is a person asking for a spot. The phone is required; the email is
// optional (owner decision 2026-11-14) and, without it, no notice is sent.
type SignUpInput struct {
	FirstName string
	LastName  string
	Phone     string
	Email     string
	// Gender (GenderMen or GenderWomen) and Level (1 to 9) are what the person
	// declares. An event with a modality needs both; any other keeps them if given.
	Gender *string
	Level  *int
}

// The gender a person declares, and the modality an event or category is open to.
const (
	GenderMen   = "men"
	GenderWomen = "women"

	ModalityMen   = "men"
	ModalityWomen = "women"
	ModalityMixed = "mixed"

	// MinLevel and MaxLevel bound a declared level: 1ra to 9na.
	MinLevel = 1
	MaxLevel = 9
)

// Stable codes a sign-up is refused with when its declaration does not fit.
// CodeModalityMismatch: the gender is not the one the modality allows.
// CodeLevelOutOfRange: the level is outside the event's range.
const (
	CodeModalityMismatch = httpx.CodeModalityMismatch
	CodeLevelOutOfRange  = httpx.CodeLevelOutOfRange
)

// noLevelRange refuses a level declared on an event that has no level range.
const noLevelRange = "must not be sent: this event has no level range"

// checkDeclaredPlayer holds a sign-up to its event before anything is held: an event
// with a modality needs the person's gender, whatever the sport, and the gender must
// fit the modality. Only an event with a level range (padel) also needs the level,
// which must sit inside that range. An event without a modality takes anyone.
func checkDeclaredPlayer(ev *eventstore.Event, in SignUpInput) error {
	if ev.Modality == nil {
		return nil
	}
	errs := map[string]string{}
	if in.Gender == nil {
		errs["gender"] = "must be provided"
	}
	if ev.LevelMin != nil && ev.LevelMax != nil {
		if in.Level == nil {
			errs["level"] = "must be provided"
		}
	} else if in.Level != nil {
		errs["level"] = noLevelRange
	}
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	if !genderFits(*ev.Modality, *in.Gender) {
		return &ValidationError{Errors: map[string]string{"gender": CodeModalityMismatch}}
	}
	if ev.LevelMin != nil && ev.LevelMax != nil && (*in.Level < *ev.LevelMin || *in.Level > *ev.LevelMax) {
		return &ValidationError{Errors: map[string]string{"level": CodeLevelOutOfRange}}
	}
	return nil
}

// genderFits is whether a person of this gender may sign up to an event of this
// modality: a men's event takes men, a women's takes women, a mixed one both.
func genderFits(modality, gender string) bool {
	switch modality {
	case ModalityMen:
		return gender == GenderMen
	case ModalityWomen:
		return gender == GenderWomen
	default:
		return true
	}
}

// SignUpResult is a held spot and the checkout to pay for it.
type SignUpResult struct {
	Registration *eventstore.Registration
	// Token is the person's credential for their spot: the status and cancel
	// routes authorize on it, and the checkout's success redirect carries it.
	Token      string
	Event      *eventstore.Event
	Complex    *complexstore.Complex
	Preference *mp.Preference
	ServiceFee int
	// Total is what the person pays: the price plus the service fee.
	Total int
}

// SignUp holds a spot for a person and opens the MercadoPago checkout that pays
// for it. The spot counts against capacity until the hold ends; the payment
// confirms it (payments.processRegistrationPayment). When MercadoPago will not
// create the checkout the hold is released at once and ErrCheckoutUnavailable is
// returned.
//
// The refusals come before the hold is taken, so a venue that cannot take money
// or a blocked phone never holds a spot.
func (s *Service) SignUp(ctx context.Context, actor Actor, slug string, eventID uuid.UUID, in SignUpInput) (*SignUpResult, error) {
	complex, err := s.publicComplex(ctx, slug)
	if err != nil {
		return nil, err
	}
	ev, err := s.ownedEvent(ctx, complex.ID, eventID)
	if err != nil {
		return nil, err
	}
	return s.SignUpWith(ctx, actor, complex, ev, in, nil)
}

// PublicComplex loads the active complex a public slug names, for the domains
// built on events.
func (s *Service) PublicComplex(ctx context.Context, slug string) (*complexstore.Complex, error) {
	return s.publicComplex(ctx, slug)
}

// SignUpWith is SignUp on an event already resolved. The tournaments domain
// calls it for a category's container with hooks that check and write the pair:
// the hold, the capacity, the checkout and the confirmation are this code's,
// unchanged.
func (s *Service) SignUpWith(ctx context.Context, actor Actor, complex *complexstore.Complex, ev *eventstore.Event, in SignUpInput, hooks *eventstore.SignUpHooks) (*SignUpResult, error) {
	if err := checkDeclaredPlayer(ev, in); err != nil {
		return nil, err
	}
	seller, err := checkout.Seller(complex)
	if err != nil {
		return nil, ErrMercadoPagoNotConnected
	}
	if err := s.refuseBlockedPhone(ctx, complex.ID, in.Phone); err != nil {
		return nil, err
	}

	fee := pricing.ServiceFee(ev.Price)
	var email *string
	if in.Email != "" {
		email = &in.Email
	}
	reg, token, err := s.store.SignUp(ctx, eventstore.SignUpInput{
		EventID: ev.ID, FirstName: in.FirstName, LastName: in.LastName, Phone: in.Phone, Email: email,
		ServiceFee: fee, IP: httpx.RateLimitKey(actor.IP), MaxHoldsPerIP: s.cfg.MaxHoldsPerIP, Hooks: hooks,
		Gender: in.Gender, Level: in.Level,
	})
	if err != nil {
		return nil, signUpError(err)
	}
	s.realtime.PublishBookingChanged(complex.ID)

	pref, err := s.openCheckout(ctx, complex, ev, reg, token, seller)
	if err != nil {
		s.releaseAfterCheckoutFailure(ctx, reg)
		return nil, ErrCheckoutUnavailable
	}
	s.recordCheckoutRow(ctx, reg, pref)
	return &SignUpResult{
		Registration: reg, Token: token, Event: ev, Complex: complex, Preference: pref,
		ServiceFee: fee, Total: reg.Price + reg.ServiceFee,
	}, nil
}

// signUpError turns what the store refused with into the error the handler maps.
func signUpError(err error) error {
	switch {
	case errors.Is(err, eventstore.ErrEventFull):
		return ErrEventFull
	case errors.Is(err, eventstore.ErrEventClosed):
		return ErrEventClosed
	case errors.Is(err, eventstore.ErrAlreadyRegistered):
		return ErrAlreadyRegistered
	case errors.Is(err, eventstore.ErrHoldLimit):
		return ErrTooManyHolds
	}
	return err
}

// refuseBlockedPhone answers ErrClientBlocked for a phone the venue blocked,
// reading it without writing anything; an unknown phone carries on.
func (s *Service) refuseBlockedPhone(ctx context.Context, complexID uuid.UUID, phone string) error {
	known, err := s.clients.GetByPhone(ctx, complexID, phone)
	switch {
	case err == nil && known.IsBlocked:
		return ErrClientBlocked
	case err != nil && !errors.Is(err, data.ErrRecordNotFound):
		return err
	}
	return nil
}

// openCheckout creates the MercadoPago preference that pays for a held spot. The
// checkout expires when the hold does.
func (s *Service) openCheckout(ctx context.Context, complex *complexstore.Complex, ev *eventstore.Event, reg *eventstore.Registration, token string, seller mp.Caller) (*mp.Preference, error) {
	title, description := checkoutCopy(complex, ev)
	in := mp.CreatePreferenceInput{
		RegistrationID: reg.ID,
		Title:          title,
		Description:    description,
		ComplexName:    complex.Name,
		Date:           ev.Date.Format(time.DateOnly),
		StartTime:      ev.StartTime,
		Amount:         reg.Price + reg.ServiceFee,
		MarketplaceFee: reg.ServiceFee,
		Caller:         seller,
		BackURLs:       s.backURLs(complex, ev, token),
		BackendURL:     s.cfg.BackendURL,
		ExpiresIn:      time.Until(reg.HeldUntil),
		PayerFirstName: reg.FirstName,
		PayerLastName:  reg.LastName,
		PayerPhone:     reg.Phone,
	}
	if reg.Email != nil {
		in.PayerEmail = *reg.Email
	}
	pref, err := checkout.CreateWithRetry(ctx, s.checkout, s.complexes, s.logger, in, complex)
	if err != nil {
		s.logger.Error("event sign-up: failed to create the MP preference", "error", err, "registration_id", reg.ID)
	}
	return pref, err
}

// checkoutCopy is the title and description MercadoPago shows for a sign-up.
func checkoutCopy(complex *complexstore.Complex, ev *eventstore.Event) (title, description string) {
	day := shortDay(ev)
	if ev.TournamentID != nil {
		return fmt.Sprintf("%s - %s - desde el %s", complex.Name, ev.Name, day),
			fmt.Sprintf("Inscripción de la pareja a %s en %s, desde el %s.", ev.Name, complex.Name, day)
	}
	return fmt.Sprintf("%s - %s - %s %s hs", complex.Name, ev.Name, day, ev.StartTime),
		fmt.Sprintf("Inscripción a %s en %s. %s a las %s hs.", ev.Name, complex.Name, day, ev.StartTime)
}

// backURLs is where the checkout sends the person back to: the event's page,
// or the tournament's for a category.
func (s *Service) backURLs(complex *complexstore.Complex, ev *eventstore.Event, token string) mp.BackURLs {
	if ev.TournamentID != nil {
		return mp.BackURLs{
			Success: booklink.TournamentSuccess(s.cfg.PublicURL, complex.Slug, *ev.TournamentID, token),
			Failure: booklink.TournamentFailure(s.cfg.PublicURL, complex.Slug, *ev.TournamentID),
			Pending: booklink.TournamentPending(s.cfg.PublicURL, complex.Slug, *ev.TournamentID, token),
		}
	}
	return mp.BackURLs{
		Success: booklink.EventSuccess(s.cfg.PublicURL, complex.Slug, ev.ID, token),
		Failure: booklink.EventFailure(s.cfg.PublicURL, complex.Slug, ev.ID),
		Pending: booklink.EventPending(s.cfg.PublicURL, complex.Slug, ev.ID, token),
	}
}

// shortDay is the event's day as the checkout title shows it ("15 nov"), the form
// MercadoPago's checkout keeps legible.
func shortDay(ev *eventstore.Event) string {
	months := [...]string{"", "ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	return fmt.Sprintf("%d %s", ev.Date.Day(), months[ev.Date.Month()])
}

// releaseAfterCheckoutFailure frees a spot whose checkout could not be created,
// on a context detached from the request: the person may have left while
// MercadoPago was refusing, and a cancelled context would fail the write and keep
// the spot held until its window ends.
func (s *Service) releaseAfterCheckoutFailure(ctx context.Context, reg *eventstore.Registration) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := s.store.CancelRegistration(ctx, reg.ID, eventstore.CancelledByParticipant); err != nil {
		s.logger.Error("event sign-up: failed to release the spot after a checkout failure", "error", err, "registration_id", reg.ID)
	}
}

// recordCheckoutRow writes the payment row the sign-up is waiting on, as the
// public booking flow does. Failing to is logged, not fatal: the webhook
// inserts the row itself when it finds none.
func (s *Service) recordCheckoutRow(ctx context.Context, reg *eventstore.Registration, pref *mp.Preference) {
	payment := &paymentstore.Payment{
		RegistrationID: &reg.ID, ComplexID: reg.ComplexID, Amount: reg.Price, ServiceFee: reg.ServiceFee,
		Method: "mercadopago", Status: "unpaid", MPPreferenceID: &pref.ID,
	}
	if err := s.payments.Insert(ctx, payment); err != nil {
		s.logger.Error("event sign-up: failed to save the payment record", "error", err, "registration_id", reg.ID)
	}
}

// View is a person's own spot with everything the status page renders.
type View struct {
	Registration *eventstore.Registration
	Event        *eventstore.Event
	Complex      *complexstore.Complex
	Cancellation Cancellation
}

// Cancellation is what cancelling would do right now.
type Cancellation struct {
	// CanCancel is false once the registration is over.
	CanCancel bool
	// WithinWindow is whether cancelling now earns the money back, by the
	// complex's own policy: pricing.CanRefund, the same rule a booking follows.
	WithinWindow bool
	// Deadline is the instant WithinWindow stops being true.
	Deadline time.Time
	// Walkover is true for a tournament pair once the draw exists: leaving is a
	// walkover, and the payment is kept.
	Walkover bool
}

// resolve loads the registration a token names with its event and complex. An
// unknown token is not found.
func (s *Service) resolve(ctx context.Context, token string) (*View, error) {
	reg, err := s.store.GetRegistrationByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	ev, err := s.store.GetEvent(ctx, reg.EventID)
	if err != nil {
		return nil, err
	}
	complex, err := s.complexes.GetByID(ctx, reg.ComplexID)
	if err != nil {
		return nil, err
	}
	view := &View{Registration: reg, Event: ev, Complex: complex, Cancellation: s.cancellationOf(reg, ev, complex)}
	if ev.TournamentID != nil && s.tournaments != nil && view.Cancellation.CanCancel {
		drawn, err := s.tournaments.DrawGenerated(ctx, ev.ID)
		if err != nil {
			return nil, err
		}
		if drawn {
			view.Cancellation.Walkover, view.Cancellation.WithinWindow = true, false
		}
	}
	return view, nil
}

// policyBooking is the booking the complex's cancellation policy is asked about:
// pricing.CanRefund reads a booking's start and creation, and a sign-up has the
// same two instants.
func policyBooking(reg *eventstore.Registration, ev *eventstore.Event) *bookingstore.Booking {
	return &bookingstore.Booking{Date: ev.Date, StartTime: ev.StartTime, CreatedAt: reg.CreatedAt}
}

func (s *Service) cancellationOf(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex) Cancellation {
	live := reg.Status == eventstore.RegistrationHeld || reg.Status == eventstore.RegistrationConfirmed
	b := policyBooking(reg, ev)
	return Cancellation{
		CanCancel:    live,
		WithinWindow: live && pricing.CanRefund(b, complex.CancellationHours, s.cfg.GracePeriod),
		Deadline:     pricing.RefundDeadline(b, complex.CancellationHours, s.cfg.GracePeriod),
	}
}

// Status answers a registration link with the spot's current state.
func (s *Service) Status(ctx context.Context, token string) (*View, error) {
	return s.resolve(ctx, token)
}

// CancelResultPublic is a person's cancelled spot and what became of their money.
type CancelResultPublic struct {
	View *View
	// Outcome is the refund: issued, queued, none (nothing was paid) or not
	// eligible (cancelled outside the complex's window).
	Outcome paymentstore.RefundOutcome
}

// CancelRegistration ends a person's spot from their link. The complex's
// cancellation policy decides the money exactly as it does for a booking: within
// the window (or the grace period after signing up) a paid spot is refunded
// through MercadoPago; outside it the spot is released and the payment kept.
func (s *Service) CancelRegistration(ctx context.Context, actor Actor, token string) (*CancelResultPublic, error) {
	view, err := s.resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	reg := view.Registration
	if !view.Cancellation.CanCancel {
		return nil, &StateError{Message: "the registration has already been cancelled"}
	}
	wasPaid := reg.Status == eventstore.RegistrationConfirmed

	// The refund the policy owes is marked in the cancellation's own write, so
	// the refund sweep finishes it if this request never does.
	res, err := s.store.CancelByParticipant(ctx, reg.ID, view.Cancellation.WithinWindow)
	if err != nil {
		if errors.Is(err, eventstore.ErrNotCancellable) {
			return nil, &StateError{Message: "the registration has already been cancelled"}
		}
		return nil, err
	}
	cancelled := res.Registration
	view.Registration = cancelled
	if res.Drawn {
		// The draw was made while this request was on its way: the pair is in the
		// bracket, so leaving is a walkover and the payment is kept.
		view.Cancellation.Walkover, view.Cancellation.WithinWindow = true, false
	}
	s.realtime.PublishBookingChanged(cancelled.ComplexID)
	s.record(actor, cancelled.ComplexID, "public_cancel", "open_play_registration", cancelled.ID, cancelled)

	within := view.Cancellation.WithinWindow
	outcome := paymentstore.RefundOutcome{Result: paymentstore.RefundNone, Reason: "the spot was never paid"}
	switch {
	case within:
		// Asked of the ledger rather than of wasPaid: a payment approved while this
		// request ran is on the ledger now, and refunds only what was collected.
		outcome = s.payments.RefundRegistration(context.WithoutCancel(ctx), cancelled.ID)
	case wasPaid:
		outcome = paymentstore.RefundOutcome{
			Result: paymentstore.RefundNotEligible, AmountCentavos: cancelled.Price + cancelled.ServiceFee,
			Reason: "cancelled outside the complex's refund window",
		}
	}
	if view.Cancellation.Walkover {
		// The pair leaves a drawn bracket: its matches become walkovers. The money
		// is kept and the spot is already ended, so a failure here is logged, not
		// returned: the owner can still load the walkovers by hand.
		if err := s.tournaments.PairWithdrawn(context.WithoutCancel(ctx), cancelled.ID); err != nil {
			s.logger.Error("tournament withdrawal: the pair's matches were not turned into walkovers", "error", err, "registration_id", cancelled.ID)
		}
	}
	s.notifyCancelled(cancelled, view.Event, view.Complex, false, outcome)
	if wasPaid {
		// The owner heard of the paid spot, so they hear it is free again. A hold
		// never paid was never announced to them (S9).
		s.notify.OwnerEventCancelled(notice.Owner(cancelled, view.Event, view.Complex))
	}
	return &CancelResultPublic{View: view, Outcome: outcome}, nil
}

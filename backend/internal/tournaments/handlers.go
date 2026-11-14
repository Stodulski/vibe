package tournaments

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/bookings"
	"github.com/stodulski/vibe-server/internal/events"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/pricing"
	tournamentstore "github.com/stodulski/vibe-server/internal/tournaments/store"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Handler serves the tournament routes. It decodes, validates the request's
// shape, and maps the service's domain errors onto HTTP; every rule lives in the
// Service.
type Handler struct {
	svc            *Service
	respond        *httpx.Refuser
	trustedProxies httpx.TrustedProxies
}

// refusals maps the sentinels the service raises onto HTTP: the sign-up ones are
// the events domain's, so a pair is refused for the same reasons, in the same
// problem kinds, as a person signing up for an event.
var refusals = httpx.Refusals{
	events.ErrMercadoPagoNotConnected: httpx.BadRequest("the complex does not have MercadoPago connected, contact the complex"),
	events.ErrCheckoutUnavailable:     httpx.Unavailable("no se pudo crear el enlace de pago, intente nuevamente"),
	events.ErrClientBlocked:           httpx.Forbidden("your account is blocked, contact the complex for more information"),
	events.ErrEventFull:               httpx.EventFull("the category is full"),
	events.ErrEventClosed:             httpx.EventClosed("sign-up for this category is closed"),
	events.ErrAlreadyRegistered:       httpx.AlreadyRegistered("one of the players is already signed up in this category"),
	events.ErrTooManyHolds:            httpx.TooManyRequests("too many unpaid spots held from this address, pay for one or wait for it to expire"),
}

// NewHandler returns a Handler backed by the given service.
func NewHandler(svc *Service, respond *httpx.Responder, trustedProxies httpx.TrustedProxies) *Handler {
	return &Handler{svc: svc, respond: respond.WithRefusals(refusals), trustedProxies: trustedProxies}
}

func (h *Handler) actor(r *http.Request) Actor {
	var userID *uuid.UUID
	if user, ok := httpx.ContextGetAuthenticatedUser(r); ok {
		userID = &user.ID
	}
	return Actor{UserID: userID, IP: httpx.ClientIPFrom(r, h.trustedProxies)}
}

// refuse answers a service error with the status it earns.
func (h *Handler) refuse(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *ValidationError
	var state *StateError
	var conflict *ConflictError
	switch {
	case errors.As(err, &invalid):
		h.respond.FailedValidation(w, r, invalid.Errors)
	case errors.As(err, &state):
		h.respond.BadRequest(w, r, state)
	case errors.As(err, &conflict):
		h.respond.Refuse(w, r, httpx.Conflict(conflict.Message))
	case errors.Is(err, ErrNoActor):
		h.respond.InvalidAuthenticationToken(w, r)
	default:
		h.respond.DomainError(w, r, err)
	}
}

// ids reads the named UUID path parameters, answering 404 for a malformed one.
func (h *Handler) ids(w http.ResponseWriter, r *http.Request, names ...string) ([]uuid.UUID, bool) {
	out := make([]uuid.UUID, len(names))
	for i, name := range names {
		id, err := httpx.ReadUUIDParam(r, name)
		if err != nil {
			h.respond.NotFound(w, r)
			return nil, false
		}
		out[i] = id
	}
	return out, true
}

func (h *Handler) complex(w http.ResponseWriter, r *http.Request) (*complexOnly, bool) {
	c, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return nil, false
	}
	return &complexOnly{c}, true
}

// courtNames reads the names of the complex's courts, for the schedule.
func (h *Handler) courtNames(ctx context.Context, complexID uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	courts, err := h.svc.courts.GetByComplex(ctx, complexID)
	if err != nil {
		h.svc.logger.Error("tournaments: cannot read the court names", "error", err, "complex_id", complexID)
		return out
	}
	for _, c := range courts {
		out[c.ID] = c.Name
	}
	return out
}

// ownerDetail is a tournament with everything the owner sees.
func (h *Handler) ownerDetail(ctx context.Context, complexID uuid.UUID, d *tournamentstore.Detail) wireDetail {
	pairs := make([]wireOwnerPair, len(d.Pairs))
	for i, p := range d.Pairs {
		pairs[i] = toWireOwnerPair(p)
	}
	return wireDetail{
		Tournament: toWireTournament(&d.Tournament), Categories: toWireCategories(d.Categories, nil), Pairs: pairs,
		Groups: toWireGroups(d), Matches: toWireMatches(d.Matches, h.courtNames(ctx, complexID)),
	}
}

// publicDetail is a tournament as a visitor sees it: the pairs that paid, without
// phones, emails or payments.
func (h *Handler) publicDetail(ctx context.Context, complexID uuid.UUID, d *tournamentstore.Detail) wireDetail {
	pairs := []wirePair{}
	for _, p := range d.Pairs {
		if p.RegistrationStatus == eventsConfirmed {
			pairs = append(pairs, toWirePair(p))
		}
	}
	fee := func(price int) *int { f := pricing.ServiceFee(price); return &f }
	return wireDetail{
		Tournament: toWireTournament(&d.Tournament), Categories: toWireCategories(d.Categories, fee), Pairs: pairs,
		Groups: toWireGroups(d), Matches: toWireMatches(d.Matches, h.courtNames(ctx, complexID)),
	}
}

// eventsConfirmed is the registration status of a paid spot.
const eventsConfirmed = "confirmed"

// Create handles POST /api/v1/complexes/{id}/tournaments.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	var input struct {
		Name       string `json:"name"`
		Sport      string `json:"sport"`
		StartDate  string `json:"start_date"`
		EndDate    string `json:"end_date"`
		Categories []struct {
			Name               string `json:"name"`
			Format             string `json:"format"`
			Capacity           int    `json:"capacity"`
			Price              int    `json:"price"`
			GroupSize          int    `json:"group_size"`
			QualifiersPerGroup int    `json:"qualifiers_per_group"`
		} `json:"categories"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	in := CreateInput{Name: input.Name, Sport: input.Sport}
	errs := map[string]string{}
	var err error
	if in.StartDate, err = time.Parse(time.DateOnly, input.StartDate); err != nil {
		errs["start_date"] = "must be in YYYY-MM-DD format"
	}
	if in.EndDate, err = time.Parse(time.DateOnly, input.EndDate); err != nil {
		errs["end_date"] = "must be in YYYY-MM-DD format"
	}
	if len(errs) > 0 {
		h.respond.FailedValidation(w, r, errs)
		return
	}
	for _, c := range input.Categories {
		in.Categories = append(in.Categories, CategoryInput(c))
	}
	detail, err := h.svc.Create(r.Context(), complex.Complex, h.actor(r), in)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusCreated, envelopeOf(h.ownerDetail(r.Context(), complex.ID, detail)))
}

// List handles GET /api/v1/complexes/{id}/tournaments.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	from, to, err := events.ParseDateRange(r.URL.Query())
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	sums, err := h.svc.List(r.Context(), complex.ID, from, to)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	out := make([]wireSummary, len(sums))
	for i, s := range sums {
		out[i] = wireSummary{toWireTournament(s.Tournament), toWireCategories(s.Categories, nil)}
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"tournaments": out})
}

// Get handles GET /api/v1/complexes/{id}/tournaments/{tournamentID}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID")
	if !ok {
		return
	}
	d, err := h.svc.Get(r.Context(), complex.ID, ids[0])
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, envelopeOf(h.ownerDetail(r.Context(), complex.ID, d)))
}

// readReason reads the optional cancellation reason.
func (h *Handler) readReason(w http.ResponseWriter, r *http.Request) (*string, bool) {
	var input struct {
		Reason *string `json:"reason"`
	}
	if r.ContentLength != 0 {
		if err := httpx.ReadJSON(w, r, &input); err != nil {
			h.respond.BadRequest(w, r, err)
			return nil, false
		}
	}
	v := validator.New()
	if input.Reason != nil {
		v.Check(len([]rune(*input.Reason)) <= 500, "reason", "must be at most 500 characters")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return nil, false
	}
	return input.Reason, true
}

func refundsBody(refunds []Refund) []httpx.Envelope {
	out := make([]httpx.Envelope, len(refunds))
	for i, ref := range refunds {
		out[i] = httpx.Envelope{"registration_id": ref.RegistrationID, "refund": bookings.RefundEnvelope(ref.Outcome)}
	}
	return out
}

// Cancel handles POST /api/v1/complexes/{id}/tournaments/{tournamentID}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID")
	if !ok {
		return
	}
	reason, ok := h.readReason(w, r)
	if !ok {
		return
	}
	res, err := h.svc.Cancel(r.Context(), complex.Complex, h.actor(r), ids[0], reason)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"tournament": toWireTournament(res.Tournament), "refunds": refundsBody(res.Refunds)})
}

// CancelCategory handles POST /api/v1/complexes/{id}/tournaments/{tournamentID}/categories/{categoryID}/cancel.
func (h *Handler) CancelCategory(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID", "categoryID")
	if !ok {
		return
	}
	reason, ok := h.readReason(w, r)
	if !ok {
		return
	}
	res, err := h.svc.CancelCategory(r.Context(), complex.Complex, h.actor(r), ids[0], ids[1], reason)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"refunds": refundsBody(res.Refunds)})
}

// GenerateDraw handles POST /api/v1/complexes/{id}/tournaments/{tournamentID}/categories/{categoryID}/draw.
func (h *Handler) GenerateDraw(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID", "categoryID")
	if !ok {
		return
	}
	var input struct {
		SeedOrder []uuid.UUID `json:"seed_order"`
	}
	if r.ContentLength != 0 {
		if err := httpx.ReadJSON(w, r, &input); err != nil {
			h.respond.BadRequest(w, r, err)
			return
		}
	}
	d, err := h.svc.GenerateDraw(r.Context(), complex.Complex, h.actor(r), ids[0], ids[1], DrawInput{SeedOrder: input.SeedOrder})
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, envelopeOf(h.ownerDetail(r.Context(), complex.ID, d)))
}

// MovePair handles POST /api/v1/complexes/{id}/tournaments/{tournamentID}/categories/{categoryID}/draw/move.
func (h *Handler) MovePair(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID", "categoryID")
	if !ok {
		return
	}
	var input struct {
		PairID     uuid.UUID `json:"pair_id"`
		WithPairID uuid.UUID `json:"with_pair_id"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	d, err := h.svc.MovePair(r.Context(), complex.Complex, h.actor(r), ids[0], ids[1], input.PairID, input.WithPairID)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, envelopeOf(h.ownerDetail(r.Context(), complex.ID, d)))
}

// ScheduleMatch handles PUT /api/v1/complexes/{id}/tournaments/{tournamentID}/matches/{matchID}/schedule.
func (h *Handler) ScheduleMatch(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID", "matchID")
	if !ok {
		return
	}
	var input struct {
		CourtID         uuid.UUID `json:"court_id"`
		Date            string    `json:"date"`
		StartTime       string    `json:"start_time"`
		DurationMinutes int       `json:"duration_minutes"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	date, err := time.Parse(time.DateOnly, input.Date)
	if err != nil {
		h.respond.FailedValidation(w, r, map[string]string{"date": "must be in YYYY-MM-DD format"})
		return
	}
	m, err := h.svc.Schedule(r.Context(), complex.Complex, h.actor(r), ids[0], ids[1], ScheduleInput{
		CourtID: input.CourtID, Date: date, StartTime: input.StartTime, Minutes: input.DurationMinutes,
	})
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"match": toWireMatch(m, h.courtNames(r.Context(), complex.ID))})
}

// UnscheduleMatch handles DELETE /api/v1/complexes/{id}/tournaments/{tournamentID}/matches/{matchID}/schedule.
func (h *Handler) UnscheduleMatch(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID", "matchID")
	if !ok {
		return
	}
	m, err := h.svc.Unschedule(r.Context(), complex.Complex, h.actor(r), ids[0], ids[1])
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"match": toWireMatch(m, h.courtNames(r.Context(), complex.ID))})
}

// SetResult handles PUT /api/v1/complexes/{id}/tournaments/{tournamentID}/matches/{matchID}/result.
func (h *Handler) SetResult(w http.ResponseWriter, r *http.Request) {
	complex, ok := h.complex(w, r)
	if !ok {
		return
	}
	ids, ok := h.ids(w, r, "tournamentID", "matchID")
	if !ok {
		return
	}
	var input struct {
		Sets []struct {
			A     int  `json:"a"`
			B     int  `json:"b"`
			Super bool `json:"super"`
		} `json:"sets"`
		WalkoverLoser int `json:"walkover_loser"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	in := ResultInput{WalkoverLoser: input.WalkoverLoser}
	for _, s := range input.Sets {
		in.Sets = append(in.Sets, drawSet(s))
	}
	d, err := h.svc.SetResult(r.Context(), complex.Complex, h.actor(r), ids[0], ids[1], in)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, envelopeOf(h.ownerDetail(r.Context(), complex.ID, d)))
}

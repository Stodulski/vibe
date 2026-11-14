package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/bookings"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/validator"
)

// Handler serves the open-play event routes. It decodes, validates the request's
// shape, and maps the service's domain errors onto HTTP; every rule lives in the
// Service.
type Handler struct {
	svc            *Service
	respond        *httpx.Refuser
	trustedProxies httpx.TrustedProxies
}

// refusals maps the sentinels the service raises onto HTTP.
var refusals = httpx.Refusals{
	ErrMercadoPagoNotConnected: httpx.BadRequest("the complex does not have MercadoPago connected, contact the complex"),
	ErrCheckoutUnavailable:     httpx.Unavailable("no se pudo crear el enlace de pago, intente nuevamente"),
	ErrClientBlocked:           httpx.Forbidden("your account is blocked, contact the complex for more information"),
	ErrEventFull:               httpx.EventFull("the event is full"),
	ErrEventClosed:             httpx.EventClosed("sign-up for this event is closed"),
	ErrAlreadyRegistered:       httpx.AlreadyRegistered("this phone is already signed up for the event"),
	ErrTooManyHolds:            httpx.TooManyRequests("too many unpaid spots held from this address, pay for one or wait for it to expire"),
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

// Create handles POST /api/v1/complexes/{id}/open-play-events.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	var input gen.EventsCreateJSONBody
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	in := CreateInput{
		Name: input.Name, Date: input.Date.Time, StartTime: input.StartTime, EndTime: input.EndTime,
		Capacity: input.Capacity, Price: input.Price,
	}
	if input.Sport != nil {
		in.Sport = string(*input.Sport)
	}
	if input.CourtIds != nil {
		in.CourtIDs = *input.CourtIds
	}

	ev, err := h.svc.Create(r.Context(), complex, h.actor(r), in)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"event": toGenEvent(ev)})
}

// List handles GET /api/v1/complexes/{id}/open-play-events.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	from, to, err := parseDateRange(r.URL.Query())
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	events, err := h.svc.List(r.Context(), complex.ID, from, to)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"events": toGenEvents(events)})
}

// parseDateRange reads date_from and date_to, both required, at most 366 days
// apart: the same range rule the blocked-slot calendar applies.
func parseDateRange(qs url.Values) (from, to time.Time, err error) {
	fromStr, toStr := httpx.ReadString(qs, "date_from", ""), httpx.ReadString(qs, "date_to", "")
	if fromStr == "" || toStr == "" {
		return from, to, fmt.Errorf("date_from and date_to query parameters are required")
	}
	if from, err = time.Parse(time.DateOnly, fromStr); err != nil {
		return from, to, fmt.Errorf("date_from must be in YYYY-MM-DD format")
	}
	if to, err = time.Parse(time.DateOnly, toStr); err != nil {
		return from, to, fmt.Errorf("date_to must be in YYYY-MM-DD format")
	}
	if to.Before(from) {
		return from, to, fmt.Errorf("date_to must be on or after date_from")
	}
	if to.Sub(from).Hours() > 366*24 {
		return from, to, fmt.Errorf("date range must not exceed 366 days")
	}
	return from, to, nil
}

// Get handles GET /api/v1/complexes/{id}/open-play-events/{eventID}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	eventID, err := httpx.ReadUUIDParam(r, "eventID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}
	detail, err := h.svc.Get(r.Context(), complex.ID, eventID)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"event":        toGenEvent(detail.Event),
		"participants": toGenParticipants(detail.Participants),
	})
}

// Cancel handles POST /api/v1/complexes/{id}/open-play-events/{eventID}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	eventID, err := httpx.ReadUUIDParam(r, "eventID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}
	var input struct {
		Reason *string `json:"reason"`
	}
	if r.ContentLength != 0 {
		if err := httpx.ReadJSON(w, r, &input); err != nil {
			h.respond.BadRequest(w, r, err)
			return
		}
	}
	v := validator.New()
	if input.Reason != nil {
		v.Check(len([]rune(*input.Reason)) <= 500, "reason", "must be at most 500 characters")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	result, err := h.svc.Cancel(r.Context(), complex, h.actor(r), eventID, input.Reason)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	refunds := make([]httpx.Envelope, len(result.Refunds))
	for i, ref := range result.Refunds {
		refunds[i] = httpx.Envelope{"registration_id": ref.RegistrationID, "refund": bookings.RefundEnvelope(ref.Outcome)}
	}
	// Cancelled events carry no spot counts that matter, but the read stays honest.
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"event": toGenEvent(result.Event), "refunds": refunds})
}

// PublicList handles GET /api/v1/public/complexes/{slug}/events.
func (h *Handler) PublicList(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.PublicList(r.Context(), httpx.ReadStringParam(r, "slug"))
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"events": toGenPublicEvents(page)})
}

// PublicGet handles GET /api/v1/public/complexes/{slug}/events/{eventID}.
func (h *Handler) PublicGet(w http.ResponseWriter, r *http.Request) {
	eventID, err := httpx.ReadUUIDParam(r, "eventID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}
	page, err := h.svc.PublicGet(r.Context(), httpx.ReadStringParam(r, "slug"), eventID)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"event": toGenPublicEvent(page.Events[0], page)})
}

// PublicSignUp handles POST /api/v1/public/complexes/{slug}/events/{eventID}/registrations.
func (h *Handler) PublicSignUp(w http.ResponseWriter, r *http.Request) {
	eventID, err := httpx.ReadUUIDParam(r, "eventID")
	if err != nil {
		h.respond.NotFound(w, r)
		return
	}
	var input gen.EventsPublicSignUpJSONBody
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	in, errs := signUpInputFrom(input)
	if len(errs) > 0 {
		h.respond.FailedValidation(w, r, errs)
		return
	}

	result, err := h.svc.SignUp(r.Context(), h.actor(r), httpx.ReadStringParam(r, "slug"), eventID, in)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	page := &Page{Complex: result.Complex, ServiceFee: result.ServiceFee}
	out := gen.PublicEventSignUp{
		Token: result.Token, Event: toGenPublicEvent(result.Event, page),
		MpInitPoint: &result.Preference.InitPoint, MpPreferenceId: &result.Preference.ID,
		ServiceFee: result.ServiceFee, TotalPrice: result.Total,
	}
	out.Registration.HeldUntil = result.Registration.HeldUntil
	out.Registration.Status = gen.PublicEventSignUpRegistrationStatus(result.Registration.Status)
	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"registration": out.Registration, "token": out.Token, "event": out.Event,
		"mp_init_point": out.MpInitPoint, "mp_preference_id": out.MpPreferenceId,
		"service_fee": out.ServiceFee, "total_price": out.TotalPrice,
	})
}

// envelopeOf is a generated wire struct as the top-level object of a response.
// The struct's own tags decide the JSON; this only carries it through the
// Responder, whose bodies are maps.
func envelopeOf(v any) httpx.Envelope {
	encoded, err := json.Marshal(v)
	if err != nil {
		return httpx.Envelope{}
	}
	out := httpx.Envelope{}
	if err := json.Unmarshal(encoded, &out); err != nil {
		return httpx.Envelope{}
	}
	return out
}

// signUpInputFrom validates a person's details: name and phone are required, the
// phone is normalized to E.164, and the email, when given, must look like one.
func signUpInputFrom(body gen.EventsPublicSignUpJSONBody) (SignUpInput, map[string]string) {
	v := validator.New()
	in := SignUpInput{FirstName: body.FirstName, LastName: body.LastName, Phone: body.Phone}
	v.Check(in.FirstName != "", "first_name", "must be provided")
	v.Check(in.LastName != "", "last_name", "must be provided")
	v.Check(len([]rune(in.FirstName)) <= 100 && len([]rune(in.LastName)) <= 100, "first_name", "is too long")
	v.Check(in.Phone != "", "phone", "must be provided")
	if in.Phone != "" {
		if normalized, err := validator.NormalizePhone(in.Phone); err != nil {
			v.AddError("phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			in.Phone = normalized
		}
	}
	if body.Email != nil && *body.Email != "" {
		in.Email = *body.Email
		v.Check(validator.Matches(in.Email, validator.EmailRX), "email", "must be a valid email address")
	}
	return in, v.Errors
}

// PublicRegistrationStatus handles GET /api/v1/public/event-registrations/status.
func (h *Handler) PublicRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	token := httpx.ReadString(r.URL.Query(), "token", "")
	if token == "" {
		h.respond.BadRequest(w, r, errors.New("token query parameter is required"))
		return
	}
	view, err := h.svc.Status(r.Context(), token)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, envelopeOf(toGenPublicRegistration(view, pricing.ServiceFee(view.Event.Price))))
}

// PublicRegistrationCancel handles POST /api/v1/public/event-registrations/cancel.
func (h *Handler) PublicRegistrationCancel(w http.ResponseWriter, r *http.Request) {
	var input gen.EventsPublicRegistrationCancelJSONBody
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	v := validator.New()
	v.Check(input.Token != "", "token", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}
	result, err := h.svc.CancelRegistration(r.Context(), h.actor(r), input.Token)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"registration": toGenPublicRegistration(result.View, pricing.ServiceFee(result.View.Event.Price)),
		"refunded":     result.Outcome.MoneyReturned(),
		"refund":       bookings.RefundEnvelope(result.Outcome),
	})
}

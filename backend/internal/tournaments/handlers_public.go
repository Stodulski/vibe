package tournaments

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/stodulski/vibe-server/internal/bookings"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/events"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/tournaments/draw"
	"github.com/stodulski/vibe-server/internal/validator"
)

// complexOnly names the complex an owner route runs for.
type complexOnly struct{ *complexstore.Complex }

func drawSet(s struct {
	A     int  `json:"a"`
	B     int  `json:"b"`
	Super bool `json:"super"`
}) draw.Set {
	return draw.Set{A: s.A, B: s.B, Super: s.Super}
}

// envelopeOf is a wire struct as the top-level object of a response. The
// struct's own tags decide the JSON; this only carries it through the Responder,
// whose bodies are maps.
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

// PublicList handles GET /api/v1/public/complexes/{slug}/tournaments.
func (h *Handler) PublicList(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.PublicList(r.Context(), httpx.ReadStringParam(r, "slug"))
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	fee := func(price int) *int { f := pricing.ServiceFee(price); return &f }
	out := make([]wireSummary, len(page.Tournaments))
	for i, s := range page.Tournaments {
		out[i] = wireSummary{toWireTournament(s.Tournament), toWireCategories(s.Categories, fee)}
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"tournaments": out})
}

// PublicGet handles GET /api/v1/public/complexes/{slug}/tournaments/{tournamentID}.
func (h *Handler) PublicGet(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "tournamentID")
	if !ok {
		return
	}
	complex, d, err := h.svc.PublicGet(r.Context(), httpx.ReadStringParam(r, "slug"), ids[0])
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	body := envelopeOf(h.publicDetail(r.Context(), complex.ID, d))
	body["complex_name"] = complex.Name
	body["cancellation_hours"] = complex.CancellationHours
	h.respond.JSON(w, r, http.StatusOK, body)
}

type signUpPlayer struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Phone     string  `json:"phone"`
	Email     *string `json:"email"`
}

// playerInput validates one player of a pair: name and phone are required, the
// phone is normalized to E.164, and the email, when given, must look like one.
func playerInput(v *validator.Validator, prefix string, p signUpPlayer) events.SignUpInput {
	in := events.SignUpInput{FirstName: p.FirstName, LastName: p.LastName, Phone: p.Phone}
	v.Check(in.FirstName != "", prefix+"first_name", "must be provided")
	v.Check(in.LastName != "", prefix+"last_name", "must be provided")
	v.Check(len([]rune(in.FirstName)) <= 100 && len([]rune(in.LastName)) <= 100, prefix+"first_name", "is too long")
	v.Check(in.Phone != "", prefix+"phone", "must be provided")
	if in.Phone != "" {
		if normalized, err := validator.NormalizePhone(in.Phone); err != nil {
			v.AddError(prefix+"phone", "must be a valid phone number (E.164 format, e.g. +5491112345678)")
		} else {
			in.Phone = normalized
		}
	}
	if p.Email != nil && *p.Email != "" {
		in.Email = *p.Email
		v.Check(validator.Matches(in.Email, validator.EmailRX), prefix+"email", "must be a valid email address")
	}
	return in
}

// PublicSignUp handles POST /api/v1/public/complexes/{slug}/tournaments/{tournamentID}/categories/{categoryID}/pairs.
func (h *Handler) PublicSignUp(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "tournamentID", "categoryID")
	if !ok {
		return
	}
	var input struct {
		signUpPlayer
		Partner signUpPlayer `json:"partner"`
	}
	if err := httpx.ReadJSON(w, r, &input); err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	v := validator.New()
	in := PairSignUpInput{Payer: playerInput(v, "", input.signUpPlayer), Partner: playerInput(v, "partner.", input.Partner)}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}
	res, err := h.svc.SignUp(r.Context(), h.actor(r), httpx.ReadStringParam(r, "slug"), ids[0], ids[1], in)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	fee := res.ServiceFee
	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{
		"registration":     httpx.Envelope{"status": res.Registration.Status, "held_until": res.Registration.HeldUntil},
		"token":            res.Token,
		"tournament":       toWireTournament(res.Tournament),
		"category":         toWireCategory(res.Category, &fee),
		"mp_init_point":    res.Preference.InitPoint,
		"mp_preference_id": res.Preference.ID,
		"service_fee":      res.ServiceFee,
		"total_price":      res.Total,
	})
}

// pairViewBody is a pair's own spot as the status page renders it.
func pairViewBody(v *PairView) httpx.Envelope {
	fee := v.ServiceFee
	return httpx.Envelope{
		"status":      v.Registration.Status,
		"pair_status": v.Pair.Status,
		"held_until":  v.Registration.HeldUntil,
		"payer":       wirePlayer{FirstName: v.Registration.FirstName, LastName: v.Registration.LastName},
		"partner":     wirePlayer{FirstName: v.Pair.Partner.FirstName, LastName: v.Pair.Partner.LastName},
		"price":       v.Registration.Price, "service_fee": v.Registration.ServiceFee,
		"tournament":      toWireTournament(v.Tournament),
		"category":        toWireCategory(v.Category, &fee),
		"complex_name":    v.Complex.Name,
		"complex_address": v.Complex.Address,
		"cancellation": httpx.Envelope{
			"can_cancel": v.Cancellation.CanCancel, "within_window": v.Cancellation.WithinWindow,
			"deadline": v.Cancellation.Deadline, "walkover": v.Cancellation.Walkover,
		},
	}
}

// PublicRegistrationStatus handles GET /api/v1/public/tournament-registrations/status.
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
	h.respond.JSON(w, r, http.StatusOK, pairViewBody(view))
}

// PublicRegistrationCancel handles POST /api/v1/public/tournament-registrations/cancel.
func (h *Handler) PublicRegistrationCancel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
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
	res, err := h.svc.CancelRegistration(r.Context(), h.actor(r), input.Token)
	if err != nil {
		h.refuse(w, r, err)
		return
	}
	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"registration": pairViewBody(res.View),
		"refunded":     res.Outcome.MoneyReturned(),
		"refund":       bookings.RefundEnvelope(res.Outcome),
	})
}

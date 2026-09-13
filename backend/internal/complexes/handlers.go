package complexes

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/validator"
)

var slugValidRX = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// List handles GET /api/v1/complexes, returning every venue this account
// owns.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	complexes, err := h.svc.List(r.Context(), user.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"complexes": toComplexResponses(complexes)})
}

// Create handles POST /api/v1/complexes.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input gen.ComplexesCreateJSONBody

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	input.Slug = slugify(input.Slug)

	depositPercentage := 0
	if input.DepositPercentage != nil {
		depositPercentage = *input.DepositPercentage
	}
	email := input.Email

	v := validator.New()
	v.Check(input.Name != "", "name", "must be provided")
	v.Check(len(input.Name) <= 200, "name", "must not be more than 200 characters")
	v.Check(input.Slug != "", "slug", "must be provided")
	v.Check(slugValidRX.MatchString(input.Slug), "slug", "must contain only lowercase letters, numbers, and hyphens")
	v.Check(len(input.Slug) <= maxSlugLength, "slug", fmt.Sprintf("must not be more than %d characters", maxSlugLength))
	// See reservedSlugs: these are frontend's own top-level routes, and a
	// complex created with one as its slug is unreachable at its own URL
	// forever, because the client's static page always wins the match.
	v.Check(!reservedSlugs[input.Slug], "slug", "is reserved")
	v.Check(input.Address != "", "address", "must be provided")
	v.Check(input.City != "", "city", "must be provided")
	v.Check(input.Province != "", "province", "must be provided")
	v.Check(input.Phone != "", "phone", "must be provided")
	v.Check(depositPercentage >= 0, "deposit_percentage", "must be 0 or greater")
	v.Check(depositPercentage <= 100, "deposit_percentage", "must not be more than 100")
	// The floor is 1, not 0. A zero window is read by pricing.CanRefund as
	// "refund always due" — its cancellationHours <= 0 branch returns true
	// unconditionally — and pricing.LinkLive ORs that answer in, so a complex
	// at zero hands out booking links that never expire. complexes_cancellation_hours_range
	// refuses the same value at the column, so this check is the message and
	// that one is the guarantee.
	v.Check(input.CancellationHours >= 1, "cancellation_hours", "must be at least 1")
	v.Check(input.CancellationHours <= 168, "cancellation_hours", "must not be more than 168")
	if email != nil {
		v.Check(validator.Matches(*email, validator.EmailRX), "email", "must be a valid email address")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	complex, err := h.svc.Create(r.Context(), user.ID, h.actor(r), CreateInput{
		Name:              input.Name,
		Slug:              input.Slug,
		Address:           input.Address,
		City:              input.City,
		Province:          input.Province,
		Phone:             input.Phone,
		Email:             email,
		DepositPercentage: depositPercentage,
		CancellationHours: input.CancellationHours,
		Latitude:          input.Latitude,
		Longitude:         input.Longitude,
	})
	if err != nil {
		if errors.Is(err, ErrSlugTaken) {
			h.respond.FailedValidation(w, r, map[string]string{"slug": httpx.CodeSlugTaken})
			return
		}
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"complex": toComplexResponse(complex)})
}

// Get handles GET /api/v1/complexes/{id}, returning one venue with its
// schedules.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"complex": toComplexResponse(complex)})
}

// SlugAvailable handles GET /api/v1/complexes/slug-available?slug=x.
//
// Answers the one question the client cannot: is this public URL free. Two
// owners can pick the same name a second apart and nothing in a browser knows
// it; without this the only way to find out is to fill the whole form and be
// refused on submit.
//
// It exists as its own route because the obvious shortcut is wrong. Probing
// GET /public/complexes/{slug} looks equivalent and is not: that endpoint 404s
// for a DEACTIVATED venue, so it would report "free" for a name the database
// will refuse — a check that lies in exactly the case it was added for.
//
// Behind RequireAuth. The answer is public information (the slug either serves
// a page or it does not), but only a signed-in owner creating or renaming a
// complex has any use for it, and an open endpoint invites enumeration.
func (h *Handler) SlugAvailable(w http.ResponseWriter, r *http.Request) {
	status, err := h.svc.SlugAvailable(r.Context(), r.URL.Query().Get("slug"))
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	result := toGenSlugAvailability(status)

	if !result.Valid {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"slug": result.Slug, "available": false, "valid": false})
		return
	}

	body := httpx.Envelope{"slug": result.Slug, "valid": true, "available": result.Available}
	if result.Suggestion != nil {
		body["suggestion"] = *result.Suggestion
	}

	h.respond.JSON(w, r, http.StatusOK, body)
}

// suggestSlug returns the first free "base-N", or "" if none is within reach.
//
// Starts at 2, not 1: the first duplicate of a name is the second club to want
// it, and "club-norte-1" implies a "club-norte-0" that does not exist.
//
// Bounded because this runs on user input. Fifty clubs sharing one name is not
// a case worth serving; past it the owner types their own, which the field
// already lets them do.
func suggestSlug(base string, taken map[string]bool) string {
	for n := 2; n <= 50; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !taken[candidate] {
			return candidate
		}
	}
	return ""
}

// Update handles PUT /api/v1/complexes/{id}. Every field is optional; an
// omitted one keeps its current value.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen,gocyclo,gocognit // see the cohesion note above
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input gen.ComplexesUpdateJSONBody

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	expectedVersion, err := httpx.ExpectedVersion(r, input.Version)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	in := UpdateInput{
		ExpectedVersion:   expectedVersion,
		Name:              input.Name,
		Address:           input.Address,
		City:              input.City,
		Province:          input.Province,
		Phone:             input.Phone,
		Email:             input.Email,
		LogoURL:           input.LogoUrl,
		CoverURL:          input.CoverUrl,
		DepositPercentage: input.DepositPercentage,
		CancellationHours: input.CancellationHours,
		IsActive:          input.IsActive,
		Latitude:          input.Latitude,
		Longitude:         input.Longitude,
	}

	v := validator.New()

	if input.Name != nil {
		v.Check(*input.Name != "", "name", "must not be empty")
		v.Check(len(*input.Name) <= 200, "name", "must not be more than 200 characters")
	}

	// The public URL can be changed. It is the address already living in shared
	// links, WhatsApp messages and printed QR codes, so the client warns before
	// letting anyone touch it — but the decision is the owner's, and refusing it
	// outright left a badly chosen name permanent.
	if input.Slug != nil {
		slug := slugify(*input.Slug)
		v.Check(slug != "", "slug", "must be provided")
		v.Check(slugValidRX.MatchString(slug), "slug", "must contain only lowercase letters, numbers, and hyphens")
		v.Check(len(slug) <= maxSlugLength, "slug", fmt.Sprintf("must not be more than %d characters", maxSlugLength))
		// See reservedSlugs on Create: the same route collision applies to a
		// rename, and a rename is exactly how an owner could talk themselves
		// into it — Create already refuses these, but Update never re-checked.
		v.Check(!reservedSlugs[slug], "slug", "is reserved")
		in.Slug = &slug
	}

	if input.Address != nil {
		v.Check(*input.Address != "", "address", "must not be empty")
	}
	if input.City != nil {
		v.Check(*input.City != "", "city", "must not be empty")
	}
	if input.Province != nil {
		v.Check(*input.Province != "", "province", "must not be empty")
	}
	if input.Amenities != nil {
		amenities := make([]string, len(*input.Amenities))
		for i, a := range *input.Amenities {
			amenities[i] = string(a)
		}
		cleaned, unknown := cleanAmenities(amenities)
		v.Check(unknown == "", "amenities", "unknown amenity: "+unknown)
		in.Amenities = &cleaned
	}
	if input.Phone != nil {
		v.Check(*input.Phone != "", "phone", "must not be empty")
	}
	if input.Email != nil {
		v.Check(validator.Matches(*input.Email, validator.EmailRX), "email", "must be a valid email address")
	}
	if input.LogoUrl != nil && *input.LogoUrl != "" {
		v.Check(isValidImageURL(*input.LogoUrl), "logo_url", "must be a valid HTTP/HTTPS URL")
	}
	if input.CoverUrl != nil && *input.CoverUrl != "" {
		v.Check(isValidImageURL(*input.CoverUrl), "cover_url", "must be a valid HTTP/HTTPS URL")
	}
	if input.DepositPercentage != nil {
		v.Check(*input.DepositPercentage >= 0, "deposit_percentage", "must be 0 or greater")
		v.Check(*input.DepositPercentage <= 100, "deposit_percentage", "must not be more than 100")
	}
	if input.CancellationHours != nil {
		// Same floor as Create, and for the same reason: see the note there.
		v.Check(*input.CancellationHours >= 1, "cancellation_hours", "must be at least 1")
		v.Check(*input.CancellationHours <= 168, "cancellation_hours", "must not be more than 168")
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	updated, err := h.svc.Update(r.Context(), complex, h.actor(r), in)
	if err != nil {
		switch {
		case errors.Is(err, ErrSlugTaken):
			h.respond.FailedValidation(w, r, map[string]string{"slug": httpx.CodeSlugTaken})
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.EditConflict(w, r)
		case errors.Is(err, data.ErrEditConflict):
			// A stale If-Match/version: the row is still there, just not at
			// the version this write named. Its own kind, distinct from the
			// generic conflict above.
			h.respond.StaleVersion(w, r)
		default:
			h.respond.DomainError(w, r, err)
		}
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"complex": toComplexResponse(updated)})
}

// deletionOutcome is what a delete did beyond the venue row itself, recorded in
// the audit trail and returned to the caller. Today that is the court count:
// deleting a venue takes its courts offline, and an owner who is told only
// "complex deleted" has no way to know how much went with it.
type deletionOutcome struct {
	CourtsDeactivated int `json:"courts_deactivated"`
}

// Delete handles DELETE /api/v1/complexes/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	courtsDeactivated, err := h.svc.Delete(r.Context(), complex, h.actor(r))
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"message":            "complex deleted",
		"courts_deactivated": courtsDeactivated,
	})
}

// PublicComplex is what a stranger is allowed to know about a venue.
//
// The public page used to serialise the whole complexstore.Complex, which carries
// owner_id and mp_user_id. mp_user_id is the MercadoPago collector id the
// payment path compares an incoming payment against to decide the money
// reached the right seller; publishing it on an unauthenticated endpoint hands
// that check's expected value to anyone who asks. owner_id names a platform
// account, which a booking decision never needs.
//
// This is an explicit projection rather than json:"-" on the domain struct so
// that the owner-facing endpoints, which legitimately return both, are
// unaffected.
type PublicComplex struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	Address           string    `json:"address"`
	City              string    `json:"city"`
	Province          string    `json:"province"`
	CountryCode       string    `json:"country_code"`
	Currency          string    `json:"currency"`
	Phone             string    `json:"phone"`
	Email             *string   `json:"email,omitempty"`
	LogoURL           *string   `json:"logo_url,omitempty"`
	CoverURL          *string   `json:"cover_url,omitempty"`
	DepositPercentage int       `json:"deposit_percentage"`
	CancellationHours int       `json:"cancellation_hours"`
	Latitude          *float64  `json:"latitude,omitempty"`
	Longitude         *float64  `json:"longitude,omitempty"`
	Amenities         []string  `json:"amenities"`
	// PaymentsEnabled replaces the leaked mp_user_id with the only thing a
	// client actually needed from it: whether this venue can take a payment at
	// all. A booking attempt against a venue with no MercadoPago connection is
	// refused, and the page can say so before the client fills in a form.
	PaymentsEnabled bool `json:"payments_enabled"`
}

func newPublicComplex(c *complexstore.Complex) PublicComplex {
	return PublicComplex{
		ID:                c.ID,
		Name:              c.Name,
		Slug:              c.Slug,
		Address:           c.Address,
		City:              c.City,
		Province:          c.Province,
		CountryCode:       c.CountryCode,
		Currency:          c.Currency,
		Phone:             c.Phone,
		Email:             c.Email,
		LogoURL:           c.LogoURL,
		CoverURL:          c.CoverURL,
		DepositPercentage: c.DepositPercentage,
		CancellationHours: c.CancellationHours,
		Latitude:          c.Latitude,
		Longitude:         c.Longitude,
		Amenities:         c.Amenities,
		PaymentsEnabled:   c.MPConnected(),
	}
}

// CourtWithPrices is one bookable court and the bands it is priced by.
type CourtWithPrices struct {
	*courtstore.Court
	Prices []*courtstore.CourtPrice `json:"prices"`
}

// GetPublic handles GET /api/v1/public/complexes/{slug}, the page a client
// lands on from a shared link. It carries only what a booking decision needs.
func (h *Handler) GetPublic(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	profile, err := h.svc.GetPublic(r.Context(), slug)
	if err != nil {
		h.respond.DomainError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"complex":   profile.Complex,
		"courts":    toGenCourtsWithPrices(profile.Courts),
		"schedules": toGenSchedules(profile.Schedules),
	})
}

// UpdateSchedules handles PUT /api/v1/complexes/{id}/schedules, replacing the
// venue's weekly opening hours.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) UpdateSchedules(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input gen.ComplexesUpdateSchedulesJSONBody

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(len(input.Schedules) == 7, "schedules", "must contain exactly 7 days")

	validDays := map[string]bool{
		"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
		"friday": true, "saturday": true, "sunday": true,
	}
	seenDays := make(map[string]bool)

	days := make([]ScheduleInput, len(input.Schedules))
	for i, s := range input.Schedules {
		day := string(s.Day)
		isClosed := s.IsClosed != nil && *s.IsClosed
		openTime := ""
		if s.OpenTime != nil {
			openTime = *s.OpenTime
		}
		closeTime := ""
		if s.CloseTime != nil {
			closeTime = *s.CloseTime
		}

		days[i] = ScheduleInput{Day: day, OpenTime: openTime, CloseTime: closeTime, IsClosed: isClosed}

		key := fmt.Sprintf("schedules[%d].day", i)
		v.Check(validDays[day], key, "must be a valid day of the week")
		v.Check(!seenDays[day], key, "duplicate day")
		seenDays[day] = true

		if !isClosed {
			openKey := fmt.Sprintf("schedules[%d].open_time", i)
			closeKey := fmt.Sprintf("schedules[%d].close_time", i)
			v.Check(openTime != "", openKey, "must be provided when not closed")
			v.Check(closeTime != "", closeKey, "must be provided when not closed")
			if openTime != "" && closeTime != "" {
				v.Check(slots.ValidFormat(openTime), openKey, "must be in HH:MM format")
				v.Check(slots.ValidFormat(closeTime), closeKey, "must be in HH:MM format")
				// close == open means 0-hour window, not allowed.
				// close < open is valid: means closing after midnight (e.g. 08:00-02:00).
				v.Check(openTime != closeTime, closeKey, "must differ from open_time")
			}
		}
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	schedules, err := h.svc.UpdateSchedules(r.Context(), complex.ID, h.actor(r), days)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"schedules": toGenSchedules(schedules)})
}

// ConnectMercadoPago handles POST /api/v1/complexes/{id}/mp/connect, exchanging
// the OAuth code for the owner's own MercadoPago credentials so payments settle
// into their account rather than the platform's.
func (h *Handler) ConnectMercadoPago(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input gen.ComplexesConnectMercadoPagoJSONBody

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Code != "", "code", "must be provided")
	v.Check(input.RedirectUri != "", "redirect_uri", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	codeVerifier := ""
	if input.CodeVerifier != nil {
		codeVerifier = *input.CodeVerifier
	}

	mpUserID, err := h.svc.ConnectMercadoPago(r.Context(), complex.ID, h.actor(r), input.Code, input.RedirectUri, codeVerifier)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"connected":  true,
		"mp_user_id": mpUserID,
	})
}

// DisconnectMercadoPago handles DELETE /api/v1/complexes/{id}/mp/connect.
func (h *Handler) DisconnectMercadoPago(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	err := h.svc.DisconnectMercadoPago(r.Context(), complex.ID, h.actor(r))
	if err != nil {
		h.respond.DomainErrorWith(w, r, err,
			"cannot disconnect MercadoPago while you have active bookings, cancel them first")
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"connected": false})
}

// MercadoPagoStatus handles GET /api/v1/complexes/{id}/mp/status, reporting
// whether the venue can currently take online payments.
func (h *Handler) MercadoPagoStatus(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	connected := complex.MPConnected()
	response := httpx.Envelope{"connected": connected}
	if h.cfg.MPAppID != "" {
		// The client needs the app id to build the authorize URL; see Config.
		response["app_id"] = h.cfg.MPAppID
	}
	if connected && complex.MPUserID != nil {
		response["mp_user_id"] = *complex.MPUserID
	}

	h.respond.JSON(w, r, http.StatusOK, response)
}

// knownAmenities is the vocabulary the complexes_amenities_known CHECK constraint enforces.
//
// Duplicated here rather than read from the database because the handler must
// reject an unknown value with a field error the client can show, and the
// constraint can only reject the whole write with a 500. The database keeps
// the last word: this list going stale fails loudly at the INSERT, not
// silently at the API.
var knownAmenities = map[string]bool{
	"parking":         true,
	"changing_rooms":  true,
	"showers":         true,
	"bar":             true,
	"racket_rental":   true,
	"pro_shop":        true,
	"wifi":            true,
	"lockers":         true,
	"lessons":         true,
	"tournaments":     true,
	"accessible":      true,
	"match_recording": true,
}

// cleanAmenities drops duplicates and preserves the caller's order, returning
// the first unknown value it finds (empty when all are known).
//
// Deduplication lives here because a CHECK constraint cannot express it — it
// would need a subquery — so this is the only layer that can. A venue claiming
// "bar" twice would otherwise render the badge twice.
func cleanAmenities(in []string) (out []string, unknown string) {
	seen := make(map[string]bool, len(in))
	out = make([]string, 0, len(in))
	for _, a := range in {
		if !knownAmenities[a] {
			return nil, a
		}
		if seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out, ""
}

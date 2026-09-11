package complexes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/google/uuid"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
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

	complexes, err := h.store.GetByOwner(r.Context(), user.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"complexes": complexes})
}

// Create handles POST /api/v1/complexes.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name              string   `json:"name"`
		Slug              string   `json:"slug"`
		Address           string   `json:"address"`
		City              string   `json:"city"`
		Province          string   `json:"province"`
		Phone             string   `json:"phone"`
		Email             *string  `json:"email"`
		DepositPercentage int      `json:"deposit_percentage"`
		CancellationHours int      `json:"cancellation_hours"`
		Latitude          *float64 `json:"latitude"`
		Longitude         *float64 `json:"longitude"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	input.Slug = slugify(input.Slug)

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
	v.Check(input.DepositPercentage >= 0, "deposit_percentage", "must be 0 or greater")
	v.Check(input.DepositPercentage <= 100, "deposit_percentage", "must not be more than 100")
	// The floor is 1, not 0. A zero window is read by pricing.CanRefund as
	// "refund always due" — its cancellationHours <= 0 branch returns true
	// unconditionally — and pricing.LinkLive ORs that answer in, so a complex
	// at zero hands out booking links that never expire. complexes_cancellation_hours_range
	// refuses the same value at the column, so this check is the message and
	// that one is the guarantee.
	v.Check(input.CancellationHours >= 1, "cancellation_hours", "must be at least 1")
	v.Check(input.CancellationHours <= 168, "cancellation_hours", "must not be more than 168")
	if input.Email != nil {
		v.Check(validator.Matches(*input.Email, validator.EmailRX), "email", "must be a valid email address")
	}
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	// Ensure slug uniqueness.
	exists, err := h.store.SlugExists(r.Context(), input.Slug)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if exists {
		h.respond.FailedValidation(w, r, map[string]string{"slug": httpx.CodeSlugTaken})
		return
	}

	user, ok := httpx.ContextGetAuthenticatedUser(r)
	if !ok {
		h.respond.InvalidAuthenticationToken(w, r)
		return
	}

	// Check complex limit per account.
	owned, err := h.store.GetByOwner(r.Context(), user.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if len(owned) >= h.cfg.MaxComplexes {
		h.respond.Error(w, r, http.StatusForbidden, fmt.Sprintf("maximum of %d complexes per account reached", h.cfg.MaxComplexes))
		return
	}

	complex := &data.Complex{
		OwnerID:           user.ID,
		Name:              input.Name,
		Slug:              input.Slug,
		Address:           input.Address,
		City:              input.City,
		Province:          input.Province,
		CountryCode:       "AR",
		Currency:          "ARS",
		Phone:             input.Phone,
		Email:             input.Email,
		DepositPercentage: input.DepositPercentage,
		CancellationHours: input.CancellationHours,
		Latitude:          input.Latitude,
		Longitude:         input.Longitude,
		// A new venue lists nothing yet. This must be an empty list, not nil:
		// the column is NOT NULL and an explicit NULL parameter does not fall
		// back to the column default, so a nil slice failed every creation
		// with a 500.
		Amenities: []string{},
	}

	err = h.store.Insert(r.Context(), complex)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateSlug):
			// The check above said the slug was free and the constraint
			// disagreed: either another request took it in between, or the
			// two are answering different questions. Either way the owner is
			// told which field to change instead of being handed a 500.
			h.respond.FailedValidation(w, r, map[string]string{"slug": httpx.CodeSlugTaken})
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.record(r, complex.ID, "create", &complex.ID, nil, complex)

	// Create default schedules (Mon-Sun, 08:00-23:00).
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		schedule := &data.Schedule{
			ComplexID: complex.ID,
			Day:       day,
			OpenTime:  "08:00",
			CloseTime: "23:00",
			IsClosed:  false,
		}
		err = h.store.UpsertSchedule(r.Context(), schedule)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}
	}

	h.respond.JSON(w, r, http.StatusCreated, httpx.Envelope{"complex": complex})
}

// Get handles GET /api/v1/complexes/:id, returning one venue with its
// schedules.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"complex": complex})
}

// SlugAvailable handles GET /api/v1/complexes/slug-available?slug=x.
//
// Answers the one question the client cannot: is this public URL free. Two
// owners can pick the same name a second apart and nothing in a browser knows
// it; without this the only way to find out is to fill the whole form and be
// refused on submit.
//
// It exists as its own route because the obvious shortcut is wrong. Probing
// GET /public/complexes/:slug looks equivalent and is not: that endpoint 404s
// for a DEACTIVATED venue, so it would report "free" for a name the database
// will refuse — a check that lies in exactly the case it was added for.
//
// Behind RequireAuth. The answer is public information (the slug either serves
// a page or it does not), but only a signed-in owner creating or renaming a
// complex has any use for it, and an open endpoint invites enumeration.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) SlugAvailable(w http.ResponseWriter, r *http.Request) {
	slug := slugify(r.URL.Query().Get("slug"))

	// An empty or malformed slug is not "taken"; it is not a slug at all. The
	// form's own validation says so, and answering `available: false` here
	// would make the field report the wrong reason.
	if slug == "" || !slugValidRX.MatchString(slug) || len(slug) > maxSlugLength {
		h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"slug": slug, "available": false, "valid": false})
		return
	}

	existing, err := h.store.SlugsWithPrefix(r.Context(), slug)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	taken := make(map[string]bool, len(existing))
	for _, s := range existing {
		taken[s] = true
	}
	// H-13: a reserved slug (see reservedSlugs) is unavailable even though no
	// complex has ever claimed it in the database — this endpoint is the
	// suggestion path too, and it must not tell an owner "admin" is free just
	// because nobody has raced them to it. Folding it into `taken` also makes
	// suggestSlug skip it for free, the same way it already skips a slug
	// another complex holds.
	taken[slug] = taken[slug] || reservedSlugs[slug]

	body := httpx.Envelope{"slug": slug, "valid": true, "available": !taken[slug]}
	if taken[slug] {
		body["suggestion"] = suggestSlug(slug, taken)
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

// Update handles PUT /api/v1/complexes/:id. Every field is optional; an
// omitted one keeps its current value.
//
// It is one cohesive request lifecycle for a single resource operation, per
// this codebase's handler conventions (CLAUDE.md); splitting it would relocate
// sequential steps into helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}
	oldLogoURL := complex.LogoURL
	oldCoverURL := complex.CoverURL

	var input struct {
		Name              *string   `json:"name"`
		Slug              *string   `json:"slug"`
		Address           *string   `json:"address"`
		City              *string   `json:"city"`
		Province          *string   `json:"province"`
		Phone             *string   `json:"phone"`
		Email             *string   `json:"email"`
		LogoURL           *string   `json:"logo_url"`
		CoverURL          *string   `json:"cover_url"`
		DepositPercentage *int      `json:"deposit_percentage"`
		CancellationHours *int      `json:"cancellation_hours"`
		IsActive          *bool     `json:"is_active"`
		Latitude          *float64  `json:"latitude"`
		Longitude         *float64  `json:"longitude"`
		Amenities         *[]string `json:"amenities"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()

	if input.Name != nil {
		v.Check(*input.Name != "", "name", "must not be empty")
		v.Check(len(*input.Name) <= 200, "name", "must not be more than 200 characters")
		complex.Name = *input.Name
	}

	// The public URL can be changed. It is the address already living in shared
	// links, WhatsApp messages and printed QR codes, so the client warns before
	// letting anyone touch it — but the decision is the owner's, and refusing it
	// outright left a badly chosen name permanent.
	//
	// Uniqueness is checked only when it actually changes: comparing against
	// itself would report every save of an unrelated field as "slug taken".
	if input.Slug != nil {
		slug := slugify(*input.Slug)
		v.Check(slug != "", "slug", "must be provided")
		v.Check(slugValidRX.MatchString(slug), "slug", "must contain only lowercase letters, numbers, and hyphens")
		v.Check(len(slug) <= maxSlugLength, "slug", fmt.Sprintf("must not be more than %d characters", maxSlugLength))
		// See reservedSlugs on Create: the same route collision applies to a
		// rename, and a rename is exactly how an owner could talk themselves
		// into it — Create already refuses these, but Update never re-checked.
		v.Check(!reservedSlugs[slug], "slug", "is reserved")
		if v.Valid() && slug != complex.Slug {
			taken, err := h.store.SlugExists(r.Context(), slug)
			if err != nil {
				h.respond.ServerError(w, r, err)
				return
			}
			if taken {
				h.respond.FailedValidation(w, r, map[string]string{"slug": httpx.CodeSlugTaken})
				return
			}
		}
		complex.Slug = slug
	}

	if input.Address != nil {
		v.Check(*input.Address != "", "address", "must not be empty")
		complex.Address = *input.Address
	}
	if input.City != nil {
		v.Check(*input.City != "", "city", "must not be empty")
		complex.City = *input.City
	}
	if input.Province != nil {
		v.Check(*input.Province != "", "province", "must not be empty")
		complex.Province = *input.Province
	}
	if input.Latitude != nil {
		complex.Latitude = input.Latitude
	}
	if input.Amenities != nil {
		cleaned, unknown := cleanAmenities(*input.Amenities)
		v.Check(unknown == "", "amenities", "unknown amenity: "+unknown)
		complex.Amenities = cleaned
	}
	if input.Longitude != nil {
		complex.Longitude = input.Longitude
	}

	if input.Phone != nil {
		v.Check(*input.Phone != "", "phone", "must not be empty")
		complex.Phone = *input.Phone
	}
	if input.Email != nil {
		v.Check(validator.Matches(*input.Email, validator.EmailRX), "email", "must be a valid email address")
		complex.Email = input.Email
	}
	if input.LogoURL != nil {
		if *input.LogoURL != "" {
			v.Check(isValidImageURL(*input.LogoURL), "logo_url", "must be a valid HTTP/HTTPS URL")
		}
		complex.LogoURL = input.LogoURL
	}
	if input.CoverURL != nil {
		if *input.CoverURL != "" {
			v.Check(isValidImageURL(*input.CoverURL), "cover_url", "must be a valid HTTP/HTTPS URL")
		}
		complex.CoverURL = input.CoverURL
	}
	if input.DepositPercentage != nil {
		v.Check(*input.DepositPercentage >= 0, "deposit_percentage", "must be 0 or greater")
		v.Check(*input.DepositPercentage <= 100, "deposit_percentage", "must not be more than 100")
		complex.DepositPercentage = *input.DepositPercentage
	}
	if input.CancellationHours != nil {
		// Same floor as Create, and for the same reason: see the note there.
		v.Check(*input.CancellationHours >= 1, "cancellation_hours", "must be at least 1")
		v.Check(*input.CancellationHours <= 168, "cancellation_hours", "must not be more than 168")
		complex.CancellationHours = *input.CancellationHours
	}
	if input.IsActive != nil {
		complex.IsActive = *input.IsActive
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	err = h.store.Update(r.Context(), complex)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.EditConflict(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	h.record(r, complex.ID, "update", &complex.ID, nil, complex)

	// Clean up old images from R2 when URLs change.
	if h.storage != nil {
		if input.LogoURL != nil && oldLogoURL != nil && *oldLogoURL != "" {
			if *input.LogoURL != *oldLogoURL {
				oldURL := *oldLogoURL
				//nolint:contextcheck // intentionally detached. R2 cleanup of a
				// replaced logo must run after the response is written and must not be
				// cancelled by request abandonment; it has no request-scoped deadline needs.
				h.run(func() {
					if key, ok := h.storage.KeyFromPublicURL(oldURL); ok {
						if err := h.storage.DeleteObject(context.Background(), key); err != nil {
							h.logger.Error("storage: failed to delete old logo", "error", err, "key", key)
						}
					}
				})
			}
		}
		if input.CoverURL != nil && oldCoverURL != nil && *oldCoverURL != "" {
			if *input.CoverURL != *oldCoverURL {
				oldURL := *oldCoverURL
				//nolint:contextcheck // see the identical justification above (logo cleanup)
				// in this file.
				h.run(func() {
					if key, ok := h.storage.KeyFromPublicURL(oldURL); ok {
						if err := h.storage.DeleteObject(context.Background(), key); err != nil {
							h.logger.Error("storage: failed to delete old cover", "error", err, "key", key)
						}
					}
				})
			}
		}
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"complex": complex})
}

// deletionOutcome is what a delete did beyond the venue row itself, recorded in
// the audit trail and returned to the caller. Today that is the court count:
// deleting a venue takes its courts offline, and an owner who is told only
// "complex deleted" has no way to know how much went with it.
type deletionOutcome struct {
	CourtsDeactivated int `json:"courts_deactivated"`
}

// Delete handles DELETE /api/v1/complexes/:id.
//
// This is the one cascading operation in the module: it soft-deletes the
// venue's courts and cancels its future bookings, so it is refused outright
// while any booking is still live.
//
// The venue and its courts go down in one transaction (SoftDeleteCascade,
// which leans on the soft-delete cascade trigger and verifies the result), so there is
// no longer a window in which the venue is deleted and its courts are not.
// Cancelling future bookings stays outside it: those are a separate table with
// its own status machine, a cancellation that fails is recoverable by hand, and
// the delete guard above has already established there are none live.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	hasActive, err := h.bookings.HasActiveBookings(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if hasActive {
		h.respond.Error(w, r, http.StatusConflict, "cannot delete complex while it has active bookings, cancel them first")
		return
	}

	courtsDeactivated, err := h.store.SoftDeleteCascade(r.Context(), complex.ID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.NotFound(w, r)
			return
		}
		h.respond.ServerError(w, r, err)
		return
	}

	outcome := deletionOutcome{CourtsDeactivated: courtsDeactivated}
	h.record(r, complex.ID, "delete", &complex.ID, complex, outcome)

	if err := h.bookings.CancelFutureByComplex(r.Context(), complex.ID); err != nil {
		h.logger.Error("failed to cancel future bookings for complex", "error", err, "complex_id", complex.ID)
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"message":            "complex deleted",
		"courts_deactivated": courtsDeactivated,
	})
}

// publicComplex is what a stranger is allowed to know about a venue.
//
// The public page used to serialise the whole data.Complex, which carries
// owner_id and mp_user_id. mp_user_id is the MercadoPago collector id the
// payment path compares an incoming payment against to decide the money
// reached the right seller; publishing it on an unauthenticated endpoint hands
// that check's expected value to anyone who asks. owner_id names a platform
// account, which a booking decision never needs.
//
// This is an explicit projection rather than json:"-" on the domain struct so
// that the owner-facing endpoints, which legitimately return both, are
// unaffected.
type publicComplex struct {
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

func newPublicComplex(c *data.Complex) publicComplex {
	return publicComplex{
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

// courtWithPrices is one bookable court and the bands it is priced by.
type courtWithPrices struct {
	*data.Court
	Prices []*data.CourtPrice `json:"prices"`
}

// publicCourts returns the complex's bookable courts with their price bands.
//
// Only active courts, matching the availability grid. This endpoint used to
// return every court the complex has ever had, so a retired court appeared on
// the page with a price list and no slots to book it in.
func (h *Handler) publicCourts(r *http.Request, complexID uuid.UUID) ([]courtWithPrices, error) {
	courts, err := h.courts.GetByComplex(r.Context(), complexID)
	if err != nil {
		return nil, err
	}

	active := make([]*data.Court, 0, len(courts))
	for _, c := range courts {
		if c.IsActive {
			active = append(active, c)
		}
	}

	// Batch-fetch all prices in a single query (instead of N queries).
	courtIDs := make([]uuid.UUID, len(active))
	for i, c := range active {
		courtIDs[i] = c.ID
	}
	allPrices, err := h.courts.GetPricesByCourtIDs(r.Context(), courtIDs)
	if err != nil {
		return nil, err
	}
	pricesByCourtID := make(map[uuid.UUID][]*data.CourtPrice, len(active))
	for _, p := range allPrices {
		pricesByCourtID[p.CourtID] = append(pricesByCourtID[p.CourtID], p)
	}

	out := make([]courtWithPrices, len(active))
	for i, c := range active {
		out[i] = courtWithPrices{Court: c, Prices: pricesByCourtID[c.ID]}
	}
	return out, nil
}

// GetPublic handles GET /api/v1/public/complexes/:slug, the page a client
// lands on from a shared link. It carries only what a booking decision needs.
func (h *Handler) GetPublic(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	complex, err := h.store.GetBySlug(r.Context(), slug)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			h.respond.NotFound(w, r)
		default:
			h.respond.ServerError(w, r, err)
		}
		return
	}

	// A deactivated venue is closed to the public. Only the booking write used
	// to check this, so a venue that switched itself off kept serving its
	// address, phone, courts and prices, and answered every booking attempt
	// with a bare 404 the page could not explain.
	if !complex.IsActive {
		h.respond.NotFound(w, r)
		return
	}

	courtsWithPrices, err := h.publicCourts(r, complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	schedules, err := h.store.GetSchedules(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"complex":   newPublicComplex(complex),
		"courts":    courtsWithPrices,
		"schedules": schedules,
	})
}

// UpdateSchedules handles PUT /api/v1/complexes/:id/schedules, replacing the
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

	var input struct {
		Schedules []struct {
			Day       string `json:"day"`
			OpenTime  string `json:"open_time"`
			CloseTime string `json:"close_time"`
			IsClosed  bool   `json:"is_closed"`
		} `json:"schedules"`
	}

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

	for i, s := range input.Schedules {
		key := fmt.Sprintf("schedules[%d].day", i)
		v.Check(validDays[s.Day], key, "must be a valid day of the week")
		v.Check(!seenDays[s.Day], key, "duplicate day")
		seenDays[s.Day] = true

		if !s.IsClosed {
			openKey := fmt.Sprintf("schedules[%d].open_time", i)
			closeKey := fmt.Sprintf("schedules[%d].close_time", i)
			v.Check(s.OpenTime != "", openKey, "must be provided when not closed")
			v.Check(s.CloseTime != "", closeKey, "must be provided when not closed")
			if s.OpenTime != "" && s.CloseTime != "" {
				v.Check(slots.ValidFormat(s.OpenTime), openKey, "must be in HH:MM format")
				v.Check(slots.ValidFormat(s.CloseTime), closeKey, "must be in HH:MM format")
				// close == open means 0-hour window, not allowed.
				// close < open is valid: means closing after midnight (e.g. 08:00-02:00).
				v.Check(s.OpenTime != s.CloseTime, closeKey, "must differ from open_time")
			}
		}
	}

	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	var schedules []*data.Schedule
	for _, s := range input.Schedules {
		schedule := &data.Schedule{
			ComplexID: complex.ID,
			Day:       s.Day,
			OpenTime:  s.OpenTime,
			CloseTime: s.CloseTime,
			IsClosed:  s.IsClosed,
		}
		err = h.store.UpsertSchedule(r.Context(), schedule)
		if err != nil {
			h.respond.ServerError(w, r, err)
			return
		}
		schedules = append(schedules, schedule)
	}

	h.record(r, complex.ID, "update_schedules", &complex.ID, nil, schedules)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"schedules": schedules})
}

// ConnectMercadoPago handles POST /api/v1/complexes/:id/mp/connect, exchanging
// the OAuth code for the owner's own MercadoPago credentials so payments settle
// into their account rather than the platform's.
func (h *Handler) ConnectMercadoPago(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	var input struct {
		Code         string `json:"code"`
		RedirectURI  string `json:"redirect_uri"`
		CodeVerifier string `json:"code_verifier"`
	}

	err := httpx.ReadJSON(w, r, &input)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.Code != "", "code", "must be provided")
	v.Check(input.RedirectURI != "", "redirect_uri", "must be provided")
	if !v.Valid() {
		h.respond.FailedValidation(w, r, v.Errors)
		return
	}

	tokens, err := h.payments.ExchangeOAuthCode(r.Context(), input.Code, input.RedirectURI, input.CodeVerifier)
	if err != nil {
		h.logger.Error("mp connect: oauth exchange failed", "error", err, "complex_id", complex.ID)
		h.respond.ServerError(w, r, fmt.Errorf("failed to connect MercadoPago: %w", err))
		return
	}

	mpUserID := fmt.Sprintf("%d", tokens.UserID)

	err = h.store.UpdateMPCredentials(r.Context(), complex.ID, tokens.AccessToken, tokens.RefreshToken, mpUserID, tokens.ExpiresIn)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "mp_connect", &complex.ID, nil, nil)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{
		"connected":  true,
		"mp_user_id": mpUserID,
	})
}

// DisconnectMercadoPago handles DELETE /api/v1/complexes/:id/mp/connect.
func (h *Handler) DisconnectMercadoPago(w http.ResponseWriter, r *http.Request) {
	complex, ok := httpx.ContextGetComplex(r)
	if !ok {
		h.respond.ServerError(w, r, fmt.Errorf("missing complex in context"))
		return
	}

	hasActive, err := h.bookings.HasActiveBookings(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	if hasActive {
		h.respond.Error(w, r, http.StatusConflict, "cannot disconnect MercadoPago while you have active bookings, cancel them first")
		return
	}

	err = h.store.ClearMPCredentials(r.Context(), complex.ID)
	if err != nil {
		h.respond.ServerError(w, r, err)
		return
	}

	h.record(r, complex.ID, "mp_disconnect", &complex.ID, nil, nil)

	h.respond.JSON(w, r, http.StatusOK, httpx.Envelope{"connected": false})
}

// MercadoPagoStatus handles GET /api/v1/complexes/:id/mp/status, reporting
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

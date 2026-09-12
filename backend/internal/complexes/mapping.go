package complexes

import (
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
)

// complexResponse is the wire shape for a Complex as returned to an
// authenticated owner (List, Create, Get, Update). It exists, rather than
// gen.Complex, because the generated type cannot reproduce this endpoint's
// existing wire output byte for byte — see toComplexResponse.
//
// Every field name and tag here matches complexstore.Complex's own json tag
// exactly, so building through this type is purely the HTTP-08 fix (no store
// struct reaches json.Marshal directly) and changes nothing on the wire.
type complexResponse struct {
	ID                uuid.UUID  `json:"id"`
	OwnerID           uuid.UUID  `json:"owner_id"`
	Name              string     `json:"name"`
	Slug              string     `json:"slug"`
	Address           string     `json:"address"`
	City              string     `json:"city"`
	Province          string     `json:"province"`
	CountryCode       string     `json:"country_code"`
	Currency          string     `json:"currency"`
	Phone             string     `json:"phone"`
	Email             *string    `json:"email"`
	LogoURL           *string    `json:"logo_url"`
	CoverURL          *string    `json:"cover_url"`
	DepositPercentage int        `json:"deposit_percentage"`
	CancellationHours int        `json:"cancellation_hours"`
	Latitude          *float64   `json:"latitude"`
	Longitude         *float64   `json:"longitude"`
	IsActive          bool       `json:"is_active"`
	Amenities         []string   `json:"amenities"`
	CourtCount        *int       `json:"court_count,omitempty"`
	PaymentsEnabled   bool       `json:"payments_enabled"`
	MPUserID          *string    `json:"mp_user_id,omitempty"`
	Version           int        `json:"version"`
	MPTokenExpiresAt  *time.Time `json:"mp_token_expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// toComplexResponse maps a store Complex onto its wire shape (HTTP-08): a
// handler must never serialize complexstore.Complex directly, because that
// struct also carries the unexported mpAccessToken/mpRefreshToken plumbing
// and would otherwise depend on nobody ever adding a new field to it without
// also deciding whether it belongs on the wire.
//
// WIRE MISMATCH: gen.Complex (generated from openapi.yaml's Complex schema)
// still cannot reproduce this response byte for byte, on one remaining
// count: email, logo_url, cover_url, mp_user_id and mp_token_expires_at are
// declared `type: [X, "null"]` in the schema, but oapi-codegen generates
// every nullable field as an `omitempty` pointer regardless — that drops the
// key from the response entirely when the pointer is nil, where this
// endpoint has always sent an explicit JSON null. There is no document
// wording that changes oapi-codegen's nullable-field codegen, so this local
// type stays.
//
// Two former mismatches were fixed in the document instead of worked around
// here (see docs/auditoria-backend-2026-09-11's step-4 report): Complex now
// declares its long-sent `version` field, and latitude/longitude now declare
// `format: double` so oapi-codegen stops narrowing them to float32.
func toComplexResponse(c *complexstore.Complex) complexResponse {
	return complexResponse{
		ID:                c.ID,
		OwnerID:           c.OwnerID,
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
		IsActive:          c.IsActive,
		Amenities:         c.Amenities,
		CourtCount:        c.CourtCount,
		PaymentsEnabled:   c.PaymentsEnabled,
		MPUserID:          c.MPUserID,
		Version:           c.Version,
		MPTokenExpiresAt:  c.MPTokenExpiresAt,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

// toComplexResponses maps a slice, for List.
func toComplexResponses(cs []*complexstore.Complex) []complexResponse {
	out := make([]complexResponse, len(cs))
	for i, c := range cs {
		out[i] = toComplexResponse(c)
	}
	return out
}

// toGenSchedule maps a store Schedule onto gen.Schedule (HTTP-08).
//
// Unlike Complex, the generated type reproduces this endpoint's wire shape
// exactly: same field names, no optional/nullable fields on either side,
// and gen.Weekday/openapi_types.UUID are a plain string and a type alias for
// uuid.UUID respectively, so they marshal identically to the string and
// uuid.UUID this endpoint has always sent.
func toGenSchedule(s *complexstore.Schedule) gen.Schedule {
	return gen.Schedule{
		Id:        s.ID,
		ComplexId: s.ComplexID,
		Day:       gen.Weekday(s.Day),
		OpenTime:  s.OpenTime,
		CloseTime: s.CloseTime,
		IsClosed:  s.IsClosed,
	}
}

// toGenSchedules maps a slice, for UpdateSchedules and GetPublic.
func toGenSchedules(schedules []*complexstore.Schedule) []gen.Schedule {
	out := make([]gen.Schedule, len(schedules))
	for i, s := range schedules {
		out[i] = toGenSchedule(s)
	}
	return out
}

// toGenCourtPrice maps a store CourtPrice onto gen.CourtPrice (HTTP-08).
//
// FromMin, ToMin and Version are plain (non-pointer) ints on the store type
// and always present on the wire today, including as zero. The generated
// type marks them optional, but taking their address rather than only
// setting the pointer when non-zero keeps every one of them on the wire
// exactly as before: encoding/json only omits a nil pointer under
// omitempty, never one pointing at a zero value.
func toGenCourtPrice(p *courtstore.CourtPrice) gen.CourtPrice {
	fromMin, toMin, version := p.FromMin, p.ToMin, p.Version
	return gen.CourtPrice{
		Id:       p.ID,
		CourtId:  p.CourtID,
		Price:    p.Price,
		DayType:  gen.Weekday(p.DayType),
		TimeFrom: p.TimeFrom,
		TimeTo:   p.TimeTo,
		FromMin:  &fromMin,
		ToMin:    &toMin,
		Version:  &version,
	}
}

// toGenCourtWithPrices maps this package's CourtWithPrices — itself an
// embedded *courtstore.Court, which is the HTTP-08 violation this closes —
// onto gen.CourtWithPrices. Version follows the same always-present-pointer
// rule as toGenCourtPrice.
func toGenCourtWithPrices(c CourtWithPrices) gen.CourtWithPrices {
	prices := make([]gen.CourtPrice, len(c.Prices))
	for i, p := range c.Prices {
		prices[i] = toGenCourtPrice(p)
	}

	version := c.Version
	return gen.CourtWithPrices{
		Id:          c.ID,
		ComplexId:   c.ComplexID,
		Name:        c.Name,
		Sport:       gen.CourtWithPricesSport(c.Sport),
		CourtType:   gen.CourtWithPricesCourtType(c.CourtType),
		IsActive:    c.IsActive,
		Description: c.Description,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
		Version:     &version,
		Prices:      prices,
	}
}

// toGenCourtsWithPrices maps a slice, for GetPublic.
func toGenCourtsWithPrices(cs []CourtWithPrices) []gen.CourtWithPrices {
	out := make([]gen.CourtWithPrices, len(cs))
	for i, c := range cs {
		out[i] = toGenCourtWithPrices(c)
	}
	return out
}

// toGenSlugAvailability maps a SlugStatus (already a local, non-store type)
// onto gen.SlugAvailability, preserving this endpoint's existing rule that
// "suggestion" is present only when the slug is taken and a free
// alternative was found — an empty SlugStatus.Suggestion leaves the
// generated type's pointer nil, so `omitempty` drops the key exactly as the
// hand-built envelope always did.
func toGenSlugAvailability(status SlugStatus) gen.SlugAvailability {
	out := gen.SlugAvailability{
		Slug:      status.Slug,
		Valid:     status.Valid,
		Available: status.Available,
	}
	if status.Suggestion != "" {
		suggestion := status.Suggestion
		out.Suggestion = &suggestion
	}
	return out
}

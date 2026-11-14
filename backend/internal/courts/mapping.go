package courts

import (
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// toGenCourt maps a store court onto the generated wire type (rule HTTP-08).
// Every field on the wire today is reproduced here by name.
func toGenCourt(c *courtstore.Court) gen.Court {
	version := c.Version
	return gen.Court{
		ComplexId:   c.ComplexID,
		CourtType:   gen.CourtCourtType(c.CourtType),
		CreatedAt:   c.CreatedAt,
		Description: c.Description,
		HasLighting: c.HasLighting,
		Id:          c.ID,
		IsActive:    c.IsActive,
		Name:        c.Name,
		Sport:       gen.CourtSport(c.Sport),
		Surface:     toGenSurface[gen.CourtSurface](c.Surface),
		UpdatedAt:   c.UpdatedAt,
		// gen.Court.Version is *int with omitempty, but the store's Version is
		// always set (never the client-supplied kind of "absent"), so taking
		// its address keeps it on the wire exactly as the plain int did:
		// omitempty on a pointer only ever hides a nil pointer.
		Version: &version,
	}
}

// toGenSurface maps an optional stored surface onto a generated surface enum.
// nil stays nil, which the wire omits (the same way a missing description is).
func toGenSurface[T ~string](s *string) *T {
	if s == nil {
		return nil
	}
	v := T(*s)
	return &v
}

// toGenCourtPrice maps a store price band onto the generated wire type.
func toGenCourtPrice(p *courtstore.CourtPrice) gen.CourtPrice {
	fromMin, toMin, version := p.FromMin, p.ToMin, p.Version
	return gen.CourtPrice{
		CourtId:  p.CourtID,
		DayType:  gen.Weekday(p.DayType),
		FromMin:  &fromMin,
		Id:       p.ID,
		Price:    p.Price,
		TimeFrom: p.TimeFrom,
		TimeTo:   p.TimeTo,
		ToMin:    &toMin,
		Version:  &version,
	}
}

// toGenCourtWithPrices maps a court together with its price bands, as
// returned by the owner's court list, onto the generated wire type.
func toGenCourtWithPrices(c CourtWithPrices) gen.CourtWithPrices {
	version := c.Version
	prices := make([]gen.CourtPrice, len(c.Prices))
	for i, p := range c.Prices {
		prices[i] = toGenCourtPrice(p)
	}
	return gen.CourtWithPrices{
		ComplexId:   c.ComplexID,
		CourtType:   gen.CourtWithPricesCourtType(c.CourtType),
		CreatedAt:   c.CreatedAt,
		Description: c.Description,
		HasLighting: c.HasLighting,
		Id:          c.ID,
		IsActive:    c.IsActive,
		Name:        c.Name,
		Prices:      prices,
		Sport:       gen.CourtWithPricesSport(c.Sport),
		Surface:     toGenSurface[gen.CourtSurface](c.Surface),
		UpdatedAt:   c.UpdatedAt,
		Version:     &version,
	}
}

// toGenAvailabilitySlot maps one bookable grid position onto the generated
// wire type.
func toGenAvailabilitySlot(s AvailabilitySlot) gen.AvailabilitySlot {
	return gen.AvailabilitySlot{
		Available:       s.Available,
		DurationMinutes: s.DurationMinutes,
		EndTime:         s.EndTime,
		Price:           s.Price,
		StartMin:        s.StartMin,
		StartTime:       s.StartTime,
	}
}

// toGenAvailabilityCourt maps one court's row of the availability grid onto
// the generated wire type.
func toGenAvailabilityCourt(c CourtAvailability) gen.AvailabilityCourt {
	courtID, err := uuid.Parse(c.CourtID)
	if err != nil {
		// buildGrid always sets CourtID from court.ID.String(), so this
		// cannot fail in practice; uuid.Nil is a harmless fallback rather
		// than a panic if that ever stops being true.
		courtID = uuid.Nil
	}

	slotsOut := make([]gen.AvailabilitySlot, len(c.Slots))
	for i, s := range c.Slots {
		slotsOut[i] = toGenAvailabilitySlot(s)
	}

	return gen.AvailabilityCourt{
		CourtId:     courtID,
		CourtName:   c.CourtName,
		CourtType:   gen.AvailabilityCourtCourtType(c.CourtType),
		Description: c.Description,
		HasLighting: c.HasLighting,
		Slots:       slotsOut,
		Sport:       gen.AvailabilityCourtSport(c.Sport),
		Surface:     toGenSurface[gen.AvailabilityCourtSurface](c.Surface),
	}
}

// toGenAvailability maps the bookable grid for one day onto the generated
// wire type. date is the same parsed value the handler already validated
// dateStr into, so this never re-parses (and cannot fail on) a string the
// service only ever echoes back unchanged.
func toGenAvailability(resp *AvailabilityResponse, date time.Time) gen.Availability {
	courtsOut := make([]gen.AvailabilityCourt, len(resp.Courts))
	for i, c := range resp.Courts {
		courtsOut[i] = toGenAvailabilityCourt(c)
	}
	return gen.Availability{
		Courts: courtsOut,
		Date:   openapi_types.Date{Time: date},
		Day:    gen.Weekday(resp.Day),
		IsOpen: resp.IsOpen,
	}
}

// blockedSlotWire is the wire shape for a blocked slot response.
//
// WIRE MISMATCH: courts BlockSlot/ListBlockedSlots date — the OpenAPI
// document declares BlockedSlot.date as `format: date` (gen.BlockedSlot.Date
// is openapi_types.Date, which marshals as "2024-01-15"), but
// courtstore.BlockedSlot.Date is a bare time.Time with no custom marshaling,
// so today's wire has always serialized it as a full RFC 3339 timestamp
// ("2024-01-15T00:00:00Z"). Adopting gen.BlockedSlot as-is would silently
// narrow the field to date-only and change the wire. This local type
// reproduces every other gen.BlockedSlot field but keeps Date as time.Time,
// so the response is unchanged; the document is left alone, per this task's
// scope.
type blockedSlotWire struct {
	ID        uuid.UUID  `json:"id"`
	CourtID   uuid.UUID  `json:"court_id"`
	Date      time.Time  `json:"date"`
	StartTime string     `json:"start_time"`
	EndTime   string     `json:"end_time"`
	Reason    *string    `json:"reason,omitempty"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	// CourtName mirrors courtstore.BlockedSlot's own behaviour — omitted when
	// there is no name — but as a pointer rather than an omitempty-on-empty
	// plain string, since a courtstore.BlockedSlot is never serialized
	// directly (rule HTTP-08). Both produce identical bytes either way.
	CourtName *string `json:"court_name,omitempty"`
}

// toBlockedSlotWire maps a store blocked slot onto blockedSlotWire.
func toBlockedSlotWire(s *courtstore.BlockedSlot) blockedSlotWire {
	var courtName *string
	if s.CourtName != "" {
		courtName = &s.CourtName
	}
	return blockedSlotWire{
		ID:        s.ID,
		CourtID:   s.CourtID,
		Date:      s.Date,
		StartTime: s.StartTime,
		EndTime:   s.EndTime,
		Reason:    s.Reason,
		CreatedBy: s.CreatedBy,
		CreatedAt: s.CreatedAt,
		CourtName: courtName,
	}
}

// toFixedBlocksWire maps the dates fixed weekly bookings hold onto the wire's
// fixed blocks, never nil. The times are the venue's wall-clock: end_time is
// earlier than start_time when the hours run past midnight.
func toFixedBlocksWire(occs []bookingstore.SeriesOccurrence) []gen.FixedBlock {
	out := make([]gen.FixedBlock, len(occs))
	for i, o := range occs {
		out[i] = gen.FixedBlock{
			SeriesId:        o.SeriesID,
			CourtId:         o.CourtID,
			Date:            openapi_types.Date{Time: o.Date},
			StartTime:       o.StartsAt.In(timezone.Argentina).Format("15:04"),
			EndTime:         o.EndsAt.In(timezone.Argentina).Format("15:04"),
			DurationMinutes: int(o.EndsAt.Sub(o.StartsAt) / time.Minute),
			ClientName:      o.ClientName,
		}
	}
	return out
}

// toEventBlocksWire maps the hours open-play events hold onto the wire's event
// blocks, never nil. The times are the venue's wall-clock: end_time is earlier
// than start_time when the hours run past midnight.
func toEventBlocksWire(occs []bookingstore.EventOccupancy) []gen.EventBlock {
	out := make([]gen.EventBlock, len(occs))
	for i, o := range occs {
		out[i] = gen.EventBlock{
			EventId:         o.EventID,
			CourtId:         o.CourtID,
			Date:            openapi_types.Date{Time: o.Date},
			StartTime:       o.StartsAt.In(timezone.Argentina).Format("15:04"),
			EndTime:         o.EndsAt.In(timezone.Argentina).Format("15:04"),
			DurationMinutes: int(o.EndsAt.Sub(o.StartsAt) / time.Minute),
			Name:            o.Name,
		}
	}
	return out
}

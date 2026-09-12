package courts

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// ErrEditConflict reports that a court moved out from under a read: the row
// the update aimed at was gone by the time it ran. It is distinct from
// data.ErrRecordNotFound, which means the court was never this complex's to
// begin with, because the two answer the caller differently.
var ErrEditConflict = errors.New("court changed before the update")

// Actor is who a change is attributed to, as the handler read it off the
// request. The service needs it for the audit trail and for nothing else.
type Actor struct {
	// UserID is the authenticated owner, or nil for a system action.
	UserID *uuid.UUID
	// IP is the address the request arrived from.
	IP string
}

// Service holds this module's rules: what a court may be, which hours an owner
// may take off sale, and what the public availability grid is allowed to
// advertise. Every store call and every audit entry of the court domain goes
// through it.
type Service struct {
	store     Store
	bookings  BookingReader
	complexes ComplexReader
	audit     Recorder
}

// NewService returns a Service backed by the given stores.
func NewService(store Store, bookings BookingReader, complexes ComplexReader, recorder Recorder) *Service {
	return &Service{
		store:     store,
		bookings:  bookings,
		complexes: complexes,
		audit:     recorder,
	}
}

// record writes an audit entry for a change to this complex's configuration.
func (s *Service) record(complexID uuid.UUID, actor Actor, action, entityType string, entityID *uuid.UUID, oldVal, newVal any) {
	s.audit.Record(audit.Entry{
		UserID:     actor.UserID,
		ComplexID:  &complexID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   oldVal,
		NewValue:   newVal,
		IPAddress:  actor.IP,
	})
}

// ownedCourt loads a court and hides one that belongs to another complex: it
// answers the same ErrRecordNotFound a court that does not exist answers, so a
// probe cannot tell the two apart.
func (s *Service) ownedCourt(ctx context.Context, complexID, courtID uuid.UUID) (*courtstore.Court, error) {
	court, err := s.store.GetByID(ctx, courtID)
	if err != nil {
		return nil, err
	}
	if court.ComplexID != complexID {
		return nil, data.ErrRecordNotFound
	}
	return court, nil
}

// CourtWithPrices is a court together with its price bands, as the owner's
// court list publishes them.
type CourtWithPrices struct {
	*courtstore.Court
	Prices []*courtstore.CourtPrice `json:"prices"`
}

// List returns every court of a complex, active or not — the owner manages
// both — each with its own price bands.
func (s *Service) List(ctx context.Context, complexID uuid.UUID) ([]CourtWithPrices, error) {
	courts, err := s.store.GetByComplex(ctx, complexID)
	if err != nil {
		return nil, err
	}

	result := make([]CourtWithPrices, len(courts))
	for i, c := range courts {
		prices, err := s.store.GetPrices(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		result[i] = CourtWithPrices{Court: c, Prices: prices}
	}
	return result, nil
}

// CreateInput is a validated request to add a court to a complex.
type CreateInput struct {
	Name        string
	Sport       string
	CourtType   string
	Description *string
}

// Create adds a court to the complex and records it. A description of "" is
// stored as no description at all, so a cleared field and a never-described
// court read the same way.
func (s *Service) Create(ctx context.Context, complexID uuid.UUID, actor Actor, in CreateInput) (*courtstore.Court, error) {
	court := &courtstore.Court{
		ComplexID:   complexID,
		Name:        in.Name,
		Sport:       in.Sport,
		CourtType:   in.CourtType,
		Description: emptyToNil(in.Description),
	}

	if err := s.store.Insert(ctx, court); err != nil {
		return nil, err
	}

	s.record(complexID, actor, "create", "court", &court.ID, nil, court)
	return court, nil
}

// UpdateInput is a validated partial update. A nil field keeps its current
// value.
type UpdateInput struct {
	Name        *string
	Sport       *string
	CourtType   *string
	IsActive    *bool
	Description *string
}

// Update applies a partial change to a court of this complex and records it.
// It reports ErrEditConflict when the row moved out from under the read,
// which is the one outcome the caller has to be told apart from a court that
// was never there.
func (s *Service) Update(ctx context.Context, complexID uuid.UUID, actor Actor, courtID uuid.UUID, in UpdateInput) (*courtstore.Court, error) {
	court, err := s.ownedCourt(ctx, complexID, courtID)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		court.Name = *in.Name
	}
	if in.Sport != nil {
		court.Sport = *in.Sport
	}
	if in.CourtType != nil {
		court.CourtType = *in.CourtType
	}
	if in.IsActive != nil {
		court.IsActive = *in.IsActive
	}
	if in.Description != nil {
		// An empty string is how a client clears a description, so it lands as
		// NULL rather than as a row holding "". Same fact, one representation.
		court.Description = emptyToNil(in.Description)
	}

	err = s.store.Update(ctx, court)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrEditConflict
		}
		return nil, err
	}

	s.record(complexID, actor, "update", "court", &court.ID, nil, court)
	return court, nil
}

// Delete soft-deletes a court of this complex.
//
// H-02: the existence test and the delete used to be two calls — a
// HasActiveBookingsByCourt check, then SoftDelete — with nothing serializing
// them. A booking committing in the gap between the two survived on a court
// the owner had just watched disappear from their own dashboard. SoftDelete
// asks and acts in one statement now, so there is no gap left for that booking
// to land in; courtstore.ErrCourtHasActiveBookings is the database's answer to
// the question this used to ask separately.
func (s *Service) Delete(ctx context.Context, complexID uuid.UUID, actor Actor, courtID uuid.UUID) error {
	if _, err := s.ownedCourt(ctx, complexID, courtID); err != nil {
		return err
	}

	if err := s.store.SoftDelete(ctx, courtID); err != nil {
		return err
	}

	s.record(complexID, actor, "delete", "court", &courtID, nil, nil)
	return nil
}

// PriceInput is one validated price band of an UpdatePrices request.
type PriceInput struct {
	Price    int
	DayType  string
	TimeFrom string
	TimeTo   string
}

// UpdatePrices replaces a court's whole price table.
//
// The band is replaced wholesale rather than merged, so the input is the
// court's complete price list — a partial update would leave the old bands in
// place alongside the new ones.
//
// H-07: the replacement used to be a DeletePricesByCourtID call followed by one
// InsertPrice per row, with no transaction around either — the delete committed
// on its own, and any insert failure after it answered a 4xx to the owner with
// the court's whole price table already gone. ReplacePrices wraps both halves
// in one transaction, so a refused write costs nothing.
//
// The returned index names the price the database refused, and is meaningful
// only alongside courtstore.ErrOverlappingPriceRule.
func (s *Service) UpdatePrices(ctx context.Context, complexID uuid.UUID, actor Actor, courtID uuid.UUID, in []PriceInput) (prices []*courtstore.CourtPrice, failedIndex int, err error) {
	if _, err = s.ownedCourt(ctx, complexID, courtID); err != nil {
		return nil, -1, err
	}

	prices = make([]*courtstore.CourtPrice, len(in))
	for i, p := range in {
		prices[i] = &courtstore.CourtPrice{
			CourtID:  courtID,
			Price:    p.Price,
			DayType:  p.DayType,
			TimeFrom: p.TimeFrom,
			TimeTo:   p.TimeTo,
		}
	}

	failedIndex, err = s.store.ReplacePrices(ctx, courtID, prices)
	if err != nil {
		return nil, failedIndex, err
	}

	s.record(complexID, actor, "update_prices", "court", &courtID, nil, prices)
	return prices, -1, nil
}

// BlockSlotInput is a validated request to take a court's hours off sale.
type BlockSlotInput struct {
	Date      time.Time
	StartTime string
	EndTime   string
	Reason    *string
	// CreatedBy is the owner taking the hours off sale, recorded on the row.
	CreatedBy uuid.UUID
}

// BlockSlot takes a time range off sale for maintenance or a private event.
//
// The booked-slot read before the insert is there for the message rather than
// for the invariant: it names the collision while the request still knows what
// the owner asked for. It runs outside any transaction, so a booking committing
// after it passes is not its business — InsertBlockedSlot asks the same
// question again under the court-day lock and answers the same sentinel.
func (s *Service) BlockSlot(ctx context.Context, complexID uuid.UUID, actor Actor, courtID uuid.UUID, in BlockSlotInput) (*courtstore.BlockedSlot, error) {
	court, err := s.ownedCourt(ctx, complexID, courtID)
	if err != nil {
		return nil, err
	}

	bookedSlots, err := s.bookings.GetBookedSlotsByCourtIDs(ctx, []uuid.UUID{courtID}, in.Date)
	if err != nil {
		return nil, err
	}

	// Compared as instants, not as times of day. A booking may end after
	// midnight, and "01:00" sorts below "23:00" — so a string comparison
	// reports no overlap for every candidate against such a booking, which
	// would let an owner block hours a client had already paid for.
	blockStart := slots.At(in.Date, in.StartTime)
	blockEnd := slots.At(in.Date, in.EndTime)
	for _, b := range bookedSlots {
		if slots.OverlapAt(blockStart, blockEnd, b.StartsAt, b.EndsAt) {
			return nil, courtstore.ErrSlotHasBooking
		}
	}

	slot := &courtstore.BlockedSlot{
		CourtID:   courtID,
		Date:      in.Date,
		StartTime: in.StartTime,
		EndTime:   in.EndTime,
		Reason:    in.Reason,
		CreatedBy: &in.CreatedBy,
	}

	if err := s.store.InsertBlockedSlot(ctx, slot); err != nil {
		return nil, err
	}

	// The court name is filled in before the entry is recorded, not after. It
	// used to be set on the next line, which left every audit row for a block
	// naming a court only by an id the row does not carry either — the reader
	// of the trail could not tell which court had been taken off sale without a
	// second query.
	slot.CourtName = court.Name

	s.record(complexID, actor, "create", "blocked_slot", &slot.ID, nil, slot)
	return slot, nil
}

// ListBlockedSlots returns the slots a complex has taken off sale over a date
// range.
//
// Pagination is opt-in (R3-blocked-slots-silent-truncation): a caller that
// names neither limit nor cursor gets every slot in the range, bounded only by
// the range cap the handler applies. A caller that names either one gets the
// limit/cursor/metadata envelope every other list endpoint already has.
func (s *Service) ListBlockedSlots(ctx context.Context, complexID uuid.UUID, dateFrom, dateTo time.Time, filters data.Filters, paginate bool) ([]*courtstore.BlockedSlot, data.Metadata, error) {
	slots, err := s.store.GetBlockedSlotsByComplex(ctx, complexID, dateFrom, dateTo)
	if err != nil {
		return nil, data.Metadata{}, err
	}
	if slots == nil {
		slots = []*courtstore.BlockedSlot{}
	}

	if !paginate {
		return slots, data.Metadata{}, nil
	}

	return trimBlockedSlotsPage(slots, filters)
}

// DeleteBlockedSlot puts a slot of this complex back on sale.
//
// H-10: the delete reports data.ErrRecordNotFound when it removed no row — a
// concurrent delete of the same slot, in particular, since the existence check
// above it already ran. Answering that rather than the success the winner gets
// is what lets a caller tell whether their own request deleted anything.
func (s *Service) DeleteBlockedSlot(ctx context.Context, complexID uuid.UUID, actor Actor, slotID uuid.UUID) error {
	slot, err := s.store.GetBlockedSlotByID(ctx, slotID)
	if err != nil {
		return err
	}

	// The slot belongs to this complex only if its court does.
	if _, err := s.ownedCourt(ctx, complexID, slot.CourtID); err != nil {
		return err
	}

	if err := s.store.DeleteBlockedSlot(ctx, slotID); err != nil {
		return err
	}

	s.record(complexID, actor, "delete", "blocked_slot", &slotID, slot, nil)
	return nil
}

// AvailabilitySlot is one bookable position on the grid.
type AvailabilitySlot struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	// Where this slot sits in its trading window, counted in minutes from the
	// window's own midnight and never wrapped: the 00:30 slot of a Thursday
	// 20:00-02:00 window is 1470, not 30.
	//
	// StartTime cannot answer that question. `slots.FromMinutes` takes the
	// value modulo a day to render a clock face, so a venue trading past
	// midnight publishes closing hours that read as the earliest of the day
	// and sort to the top of any list ordered by the string. This is the same
	// trap the booking lookup fell into — see the note on the overnight
	// comparison below, where '01:00' read as earlier than '23:00' and the
	// hours a client had paid for went back on sale. A consumer that needs
	// chronological order sorts on this and never re-derives it from a clock.
	StartMin        int  `json:"start_min"`
	DurationMinutes int  `json:"duration_minutes"`
	Price           int  `json:"price"`
	Available       bool `json:"available"`
}

// CourtAvailability is one court's row of the grid.
type CourtAvailability struct {
	CourtID   string `json:"court_id"`
	CourtName string `json:"court_name"`
	Sport     string `json:"sport"`
	CourtType string `json:"court_type"`
	// What the court is like, when the owner has said. Omitted rather than
	// sent empty so the storefront renders nothing at all for a court that
	// has never been described, instead of an empty line where text goes.
	Description *string            `json:"description,omitempty"`
	Slots       []AvailabilitySlot `json:"slots"`
}

// AvailabilityResponse is the bookable grid for one day.
type AvailabilityResponse struct {
	Date   string              `json:"date"`
	Day    string              `json:"day"`
	IsOpen bool                `json:"is_open"`
	Courts []CourtAvailability `json:"courts"`
}

// Availability computes the bookable grid for one day: the complex's open
// hours, minus what is unpriced, booked, blocked, or already in the past.
//
// A complex that does not exist and one that has been deactivated answer the
// same data.ErrRecordNotFound: the booking write refuses a deactivated venue,
// so publishing a live grid for it can only end in a client picking a slot and
// being answered with a bare 404.
//
// It is one cohesive read — load the schedule, courts and prices, compute the
// slots per court — and splitting it would relocate sequential steps into
// helpers without reducing what a reader holds at once.
//
//nolint:funlen // see the cohesion note above
func (s *Service) Availability(ctx context.Context, slug, dateStr string, date time.Time, duration int) (*AvailabilityResponse, error) {
	complex, err := s.complexes.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if !complex.IsActive {
		return nil, data.ErrRecordNotFound
	}

	dayName := slots.DayName(date.Weekday())

	schedules, err := s.complexes.GetSchedules(ctx, complex.ID)
	if err != nil {
		return nil, err
	}

	var schedule *complexstore.Schedule
	for _, sc := range schedules {
		if sc.Day == dayName {
			schedule = sc
			break
		}
	}

	if schedule == nil || schedule.IsClosed {
		return &AvailabilityResponse{
			Date:   dateStr,
			Day:    dayName,
			IsOpen: false,
			Courts: []CourtAvailability{},
		}, nil
	}

	courts, err := s.store.GetByComplex(ctx, complex.ID)
	if err != nil {
		return nil, err
	}

	// Filter only active courts.
	activeCourts := make([]*courtstore.Court, 0, len(courts))
	for _, c := range courts {
		if c.IsActive {
			activeCourts = append(activeCourts, c)
		}
	}

	// No active courts → an open day with an empty courts list.
	if len(activeCourts) == 0 {
		return &AvailabilityResponse{
			Date:   dateStr,
			Day:    dayName,
			IsOpen: true,
			Courts: []CourtAvailability{},
		}, nil
	}

	// Batch-fetch all prices, bookings, and blocked slots (3 queries total).
	courtIDs := make([]uuid.UUID, len(activeCourts))
	for i, c := range activeCourts {
		courtIDs[i] = c.ID
	}

	allPrices, err := s.store.GetPricesByCourtIDs(ctx, courtIDs)
	if err != nil {
		return nil, err
	}

	allBookedSlots, err := s.bookings.GetBookedSlotsByCourtIDs(ctx, courtIDs, date)
	if err != nil {
		return nil, err
	}

	allBlockedSlots, err := s.store.GetBlockedSlotsByCourtIDs(ctx, courtIDs, date)
	if err != nil {
		return nil, err
	}

	return &AvailabilityResponse{
		Date:   dateStr,
		Day:    dayName,
		IsOpen: true,
		Courts: buildGrid(activeCourts, schedule, dayName, date, duration, allPrices, allBookedSlots, allBlockedSlots),
	}, nil
}

// buildGrid turns one day's courts, prices and obstacles into the published
// grid. Split out of Availability so the fetch and the arithmetic can each be
// read on their own.
//
//nolint:funlen // one cohesive computation over the batch-fetched day
func buildGrid(
	activeCourts []*courtstore.Court,
	schedule *complexstore.Schedule,
	dayName string,
	date time.Time,
	duration int,
	allPrices []*courtstore.CourtPrice,
	allBookedSlots []bookingstore.BookedSpan,
	allBlockedSlots []*courtstore.BlockedSlot,
) []CourtAvailability {
	// Index data by court ID for O(1) lookups.
	pricesByCourtID := make(map[uuid.UUID][]*courtstore.CourtPrice, len(activeCourts))
	for _, p := range allPrices {
		pricesByCourtID[p.CourtID] = append(pricesByCourtID[p.CourtID], p)
	}
	bookedByCourtID := make(map[uuid.UUID][]bookingstore.BookedSpan, len(activeCourts))
	for _, b := range allBookedSlots {
		bookedByCourtID[b.CourtID] = append(bookedByCourtID[b.CourtID], b)
	}
	blockedByCourtID := make(map[uuid.UUID][]*courtstore.BlockedSlot, len(activeCourts))
	for _, b := range allBlockedSlots {
		blockedByCourtID[b.CourtID] = append(blockedByCourtID[b.CourtID], b)
	}

	now := time.Now().In(timezone.Argentina)
	isToday := date.Year() == now.Year() && date.YearDay() == now.YearDay()
	currentTime := now.Format("15:04")

	courtResults := make([]CourtAvailability, 0, len(activeCourts))

	for _, court := range activeCourts {
		bookedSlots := bookedByCourtID[court.ID]
		blockedSlots := blockedByCourtID[court.ID]

		// The grid is derived, not generated here: slots.NewGrid is the same
		// value the booking handlers validate a write against, so this loop
		// cannot offer a position that the write path would refuse. Every
		// start it yields ends before midnight, which is also why the
		// lexicographic comparisons below are safe — no time string wraps.
		grid := slots.NewGrid(schedule.OpenTime, schedule.CloseTime)
		gridSlots := grid.Slots(duration)

		courtSlots := make([]AvailabilitySlot, 0, len(gridSlots))
		for _, gs := range gridSlots {
			// pricing.BookingPrice is the same function the booking handlers
			// charge from. A booking no rule fully covers is not on sale, so
			// it is omitted rather than published at 0 — the storefront would
			// otherwise advertise a price the write path never charges.
			//
			// `dayName` and `gs.StartMin` name the window this slot came out
			// of and where in it the slot sits. For a venue trading past
			// midnight that is not the calendar day: the 00:30 slot of a
			// Thursday 08:00-01:30 window is Thursday minute 1470, and pays
			// Thursday's rate.
			price, priceErr := pricing.BookingPrice(pricesByCourtID[court.ID], dayName, gs.StartMin, duration)
			if priceErr != nil {
				continue
			}

			// Both kinds of obstacle are compared as instants. The grid emits
			// times of day, but a booking already on the books may run past
			// midnight, and "01:00" reads as earlier than "23:00" under a
			// string comparison — so such a booking matched nothing and the
			// hours a client had paid for went back on sale.
			//
			// The end is derived by adding the duration rather than reading
			// gs.End, so it stays right if the grid is ever allowed to emit a
			// slot whose end lands on the following day.
			slotStart := slots.At(date, gs.Start)
			slotEnd := slotStart.Add(time.Duration(duration) * time.Minute)

			available := slotIsFree(slotStart, slotEnd, date, bookedSlots, blockedSlots)

			// Check if slot is in the past (if today).
			if available && isToday && gs.Start <= currentTime {
				available = false
			}

			courtSlots = append(courtSlots, AvailabilitySlot{
				StartTime:       gs.Start,
				EndTime:         gs.End,
				StartMin:        gs.StartMin,
				DurationMinutes: duration,
				Price:           price,
				Available:       available,
			})
		}

		courtResults = append(courtResults, CourtAvailability{
			CourtID:     court.ID.String(),
			CourtName:   court.Name,
			Sport:       court.Sport,
			CourtType:   court.CourtType,
			Description: court.Description,
			Slots:       courtSlots,
		})
	}

	return courtResults
}

// slotIsFree reports whether a grid position collides with nothing already on
// the books and nothing the owner has taken off sale.
func slotIsFree(slotStart, slotEnd, date time.Time, booked []bookingstore.BookedSpan, blocked []*courtstore.BlockedSlot) bool {
	for _, b := range booked {
		if slots.OverlapAt(slotStart, slotEnd, b.StartsAt, b.EndsAt) {
			return false
		}
	}
	for _, b := range blocked {
		// Placed on `date` rather than on b.Date: the query that fetched these
		// was scoped to this day, so they are the same calendar date, and
		// `date` is the one already anchored in the product's wall-clock.
		// Anchoring the two sides of a comparison differently is a three-hour
		// error that looks like nothing.
		//
		// A blocked slot cannot itself cross midnight — blocked_slots has
		// carried CHECK (start_time < end_time) since the initial schema — so
		// its two times of day belong to that date and placing them there is
		// exact.
		if slots.OverlapAt(slotStart, slotEnd, slots.At(date, b.StartTime), slots.At(date, b.EndTime)) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Cross-domain reads
//
// These proxy the store with its exact signature. They exist so another
// domain's entry point into courts is this service rather than the court
// store, which is what lets a rule added later (authorization, caching) land
// in one place.
// ---------------------------------------------------------------------------

// GetByID returns one court. Exported for bookings and payments.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*courtstore.Court, error) {
	return s.store.GetByID(ctx, id)
}

// GetByComplex returns a complex's courts. Exported for complexes and
// reporting.
func (s *Service) GetByComplex(ctx context.Context, complexID uuid.UUID) ([]*courtstore.Court, error) {
	return s.store.GetByComplex(ctx, complexID)
}

// GetPrices returns one court's price bands. Exported for bookings.
func (s *Service) GetPrices(ctx context.Context, courtID uuid.UUID) ([]*courtstore.CourtPrice, error) {
	return s.store.GetPrices(ctx, courtID)
}

// GetPricesByCourtIDs returns the price bands of several courts at once.
// Exported for complexes.
func (s *Service) GetPricesByCourtIDs(ctx context.Context, courtIDs []uuid.UUID) ([]*courtstore.CourtPrice, error) {
	return s.store.GetPricesByCourtIDs(ctx, courtIDs)
}

// GetBlockedSlots returns one court's blocked slots for a date. Exported for
// bookings.
func (s *Service) GetBlockedSlots(ctx context.Context, courtID uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error) {
	return s.store.GetBlockedSlots(ctx, courtID, date)
}

// GetBlockedSlotsByCourtIDs returns the blocked slots of several courts at
// once. Exported for bookings.
func (s *Service) GetBlockedSlotsByCourtIDs(ctx context.Context, courtIDs []uuid.UUID, date time.Time) ([]*courtstore.BlockedSlot, error) {
	return s.store.GetBlockedSlotsByCourtIDs(ctx, courtIDs, date)
}

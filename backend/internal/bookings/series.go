// series.go — fixed weekly bookings ("turnos fijos").
//
// A series is a weekly RULE: this court, this hour, this person, every week,
// with no horizon. Nothing is booked ahead. Every date the rule matches holds
// its court and hours exactly as a booking would — the storefront's grid, the
// owner's guards and the owner's calendar all read the dates through one
// expansion (store.SeriesOccurrences / the SQL function it wraps) — until the
// date is settled in the ledger (booking_series_dates):
//
//   - on the venue's own day the materialization job turns it into an ordinary
//     booking (series_id set), so the cash desk, the reports and the reminders
//     work on it unchanged; or
//   - the owner cancels that one date, or it turns out it cannot be booked, and
//     it becomes an exception whose hours are free.
//
// Every booking goes through the same prepareCreate an owner-typed booking does,
// so validation and pricing have one implementation. The two things that differ
// are deliberate: an occurrence sends the client no confirmation (a weekly
// "reserva confirmada" would be spam, S8), and the store records its date as
// settled in the same transaction that writes it (at most once ever).
package bookings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist"
)

// Series is the domain type of a fixed weekly series.
type Series = bookingstore.Series

const (
	// MaxSeriesWeeks bounds the "weeks" shorthand.
	MaxSeriesWeeks = 156
	// seriesUpcomingCount is how many next dates the series detail lists.
	seriesUpcomingCount = 8
	// earlyMorningCutoff is the start of day an occurrence of tomorrow still
	// counts as due today. See occurrenceDue.
	earlyMorningCutoff = "06:00"
)

// Reasons a date is an exception, as the ledger and the API report them.
const (
	ReasonCancelledByOwner = bookingstore.ReasonCancelledByOwner
	ReasonOverlap          = bookingstore.ReasonOverlap
	ReasonBlocked          = bookingstore.ReasonBlocked
	ReasonClosedDay        = bookingstore.ReasonClosedDay
	ReasonNoPrice          = bookingstore.ReasonNoPrice
	ReasonClientBlocked    = bookingstore.ReasonClientBlocked
	ReasonMissed           = bookingstore.ReasonMissed
)

// States of an upcoming date in a series' detail.
const (
	SeriesStatePending   = "pending"
	SeriesStateBooked    = "booked"
	SeriesStateException = "exception"
)

// fixedBookingMessage is what the owner's write guards answer with when the
// hours belong to a fixed weekly booking that has no booking row to name.
const fixedBookingMessage = "the selected time belongs to a fixed weekly booking (turno fijo)"

// SeriesStore is the persistence a series needs beyond the booking store's.
type SeriesStore interface {
	InsertSeries(ctx context.Context, s *Series, exceptions []bookingstore.SeriesException) error
	GetSeries(ctx context.Context, id uuid.UUID) (*Series, error)
	EndExpiredSeries(ctx context.Context, day time.Time) (int64, error)
	EndSeries(ctx context.Context, id uuid.UUID, endsOn *time.Time, status string) (*Series, error)
	SeriesDates(ctx context.Context, seriesID uuid.UUID, from, to time.Time) ([]bookingstore.SeriesDate, error)
	RecordSeriesException(ctx context.Context, s *Series, date time.Time, reason string) (bool, error)
	SeriesBookingsFrom(ctx context.Context, seriesID uuid.UUID, from time.Time) ([]*bookingstore.Booking, error)
}

// SeriesConflict is a date of a series that is not part of it, and why.
type SeriesConflict struct {
	Date   time.Time
	Reason string
}

// SeriesConflictsError refuses a series whose dates collide with something that
// already holds them, when the owner did not ask to skip them. Nothing was
// written.
type SeriesConflictsError struct {
	Conflicts []SeriesConflict
}

func (e *SeriesConflictsError) Error() string {
	return fmt.Sprintf("%d dates of the series cannot be booked", len(e.Conflicts))
}

// SeriesInput is a validated request to create a fixed weekly booking. The
// weekday is the one FirstDate falls on.
type SeriesInput struct {
	CourtID         uuid.UUID
	FirstDate       time.Time
	StartTime       string
	DurationMinutes int
	ClientFirstName string
	ClientLastName  string
	ClientPhone     string
	ClientEmail     string
	Notes           string
	Price           *int
	// EndsOn and Weeks both bound the series; at most one is set. Neither means
	// the series has no end.
	EndsOn        *time.Time
	Weeks         int
	SkipConflicts bool
}

// SeriesResult is what creating a series produced. Bookings is empty unless the
// first date was already due, in which case its booking was created at once.
type SeriesResult struct {
	Series   *Series
	Bookings []*bookingstore.Booking
	Skipped  []SeriesConflict
}

// SeriesUpcoming is one upcoming date of a series and where it stands.
type SeriesUpcoming struct {
	Date  time.Time
	State string
	// BookingID names the booking of a booked date.
	BookingID *uuid.UUID
	// Reason says why an exception date is one.
	Reason *string
}

// SeriesDetail is a series with its next dates.
type SeriesDetail struct {
	Series   *Series
	Upcoming []SeriesUpcoming
}

// SeriesEndResult is what ending a series from a date did.
type SeriesEndResult struct {
	Series    *Series
	Cancelled []*bookingstore.Booking
	// Kept are the bookings that already carry a payment and so stand.
	Kept []*bookingstore.Booking
}

// seriesDay is a calendar date as the series reasons about it: midnight UTC
// carrying the date's fields, the way a DATE column decodes.
func seriesDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// today is the venue's current date, as a series reasons about dates.
func (s *Service) today() time.Time { return seriesDay(s.now().In(timezone.Argentina)) }

// ruleDates lists the dates the series' rule matches within [from, to], never
// before its start or after its end. It is the Go twin of the SQL function
// booking_series_occurrences minus the ledger; the integration tests hold the
// two to the same answer.
func ruleDates(s *Series, from, to time.Time) []time.Time {
	start := seriesDay(from)
	if s.StartsOn.After(start) {
		start = seriesDay(s.StartsOn)
	}
	end := seriesDay(to)
	if s.EndsOn != nil && seriesDay(*s.EndsOn).Before(end) {
		end = seriesDay(*s.EndsOn)
	}
	offset := (s.Weekday - int(start.Weekday()) + 7) % 7
	var out []time.Time
	for d := start.AddDate(0, 0, offset); !d.After(end); d = d.AddDate(0, 0, 7) {
		out = append(out, d)
	}
	return out
}

// onRule reports whether date is one the series' rule matches.
func onRule(s *Series, date time.Time) bool {
	return len(ruleDates(s, date, date)) == 1
}

// closedDay reports whether the venue has no window selling this hour: neither
// last night's window running past midnight nor the date's own. Outside opening
// hours on an open day is not a conflict, as for any owner booking.
func closedDay(schedules []*complexstore.Schedule, date time.Time, startTime string) bool {
	if grid, ok := gridForDay(schedules, slots.DayName(date.AddDate(0, 0, -1).Weekday())); ok {
		if _, owned := grid.OwnsNextDay(startTime); owned {
			return false
		}
	}
	_, ok := gridForDay(schedules, slots.DayName(date.Weekday()))
	return !ok
}

// seriesOccurrence is the CreateInput of one occurrence of s.
func seriesOccurrence(s *Series, client *clientstore.Client, date time.Time) CreateInput {
	in := CreateInput{
		CourtID:         s.CourtID,
		Date:            date,
		StartTime:       s.StartTime,
		DurationMinutes: s.DurationMinutes,
		Price:           s.Price,
		seriesID:        &s.ID,
		client:          client,
	}
	if s.Notes != nil {
		in.Notes = *s.Notes
	}
	return in
}

// heldByFixedBooking reports whether a fixed weekly series holds any of the
// candidate's hours on its court. own is the series the candidate is itself an
// occurrence of, whose own date is not a clash.
//
// The owner's booking, the series' own occurrences and the storefront's
// pre-check ask it; the insert-time guard (store.SlotTaken) asks the same
// question in SQL, under the court-day lock, for the writers that race.
func (s *Service) heldByFixedBooking(
	ctx context.Context, courtID uuid.UUID, date time.Time, startAt, endAt time.Time, own *uuid.UUID,
) (bool, error) {
	occs, err := s.store.SeriesOccurrences(ctx, nil, []uuid.UUID{courtID}, date.AddDate(0, 0, -1), date.AddDate(0, 0, 1))
	if err != nil {
		return false, err
	}
	day := seriesDay(date)
	for _, o := range occs {
		if own != nil && o.SeriesID == *own && seriesDay(o.Date).Equal(day) {
			continue
		}
		if slots.OverlapAt(startAt, endAt, o.StartsAt, o.EndsAt) {
			return true, nil
		}
	}
	return false, nil
}

// occurrenceConflict maps what writing an occurrence refused to the reason a
// date becomes an exception. ok is false for any other failure, which is a fault
// to retry rather than a date to give up on.
func occurrenceConflict(err error) (reason string, ok bool) {
	var conflict *ConflictError
	var field *FieldError
	switch {
	case errors.Is(err, bookingstore.ErrDuplicateBooking), errors.Is(err, bookingstore.ErrSlotUnavailable):
		return ReasonOverlap, true
	case errors.As(err, &conflict):
		switch conflict.Message {
		case fixedBookingMessage:
			return ReasonOverlap, true
		case blockedSlotMessage:
			return ReasonBlocked, true
		}
		return ReasonClientBlocked, true
	case errors.As(err, &field) && field.Field == "price":
		return ReasonNoPrice, true
	case errors.Is(err, data.ErrRecordNotFound):
		// The court was removed since the series was made.
		return ReasonBlocked, true
	}
	return "", false
}

// bookOccurrence writes one occurrence, without telling the client. It returns
// nil, nil when the date was already settled.
func (s *Service) bookOccurrence(ctx context.Context, complex *complexstore.Complex, ser *Series, client *clientstore.Client, date time.Time) (*bookingstore.Booking, error) {
	createdBy := ser.CreatedBy
	p, err := s.prepareCreate(ctx, complex, Actor{UserID: &createdBy}, seriesOccurrence(ser, client, date))
	if err != nil {
		return nil, err
	}
	if err := s.store.InsertSafeWithPayment(ctx, p.booking, p.counter); err != nil {
		if errors.Is(err, bookingstore.ErrSeriesDateGenerated) {
			return nil, nil //nolint:nilnil // the date was settled already: nothing to add, nothing wrong
		}
		return nil, err
	}
	return p.booking, nil
}

// occurrenceDue reports whether a date of a series should have its booking
// created at now.
//
// The booking is created on the venue's own day: every date of today or earlier
// (earlier only when the job was down), plus tomorrow's dates that start before
// 06:00. Those early ones are created the evening before so the 2-hour reminder
// can still fire: it needs the booking to be more than two hours old when the
// game is two hours away (store.GetForReminder2hEnriched), and an occurrence at
// 00:30 created by the job's first run after midnight would be 30 minutes old.
// Occurrences from 06:00 on are created at the first run of their day, five or
// more hours ahead. startTime is "HH:MM", which orders lexicographically.
func occurrenceDue(date time.Time, startTime string, now time.Time) bool {
	t := seriesDay(now.In(timezone.Argentina))
	d := seriesDay(date)
	switch {
	case !d.After(t):
		return true
	case d.Equal(t.AddDate(0, 0, 1)):
		return startTime < earlyMorningCutoff
	}
	return false
}

// settleDate turns one due date of a series into its booking, or into an
// exception with the reason it cannot be one. It returns the booking made (nil
// when the date became an exception or was settled already) and the reason when
// it became an exception. Any other failure is returned: the date stays held and
// the next run retries it.
func (s *Service) settleDate(
	ctx context.Context,
	complex *complexstore.Complex,
	ser *Series,
	client *clientstore.Client,
	schedules []*complexstore.Schedule,
	date time.Time,
	now time.Time,
) (*bookingstore.Booking, string, error) {
	reason := ""
	startAt := slots.At(timezone.Day(date), ser.StartTime)
	switch {
	case !startAt.Add(time.Duration(ser.DurationMinutes) * time.Minute).After(now):
		reason = ReasonMissed
	case closedDay(schedules, date, ser.StartTime):
		reason = ReasonClosedDay
	default:
		booking, err := s.bookOccurrence(ctx, complex, ser, client, date)
		if err == nil {
			return booking, "", nil
		}
		var ok bool
		if reason, ok = occurrenceConflict(err); !ok {
			return nil, "", err
		}
	}
	if _, err := s.series.RecordSeriesException(ctx, ser, date, reason); err != nil {
		return nil, "", err
	}
	return nil, reason, nil
}

// seriesTermination validates a request's bounds and returns the series' last
// day, nil for none.
func seriesTermination(in SeriesInput, first time.Time) (*time.Time, error) {
	switch {
	case in.EndsOn != nil && in.Weeks > 0:
		return nil, &FieldError{Field: "weeks", Message: "send either ends_on or weeks, not both"}
	case in.EndsOn != nil:
		end := seriesDay(*in.EndsOn)
		if end.Before(first) {
			return nil, &FieldError{Field: "ends_on", Message: "must not be before first_date"}
		}
		return &end, nil
	case in.Weeks > 0:
		end := first.AddDate(0, 0, 7*(in.Weeks-1))
		return &end, nil
	}
	return nil, nil //nolint:nilnil // no bound: the series has no end
}

// newSeries validates a creation request against the court and the clock and
// builds the series it asks for, without touching the client or the store.
func (s *Service) newSeries(ctx context.Context, complex *complexstore.Complex, actor Actor, in SeriesInput) (*Series, error) {
	court, err := s.courts.GetByID(ctx, in.CourtID)
	if err != nil {
		return nil, err
	}
	if court.ComplexID != complex.ID {
		return nil, data.ErrRecordNotFound
	}
	if !slots.OnGridStep(in.StartTime) {
		return nil, &FieldError{Field: "start_time", Message: offBoundaryMessage}
	}
	first := seriesDay(in.FirstDate)
	if first.Before(s.today()) {
		return nil, &FieldError{Field: "first_date", Message: "must not be in the past"}
	}
	if first.After(s.today().AddDate(0, 0, bookingstore.MaxBookingHorizonDays)) {
		return nil, &FieldError{Field: "first_date", Message: "is too far in the future"}
	}
	firstEnd := slots.At(timezone.Day(first), in.StartTime).Add(time.Duration(in.DurationMinutes) * time.Minute)
	if !firstEnd.After(s.now()) {
		return nil, &FieldError{Field: "first_date", Message: "the first occurrence is already over"}
	}
	endsOn, err := seriesTermination(in, first)
	if err != nil {
		return nil, err
	}
	var notes *string
	if in.Notes != "" {
		notes = &in.Notes
	}
	return &Series{
		ComplexID:       complex.ID,
		CourtID:         in.CourtID,
		Weekday:         int(first.Weekday()),
		StartTime:       in.StartTime,
		DurationMinutes: in.DurationMinutes,
		Price:           in.Price,
		Notes:           notes,
		StartsOn:        first,
		EndsOn:          endsOn,
		Status:          bookingstore.SeriesStatusActive,
		CreatedBy:       *actor.UserID,
	}, nil
}

// seriesWideChecks refuses a series that could never book any date: a weekday the
// venue is closed, or an hour no price rule covers when no manual price is given.
// Those hold for every date, so they refuse the series rather than listing every
// date as a conflict.
func (s *Service) seriesWideChecks(ctx context.Context, ser *Series) error {
	schedules, err := s.complexes.GetSchedules(ctx, ser.ComplexID)
	if err != nil {
		return err
	}
	if closedDay(schedules, ser.StartsOn, ser.StartTime) {
		return &FieldError{Field: "first_date", Message: closedMessage}
	}
	if ser.Price == nil {
		if _, err := s.findPrice(ctx, ser.ComplexID, ser.CourtID, ser.StartsOn, ser.StartTime, ser.DurationMinutes); err != nil {
			if errors.Is(err, pricing.ErrNoPriceRule) {
				return &FieldError{Field: "price", Message: httpx.CodePriceRequired}
			}
			return err
		}
	}
	return nil
}

// seriesConflicts lists the dates of a new series that something already holds:
// a booking, another series, or a blocked slot, up to the booking horizon. It
// reads in ranges — one booking read and one blocked-slot read for the whole
// series, not one per date.
func (s *Service) seriesConflicts(ctx context.Context, ser *Series) ([]SeriesConflict, error) {
	horizon := s.today().AddDate(0, 0, bookingstore.MaxBookingHorizonDays)
	dates := ruleDates(ser, ser.StartsOn, horizon)
	if len(dates) == 0 {
		return nil, nil
	}
	first, last := dates[0], dates[len(dates)-1]
	spans, err := s.store.BookedSpansBetween(ctx, []uuid.UUID{ser.CourtID}, first, last.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	blocks, err := s.courts.GetBlockedSlotsByComplex(ctx, ser.ComplexID, first, last.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	var out []SeriesConflict
	for _, d := range dates {
		startAt := slots.At(timezone.Day(d), ser.StartTime)
		endAt := startAt.Add(time.Duration(ser.DurationMinutes) * time.Minute)
		switch {
		case blockedBySlots(blocks, ser.CourtID, d, startAt, endAt):
			out = append(out, SeriesConflict{Date: d, Reason: ReasonBlocked})
		case heldBySpans(spans, ser.CourtID, startAt, endAt):
			out = append(out, SeriesConflict{Date: d, Reason: ReasonOverlap})
		}
	}
	return out, nil
}

func heldBySpans(spans []bookingstore.BookedSpan, courtID uuid.UUID, startAt, endAt time.Time) bool {
	return slices.ContainsFunc(spans, func(sp bookingstore.BookedSpan) bool {
		return sp.CourtID == courtID && slots.OverlapAt(startAt, endAt, sp.StartsAt, sp.EndsAt)
	})
}

// blockedBySlots is slotIsBlocked over blocks already loaded: the candidate
// starting on date may meet a block filed that day or, when it runs past
// midnight, the next.
func blockedBySlots(blocks []*courtstore.BlockedSlot, courtID uuid.UUID, date time.Time, startAt, endAt time.Time) bool {
	for _, day := range blockedDays(date, endAt) {
		for _, b := range blocks {
			if b.CourtID == courtID && seriesDay(b.Date).Equal(seriesDay(day)) &&
				slots.OverlapAt(startAt, endAt, slots.At(day, b.StartTime), slots.At(day, b.EndTime)) {
				return true
			}
		}
	}
	return false
}

// CreateSeries creates a fixed weekly booking. It refuses, writing nothing,
// when dates it would hold are already held and the owner did not ask to skip
// them. No booking is made ahead: the series holds its dates as a rule. When its
// first date is already due (today, or tomorrow's early morning), that one
// booking is created at once.
func (s *Service) CreateSeries(ctx context.Context, complex *complexstore.Complex, actor Actor, in SeriesInput) (*SeriesResult, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
	ser, err := s.newSeries(ctx, complex, actor, in)
	if err != nil {
		return nil, err
	}
	if err := s.seriesWideChecks(ctx, ser); err != nil {
		return nil, err
	}
	conflicts, err := s.seriesConflicts(ctx, ser)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 && !in.SkipConflicts {
		return nil, &SeriesConflictsError{Conflicts: conflicts}
	}

	client, err := s.clients.GetOrCreate(ctx, complex.ID, in.ClientFirstName, in.ClientLastName, in.ClientPhone, in.ClientEmail, true)
	if err != nil {
		return nil, err
	}
	if client.IsBlocked {
		return nil, &ConflictError{Message: "client is blocked, cannot create booking"}
	}
	ser.ClientID = client.ID

	exceptions := make([]bookingstore.SeriesException, len(conflicts))
	for i, c := range conflicts {
		exceptions[i] = bookingstore.SeriesException{Date: c.Date, Reason: c.Reason}
	}
	if err := s.series.InsertSeries(ctx, ser, exceptions); err != nil {
		if errors.Is(err, bookingstore.ErrSlotUnavailable) {
			return nil, &ConflictError{Message: "a booking was made on one of the series' dates while it was being created, please try again"}
		}
		return nil, err
	}
	s.audit.Record(audit.Entry{
		UserID: actor.UserID, ComplexID: &complex.ID, Action: "create",
		EntityType: "booking_series", EntityID: &ser.ID, NewValue: ser, IPAddress: actor.IP,
	})

	result := &SeriesResult{Series: ser, Skipped: conflicts}
	if err := s.settleDueFirstDate(ctx, complex, ser, client, result); err != nil {
		return nil, err
	}
	s.notifySeriesCreated(ctx, complex, ser, client)
	s.realtime.PublishBookingChanged(complex.ID)
	return result, nil
}

// settleDueFirstDate creates the booking of the series' first date when it is
// already due, recording a date that cannot be booked as skipped.
func (s *Service) settleDueFirstDate(ctx context.Context, complex *complexstore.Complex, ser *Series, client *clientstore.Client, result *SeriesResult) error {
	if slices.ContainsFunc(result.Skipped, func(c SeriesConflict) bool { return c.Date.Equal(ser.StartsOn) }) {
		return nil
	}
	now := s.now()
	if !occurrenceDue(ser.StartsOn, ser.StartTime, now) {
		return nil
	}
	schedules, err := s.complexes.GetSchedules(ctx, ser.ComplexID)
	if err != nil {
		return err
	}
	booking, reason, err := s.settleDate(ctx, complex, ser, client, schedules, ser.StartsOn, now)
	if err != nil {
		return err
	}
	if booking != nil {
		result.Bookings = append(result.Bookings, booking)
	}
	if reason != "" {
		result.Skipped = append(result.Skipped, SeriesConflict{Date: ser.StartsOn, Reason: reason})
	}
	return nil
}

// ownedSeries loads a series of the complex, or answers not found.
func (s *Service) ownedSeries(ctx context.Context, complexID, seriesID uuid.UUID) (*Series, error) {
	ser, err := s.series.GetSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if ser.ComplexID != complexID {
		return nil, data.ErrRecordNotFound
	}
	return ser, nil
}

// GetSeries returns a series with its next dates, computed from the rule: each
// is pending (holding its hours, not booked yet), booked (with its booking), or
// an exception (with why).
func (s *Service) GetSeries(ctx context.Context, complexID, seriesID uuid.UUID) (*SeriesDetail, error) {
	ser, err := s.ownedSeries(ctx, complexID, seriesID)
	if err != nil {
		return nil, err
	}
	from := s.today()
	dates := ruleDates(ser, from, from.AddDate(0, 0, 7*seriesUpcomingCount-1))
	detail := &SeriesDetail{Series: ser, Upcoming: make([]SeriesUpcoming, 0, len(dates))}
	if len(dates) == 0 || ser.Status != bookingstore.SeriesStatusActive {
		return detail, nil
	}
	ledger, err := s.series.SeriesDates(ctx, ser.ID, dates[0], dates[len(dates)-1])
	if err != nil {
		return nil, err
	}
	bookings, err := s.series.SeriesBookingsFrom(ctx, ser.ID, dates[0])
	if err != nil {
		return nil, err
	}
	settled := make(map[string]bookingstore.SeriesDate, len(ledger))
	for _, l := range ledger {
		settled[l.Date.Format(time.DateOnly)] = l
	}
	bookingOn := make(map[string]uuid.UUID, len(bookings))
	for _, b := range bookings {
		bookingOn[b.Date.Format(time.DateOnly)] = b.ID
	}
	for _, d := range dates {
		key := d.Format(time.DateOnly)
		up := SeriesUpcoming{Date: d, State: SeriesStatePending}
		if l, ok := settled[key]; ok {
			if l.Outcome == bookingstore.SeriesDateBooked {
				up.State = SeriesStateBooked
				if id, found := bookingOn[key]; found {
					up.BookingID = &id
				}
			} else {
				up.State, up.Reason = SeriesStateException, l.Reason
			}
		}
		detail.Upcoming = append(detail.Upcoming, up)
	}
	return detail, nil
}

// SkipSeriesDate cancels one future date of a series: the date becomes an
// exception, and its hours are free. A date that already has its booking is
// cancelled through that booking instead, and a date already excepted is
// refused.
func (s *Service) SkipSeriesDate(ctx context.Context, complex *complexstore.Complex, actor Actor, seriesID uuid.UUID, date time.Time) (*SeriesConflict, error) {
	ser, err := s.ownedSeries(ctx, complex.ID, seriesID)
	if err != nil {
		return nil, err
	}
	day := seriesDay(date)
	switch {
	case day.Before(s.today()):
		return nil, &FieldError{Field: "date", Message: "must not be in the past"}
	case ser.Status != bookingstore.SeriesStatusActive || !onRule(ser, day):
		return nil, &FieldError{Field: "date", Message: "is not a date of this series"}
	}
	recorded, err := s.series.RecordSeriesException(ctx, ser, day, ReasonCancelledByOwner)
	if err != nil {
		return nil, err
	}
	if !recorded {
		return nil, s.settledDateRefusal(ctx, ser, day)
	}
	skipped := &SeriesConflict{Date: day, Reason: ReasonCancelledByOwner}
	s.audit.Record(audit.Entry{
		UserID: actor.UserID, ComplexID: &complex.ID, Action: "skip_date",
		EntityType: "booking_series", EntityID: &ser.ID, NewValue: skipped, IPAddress: actor.IP,
	})
	s.realtime.PublishBookingChanged(complex.ID)
	return skipped, nil
}

// settledDateRefusal says why a date that already has a ledger row cannot be
// skipped.
func (s *Service) settledDateRefusal(ctx context.Context, ser *Series, day time.Time) error {
	ledger, err := s.series.SeriesDates(ctx, ser.ID, day, day)
	if err != nil {
		return err
	}
	if len(ledger) == 1 && ledger[0].Outcome == bookingstore.SeriesDateBooked {
		return &FieldError{Field: "date", Message: "already has a booking, cancel that booking instead"}
	}
	return &FieldError{Field: "date", Message: "was already cancelled"}
}

// EndSeries ends a series from a date: the series stops holding its dates from
// then on (ends_on becomes the day before), and the bookings it already created
// for dates on or after it are dealt with — those nobody has paid for are
// cancelled, those with a payment stand and are reported back. The series is
// marked ended either way; a kept booking is an ordinary booking from here.
//
// Cancelling the individual bookings is quiet and involves no refund, since only
// unpaid ones are touched; the client gets one notice for the whole series
// instead (notifySeriesCancelled).
func (s *Service) EndSeries(ctx context.Context, complex *complexstore.Complex, actor Actor, seriesID uuid.UUID, from time.Time) (*SeriesEndResult, error) {
	ser, err := s.ownedSeries(ctx, complex.ID, seriesID)
	if err != nil {
		return nil, err
	}
	from = seriesDay(from)
	if from.Before(s.today()) {
		return nil, &FieldError{Field: "from_date", Message: "must not be in the past"}
	}

	created, err := s.series.SeriesBookingsFrom(ctx, ser.ID, from)
	if err != nil {
		return nil, err
	}
	result := &SeriesEndResult{}
	for _, b := range created {
		if b.Status != bookingstore.BookingStatusPending && b.Status != bookingstore.BookingStatusConfirmed {
			continue
		}
		if b.CollectionStatus != bookingstore.CollectionStatusUnpaid {
			result.Kept = append(result.Kept, b)
			continue
		}
		if err := s.cancelQuietly(ctx, complex, actor, b); err != nil {
			return nil, err
		}
		result.Cancelled = append(result.Cancelled, b)
	}

	endsOn := from.AddDate(0, 0, -1)
	if ser.EndsOn != nil && seriesDay(*ser.EndsOn).Before(endsOn) {
		// Already ended earlier than the date asked for: keep the earlier end.
		endsOn = seriesDay(*ser.EndsOn)
	}
	if floor := seriesDay(ser.StartsOn).AddDate(0, 0, -1); endsOn.Before(floor) {
		endsOn = floor
	}
	result.Series, err = s.series.EndSeries(ctx, ser.ID, &endsOn, bookingstore.SeriesStatusEnded)
	if err != nil {
		return nil, err
	}
	s.audit.Record(audit.Entry{
		UserID: actor.UserID, ComplexID: &complex.ID, Action: "end",
		EntityType: "booking_series", EntityID: &ser.ID, NewValue: result.Series, IPAddress: actor.IP,
	})
	s.notifySeriesCancelled(ctx, complex, ser, from, len(result.Kept) > 0)
	s.realtime.PublishBookingChanged(complex.ID)
	return result, nil
}

// cancelQuietly cancels an unpaid occurrence with no message to the client.
func (s *Service) cancelQuietly(ctx context.Context, complex *complexstore.Complex, actor Actor, b *bookingstore.Booking) error {
	b.Status = bookingstore.BookingStatusCancelled
	note := "Cancelación: baja del turno fijo"
	if b.Notes != nil {
		note = *b.Notes + " | " + note
	}
	b.Notes = &note
	if err := s.store.Update(ctx, b); err != nil {
		return err
	}
	s.record(actor, complex.ID, "cancel", &b.ID, b)
	s.freeWaitlistSlot(ctx, waitlist.Freed{ComplexID: complex.ID, ComplexName: complex.Name, ComplexSlug: complex.Slug, Day: b.Date})
	return nil
}

// MaterializeSeries is the scheduled job: it marks ended the series whose last
// day has passed and turns every due date of an active series into its booking
// (see occurrenceDue for which dates are due), or into an exception when it
// cannot be booked.
//
// It runs often (hourly) and is idempotent and safe on several replicas: the
// scheduler's advisory lock admits one run at a time, and beneath it the primary
// key on (series_id, date) settles any date at most once. A date the venue
// cannot sell becomes an exception and is not retried; a failure that is not a
// verdict on the date (a database error) leaves it held for the next run.
func (s *Service) MaterializeSeries(ctx context.Context) {
	s.materializeSeries(ctx, s.now())
}

func (s *Service) materializeSeries(ctx context.Context, now time.Time) {
	day := seriesDay(now.In(timezone.Argentina))
	if n, err := s.series.EndExpiredSeries(ctx, day); err != nil {
		s.logger.Error("cron_materialize_series: failed to end expired series", "error", err)
	} else if n > 0 {
		s.logger.Info("cron_materialize_series: ended expired series", "series", n)
	}

	// From yesterday: an occurrence the job missed while it was down is caught up
	// (as 'missed' once its hours are over), and tomorrow's early ones are due.
	occs, err := s.store.SeriesOccurrences(ctx, nil, nil, day.AddDate(0, 0, -1), day.AddDate(0, 0, 1))
	if err != nil {
		s.logger.Error("cron_materialize_series: failed to read occurrences", "error", err)
		return
	}
	booked, excepted := 0, 0
	for _, o := range occs {
		if !occurrenceDue(o.Date, o.StartsAt.In(timezone.Argentina).Format("15:04"), now) {
			continue
		}
		wasBooked, err := s.materializeOne(ctx, o, now)
		switch {
		case err != nil:
			s.logger.Error("cron_materialize_series: date not settled, the next run retries",
				"error", err, "series_id", o.SeriesID, "complex_id", o.ComplexID, "date", o.Date.Format(time.DateOnly))
		case wasBooked:
			booked++
			s.realtime.PublishBookingChanged(o.ComplexID)
		default:
			excepted++
		}
	}
	s.logger.Info("cron_materialize_series: completed", "booked", booked, "excepted", excepted)
}

// materializeOne settles one due date, scoped to its own tenant. It reports
// whether a booking was made.
func (s *Service) materializeOne(ctx context.Context, o bookingstore.SeriesOccurrence, now time.Time) (bool, error) {
	ctx = data.ContextWithTenant(ctx, o.ComplexID)
	complex, err := s.complexes.GetByID(ctx, o.ComplexID)
	if err != nil {
		return false, err
	}
	ser, err := s.series.GetSeries(ctx, o.SeriesID)
	if err != nil {
		return false, err
	}
	client, err := s.clients.GetByID(ctx, ser.ClientID)
	if err != nil {
		return false, err
	}
	schedules, err := s.complexes.GetSchedules(ctx, o.ComplexID)
	if err != nil {
		return false, err
	}
	booking, _, err := s.settleDate(ctx, complex, ser, client, schedules, o.Date, now)
	return booking != nil, err
}

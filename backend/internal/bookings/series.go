// series.go — fixed weekly bookings ("turnos fijos").
//
// A series is a standing order: this court, this hour, this person, every week.
// Its occurrences are ordinary bookings with series_id set, materialized eight
// weeks ahead so the grid, the cash desk, the reports and the storefront's
// availability need no knowledge of series at all. A daily job (ExtendSeries)
// keeps the window full.
//
// Every occurrence goes through the same prepareCreate an owner-typed booking
// does, so validation and pricing have one implementation. The two things that
// differ are deliberate: an occurrence sends the client no confirmation (a
// weekly "reserva confirmada" would be spam, S8), and the store records its
// date as generated in the same transaction that writes it (at most once ever).
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
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist"
)

// Series is the domain type of a fixed weekly series.
type Series = bookingstore.Series

const (
	// seriesWindowDays is how far ahead occurrences are materialized: dates in
	// [today, today+56), which is exactly eight occurrences for any weekday.
	seriesWindowDays = 56
	// MaxSeriesWeeks bounds the "weeks" shorthand.
	MaxSeriesWeeks = 156
)

// Reasons a date cannot be booked, as the API reports them.
const (
	ReasonOverlap       = "overlap"
	ReasonBlocked       = "blocked"
	ReasonClosedDay     = "closed_day"
	ReasonNoPrice       = "no_price"
	ReasonClientBlocked = "client_blocked"
)

// SeriesStore is the persistence a series needs beyond the booking store's.
type SeriesStore interface {
	InsertSeries(ctx context.Context, s *Series) error
	GetSeries(ctx context.Context, id uuid.UUID) (*Series, error)
	ListActiveSeries(ctx context.Context) ([]*Series, error)
	EndSeries(ctx context.Context, id uuid.UUID, endsOn *time.Time, status string) (*Series, error)
	SeriesDates(ctx context.Context, seriesID uuid.UUID, from, to time.Time) ([]bookingstore.SeriesDate, error)
	RecordSeriesSkip(ctx context.Context, s *Series, date time.Time, reason string) error
	SeriesBookingsFrom(ctx context.Context, seriesID uuid.UUID, from time.Time) ([]*bookingstore.Booking, error)
}

// SeriesConflict is a date a series could not be booked on, and why.
type SeriesConflict struct {
	Date   time.Time
	Reason string
}

// SeriesConflictsError refuses a series whose first window has dates that
// cannot be booked, when the owner did not ask to skip them. Nothing was
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

// SeriesResult is what creating a series produced.
type SeriesResult struct {
	Series   *Series
	Bookings []*bookingstore.Booking
	Skipped  []SeriesConflict
}

// SeriesDetail is a series with what is ahead of it.
type SeriesDetail struct {
	Series   *Series
	Upcoming []*bookingstore.Booking
	Skipped  []SeriesConflict
}

// SeriesEndResult is what ending a series from a date did.
type SeriesEndResult struct {
	Series    *Series
	Cancelled []*bookingstore.Booking
	// Kept are the occurrences that already carry a payment and so stand.
	Kept []*bookingstore.Booking
}

// seriesDay is a calendar date as the series reasons about it: midnight UTC
// carrying the date's fields, the way a DATE column decodes.
func seriesDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func today() time.Time { return seriesDay(timezone.Today()) }

// windowDates lists the dates a series should have occurrences on within
// [from, from+seriesWindowDays), never before its start or after its end.
func windowDates(s *Series, from time.Time) []time.Time {
	start := seriesDay(from)
	if s.StartsOn.After(start) {
		start = seriesDay(s.StartsOn)
	}
	limit := seriesDay(from).AddDate(0, 0, seriesWindowDays)
	// Align to the series' weekday.
	offset := (s.Weekday - int(start.Weekday()) + 7) % 7
	var out []time.Time
	for d := start.AddDate(0, 0, offset); d.Before(limit); d = d.AddDate(0, 0, 7) {
		if s.EndsOn != nil && d.After(seriesDay(*s.EndsOn)) {
			break
		}
		out = append(out, d)
	}
	return out
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

// dateConflict answers, with reads only, why a date cannot be booked ("" when
// it can). It mirrors what writing the occurrence would refuse, so the owner
// hears about every clash before anything is created.
func (s *Service) dateConflict(ctx context.Context, ser *Series, schedules []*complexstore.Schedule, date time.Time) (string, error) {
	if closedDay(schedules, date, ser.StartTime) {
		return ReasonClosedDay, nil
	}

	startAt := slots.At(timezone.Day(date), ser.StartTime)
	endAt := startAt.Add(time.Duration(ser.DurationMinutes) * time.Minute)

	blocked, err := s.slotIsBlocked(ctx, ser.CourtID, date, startAt, endAt)
	if err != nil {
		return "", err
	}
	if blocked {
		return ReasonBlocked, nil
	}

	days := []time.Time{date}
	if timezone.EndsOnALaterDay(startAt, endAt) {
		days = append(days, date.AddDate(0, 0, 1))
	}
	for _, day := range days {
		spans, err := s.store.GetBookedSlotsByCourtIDs(ctx, []uuid.UUID{ser.CourtID}, day)
		if err != nil {
			return "", err
		}
		for _, sp := range spans {
			if slots.OverlapAt(startAt, endAt, sp.StartsAt, sp.EndsAt) {
				return ReasonOverlap, nil
			}
		}
	}

	if ser.Price == nil {
		if _, err := s.findPrice(ctx, ser.ComplexID, ser.CourtID, date, ser.StartTime, ser.DurationMinutes); err != nil {
			if errors.Is(err, pricing.ErrNoPriceRule) {
				return ReasonNoPrice, nil
			}
			return "", err
		}
	}
	return "", nil
}

// occurrenceConflict maps what writing an occurrence refused to the reason a
// date is skipped. ok is false for any other failure, which is a fault to retry
// rather than a date to give up on.
func occurrenceConflict(err error) (reason string, ok bool) {
	var conflict *ConflictError
	var field *FieldError
	switch {
	case errors.Is(err, bookingstore.ErrDuplicateBooking), errors.Is(err, bookingstore.ErrSlotUnavailable):
		return ReasonOverlap, true
	case errors.As(err, &conflict):
		if conflict.Message == blockedSlotMessage {
			return ReasonBlocked, true
		}
		return ReasonClientBlocked, true
	case errors.As(err, &field) && field.Field == "price":
		return ReasonNoPrice, true
	}
	return "", false
}

// bookOccurrence writes one occurrence, without telling the client. It returns
// nil, nil when the date was already generated.
func (s *Service) bookOccurrence(ctx context.Context, complex *complexstore.Complex, ser *Series, client *clientstore.Client, date time.Time) (*bookingstore.Booking, error) {
	createdBy := ser.CreatedBy
	p, err := s.prepareCreate(ctx, complex, Actor{UserID: &createdBy}, seriesOccurrence(ser, client, date))
	if err != nil {
		return nil, err
	}
	if err := s.store.InsertSafeWithPayment(ctx, p.booking, p.counter); err != nil {
		if errors.Is(err, bookingstore.ErrSeriesDateGenerated) {
			return nil, nil //nolint:nilnil // the date was generated already: nothing to add, nothing wrong
		}
		return nil, err
	}
	return p.booking, nil
}

// CreateSeries creates a fixed weekly booking and materializes its first
// window. It refuses, writing nothing, when dates of that window cannot be
// booked and the owner did not ask to skip them.
//
//nolint:funlen // one cohesive request: validate, pre-check, write series, write occurrences
func (s *Service) CreateSeries(ctx context.Context, complex *complexstore.Complex, actor Actor, in SeriesInput) (*SeriesResult, error) {
	if actor.UserID == nil {
		return nil, ErrNoActor
	}
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
	if first.Before(today()) {
		return nil, &FieldError{Field: "first_date", Message: "must not be in the past"}
	}
	if first.After(today().AddDate(0, 0, bookingstore.MaxBookingHorizonDays)) {
		return nil, &FieldError{Field: "first_date", Message: "is too far in the future"}
	}
	var endsOn *time.Time
	switch {
	case in.EndsOn != nil && in.Weeks > 0:
		return nil, &FieldError{Field: "weeks", Message: "send either ends_on or weeks, not both"}
	case in.EndsOn != nil:
		end := seriesDay(*in.EndsOn)
		if end.Before(first) {
			return nil, &FieldError{Field: "ends_on", Message: "must not be before first_date"}
		}
		endsOn = &end
	case in.Weeks > 0:
		end := first.AddDate(0, 0, 7*(in.Weeks-1))
		endsOn = &end
	}
	notes := (*string)(nil)
	if in.Notes != "" {
		notes = &in.Notes
	}
	ser := &Series{
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
	}

	// Read-only pre-check of the whole first window, before the client or the
	// series exist.
	schedules, err := s.complexes.GetSchedules(ctx, complex.ID)
	if err != nil {
		return nil, err
	}
	var conflicts []SeriesConflict
	var dates []time.Time
	for _, d := range windowDates(ser, today()) {
		reason, err := s.dateConflict(ctx, ser, schedules, d)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			conflicts = append(conflicts, SeriesConflict{Date: d, Reason: reason})
			continue
		}
		dates = append(dates, d)
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
	if err := s.series.InsertSeries(ctx, ser); err != nil {
		return nil, err
	}
	s.audit.Record(audit.Entry{
		UserID: actor.UserID, ComplexID: &complex.ID, Action: "create",
		EntityType: "booking_series", EntityID: &ser.ID, NewValue: ser, IPAddress: actor.IP,
	})

	result := &SeriesResult{Series: ser}
	for _, c := range conflicts {
		if err := s.series.RecordSeriesSkip(ctx, ser, c.Date, c.Reason); err != nil {
			return nil, err
		}
		result.Skipped = append(result.Skipped, c)
	}
	for _, d := range dates {
		booking, err := s.bookOccurrence(ctx, complex, ser, client, d)
		if err != nil {
			// Lost a race between the pre-check and the write: report the date
			// as skipped rather than failing a series that is already created.
			reason, ok := occurrenceConflict(err)
			if !ok {
				return nil, err
			}
			if err := s.series.RecordSeriesSkip(ctx, ser, d, reason); err != nil {
				return nil, err
			}
			result.Skipped = append(result.Skipped, SeriesConflict{Date: d, Reason: reason})
			continue
		}
		if booking != nil {
			result.Bookings = append(result.Bookings, booking)
		}
	}
	slices.SortFunc(result.Skipped, func(a, b SeriesConflict) int { return a.Date.Compare(b.Date) })

	s.realtime.PublishBookingChanged(complex.ID)
	return result, nil
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

// GetSeries returns a series with its occurrences from today on and the dates
// that were skipped.
func (s *Service) GetSeries(ctx context.Context, complexID, seriesID uuid.UUID) (*SeriesDetail, error) {
	ser, err := s.ownedSeries(ctx, complexID, seriesID)
	if err != nil {
		return nil, err
	}
	from := today()
	upcoming, err := s.series.SeriesBookingsFrom(ctx, ser.ID, from)
	if err != nil {
		return nil, err
	}
	dates, err := s.series.SeriesDates(ctx, ser.ID, from, from.AddDate(0, 0, bookingstore.MaxBookingHorizonDays))
	if err != nil {
		return nil, err
	}
	detail := &SeriesDetail{Series: ser, Upcoming: upcoming}
	for _, d := range dates {
		if d.Outcome == bookingstore.SeriesDateSkipped && d.Reason != nil {
			detail.Skipped = append(detail.Skipped, SeriesConflict{Date: d.Date, Reason: *d.Reason})
		}
	}
	return detail, nil
}

// EndSeries ends a series from a date: occurrences on or after it that nobody
// has paid for are cancelled, those with a payment stand and are reported back,
// and the series stops generating from that date on. The series is marked ended
// once nothing paid remains ahead of it; otherwise the daily job marks it when
// its last day passes.
//
// Cancelling is quiet: no message goes to the client and no refund is involved,
// since only unpaid occurrences are touched.
func (s *Service) EndSeries(ctx context.Context, complex *complexstore.Complex, actor Actor, seriesID uuid.UUID, from time.Time) (*SeriesEndResult, error) {
	ser, err := s.ownedSeries(ctx, complex.ID, seriesID)
	if err != nil {
		return nil, err
	}
	from = seriesDay(from)
	if from.Before(today()) {
		return nil, &FieldError{Field: "from_date", Message: "must not be in the past"}
	}

	occurrences, err := s.series.SeriesBookingsFrom(ctx, ser.ID, from)
	if err != nil {
		return nil, err
	}
	result := &SeriesEndResult{}
	for _, b := range occurrences {
		if b.Status != bookingstore.BookingStatusPending && b.Status != bookingstore.BookingStatusConfirmed {
			continue
		}
		if b.CollectionStatus != bookingstore.CollectionStatusUnpaid {
			result.Kept = append(result.Kept, b)
			continue
		}
		b.Status = bookingstore.BookingStatusCancelled
		note := "Cancelación: baja del turno fijo"
		if b.Notes != nil {
			note = *b.Notes + " | " + note
		}
		b.Notes = &note
		if err := s.store.Update(ctx, b); err != nil {
			return nil, err
		}
		s.record(actor, complex.ID, "cancel", &b.ID, b)
		s.freeWaitlistSlot(ctx, waitlist.Freed{ComplexID: complex.ID, ComplexName: complex.Name, ComplexSlug: complex.Slug, Day: b.Date})
		result.Cancelled = append(result.Cancelled, b)
	}

	endsOn := from.AddDate(0, 0, -1)
	status := bookingstore.SeriesStatusActive
	if len(result.Kept) == 0 {
		status = bookingstore.SeriesStatusEnded
	}
	if ser.EndsOn != nil && ser.EndsOn.Before(endsOn) {
		// Already ended earlier than the date asked for: keep the earlier end.
		endsOn = seriesDay(*ser.EndsOn)
	}
	if endsOn.Before(seriesDay(ser.StartsOn).AddDate(0, 0, -1)) {
		endsOn = seriesDay(ser.StartsOn).AddDate(0, 0, -1)
	}
	result.Series, err = s.series.EndSeries(ctx, ser.ID, &endsOn, status)
	if err != nil {
		return nil, err
	}
	s.audit.Record(audit.Entry{
		UserID: actor.UserID, ComplexID: &complex.ID, Action: "end",
		EntityType: "booking_series", EntityID: &ser.ID, NewValue: result.Series, IPAddress: actor.IP,
	})
	s.realtime.PublishBookingChanged(complex.ID)
	return result, nil
}

// ExtendSeries is the daily job: it tops every active series up to eight weeks
// ahead and marks the ones whose last day has passed as ended.
//
// It is idempotent and safe on several replicas: the scheduler's advisory lock
// admits one run at a time, and beneath it the primary key on
// (series_id, date) refuses a second generation of any date. A date the venue
// cannot sell is recorded as skipped and not retried; a failure that is not a
// verdict on the date (a database error) leaves it unrecorded for the next run.
func (s *Service) ExtendSeries(ctx context.Context) {
	s.extendSeries(ctx, today())
}

func (s *Service) extendSeries(ctx context.Context, day time.Time) {
	all, err := s.series.ListActiveSeries(ctx)
	if err != nil {
		s.logger.Error("cron_extend_series: failed to list series", "error", err)
		return
	}
	booked, skipped := 0, 0
	for _, ser := range all {
		b, k, err := s.extendOne(ctx, ser, day)
		booked += b
		skipped += k
		if err != nil {
			s.logger.Error("cron_extend_series: series not fully extended, the next run retries",
				"error", err, "series_id", ser.ID, "complex_id", ser.ComplexID)
		}
	}
	s.logger.Info("cron_extend_series: completed", "series", len(all), "booked", booked, "skipped", skipped)
}

// extendOne tops one series up, scoped to its own tenant.
func (s *Service) extendOne(ctx context.Context, ser *Series, day time.Time) (booked, skipped int, err error) {
	complex, err := s.complexes.GetByID(ctx, ser.ComplexID)
	if err != nil {
		return 0, 0, err
	}
	ctx = data.ContextWithTenant(ctx, ser.ComplexID)

	if ser.EndsOn != nil && seriesDay(*ser.EndsOn).Before(day) {
		_, err := s.series.EndSeries(ctx, ser.ID, ser.EndsOn, bookingstore.SeriesStatusEnded)
		return 0, 0, err
	}

	dates := windowDates(ser, day)
	if len(dates) == 0 {
		return 0, 0, nil
	}
	generated, err := s.series.SeriesDates(ctx, ser.ID, dates[0], dates[len(dates)-1])
	if err != nil {
		return 0, 0, err
	}
	done := make(map[string]struct{}, len(generated))
	for _, g := range generated {
		done[g.Date.Format(time.DateOnly)] = struct{}{}
	}
	schedules, err := s.complexes.GetSchedules(ctx, ser.ComplexID)
	if err != nil {
		return 0, 0, err
	}
	client, err := s.clients.GetByID(ctx, ser.ClientID)
	if err != nil {
		return 0, 0, err
	}

	for _, d := range dates {
		if _, ok := done[d.Format(time.DateOnly)]; ok {
			continue
		}
		reason := ""
		if closedDay(schedules, d, ser.StartTime) {
			reason = ReasonClosedDay
		} else {
			booking, err := s.bookOccurrence(ctx, complex, ser, client, d)
			if err == nil {
				if booking != nil {
					booked++
				}
				continue
			}
			var ok bool
			if reason, ok = occurrenceConflict(err); !ok {
				return booked, skipped, err
			}
		}
		if err := s.series.RecordSeriesSkip(ctx, ser, d, reason); err != nil {
			return booked, skipped, err
		}
		skipped++
	}
	if booked > 0 {
		s.realtime.PublishBookingChanged(ser.ComplexID)
	}
	return booked, skipped, nil
}

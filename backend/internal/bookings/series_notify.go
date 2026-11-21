package bookings

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/booklink"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist"
)

// seriesHours renders the hours a series holds as a person reads them
// ("19:00 a 20:30"), with the next-day marker when they cross midnight. A
// start time that does not parse yields the empty string, which the notice
// then simply lacks.
func seriesHours(ser *Series) string {
	clock, err := time.Parse("15:04", ser.StartTime)
	if err != nil {
		return ""
	}
	day := seriesDay(ser.StartsOn)
	start := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, timezone.Argentina)
	return timezone.HoursLabel(start, start.Add(time.Duration(ser.DurationMinutes)*time.Minute))
}

// notifySeriesCreated tells the client by email that their fixed booking was
// set up. It is skipped when the court cannot be read: there is nothing to name.
// created are the bookings made with it; any on another court than the series'
// own is a date moved on conflict, and the notice names where it is played.
func (s *Service) notifySeriesCreated(ctx context.Context, complex *complexstore.Complex, ser *Series, client *clientstore.Client, created []*bookingstore.Booking) {
	if client.Email == nil || *client.Email == "" {
		return
	}
	court, err := s.courts.GetByID(ctx, ser.CourtID)
	if err != nil {
		return
	}
	c := notifications.SeriesCreation{
		Email:        *client.Email,
		ComplexName:  complex.Name,
		CourtName:    court.Name,
		Weekday:      time.Weekday(ser.Weekday),
		Hours:        seriesHours(ser),
		From:         seriesDay(ser.StartsOn).Format("02/01"),
		Address:      booklink.Address(complex.Address, complex.City),
		MapsURL:      booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude),
		ComplexPhone: complex.Phone,
		OwnerID:      complex.OwnerID.String(),
		SeriesID:     ser.ID.String(),
		Moved:        s.movedDates(ctx, ser, created),
	}
	if ser.EndsOn != nil {
		c.Until = seriesDay(*ser.EndsOn).Format("02/01")
	}
	s.notify.SeriesCreated(c)
}

// movedDates names each created booking played on another court than the
// series' own, in date order. A court that cannot be read is named by nothing
// rather than dropping the date: the client still learns that date is elsewhere.
func (s *Service) movedDates(ctx context.Context, ser *Series, created []*bookingstore.Booking) []notifications.MovedDate {
	var moved []notifications.MovedDate
	for _, b := range created {
		if b.CourtID == ser.CourtID {
			continue
		}
		name := ""
		if court, err := s.courts.GetByID(ctx, b.CourtID); err == nil {
			name = court.Name
		} else {
			s.logger.Warn("series notice: the court of a moved date could not be read", "error", err, "booking_id", b.ID)
		}
		moved = append(moved, notifications.MovedDate{Date: b.Date.Format("02/01"), CourtName: name})
	}
	return moved
}

// notifySeriesCancelled tells the client the owner cancelled their fixed
// booking from a date on. keptPaid says some booked dates stand because they
// carry a payment. One message per event, whatever number of dates it covers.
func (s *Service) notifySeriesCancelled(ctx context.Context, complex *complexstore.Complex, ser *Series, from time.Time, keptPaid bool) {
	court, err := s.courts.GetByID(ctx, ser.CourtID)
	if err != nil {
		return
	}
	client, err := s.clients.GetByID(ctx, ser.ClientID)
	if err != nil {
		return
	}
	c := notifications.SeriesCancellation{
		ComplexName: complex.Name,
		CourtName:   court.Name,
		Weekday:     time.Weekday(ser.Weekday),
		Hours:       seriesHours(ser),
		// The first date the series stops holding, so that "lunes 14/12" is
		// always a Monday of the series even when the owner picked another day.
		From:         firstRuleDate(ser, from).Format("02/01"),
		KeptPaid:     keptPaid,
		BookURL:      booklink.Book(s.cfg.PublicURL, complex.Slug),
		ComplexPhone: complex.Phone,
		OwnerID:      complex.OwnerID.String(),
		SeriesID:     ser.ID.String(),
	}
	if client.Email != nil {
		c.Email = *client.Email
	}
	s.notify.SeriesCancelled(c)
}

// notifySeriesDateSkipped tells the client that the owner took one date off
// their fixed booking. The date had no booking yet (a booked date is cancelled
// through its booking), so nothing was paid on it: the notice is the ordinary
// cancellation email with nothing to give back, and the owner, who skipped it,
// is not emailed. Its subject is the series and the date, so each skipped date
// is a notice of its own. Hours already started are not announced.
func (s *Service) notifySeriesDateSkipped(ctx context.Context, complex *complexstore.Complex, ser *Series, day, startsAt, endsAt time.Time) {
	if !startsAt.After(s.now()) {
		return
	}
	court, err := s.courts.GetByID(ctx, ser.CourtID)
	if err != nil {
		s.logger.Error("skip series date: the client notice was not sent, the court could not be read",
			"error", err, "series_id", ser.ID, "complex_id", complex.ID)
		return
	}
	client, err := s.clients.GetByID(ctx, ser.ClientID)
	if err != nil || client.Email == nil || *client.Email == "" {
		if err != nil {
			s.logger.Error("skip series date: the client notice was not sent, the client could not be read",
				"error", err, "series_id", ser.ID, "complex_id", complex.ID)
		}
		return
	}
	refundLine, refundAmount := refundNotice(paymentstore.RefundOutcome{Result: paymentstore.RefundNone})
	if err := s.notify.BookingCancelled(notifications.Cancellation{
		Email:        *client.Email,
		ComplexName:  complex.Name,
		CourtName:    court.Name,
		Date:         day.Format("02/01"),
		StartTime:    timezone.HoursLabel(startsAt, endsAt),
		RefundLine:   refundLine,
		RefundAmount: refundAmount,
		BookURL:      booklink.Book(s.cfg.PublicURL, complex.Slug),
		ClientName:   strings.TrimSpace(client.FirstName + " " + client.LastName),
		OwnerID:      complex.OwnerID.String(),
		ComplexPhone: complex.Phone,
		Source:       notifications.SourceOwnerCancel,
		BookingID:    ser.ID.String() + ":" + day.Format(time.DateOnly),
	}); err != nil {
		s.logger.Error("skip series date: the client notice was not queued",
			"error", err, "series_id", ser.ID, "complex_id", complex.ID)
	}
}

// firstRuleDate is the first date on or after from that falls on the series'
// weekday.
func firstRuleDate(ser *Series, from time.Time) time.Time {
	from = seriesDay(from)
	return from.AddDate(0, 0, (ser.Weekday-int(from.Weekday())+7)%7)
}

// slotReleased tells the waitlist and, when the hours had been off sale
// (announce), the freed-slot listener that a series' hours are free again: a
// cancelled occurrence, a skipped date, or the dates an ended series stops
// holding. Hours that have already started free nothing anyone can book, so
// they are announced to nobody.
func (s *Service) slotReleased(
	ctx context.Context, complex *complexstore.Complex, courtID uuid.UUID, day, startsAt, endsAt time.Time, announce bool,
) {
	if !startsAt.After(s.now()) {
		return
	}
	s.freeWaitlistSlot(ctx, waitlist.Freed{ComplexID: complex.ID, ComplexName: complex.Name, ComplexSlug: complex.Slug, Day: day})
	if announce && s.freedSlots != nil {
		ctx = context.WithoutCancel(ctx)
		complexID, slug := complex.ID, complex.Slug
		s.run(func() { s.freedSlots.SlotCancelled(ctx, complexID, courtID, slug, startsAt, endsAt) })
	}
}

// notifySeriesDatesLost tells the owner which dates of a series the job turned
// into exceptions in one run: nobody decided to give them up, so without this
// the owner would learn it from the client standing at a taken court.
func (s *Service) notifySeriesDatesLost(ctx context.Context, loss *seriesLoss) {
	courtName := ""
	if court, err := s.courts.GetByID(ctx, loss.series.CourtID); err == nil {
		courtName = court.Name
	}
	s.notify.SeriesDatesLost(notifications.SeriesDatesLost{
		OwnerID:     loss.complex.OwnerID.String(),
		ComplexName: loss.complex.Name,
		CourtName:   courtName,
		ClientName:  strings.TrimSpace(loss.client.FirstName + " " + loss.client.LastName),
		Weekday:     time.Weekday(loss.series.Weekday),
		Hours:       seriesHours(loss.series),
		Dates:       loss.dates,
		SeriesID:    loss.series.ID.String(),
	})
}

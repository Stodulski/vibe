package bookings

import (
	"context"
	"time"

	"github.com/stodulski/vibe-server/internal/booklink"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
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
func (s *Service) notifySeriesCreated(ctx context.Context, complex *complexstore.Complex, ser *Series, client *clientstore.Client) {
	if client.Email == nil || *client.Email == "" {
		return
	}
	court, err := s.courts.GetByID(ctx, ser.CourtID)
	if err != nil {
		return
	}
	c := notifications.SeriesCreation{
		Email:       *client.Email,
		ComplexName: complex.Name,
		CourtName:   court.Name,
		Weekday:     time.Weekday(ser.Weekday),
		Hours:       seriesHours(ser),
		From:        seriesDay(ser.StartsOn).Format("02/01"),
		Address:     booklink.Address(complex.Address, complex.City),
		MapsURL:     booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude),
		SeriesID:    ser.ID.String(),
	}
	if ser.EndsOn != nil {
		c.Until = seriesDay(*ser.EndsOn).Format("02/01")
	}
	s.notify.SeriesCreated(c)
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
		Phone:       client.Phone,
		ComplexName: complex.Name,
		CourtName:   court.Name,
		Weekday:     time.Weekday(ser.Weekday),
		Hours:       seriesHours(ser),
		// The first date the series stops holding, so that "lunes 14/12" is
		// always a Monday of the series even when the owner picked another day.
		From:     firstRuleDate(ser, from).Format("02/01"),
		KeptPaid: keptPaid,
		BookPath: booklink.BookPath(complex.Slug),
		BookURL:  booklink.Book(s.cfg.PublicURL, complex.Slug),
		SeriesID: ser.ID.String(),
	}
	if client.Email != nil {
		c.Email = *client.Email
	}
	s.notify.SeriesCancelled(c)
}

// firstRuleDate is the first date on or after from that falls on the series'
// weekday.
func firstRuleDate(ser *Series, from time.Time) time.Time {
	from = seriesDay(from)
	return from.AddDate(0, 0, (ser.Weekday-int(from.Weekday())+7)%7)
}

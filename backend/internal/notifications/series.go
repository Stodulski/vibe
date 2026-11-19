package notifications

import (
	"context"
	"time"
)

// Notices of a fixed weekly booking ("turno fijo"). They are one message per
// series event, never one per occurrence: a weekly occurrence is booked by the
// scheduler with no confirmation of its own (see internal/bookings/series.go).
const (
	// TaskEmailSeriesCreated tells the client a fixed booking was set up for
	// them.
	TaskEmailSeriesCreated = "email:series_created"
	// TaskEmailSeriesCancelled is the email half of the series cancellation.
	TaskEmailSeriesCancelled = "email:series_cancelled"
)

// SeriesCreation is the notice that a fixed weekly booking was set up. An empty
// Email skips the notice.
type SeriesCreation struct {
	Email       string
	ComplexName string
	CourtName   string
	// Weekday is the day the series repeats on.
	Weekday time.Weekday
	// Hours is the span as a person reads it ("19:00 a 20:30"), from
	// timezone.HoursLabel.
	Hours string
	// From is the first date as dd/mm. Until is the last as dd/mm, or empty
	// when the series has no end.
	From  string
	Until string
	// Address and MapsURL are the "Cómo llegar" line, as in the booking
	// confirmation.
	Address string
	MapsURL string

	// ComplexPhone and OwnerID are the complex's contact the email carries:
	// its phone, and the owner whose address the reply goes to.
	ComplexPhone string
	OwnerID      string

	// SeriesID is the deduplication subject and never travels in the payload.
	SeriesID string
}

// SeriesCancellation is the notice that the owner cancelled a fixed weekly
// booking from a date on.
type SeriesCancellation struct {
	Email       string
	ComplexName string
	CourtName   string
	Weekday     time.Weekday
	Hours       string
	// From is the first date the series no longer holds, as dd/mm.
	From string
	// KeptPaid is true when some already-created occurrences carry a payment and
	// so stand.
	KeptPaid bool
	// BookURL is the booking page, absolute, for the email.
	BookURL string

	// ComplexPhone and OwnerID are the complex's contact the email carries:
	// its phone, and the owner whose address the reply goes to.
	ComplexPhone string
	OwnerID      string

	// SeriesID is the deduplication subject, with From: ending the same series
	// from two dates is two events.
	SeriesID string
}

// seriesCreatedEmail is what the email worker needs, and nothing else.
type seriesCreatedEmail struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	Weekdays    string `json:"weekdays"`
	Hours       string `json:"hours"`
	From        string `json:"from"`
	Until       string `json:"until"`
	Address     string `json:"address"`
	MapsURL     string `json:"maps_url"`
	clientContact
}

type seriesCancelledEmail struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	Weekdays    string `json:"weekdays"`
	Hours       string `json:"hours"`
	From        string `json:"from"`
	KeptPaid    bool   `json:"kept_paid"`
	BookURL     string `json:"book_url"`
	clientContact
}

var spanishWeekdays = [...]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}

// SpanishWeekday is the name of a weekday, lowercase: "lunes".
func SpanishWeekday(d time.Weekday) string { return spanishWeekdays[d] }

// SpanishWeekdays is the plural a person says after "todos los": "lunes",
// "sábados".
func SpanishWeekdays(d time.Weekday) string {
	if d == time.Saturday || d == time.Sunday {
		return SpanishWeekday(d) + "s"
	}
	return SpanishWeekday(d)
}

// SeriesCreated tells the client their fixed booking is set up, by email.
func (s *Service) SeriesCreated(c SeriesCreation) {
	if c.Email == "" {
		return
	}
	s.queue.Enqueue(TaskEmailSeriesCreated, seriesCreatedEmail{
		Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
		Weekdays: SpanishWeekdays(c.Weekday), Hours: c.Hours, From: c.From, Until: c.Until,
		Address: c.Address, MapsURL: c.MapsURL,
		clientContact: clientContact{ComplexPhone: c.ComplexPhone, OwnerID: c.OwnerID},
	}, dedup(TaskEmailSeriesCreated, c.Email, c.SeriesID))
}

// SeriesCancelled tells the client their fixed booking was cancelled, by email.
func (s *Service) SeriesCancelled(c SeriesCancellation) {
	if c.Email == "" {
		return
	}
	subject := ""
	if c.SeriesID != "" {
		subject = c.SeriesID + ":" + c.From
	}
	s.queue.Enqueue(TaskEmailSeriesCancelled, seriesCancelledEmail{
		Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
		Weekdays: SpanishWeekdays(c.Weekday), Hours: c.Hours, From: c.From,
		KeptPaid: c.KeptPaid, BookURL: c.BookURL,
		clientContact: clientContact{ComplexPhone: c.ComplexPhone, OwnerID: c.OwnerID},
	}, dedup(TaskEmailSeriesCancelled, c.Email, subject))
}

// registerSeriesWorkers wires the delivery of the series notices.
// RegisterWorkers calls it.
func (s *Service) registerSeriesWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailSeriesCreated, func(ctx context.Context, p seriesCreatedEmail) error {
		return s.mailer.SendSeriesCreated(ctx, p.Email, p.ComplexName, p.CourtName, p.Weekdays, p.Hours,
			p.From, p.Until, p.Address, p.MapsURL, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailSeriesCancelled, func(ctx context.Context, p seriesCancelledEmail) error {
		return s.mailer.SendSeriesCancelled(ctx, p.Email, p.ComplexName, p.CourtName, p.Weekdays, p.Hours,
			p.From, p.KeptPaid, p.BookURL, s.contactOf(ctx, p.clientContact))
	}))
}

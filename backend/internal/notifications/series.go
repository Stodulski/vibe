package notifications

import (
	"context"
	"strings"
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
	// TaskEmailOwnerSeriesDatesLost tells the owner which dates of a fixed
	// booking the scheduler could not book.
	TaskEmailOwnerSeriesDatesLost = "email:owner_series_dates_lost"
)

// LostDate is one date of a series the scheduler turned into an exception, and
// the ledger's reason for it (internal/bookings/store's Reason constants).
type LostDate struct {
	Date   time.Time
	Reason string
}

// SeriesDatesLost is the owner's notice that dates of a fixed booking became
// exceptions on their own: nobody cancelled them, the scheduler could not book
// them. One notice per series and run, with every date it lost.
type SeriesDatesLost struct {
	OwnerID     string
	ComplexName string
	CourtName   string
	ClientName  string
	Weekday     time.Weekday
	Hours       string
	Dates       []LostDate

	// SeriesID, with the dates, is the deduplication subject.
	SeriesID string
}

// lostReasonText says why a date was lost, the way the owner reads it.
var lostReasonText = map[string]string{
	"overlap":        "el horario ya estaba ocupado",
	"blocked":        "la cancha estaba bloqueada o ya no existe",
	"closed_day":     "el complejo estaba cerrado ese día",
	"no_price":       "el horario no tiene precio cargado",
	"client_blocked": "el cliente está bloqueado",
	"missed":         "el turno ya había pasado cuando se intentó reservar",
}

type ownerSeriesDatesLost struct {
	OwnerID     string   `json:"owner_id"`
	ComplexName string   `json:"complex_name"`
	CourtName   string   `json:"court_name"`
	ClientName  string   `json:"client_name"`
	Weekdays    string   `json:"weekdays"`
	Hours       string   `json:"hours"`
	Lines       []string `json:"lines"`
}

// SeriesDatesLost tells the owner, by email, which dates of a fixed booking
// the scheduler could not book and why.
func (s *Service) SeriesDatesLost(c SeriesDatesLost) {
	if c.OwnerID == "" || len(c.Dates) == 0 {
		return
	}
	lines := make([]string, 0, len(c.Dates))
	keys := make([]string, 0, len(c.Dates))
	for _, d := range c.Dates {
		why, ok := lostReasonText[d.Reason]
		if !ok {
			why = "no se pudo reservar"
		}
		lines = append(lines, SpanishWeekday(d.Date.Weekday())+" "+d.Date.Format("02/01")+": "+why)
		keys = append(keys, d.Date.Format(time.DateOnly))
	}
	subject := ""
	if c.SeriesID != "" {
		subject = c.SeriesID + ":" + strings.Join(keys, ",")
	}
	s.queue.Enqueue(TaskEmailOwnerSeriesDatesLost, ownerSeriesDatesLost{
		OwnerID: c.OwnerID, ComplexName: c.ComplexName, CourtName: c.CourtName, ClientName: c.ClientName,
		Weekdays: SpanishWeekdays(c.Weekday), Hours: c.Hours, Lines: lines,
	}, dedup(TaskEmailOwnerSeriesDatesLost, c.OwnerID, subject))
}

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

	// Moved are the dates played on another court because the series' own
	// court was taken on them, in date order.
	Moved []MovedDate

	// SeriesID is the deduplication subject and never travels in the payload.
	SeriesID string
}

// MovedDate is one date of a fixed booking played on another court: Date as
// dd/mm and the court it is played on.
type MovedDate struct {
	Date      string
	CourtName string
}

// movedLines renders each moved date as the email lists it ("23/11: Cancha 3").
func movedLines(moved []MovedDate) []string {
	if len(moved) == 0 {
		return nil
	}
	lines := make([]string, 0, len(moved))
	for _, m := range moved {
		lines = append(lines, m.Date+": "+m.CourtName)
	}
	return lines
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
	// Moved is one line per date played on another court; absent from a task
	// queued before it existed, which then lists none.
	Moved []string `json:"moved,omitempty"`
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
		Address: c.Address, MapsURL: c.MapsURL, Moved: movedLines(c.Moved),
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
			p.From, p.Until, p.Address, p.MapsURL, p.Moved, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailSeriesCancelled, func(ctx context.Context, p seriesCancelledEmail) error {
		return s.mailer.SendSeriesCancelled(ctx, p.Email, p.ComplexName, p.CourtName, p.Weekdays, p.Hours,
			p.From, p.KeptPaid, p.BookURL, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailOwnerSeriesDatesLost, func(ctx context.Context, p ownerSeriesDatesLost) error {
		to, err := s.ownerAddress(ctx, "owner series dates lost", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerSeriesDatesLost(ctx, to, p.ComplexName, p.CourtName, p.ClientName,
			p.Weekdays, p.Hours, p.Lines)
	}))
}

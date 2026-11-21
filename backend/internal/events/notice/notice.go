// Package notice builds the notices of an open-play event sign-up from the
// event, the registration and the complex. Two domains send them — payments
// when the money lands or goes back, events when a spot is cancelled — so the
// wording of "when, where, how much" lives once, in a leaf both can import.
package notice

import (
	"fmt"
	"strings"
	"time"

	"github.com/stodulski/vibe-server/internal/booklink"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// Refund lines, in the lowercase-free sentences the cancellation email places
// before the amount.
const (
	// LineRefunded precedes the amount that came back.
	LineRefunded = "Te devolvimos el pago:"
	// LineRefundQueued says the money is on its way back.
	LineRefundQueued = "Tu pago se está devolviendo y lo vas a ver acreditado en los próximos días."
	// LineKept says the cancellation came too late for a refund.
	LineKept = "La cancelación fue fuera de plazo, así que el pago no se devuelve."
)

// Day is the event's day as a person reads it: "sábado 15/11".
func Day(ev *eventstore.Event) string {
	return notifications.SpanishWeekday(timezone.Day(ev.Date).Weekday()) + " " + ev.Date.Format("02/01")
}

// Hours is the event's span as a person reads it: "19:00 a 21:00". A tournament
// category has no hours of its own: its matches are scheduled one by one.
func Hours(ev *eventstore.Event) string {
	if ev.TournamentID != nil {
		return ""
	}
	return timezone.HoursLabel(ev.StartsAt(), ev.EndsAt())
}

// Registered is the confirmation of a person's spot.
func Registered(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex, publicURL string) notifications.EventRegistration {
	n := notifications.EventRegistration{
		FirstName:   reg.FirstName,
		ComplexName: complex.Name,
		EventName:   ev.Name,
		Date:        Day(ev),
		Hours:       Hours(ev),
		// With the city, as every other email writes it: a street address
		// alone is ambiguous across towns.
		Address:        booklink.Address(complex.Address, complex.City),
		MapsURL:        booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude),
		Amount:         notifications.FormatARS(reg.Price + reg.ServiceFee),
		EventURL:       pageOf(publicURL, complex.Slug, ev),
		ComplexPhone:   complex.Phone,
		OwnerID:        complex.OwnerID.String(),
		RegistrationID: reg.ID.String(),
	}
	if reg.Email != nil {
		n.Email = *reg.Email
	}
	return n
}

// Cancelled is the notice that a person's spot is over. refundLine and
// refundCentavos say what happened to their money; refundCentavos is the figure
// quoted after the line when it is positive.
func Cancelled(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex, publicURL string, byOwner bool, refundLine string, refundCentavos int) notifications.EventCancellation {
	n := notifications.EventCancellation{
		FirstName:      reg.FirstName,
		ComplexName:    complex.Name,
		EventName:      ev.Name,
		Date:           Day(ev),
		Hours:          Hours(ev),
		ByOwner:        byOwner,
		RefundLine:     refundLine,
		EventsURL:      fmt.Sprintf("%s/c/%s", publicURL, complex.Slug),
		ComplexPhone:   complex.Phone,
		OwnerID:        complex.OwnerID.String(),
		RegistrationID: reg.ID.String(),
	}
	if refundCentavos > 0 {
		n.RefundAmount = notifications.FormatARS(refundCentavos)
	}
	if reg.Email != nil {
		n.Email = *reg.Email
	}
	return n
}

// pageOf is where a notice sends the person: the event's page, or the
// tournament's for a category.
func pageOf(publicURL, slug string, ev *eventstore.Event) string {
	if ev.TournamentID != nil {
		return booklink.TournamentPage(publicURL, slug, *ev.TournamentID)
	}
	return booklink.EventPage(publicURL, slug, ev.ID)
}

// LineExpired is the line of the notice that tells a person their unpaid hold
// ran out: the spot went back on sale and nothing was charged.
const LineExpired = "Tu lugar se liberó porque el pago no se completó a tiempo, así que no hay nada que devolver. Si todavía querés ir, podés anotarte de nuevo."

// Expired is the notice that a person's unpaid hold ran out, sent as the
// cancellation of their spot with LineExpired as its money line.
func Expired(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex, publicURL string) notifications.EventCancellation {
	return Cancelled(reg, ev, complex, publicURL, false, LineExpired, 0)
}

// Owner is the owner's notice about a person's spot: who, for which event,
// when, and what they paid.
func Owner(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex) notifications.OwnerEventNotice {
	return notifications.OwnerEventNotice{
		OwnerID:         complex.OwnerID.String(),
		ComplexName:     complex.Name,
		EventName:       ev.Name,
		Date:            Day(ev),
		Hours:           Hours(ev),
		ParticipantName: strings.TrimSpace(reg.FirstName + " " + reg.LastName),
		Amount:          notifications.FormatARS(reg.Price + reg.ServiceFee),
		RegistrationID:  reg.ID.String(),
	}
}

// Refunded is the notice that part of a person's payment came back while their
// spot stays. movedCentavos is what came back this time; refundedTotal is the
// payment's running total after it, which tells one refund from the next.
func Refunded(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex, publicURL string, movedCentavos, refundedTotal int) notifications.EventRefund {
	n := notifications.EventRefund{
		FirstName:      reg.FirstName,
		ComplexName:    complex.Name,
		EventName:      ev.Name,
		Date:           Day(ev),
		EventURL:       pageOf(publicURL, complex.Slug, ev),
		ComplexPhone:   complex.Phone,
		OwnerID:        complex.OwnerID.String(),
		RegistrationID: reg.ID.String(),
		RefundedTotal:  refundedTotal,
	}
	if movedCentavos > 0 {
		n.Amount = notifications.FormatARS(movedCentavos)
	}
	if reg.Email != nil {
		n.Email = *reg.Email
	}
	return n
}

// Reminder is the day-before reminder of a person's spot. The email goes to
// email under firstName, so a tournament partner gets their own.
func Reminder(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex, publicURL, email, firstName string, startsAt time.Time, hasTime bool, now time.Time) notifications.EventReminder {
	return notifications.EventReminder{
		Email:          email,
		FirstName:      firstName,
		ComplexName:    complex.Name,
		EventName:      ev.Name,
		When:           When(startsAt, hasTime, now),
		Address:        booklink.Address(complex.Address, complex.City),
		MapsURL:        booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude),
		EventURL:       pageOf(publicURL, complex.Slug, ev),
		ComplexPhone:   complex.Phone,
		OwnerID:        complex.OwnerID.String(),
		RegistrationID: reg.ID.String(),
	}
}

// When is an instant as a person reads it on the day the email arrives: "hoy
// sábado 15/11 a las 19:00", "mañana sábado 15/11 a las 19:00", or "el sábado
// 15/11 a las 19:00" further out. Without hasTime the hour is left out: the
// owner gave the day alone.
func When(startsAt time.Time, hasTime bool, now time.Time) string {
	start := startsAt.In(timezone.Argentina)
	day := timezone.Day(start)
	today := timezone.Day(now.In(timezone.Argentina))
	label := notifications.SpanishWeekday(day.Weekday()) + " " + start.Format("02/01")
	switch {
	case day.Equal(today):
		label = "hoy " + label
	case day.Equal(today.AddDate(0, 0, 1)):
		label = "mañana " + label
	default:
		label = "el " + label
	}
	if hasTime {
		label += " a las " + start.Format("15:04")
	}
	return label
}

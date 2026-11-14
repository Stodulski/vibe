// Package notice builds the notices of an open-play event sign-up from the
// event, the registration and the complex. Two domains send them — payments
// when the money lands or goes back, events when a spot is cancelled — so the
// wording of "when, where, how much" lives once, in a leaf both can import.
package notice

import (
	"fmt"

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
	LineRefunded = "Le devolvimos el pago:"
	// LineRefundQueued says the money is on its way back.
	LineRefundQueued = "Su pago se está devolviendo y lo verá acreditado en los próximos días."
	// LineKept says the cancellation came too late for a refund.
	LineKept = "La cancelación fue fuera de plazo, por lo que el pago no se devuelve."
)

// Day is the event's day as a person reads it: "sábado 15/11".
func Day(ev *eventstore.Event) string {
	return notifications.SpanishWeekday(timezone.Day(ev.Date).Weekday()) + " " + ev.Date.Format("02/01")
}

// Hours is the event's span as a person reads it: "19:00 a 21:00".
func Hours(ev *eventstore.Event) string {
	return timezone.HoursLabel(ev.StartsAt(), ev.EndsAt())
}

// Registered is the confirmation of a person's spot.
func Registered(reg *eventstore.Registration, ev *eventstore.Event, complex *complexstore.Complex, publicURL string) notifications.EventRegistration {
	n := notifications.EventRegistration{
		FirstName:      reg.FirstName,
		ComplexName:    complex.Name,
		EventName:      ev.Name,
		Date:           Day(ev),
		Hours:          Hours(ev),
		Address:        complex.Address,
		MapsURL:        booklink.MapsURL(complex.Name, complex.Address, complex.City, complex.Latitude, complex.Longitude),
		Amount:         notifications.FormatARS(reg.Price + reg.ServiceFee),
		EventURL:       booklink.EventPage(publicURL, complex.Slug, ev.ID),
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

package events

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// endTimeOf is the event's end as a person reads it: HH:MM on the venue's
// wall-clock, earlier than the start when the event runs past midnight.
func endTimeOf(e *eventstore.Event) string {
	return e.EndsAt().In(timezone.Argentina).Format("15:04")
}

// toGenEvent maps an event onto the owner's wire type.
func toGenEvent(e *eventstore.Event) gen.OpenPlayEvent {
	courts := make([]struct {
		Id   openapi_types.UUID `json:"id"`
		Name string             `json:"name"`
	}, len(e.Courts))
	for i, c := range e.Courts {
		courts[i].Id, courts[i].Name = c.ID, c.Name
	}
	return gen.OpenPlayEvent{
		Id: e.ID, ComplexId: e.ComplexID, Name: e.Name, Sport: gen.OpenPlayEventSport(e.Sport),
		Date: openapi_types.Date{Time: e.Date}, StartTime: e.StartTime, EndTime: endTimeOf(e),
		DurationMinutes: e.DurationMinutes, Capacity: e.Capacity, Price: e.Price,
		Status: gen.OpenPlayEventStatus(e.Status), CancelReason: e.CancelReason, CancelledAt: e.CancelledAt,
		Courts: courts, ConfirmedCount: e.ConfirmedCount, HeldCount: e.HeldCount, SpotsLeft: e.SpotsLeft(),
		CreatedAt: e.CreatedAt,
	}
}

func toGenEvents(events []*eventstore.Event) []gen.OpenPlayEvent {
	out := make([]gen.OpenPlayEvent, len(events))
	for i, e := range events {
		out[i] = toGenEvent(e)
	}
	return out
}

// toGenParticipant maps one registration with its payment onto the owner's wire
// type.
func toGenParticipant(r *eventstore.RegistrationRow) gen.OpenPlayParticipant {
	p := gen.OpenPlayParticipant{
		Id: r.ID, FirstName: r.FirstName, LastName: r.LastName, Phone: r.Phone, Email: r.Email,
		Status: gen.OpenPlayParticipantStatus(r.Status), Price: r.Price, ServiceFee: r.ServiceFee,
		CreatedAt: r.CreatedAt, ConfirmedAt: r.ConfirmedAt, CancelledAt: r.CancelledAt,
	}
	if r.CancelledBy != nil {
		by := gen.OpenPlayParticipantCancelledBy(*r.CancelledBy)
		p.CancelledBy = &by
	}
	if r.Payment != nil {
		p.Payment = &struct {
			Amount       int                                  `json:"amount"`
			Method       gen.OpenPlayParticipantPaymentMethod `json:"method"`
			RefundAmount int                                  `json:"refund_amount"`
			ServiceFee   int                                  `json:"service_fee"`
			Status       string                               `json:"status"`
		}{
			Amount: r.Payment.Amount, Method: "mercadopago", RefundAmount: r.Payment.RefundAmount,
			ServiceFee: r.Payment.ServiceFee, Status: r.Payment.Status,
		}
	}
	return p
}

func toGenParticipants(rows []*eventstore.RegistrationRow) []gen.OpenPlayParticipant {
	out := make([]gen.OpenPlayParticipant, len(rows))
	for i, r := range rows {
		out[i] = toGenParticipant(r)
	}
	return out
}

// toGenPublicEvent maps an event onto what a visitor sees.
func toGenPublicEvent(e *eventstore.Event, page *Page) gen.PublicOpenPlayEvent {
	names := make([]string, len(e.Courts))
	for i, c := range e.Courts {
		names[i] = c.Name
	}
	complexName := page.Complex.Name
	hours := page.Complex.CancellationHours
	return gen.PublicOpenPlayEvent{
		Id: e.ID, Name: e.Name, Sport: gen.PublicOpenPlayEventSport(e.Sport),
		Date: openapi_types.Date{Time: e.Date}, StartTime: e.StartTime, EndTime: endTimeOf(e),
		DurationMinutes: e.DurationMinutes, Capacity: e.Capacity, SpotsLeft: e.SpotsLeft(),
		Price: e.Price, ServiceFee: page.ServiceFee, TotalPrice: e.Price + page.ServiceFee,
		Status: gen.PublicOpenPlayEventStatus(e.Status), CourtNames: names,
		ComplexName: &complexName, CancellationHours: &hours,
	}
}

func toGenPublicEvents(page *Page) []gen.PublicOpenPlayEvent {
	out := make([]gen.PublicOpenPlayEvent, len(page.Events))
	for i, e := range page.Events {
		out[i] = toGenPublicEvent(e, page)
	}
	return out
}

// toGenPublicRegistration maps a person's own spot onto the status wire type.
func toGenPublicRegistration(v *View, serviceFee int) gen.PublicEventRegistration {
	page := &Page{Complex: v.Complex, ServiceFee: serviceFee}
	address, name := v.Complex.Address, v.Complex.Name
	held := v.Registration.HeldUntil
	out := gen.PublicEventRegistration{
		Status:    gen.PublicEventRegistrationStatus(v.Registration.Status),
		FirstName: v.Registration.FirstName, LastName: v.Registration.LastName,
		Price: v.Registration.Price, ServiceFee: v.Registration.ServiceFee,
		HeldUntil: &held, Event: toGenPublicEvent(v.Event, page),
		ComplexName: &name, ComplexAddress: &address,
	}
	out.Cancellation.CanCancel = v.Cancellation.CanCancel
	out.Cancellation.WithinWindow = v.Cancellation.WithinWindow
	out.Cancellation.Deadline = v.Cancellation.Deadline
	return out
}

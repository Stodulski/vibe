package notifications

import "context"

// Notices of an open-play event ("canchas libres"). They are email only: the
// approved WhatsApp templates all carry a button bound to a booking, and a
// sign-up has none, so a person who gave no email hears nothing here (email is
// optional everywhere; phone is the required contact).
const (
	// TaskEmailEventRegistered confirms a person's spot once their payment is
	// approved.
	TaskEmailEventRegistered = "email:event_registered"
	// TaskEmailEventCancelled tells a person their spot is over: the owner
	// cancelled the event, or they cancelled their own.
	TaskEmailEventCancelled = "email:event_cancelled"
)

// EventRegistration is the confirmation of a person's spot. An empty Email
// skips the notice: there is no other channel for it.
type EventRegistration struct {
	Email       string
	FirstName   string
	ComplexName string
	EventName   string
	// Date is the event's day as a person reads it ("sábado 15/11"); Hours is
	// the span ("19:00 a 21:00").
	Date  string
	Hours string
	// Address and MapsURL are the "Cómo llegar" line, as in the booking
	// confirmation.
	Address string
	MapsURL string
	// Amount is what was paid, formatted ("$ 8.000").
	Amount string
	// EventURL is the event's public page.
	EventURL string

	// RegistrationID is the deduplication subject and never travels in the
	// payload.
	RegistrationID string
}

// EventCancellation is the notice that a person's spot is over.
type EventCancellation struct {
	Email       string
	FirstName   string
	ComplexName string
	EventName   string
	Date        string
	Hours       string
	// ByOwner is true when the owner cancelled the whole event.
	ByOwner bool
	// RefundLine is the sentence about the money ("Le devolvimos el pago:"); empty
	// says nothing. RefundAmount is the figure that goes after it.
	RefundLine   string
	RefundAmount string
	// EventsURL is the complex's public page, where the agenda is.
	EventsURL string

	// RegistrationID is the deduplication subject.
	RegistrationID string
}

type eventRegisteredEmail struct {
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	ComplexName string `json:"complex_name"`
	EventName   string `json:"event_name"`
	Date        string `json:"date"`
	Hours       string `json:"hours"`
	Address     string `json:"address"`
	MapsURL     string `json:"maps_url"`
	Amount      string `json:"amount"`
	EventURL    string `json:"event_url"`
}

type eventCancelledEmail struct {
	Email        string `json:"email"`
	FirstName    string `json:"first_name"`
	ComplexName  string `json:"complex_name"`
	EventName    string `json:"event_name"`
	Date         string `json:"date"`
	Hours        string `json:"hours"`
	ByOwner      bool   `json:"by_owner"`
	RefundLine   string `json:"refund_line"`
	RefundAmount string `json:"refund_amount"`
	EventsURL    string `json:"events_url"`
}

// EventRegistered confirms a person's spot by email.
func (s *Service) EventRegistered(c EventRegistration) {
	if c.Email == "" {
		return
	}
	s.queue.Enqueue(TaskEmailEventRegistered, eventRegisteredEmail{
		Email: c.Email, FirstName: c.FirstName, ComplexName: c.ComplexName, EventName: c.EventName,
		Date: c.Date, Hours: c.Hours, Address: c.Address, MapsURL: c.MapsURL, Amount: c.Amount, EventURL: c.EventURL,
	}, dedup(TaskEmailEventRegistered, c.Email, c.RegistrationID))
}

// EventCancelled tells a person their spot is over, by email. The subject carries
// who ended it and whether money came back, so a refund that finishes after the
// cancellation notice is a notice of its own.
func (s *Service) EventCancelled(c EventCancellation) {
	if c.Email == "" {
		return
	}
	subject := c.RegistrationID
	if subject != "" {
		subject += ":" + c.RefundAmount
	}
	s.queue.Enqueue(TaskEmailEventCancelled, eventCancelledEmail{
		Email: c.Email, FirstName: c.FirstName, ComplexName: c.ComplexName, EventName: c.EventName,
		Date: c.Date, Hours: c.Hours, ByOwner: c.ByOwner,
		RefundLine: c.RefundLine, RefundAmount: c.RefundAmount, EventsURL: c.EventsURL,
	}, dedup(TaskEmailEventCancelled, c.Email, subject))
}

// registerEventWorkers wires the delivery of the event notices.
// RegisterWorkers calls it.
func (s *Service) registerEventWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailEventRegistered, func(ctx context.Context, p eventRegisteredEmail) error {
		return s.mailer.SendEventRegistered(ctx, p.Email, p.FirstName, p.ComplexName, p.EventName,
			p.Date, p.Hours, p.Address, p.MapsURL, p.Amount, p.EventURL)
	}))
	s.queue.RegisterHandler(handle(TaskEmailEventCancelled, func(ctx context.Context, p eventCancelledEmail) error {
		return s.mailer.SendEventCancelled(ctx, p.Email, p.FirstName, p.ComplexName, p.EventName,
			p.Date, p.Hours, p.ByOwner, p.RefundLine, p.RefundAmount, p.EventsURL)
	}))
}

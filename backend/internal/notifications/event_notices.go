package notifications

import (
	"context"
	"fmt"

	"github.com/stodulski/vibe-server/internal/mailer"
)

// The notices of an event or a tournament beyond the sign-up's confirmation
// and cancellation (events.go): the owner's two about a paid spot, the
// participant's partial refund and reminder, and the tournament's draw and
// match notices.
const (
	// TaskEmailOwnerEventRegistered tells the owner a person paid their spot.
	TaskEmailOwnerEventRegistered = "email:owner_event_registered"
	// TaskEmailOwnerEventCancelled tells the owner a person who had paid gave
	// up their spot.
	TaskEmailOwnerEventCancelled = "email:owner_event_cancelled"
	// TaskEmailEventRefunded tells a person part of their payment came back
	// while their spot stays.
	TaskEmailEventRefunded = "email:event_refunded"
	// TaskEmailEventReminder reminds a person of their event the day before.
	TaskEmailEventReminder = "email:event_reminder"
	// TaskEmailTournamentNotice tells a player about their tournament's draw or
	// one of their matches.
	TaskEmailTournamentNotice = "email:tournament_notice"
)

// OwnerEventNotice is the owner's notice about one person's spot. Only the
// person's own actions reach the owner: their payment, their cancellation.
type OwnerEventNotice struct {
	OwnerID         string
	ComplexName     string
	EventName       string
	Date            string
	Hours           string
	ParticipantName string
	Amount          string
	// RegistrationID is the deduplication subject.
	RegistrationID string
}

// EventRefund is a partial refund of a person's spot that left the spot theirs.
type EventRefund struct {
	Email       string
	FirstName   string
	ComplexName string
	EventName   string
	Date        string
	// Amount is what came back this time, formatted.
	Amount   string
	EventURL string

	ComplexPhone string
	OwnerID      string

	// RegistrationID and RefundedTotal are the deduplication subject: each
	// refund moves the payment's running total, so each is a notice of its own.
	RegistrationID string
	RefundedTotal  int
}

// EventReminder is the day-before reminder of a person's spot.
type EventReminder struct {
	Email       string
	FirstName   string
	ComplexName string
	EventName   string
	// When is the start as the person reads it: "mañana sábado 15/11 a las 19:00".
	When     string
	Address  string
	MapsURL  string
	EventURL string

	ComplexPhone string
	OwnerID      string

	// RegistrationID is the deduplication subject.
	RegistrationID string
}

// TournamentNotice is one player's notice about their tournament.
type TournamentNotice struct {
	Email string
	mailer.TournamentNotice

	ComplexPhone string
	OwnerID      string

	// Subject is the deduplication subject: the draw or the match and the
	// version of it this notice is about (see the builders in tournaments).
	Subject string
}

type ownerEventEmail struct {
	OwnerID         string `json:"owner_id"`
	ComplexName     string `json:"complex_name"`
	EventName       string `json:"event_name"`
	Date            string `json:"date"`
	Hours           string `json:"hours"`
	ParticipantName string `json:"participant_name"`
	Amount          string `json:"amount"`
}

type eventRefundedEmail struct {
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	ComplexName string `json:"complex_name"`
	EventName   string `json:"event_name"`
	Date        string `json:"date"`
	Amount      string `json:"amount"`
	EventURL    string `json:"event_url"`
	clientContact
}

type eventReminderEmail struct {
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	ComplexName string `json:"complex_name"`
	EventName   string `json:"event_name"`
	When        string `json:"when"`
	Address     string `json:"address"`
	MapsURL     string `json:"maps_url"`
	EventURL    string `json:"event_url"`
	clientContact
}

type tournamentNoticeEmail struct {
	Email          string `json:"email"`
	Kind           string `json:"kind"`
	FirstName      string `json:"first_name"`
	ComplexName    string `json:"complex_name"`
	TournamentName string `json:"tournament_name"`
	CategoryName   string `json:"category_name"`
	When           string `json:"when"`
	Court          string `json:"court"`
	Opponent       string `json:"opponent"`
	Address        string `json:"address"`
	MapsURL        string `json:"maps_url"`
	PageURL        string `json:"page_url"`
	clientContact
}

func ownerEventPayload(n OwnerEventNotice) ownerEventEmail {
	return ownerEventEmail{
		OwnerID: n.OwnerID, ComplexName: n.ComplexName, EventName: n.EventName, Date: n.Date, Hours: n.Hours,
		ParticipantName: n.ParticipantName, Amount: n.Amount,
	}
}

// OwnerEventRegistered tells the owner a person paid their spot.
func (s *Service) OwnerEventRegistered(n OwnerEventNotice) {
	if n.OwnerID == "" {
		return
	}
	s.queue.Enqueue(TaskEmailOwnerEventRegistered, ownerEventPayload(n),
		dedup(TaskEmailOwnerEventRegistered, n.OwnerID, n.RegistrationID))
}

// OwnerEventCancelled tells the owner a person who had paid gave up their spot.
// The owner's own cancellations never come here: the owner does not need an
// email about their own click, as with bookings (SourceClientCancel).
func (s *Service) OwnerEventCancelled(n OwnerEventNotice) {
	if n.OwnerID == "" {
		return
	}
	s.queue.Enqueue(TaskEmailOwnerEventCancelled, ownerEventPayload(n),
		dedup(TaskEmailOwnerEventCancelled, n.OwnerID, n.RegistrationID))
}

// EventRefunded tells a person part of their payment came back.
func (s *Service) EventRefunded(r EventRefund) {
	if r.Email == "" || r.Amount == "" {
		return
	}
	subject := ""
	if r.RegistrationID != "" {
		subject = fmt.Sprintf("%s:%d", r.RegistrationID, r.RefundedTotal)
	}
	s.queue.Enqueue(TaskEmailEventRefunded, eventRefundedEmail{
		Email: r.Email, FirstName: r.FirstName, ComplexName: r.ComplexName, EventName: r.EventName,
		Date: r.Date, Amount: r.Amount, EventURL: r.EventURL,
		clientContact: clientContact{ComplexPhone: r.ComplexPhone, OwnerID: r.OwnerID},
	}, dedup(TaskEmailEventRefunded, r.Email, subject))
}

// EventReminderDue queues a person's day-before reminder. It returns the
// queue's refusal, so the sweep leaves the registration unmarked and tries
// again; a dedup hit is not a refusal.
func (s *Service) EventReminderDue(r EventReminder) error {
	if r.Email == "" {
		return nil
	}
	if err := s.queue.TryEnqueue(TaskEmailEventReminder, eventReminderEmail{
		Email: r.Email, FirstName: r.FirstName, ComplexName: r.ComplexName, EventName: r.EventName,
		When: r.When, Address: r.Address, MapsURL: r.MapsURL, EventURL: r.EventURL,
		clientContact: clientContact{ComplexPhone: r.ComplexPhone, OwnerID: r.OwnerID},
	}, dedup(TaskEmailEventReminder, r.Email, r.RegistrationID)); err != nil {
		return fmt.Errorf("queue event reminder email: %w", err)
	}
	return nil
}

// TournamentNoticeDue queues a player's tournament notice. It returns the
// queue's refusal, for a sweep that must try again.
func (s *Service) TournamentNoticeDue(n TournamentNotice) error {
	if n.Email == "" {
		return nil
	}
	if err := s.queue.TryEnqueue(TaskEmailTournamentNotice, tournamentNoticeEmail{
		Email: n.Email, Kind: n.Kind, FirstName: n.FirstName, ComplexName: n.ComplexName,
		TournamentName: n.TournamentName, CategoryName: n.CategoryName,
		When: n.When, Court: n.Court, Opponent: n.Opponent, Address: n.Address, MapsURL: n.MapsURL, PageURL: n.PageURL,
		clientContact: clientContact{ComplexPhone: n.ComplexPhone, OwnerID: n.OwnerID},
	}, dedup(TaskEmailTournamentNotice, n.Email, n.Subject)); err != nil {
		return fmt.Errorf("queue tournament notice email: %w", err)
	}
	return nil
}

// registerEventNoticeWorkers wires the delivery of the notices above.
func (s *Service) registerEventNoticeWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailOwnerEventRegistered, func(ctx context.Context, p ownerEventEmail) error {
		to, err := s.ownerAddress(ctx, "owner event registered", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerEventRegistered(ctx, to, p.notice())
	}))
	s.queue.RegisterHandler(handle(TaskEmailOwnerEventCancelled, func(ctx context.Context, p ownerEventEmail) error {
		to, err := s.ownerAddress(ctx, "owner event cancelled", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerEventCancelled(ctx, to, p.notice())
	}))
	s.queue.RegisterHandler(handle(TaskEmailEventRefunded, func(ctx context.Context, p eventRefundedEmail) error {
		return s.mailer.SendEventRefunded(ctx, p.Email, p.FirstName, p.ComplexName, p.EventName, p.Date,
			p.Amount, p.EventURL, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailEventReminder, func(ctx context.Context, p eventReminderEmail) error {
		return s.mailer.SendEventReminder(ctx, p.Email, p.FirstName, p.ComplexName, p.EventName, p.When,
			p.Address, p.MapsURL, p.EventURL, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailTournamentNotice, func(ctx context.Context, p tournamentNoticeEmail) error {
		return s.mailer.SendTournamentNotice(ctx, p.Email, mailer.TournamentNotice{
			Kind: p.Kind, FirstName: p.FirstName, ComplexName: p.ComplexName, TournamentName: p.TournamentName,
			CategoryName: p.CategoryName, When: p.When, Court: p.Court, Opponent: p.Opponent,
			Address: p.Address, MapsURL: p.MapsURL, PageURL: p.PageURL,
		}, s.contactOf(ctx, p.clientContact))
	}))
}

func (p ownerEventEmail) notice() mailer.OwnerEventNotice {
	return mailer.OwnerEventNotice{
		ComplexName: p.ComplexName, EventName: p.EventName, Date: p.Date, Hours: p.Hours,
		ParticipantName: p.ParticipantName, Amount: p.Amount,
	}
}

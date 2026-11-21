package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mailer"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// ownerAddress resolves an owner's current email from the id a task carries,
// so a task sitting in the queue cannot deliver to an address the owner has
// since changed. op names the task in the errors, so a failed delivery says
// which email it was.
func (s *Service) ownerAddress(ctx context.Context, op, ownerID string) (string, error) {
	id, err := uuid.Parse(ownerID)
	if err != nil {
		return "", fmt.Errorf("%s: invalid owner id: %w", op, err)
	}
	owner, err := s.users.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("%s: fetch owner: %w", op, err)
	}
	return owner.Email, nil
}

// contactOf resolves the complex's contact a client email carries. The owner's
// address is read now, like every owner address, so a queued email replies to
// the address the owner has today. A failed read costs the reply-to and nothing
// else: the client is still told, and the phone line still stands.
func (s *Service) contactOf(ctx context.Context, c clientContact) mailer.Contact {
	contact := mailer.Contact{Phone: c.ComplexPhone}
	if c.OwnerID == "" {
		return contact
	}
	to, err := s.ownerAddress(ctx, "client email reply-to", c.OwnerID)
	if err != nil {
		s.logger.Warn("notifications: sending a client email without the complex's reply-to", "error", err)
		return contact
	}
	contact.ReplyTo = to
	return contact
}

// deliverReminder sends a two-hour reminder, unless its booking stopped being
// one to remind about while the task waited in the queue: it was cancelled, or
// it has already begun. A reminder for a cancelled booking is the email clients
// write in about, and a late one is noise.
//
// A reminder queued before the booking id travelled in the payload has none,
// and is sent as it was. A booking that cannot be read for any reason but its
// absence fails the task, so the queue retries it instead of guessing.
func (s *Service) deliverReminder(ctx context.Context, p Reminder) error {
	contact := s.contactOf(ctx, p.clientContact())
	when := reminderWhen(p, time.Time{}, s.now())
	if p.BookingID != "" {
		id, err := uuid.Parse(p.BookingID)
		if err != nil {
			return fmt.Errorf("reminder: invalid booking id: %w", err)
		}
		// The worker runs outside any request, so it carries no tenant; the
		// booking is read by its id across tenants, as the sweep that queued it
		// did.
		booking, err := s.bookings.GetByID(data.ContextWithTenantBypass(ctx), id)
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			s.logger.Info("reminder: dropped, the booking no longer exists", "booking_id", p.BookingID)
			return nil
		case err != nil:
			return fmt.Errorf("reminder: read booking: %w", err)
		}
		if booking.Status != bookingstore.BookingStatusConfirmed {
			s.logger.Info("reminder: dropped, the booking is no longer confirmed",
				"booking_id", p.BookingID, "status", booking.Status)
			return nil
		}
		if !booking.StartsAt.IsZero() && !s.now().Before(booking.StartsAt) {
			s.logger.Info("reminder: dropped, the booking has already begun", "booking_id", p.BookingID)
			return nil
		}
		when = reminderWhen(p, booking.StartsAt, s.now())
	}
	return s.mailer.SendReminder2h(ctx, p.Email, p.ComplexName, p.CourtName, p.Date, p.StartTime, when,
		p.Address, p.MapsURL, p.BalanceAmount, p.CancelURL, contact)
}

// reReminderClock finds the start of a reminder's hours label ("18:00 a 19:30").
var reReminderClock = regexp.MustCompile(`^\d{2}:\d{2}`)

// reminderWhen is the start as the client reads it on the day the reminder
// arrives: "hoy a las 18:00", or "mañana a las 00:30" for a game just past
// midnight. Without the booking's start (a reminder queued before the booking
// id travelled) it falls back to the date the payload carries.
func reminderWhen(p Reminder, startsAt, now time.Time) string {
	if !startsAt.IsZero() {
		start := startsAt.In(timezone.Argentina)
		today := timezone.Day(now.In(timezone.Argentina))
		switch timezone.Day(start) {
		case today:
			return "hoy a las " + start.Format("15:04")
		case today.AddDate(0, 0, 1):
			return "mañana a las " + start.Format("15:04")
		}
		return "el " + start.Format("02/01") + " a las " + start.Format("15:04")
	}
	clock := reReminderClock.FindString(p.StartTime)
	if clock == "" {
		return "el " + p.Date
	}
	return "el " + p.Date + " a las " + clock
}

// handle adapts a typed delivery function into the queue's raw-payload
// handler, so each registration below is one line about what it delivers
// rather than four about unmarshalling.
func handle[T any](taskType string, deliver func(ctx context.Context, p T) error) (string, func(context.Context, json.RawMessage) error) {
	return taskType, func(ctx context.Context, raw json.RawMessage) error {
		var p T
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("%s: unmarshal payload: %w", taskType, err)
		}
		return deliver(ctx, p)
	}
}

// RegisterWorkers wires each task type to the delivery that performs it. It
// must be called before the queue is started, on every instance that consumes.
//
// one flat registration table: one line of dispatch per task type. Splitting it
// into client workers and account workers would hide the property it exists to
// make readable — that every task type the service can enqueue has exactly one
// handler here, which TestWorkersDeliverEachTaskType checks by name, and a task
// with no handler sits in Redis forever.
//
//nolint:funlen // see the registration-table note above
func (s *Service) RegisterWorkers() {
	s.registerWaitlistWorkers()
	s.registerWaitlistPlaceWorkers()
	s.registerLifecycleWorkers()
	s.registerCampaignWorkers()
	s.registerSeriesWorkers()
	s.registerEventWorkers()
	s.registerBookingNoticeWorkers()

	s.queue.RegisterHandler(handle(TaskEmailBookingConfirmation, func(ctx context.Context, p bookingConfirmationEmail) error {
		return s.mailer.SendBookingConfirmation(ctx, p.Email, p.ComplexName, p.CourtName,
			p.Date, p.StartTime, p.Address, p.MapsURL, p.CancelURL, p.DepositAmount, p.BalanceAmount, p.CancellationLine,
			p.PaymentState, s.contactOf(ctx, p.clientContact))
	}))

	s.queue.RegisterHandler(handle(TaskEmailReminder2h, s.deliverReminder))

	s.queue.RegisterHandler(handle(TaskEmailBookingCancelled, func(ctx context.Context, p bookingCancelledEmail) error {
		return s.mailer.SendBookingCancelled(ctx, p.Email, p.ComplexName, p.CourtName, p.Date, p.StartTime,
			p.RefundLine, p.RefundAmount, p.BookURL, s.contactOf(ctx, p.clientContact))
	}))

	s.queue.RegisterHandler(handle(TaskEmailDepositRefunded, func(ctx context.Context, p depositRefundedEmail) error {
		return s.mailer.SendDepositRefunded(ctx, p.Email, p.ComplexName, p.Amount, p.BookURL,
			p.Cancelled, p.CourtName, p.Date, p.StartTime, p.Reason, p.ByHand, s.contactOf(ctx, p.clientContact))
	}))

	// The owner's address is resolved here rather than captured at enqueue
	// time, so a task sitting in the queue cannot deliver to an address the
	// owner has since changed.
	s.queue.RegisterHandler(handle(TaskEmailOwnerNewBooking, func(ctx context.Context, p ownerNewBooking) error {
		to, err := s.ownerAddress(ctx, "owner new booking", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerNewBooking(ctx, to, p.ComplexName, p.CourtName,
			p.ClientName, p.Date, p.StartTime)
	}))

	s.queue.RegisterHandler(handle(TaskEmailOwnerBookingCancelled, func(ctx context.Context, p ownerBookingCancelled) error {
		to, err := s.ownerAddress(ctx, "owner booking cancelled", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerBookingCancelled(ctx, to, p.ComplexName, p.CourtName,
			p.ClientName, p.Date, p.StartTime)
	}))

	s.queue.RegisterHandler(handle(TaskEmailOwnerBookingRefunded, func(ctx context.Context, p ownerBookingRefunded) error {
		to, err := s.ownerAddress(ctx, "owner booking refunded", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerBookingRefunded(ctx, to, p.ComplexName, p.CourtName,
			p.ClientName, p.Date, p.StartTime)
	}))

	s.queue.RegisterHandler(handle(TaskEmailVerification, func(ctx context.Context, p VerificationEmail) error {
		return s.mailer.SendEmailVerification(ctx, p.To, p.FirstName, p.VerifyURL)
	}))

	s.queue.RegisterHandler(handle(TaskEmailWelcome, func(ctx context.Context, p WelcomeEmail) error {
		return s.mailer.SendWelcome(ctx, p.To, p.FirstName)
	}))

	s.queue.RegisterHandler(handle(TaskEmailPasswordReset, func(ctx context.Context, p PasswordResetEmail) error {
		return s.mailer.SendPasswordReset(ctx, p.To, p.FirstName, p.ResetURL)
	}))

	s.queue.RegisterHandler(handle(TaskEmailDuplicateRegistration, func(ctx context.Context, p DuplicateRegistrationEmail) error {
		return s.mailer.SendDuplicateRegistration(ctx, p.To, p.FirstName, p.LoginURL, p.ResetURL)
	}))

	s.queue.RegisterHandler(handle(TaskEmailChangeRequested, func(ctx context.Context, p EmailChangeRequestedEmail) error {
		return s.mailer.SendEmailChangeRequested(ctx, p.To, p.FirstName, p.NewEmail, p.ConfirmURL, p.ExpiresIn)
	}))

	s.registerLegacyWhatsAppDrain()
}

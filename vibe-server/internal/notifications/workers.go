package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/whatsapp"
)

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
// one flat registration table: twelve task types, one line of dispatch each.
// Splitting it into client workers and account workers would hide the property
// it exists to make readable — that every task type the service can enqueue
// has exactly one handler here, which TestWorkersDeliverEachTaskType checks by
// name, and a task with no handler sits in Redis forever.
//
//nolint:funlen // see the registration-table note above
func (s *Service) RegisterWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailBookingConfirmation, func(ctx context.Context, p bookingConfirmationEmail) error {
		return s.mailer.SendBookingConfirmation(ctx, p.Email, p.ComplexName, p.CourtName,
			p.Date, p.StartTime, p.Address, p.MapsURL, p.CancelURL, p.DepositAmount, p.BalanceAmount, p.CancellationLine)
	}))

	s.queue.RegisterHandler(handle(TaskEmailReminder2h, func(ctx context.Context, p Reminder) error {
		return s.mailer.SendReminder2h(ctx, p.Email, p.ComplexName, p.CourtName, p.Date, p.StartTime,
			p.Address, p.MapsURL, p.BalanceAmount, p.CancelURL)
	}))

	s.queue.RegisterHandler(handle(TaskEmailBookingCancelled, func(ctx context.Context, p bookingCancelledEmail) error {
		return s.mailer.SendBookingCancelled(ctx, p.Email, p.ComplexName, p.CourtName, p.Date, p.StartTime,
			p.RefundLine, p.RefundAmount, p.BookURL)
	}))

	s.queue.RegisterHandler(handle(TaskEmailDepositRefunded, func(ctx context.Context, p depositRefundedEmail) error {
		return s.mailer.SendDepositRefunded(ctx, p.Email, p.ComplexName, p.Amount, p.BookURL)
	}))

	// The owner's address is resolved here rather than captured at enqueue
	// time, so a task sitting in the queue cannot deliver to an address the
	// owner has since changed.
	s.queue.RegisterHandler(handle(TaskEmailOwnerNewBooking, func(ctx context.Context, p ownerNewBooking) error {
		ownerID, err := uuid.Parse(p.OwnerID)
		if err != nil {
			return fmt.Errorf("owner new booking: invalid owner id: %w", err)
		}
		owner, err := s.users.GetByID(ctx, ownerID)
		if err != nil {
			return fmt.Errorf("owner new booking: fetch owner: %w", err)
		}
		return s.mailer.SendOwnerNewBooking(ctx, owner.Email, p.ComplexName, p.CourtName,
			p.ClientName, p.Date, p.StartTime)
	}))

	s.queue.RegisterHandler(handle(TaskEmailVerification, func(ctx context.Context, p VerificationEmail) error {
		return s.mailer.SendEmailVerification(ctx, p.To, p.FirstName, p.VerifyURL)
	}))

	s.queue.RegisterHandler(handle(TaskEmailPasswordReset, func(ctx context.Context, p PasswordResetEmail) error {
		return s.mailer.SendPasswordReset(ctx, p.To, p.FirstName, p.ResetURL)
	}))

	s.queue.RegisterHandler(handle(TaskEmailDuplicateRegistration, func(ctx context.Context, p DuplicateRegistrationEmail) error {
		return s.mailer.SendDuplicateRegistration(ctx, p.To, p.FirstName, p.LoginURL, p.ResetURL)
	}))

	s.queue.RegisterHandler(handle(TaskWABookingConfirmation, func(ctx context.Context, p waBookingConfirmation) error {
		tmpl := whatsapp.BookingConfirmationTemplate(p.CourtName, p.ComplexName, p.Date, p.StartTime,
			p.DepositAmount, p.BalanceAmount, p.CancellationLine, p.MapsQuery, p.CancelPath)
		return s.whatsapp.SendTemplate(ctx, p.Phone, tmpl)
	}))

	s.queue.RegisterHandler(handle(TaskWAReminder2h, func(ctx context.Context, p waReminder) error {
		tmpl := whatsapp.Reminder2hTemplate(p.CourtName, p.ComplexName, p.Date, p.StartTime,
			p.Address, p.BalanceAmount, p.MapsQuery, p.CancelPath)
		return s.whatsapp.SendTemplate(ctx, p.Phone, tmpl)
	}))

	s.queue.RegisterHandler(handle(TaskWABookingCancelled, func(ctx context.Context, p waBookingCancelled) error {
		tmpl := whatsapp.BookingCancelledTemplate(p.CourtName, p.ComplexName, p.Date, p.StartTime,
			p.RefundLine, p.BookPath)
		return s.whatsapp.SendTemplate(ctx, p.Phone, tmpl)
	}))

	s.queue.RegisterHandler(handle(TaskWADepositRefunded, func(ctx context.Context, p waDepositRefunded) error {
		tmpl := whatsapp.DepositRefundedTemplate(p.Amount, p.ComplexName, p.BookPath)
		return s.whatsapp.SendTemplate(ctx, p.Phone, tmpl)
	}))
}

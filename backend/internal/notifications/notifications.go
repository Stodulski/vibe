package notifications

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/whatsapp"
)

// Queue is the durable task queue this package publishes to and consumes from.
type Queue interface {
	Enqueue(taskType string, payload any)
	RegisterHandler(taskType string, h func(ctx context.Context, payload json.RawMessage) error)
}

// Mailer is the mail side of delivery.
type Mailer interface {
	SendBookingConfirmation(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine string) error
	SendReminder2h(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, balanceAmount, cancelURL string) error
	SendBookingCancelled(ctx context.Context, to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL string) error
	SendDepositRefunded(ctx context.Context, to, complexName, amount, bookURL string) error
	SendOwnerNewBooking(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error
	SendEmailVerification(ctx context.Context, to, firstName, verifyURL string) error
	SendPasswordReset(ctx context.Context, to, firstName, resetURL string) error
	SendDuplicateRegistration(ctx context.Context, to, firstName, loginURL, resetURL string) error
}

// WhatsAppSender is the WhatsApp side of delivery.
type WhatsAppSender interface {
	SendTemplate(ctx context.Context, to string, msg whatsapp.TemplateMessage) error
}

// UserReader resolves the owner an owner-facing email is addressed to.
type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*authstore.User, error)
}

// Service enqueues notifications and, on the worker side, delivers them.
//
// Unlike circuitbreaker.CircuitBreaker and auth.TokenBlacklist, this type is
// intentionally NOT safe on a nil receiver, and must not be made so. A nil
// breaker is a valid configuration — the caller chose not to wrap the service.
// A nil Service is never a configuration: it is a wiring bug, and the only way
// it reaches a handler is by being captured before it was constructed. Adding
// no-op nil guards here would turn that panic into silently dropped
// verification emails, password resets and booking confirmations, which is
// strictly worse than crashing. The guard belongs at boot (see
// newApplication/validateDeps in cmd/api), not on every method here.
type Service struct {
	queue           Queue
	mailer          Mailer
	whatsapp        WhatsAppSender
	users           UserReader
	logger          *slog.Logger
	whatsappEnabled bool
}

// NewService returns a Service. whatsappEnabled reflects whether WhatsApp
// credentials are configured; when it is false, WhatsApp notifications are
// skipped rather than enqueued and failed.
func NewService(queue Queue, mailer Mailer, sender WhatsAppSender, users UserReader, logger *slog.Logger, whatsappEnabled bool) *Service {
	return &Service{
		queue:           queue,
		mailer:          mailer,
		whatsapp:        sender,
		users:           users,
		logger:          logger,
		whatsappEnabled: whatsappEnabled,
	}
}

// wantsWhatsApp reports whether a WhatsApp message should be enqueued for this
// recipient: the channel has to be configured and they have to have given a
// number.
func (s *Service) wantsWhatsApp(phone string) bool {
	return s.whatsappEnabled && phone != ""
}

// BookingConfirmed notifies the client on every channel they gave us, and the
// owner by email.
//
// Each channel is independent: a client with no email still gets WhatsApp, and
// the owner is told either way.
func (s *Service) BookingConfirmed(c BookingConfirmation) {
	if c.Email != "" {
		s.queue.Enqueue(TaskEmailBookingConfirmation, bookingConfirmationEmail{
			Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
			Date: c.Date, StartTime: c.StartTime,
			Address: c.Address, MapsURL: c.MapsURL,
			CancelURL:     c.CancelURL,
			DepositAmount: c.DepositAmount, BalanceAmount: c.BalanceAmount, CancellationLine: c.CancellationLine,
		})
	}

	if s.wantsWhatsApp(c.Phone) {
		// The template binds both buttons, so a message missing either
		// parameter is rejected by Meta and the task can only fail.
		//
		// This guard used to also cover a complex with no coordinates, which
		// meant every venue that had never filled in its latitude lost the
		// WhatsApp channel entirely — silently, with the email still going out
		// to hide it. booklink.MapsQuery now falls back to the written
		// address, so an empty maps query means a complex with no name and no
		// address, which is not a thing this system can produce.
		if c.CancelPath == "" || c.MapsQuery == "" {
			s.logger.Warn("notifications: skipping WhatsApp confirmation, missing button parameters",
				"source", c.Source,
				"cancel_path_empty", c.CancelPath == "",
				"maps_query_empty", c.MapsQuery == "",
			)
		} else {
			s.queue.Enqueue(TaskWABookingConfirmation, waBookingConfirmation{
				Phone: c.Phone, ComplexName: c.ComplexName, CourtName: c.CourtName,
				Date: c.Date, StartTime: c.StartTime,
				DepositAmount: c.DepositAmount, BalanceAmount: c.BalanceAmount, CancellationLine: c.CancellationLine,
				CancelPath: c.CancelPath, MapsQuery: c.MapsQuery,
			})
		}
	}

	// The owner is told about a sale, not about their own data entry. An owner
	// who books a walk-in from the dashboard used to get an email announcing
	// the booking they had just made, every time.
	if c.Source != SourceStaffCreate {
		s.queue.Enqueue(TaskEmailOwnerNewBooking, ownerNewBooking{
			OwnerID: c.OwnerID, ComplexName: c.ComplexName, CourtName: c.CourtName,
			ClientName: c.ClientName, Date: c.Date, StartTime: c.StartTime,
		})
	}
}

// ReminderDue notifies the client that their booking starts in two hours.
func (s *Service) ReminderDue(rem Reminder) {
	if rem.Email != "" {
		s.queue.Enqueue(TaskEmailReminder2h, rem)
	}
	if s.wantsWhatsApp(rem.Phone) {
		// Same unbound-button rule as the confirmation. The cancel path is the
		// one that can genuinely be missing here: the cron mints a fresh
		// access token per reminder, and a mint that fails leaves nothing to
		// bind the button to.
		if rem.CancelPath == "" || rem.MapsQuery == "" {
			s.logger.Warn("notifications: skipping WhatsApp reminder, missing button parameters",
				"cancel_path_empty", rem.CancelPath == "",
				"maps_query_empty", rem.MapsQuery == "",
			)
		} else {
			s.queue.Enqueue(TaskWAReminder2h, waReminder{
				Phone: rem.Phone, ComplexName: rem.ComplexName, CourtName: rem.CourtName,
				Date: rem.Date, StartTime: rem.StartTime,
				Address: rem.Address, BalanceAmount: rem.BalanceAmount,
				MapsQuery: rem.MapsQuery, CancelPath: rem.CancelPath,
			})
		}
	}
}

// BookingCancelled notifies the client that their booking is off, and what
// became of their deposit.
func (s *Service) BookingCancelled(c Cancellation) {
	if c.Email != "" {
		s.queue.Enqueue(TaskEmailBookingCancelled, bookingCancelledEmail{
			Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
			Date: c.Date, StartTime: c.StartTime,
			RefundLine: c.RefundLine, RefundAmount: c.RefundAmount, BookURL: c.BookURL,
		})
	}
	if s.wantsWhatsApp(c.Phone) {
		if c.BookPath == "" {
			s.logger.Warn("notifications: skipping WhatsApp cancellation, missing button parameter")
		} else {
			s.queue.Enqueue(TaskWABookingCancelled, waBookingCancelled{
				Phone: c.Phone, ComplexName: c.ComplexName, CourtName: c.CourtName,
				Date: c.Date, StartTime: c.StartTime,
				RefundLine: c.RefundLine, BookPath: c.BookPath,
			})
		}
	}
}

// DepositRefunded notifies the client that their deposit is on its way back.
func (s *Service) DepositRefunded(ref Refund) {
	if ref.Email != "" {
		s.queue.Enqueue(TaskEmailDepositRefunded, depositRefundedEmail{
			Email: ref.Email, ComplexName: ref.ComplexName,
			Amount: ref.Amount, BookURL: ref.BookURL,
		})
	}
	if s.wantsWhatsApp(ref.Phone) {
		if ref.BookPath == "" {
			s.logger.Warn("notifications: skipping WhatsApp refund notice, missing button parameter")
		} else {
			s.queue.Enqueue(TaskWADepositRefunded, waDepositRefunded{
				Phone: ref.Phone, ComplexName: ref.ComplexName,
				Amount: ref.Amount, BookPath: ref.BookPath,
			})
		}
	}
}

// EmailVerification asks a new account to confirm its address.
func (s *Service) EmailVerification(e VerificationEmail) {
	s.queue.Enqueue(TaskEmailVerification, e)
}

// PasswordReset sends the reset link.
func (s *Service) PasswordReset(e PasswordResetEmail) {
	s.queue.Enqueue(TaskEmailPasswordReset, e)
}

// DuplicateRegistration tells someone who tried to register again that they
// already have an account, and offers a sign-in and a reset link.
//
// It exists so registration can answer identically whether or not the address
// is taken: telling the caller would make the endpoint an account oracle.
func (s *Service) DuplicateRegistration(e DuplicateRegistrationEmail) {
	s.queue.Enqueue(TaskEmailDuplicateRegistration, e)
}

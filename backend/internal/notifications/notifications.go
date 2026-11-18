package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/jobs"
)

// Queue is the durable task queue this package publishes to and consumes from.
//
// dedupKey stops the same delivery being queued twice: an enqueue carrying a key
// already in the queue does nothing, so a redelivered provider webhook adds no
// second task for one booking. It does not make the delivery idempotent. The
// queue is at-least-once, so a claim whose acknowledgement was lost, or a crash
// after the provider accepted the message, runs the task again, and for this
// package "again" can mean a second confirmation email for one booking. The
// mail client in this repository does not send an idempotency key. An empty key
// means this delivery is not deduplicated.
type Queue interface {
	Enqueue(taskType string, payload any, dedupKey string)
	// TryEnqueue is Enqueue for a task whose caller must know it was not
	// recorded: it returns the failure instead of logging it. A dedup hit is
	// not a failure and returns nil.
	TryEnqueue(taskType string, payload any, dedupKey string) error
	RegisterHandler(taskType string, h func(ctx context.Context, payload json.RawMessage) error)
}

// Mailer is the mail side of delivery.
type Mailer interface {
	SendBookingConfirmation(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine string) error
	SendReminder2h(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, balanceAmount, cancelURL string) error
	SendBookingCancelled(ctx context.Context, to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL string) error
	SendDepositRefunded(ctx context.Context, to, complexName, amount, bookURL string) error
	SendOwnerNewBooking(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error
	SendOwnerBookingCancelled(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error
	SendOwnerBookingRefunded(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error
	SendEmailVerification(ctx context.Context, to, firstName, verifyURL string) error
	SendPasswordReset(ctx context.Context, to, firstName, resetURL string) error
	SendDuplicateRegistration(ctx context.Context, to, firstName, loginURL, resetURL string) error
	SendEmailChangeRequested(ctx context.Context, to, firstName, newEmail, confirmURL, expiresIn string) error
	// SendSeriesCreated and SendSeriesCancelled are the two notices of a fixed
	// weekly booking. See series.go.
	SendSeriesCreated(ctx context.Context, to, complexName, courtName, weekdays, hours, from, until, address, mapsURL string) error
	SendSeriesCancelled(ctx context.Context, to, complexName, courtName, weekdays, hours, from string, keptPaid bool, bookURL string) error
	// SendEventRegistered and SendEventCancelled are the two notices of an
	// open-play event sign-up. See events.go.
	SendEventRegistered(ctx context.Context, to, firstName, complexName, eventName, date, hours, address, mapsURL, amount, eventURL string) error
	SendEventCancelled(ctx context.Context, to, firstName, complexName, eventName, date, hours string, byOwner bool, refundLine, refundAmount, eventsURL string) error
	SendWaitlistSlotFreed(ctx context.Context, to, complexName, date, bookURL string) error
	SendWaitlistPlaceFreed(ctx context.Context, to, complexName, placeLabel, bookURL string) error
	SendLifecycleNudge(ctx context.Context, to, kind, firstName, complexName, ctaURL, unsubscribeURL, optOutURL string) error
	SendWelcome(ctx context.Context, to, firstName string) error
	// SendBenefitsCampaign is the one-off benefits email. See campaign.go.
	SendBenefitsCampaign(ctx context.Context, to, firstName, segment, segmentURL, panelURL, unsubscribeURL, optOutURL string) error
	// SendSocialKitCampaign is the one-off social kit email. See campaign.go.
	SendSocialKitCampaign(ctx context.Context, to, firstName, imageURL, kitURL, unsubscribeURL, optOutURL string) error
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
	queue  Queue
	mailer Mailer
	users  UserReader
	logger *slog.Logger
}

// NewService returns a Service.
func NewService(queue Queue, mailer Mailer, users UserReader, logger *slog.Logger) *Service {
	return &Service{
		queue:  queue,
		mailer: mailer,
		users:  users,
		logger: logger,
	}
}

// BookingConfirmed notifies the client by email, when they gave us an address,
// and the owner by email.
//
// Each recipient is independent: a client with no email is skipped, and the
// owner is told either way.
func (s *Service) BookingConfirmed(c BookingConfirmation) {
	if c.Email != "" {
		s.queue.Enqueue(TaskEmailBookingConfirmation, bookingConfirmationEmail{
			Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
			Date: c.Date, StartTime: c.StartTime,
			Address: c.Address, MapsURL: c.MapsURL,
			CancelURL:     c.CancelURL,
			DepositAmount: c.DepositAmount, BalanceAmount: c.BalanceAmount, CancellationLine: c.CancellationLine,
		}, dedup(TaskEmailBookingConfirmation, c.Email, c.BookingID))
	}

	// The owner is told about a sale, not about their own data entry. An owner
	// who books a walk-in from the dashboard used to get an email announcing
	// the booking they had just made, every time.
	if c.Source != SourceStaffCreate {
		s.queue.Enqueue(TaskEmailOwnerNewBooking, ownerNewBooking{
			OwnerID: c.OwnerID, ComplexName: c.ComplexName, CourtName: c.CourtName,
			ClientName: c.ClientName, Date: c.Date, StartTime: c.StartTime,
		}, dedup(TaskEmailOwnerNewBooking, c.OwnerID, c.BookingID))
	}
}

// ReminderDue queues the client's two-hour reminder by email.
//
// It returns the queue's refusal, so the caller can leave the booking unflagged
// and try again. A dedup hit is not a refusal: the queue reports it as success,
// and a retry queues the email again under the same key, which does nothing.
func (s *Service) ReminderDue(rem Reminder) error {
	if rem.Email != "" {
		if err := s.queue.TryEnqueue(TaskEmailReminder2h, rem, dedup(TaskEmailReminder2h, rem.Email, rem.BookingID)); err != nil {
			return fmt.Errorf("queue 2h reminder email: %w", err)
		}
	}
	return nil
}

// BookingCancelled notifies the client that their booking is off, and what
// became of their deposit.
func (s *Service) BookingCancelled(c Cancellation) {
	if c.Email != "" {
		s.queue.Enqueue(TaskEmailBookingCancelled, bookingCancelledEmail{
			Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
			Date: c.Date, StartTime: c.StartTime,
			RefundLine: c.RefundLine, RefundAmount: c.RefundAmount, BookURL: c.BookURL,
		}, dedup(TaskEmailBookingCancelled, c.Email, c.BookingID))
	}

	// The owner is told only when the client cancelled: that is the one way a
	// slot comes back that the owner did not decide on. See SourceClientCancel.
	if c.Source == SourceClientCancel && c.OwnerID != "" {
		s.queue.Enqueue(TaskEmailOwnerBookingCancelled, ownerBookingCancelled{
			OwnerID: c.OwnerID, ComplexName: c.ComplexName, CourtName: c.CourtName,
			ClientName: c.ClientName, Date: c.Date, StartTime: c.StartTime,
		}, dedup(TaskEmailOwnerBookingCancelled, c.OwnerID, c.BookingID))
	}
}

// OwnerBookingRefunded tells the complex owner that a refund MercadoPago issued
// on its own side cancelled a live booking. It is the owner's only notice for
// that cancellation; the client is told about the money through DepositRefunded.
func (s *Service) OwnerBookingRefunded(r OwnerBookingRefund) {
	if r.OwnerID == "" {
		return
	}
	s.queue.Enqueue(TaskEmailOwnerBookingRefunded, ownerBookingRefunded{
		OwnerID: r.OwnerID, ComplexName: r.ComplexName, CourtName: r.CourtName,
		ClientName: r.ClientName, Date: r.Date, StartTime: r.StartTime,
	}, dedup(TaskEmailOwnerBookingRefunded, r.OwnerID, r.BookingID))
}

// DepositRefunded notifies the client that their deposit is on its way back.
func (s *Service) DepositRefunded(ref Refund) {
	if ref.Email != "" {
		s.queue.Enqueue(TaskEmailDepositRefunded, depositRefundedEmail{
			Email: ref.Email, ComplexName: ref.ComplexName,
			Amount: ref.Amount, BookURL: ref.BookURL,
		}, dedup(TaskEmailDepositRefunded, ref.Email, refundSubject(ref)))
	}
}

// refundSubject is what a refund notice is about, for its deduplication key: the
// booking and which refund of it this is. A booking refunded in parts has one
// notice per part, so the booking alone would let the queue drop every part after
// the first as a redelivery. A notice missing either half gets no key at all, the
// same honest answer dedup gives every other blank subject.
func refundSubject(ref Refund) string {
	if ref.BookingID == "" || ref.RefundID == "" {
		return ""
	}
	return ref.BookingID + ":" + ref.RefundID
}

// EmailVerification asks a new account to confirm its address.
func (s *Service) EmailVerification(e VerificationEmail) {
	s.queue.Enqueue(TaskEmailVerification, e, dedup(TaskEmailVerification, e.To, e.VerifyURL))
}

// PasswordReset sends the reset link.
func (s *Service) PasswordReset(e PasswordResetEmail) {
	s.queue.Enqueue(TaskEmailPasswordReset, e, dedup(TaskEmailPasswordReset, e.To, e.ResetURL))
}

// DuplicateRegistration tells someone who tried to register again that they
// already have an account, and offers a sign-in and a reset link.
//
// It exists so registration can answer identically whether or not the address
// is taken: telling the caller would make the endpoint an account oracle.
func (s *Service) DuplicateRegistration(e DuplicateRegistrationEmail) {
	s.queue.Enqueue(TaskEmailDuplicateRegistration, e, dedup(TaskEmailDuplicateRegistration, e.To, e.ResetURL))
}

// EmailChangeRequested tells the account's current address that a session
// asked to move it to e.NewEmail, with a link to confirm — or ignore, if this
// was not them.
func (s *Service) EmailChangeRequested(e EmailChangeRequestedEmail) {
	s.queue.Enqueue(TaskEmailChangeRequested, e, dedup(TaskEmailChangeRequested, e.To, e.ConfirmURL))
}

// dedup builds the key that makes one delivery idempotent: the task type, the
// address it is going to, and the thing it is about — a booking, or the
// one-use token in the link the mail carries.
//
// An empty result means this delivery is not deduplicated, and that is the
// honest answer when either part is missing: a key built from a blank subject
// would be the same key for every recipient of that task type, which is not a
// weaker guarantee but a wrong one — the second client's confirmation would be
// silently dropped as a duplicate of the first's.
func dedup(taskType, recipient, subject string) string {
	if recipient == "" || subject == "" {
		return ""
	}
	return jobs.DedupKey(taskType, recipient, subject)
}

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/mailer"
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
	// The client emails about a booking, a fixed booking or an event end with
	// a mailer.Contact: the complex's phone, and the owner's address as the
	// reply-to. See contact.go.
	SendBookingConfirmation(ctx context.Context, to, complexName, courtName, date, startTime, address, mapsURL, cancelURL, depositAmount, balanceAmount, cancellationLine, paymentState string, contact mailer.Contact) error
	SendReminder2h(ctx context.Context, to, complexName, courtName, date, startTime, when, address, mapsURL, balanceAmount, cancelURL string, contact mailer.Contact) error
	SendBookingCancelled(ctx context.Context, to, complexName, courtName, date, startTime, refundLine, refundAmount, bookURL string, contact mailer.Contact) error
	SendDepositRefunded(ctx context.Context, to, complexName, amount, bookURL string, cancelled bool, courtName, date, startTime, reason string, byHand bool, contact mailer.Contact) error
	SendOwnerNewBooking(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error
	SendOwnerBookingCancelled(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error
	SendOwnerBookingRefunded(ctx context.Context, to, complexName, courtName, clientName, date, startTime string) error
	SendEmailVerification(ctx context.Context, to, firstName, verifyURL string) error
	SendPasswordReset(ctx context.Context, to, firstName, resetURL string) error
	SendDuplicateRegistration(ctx context.Context, to, firstName, loginURL, resetURL string) error
	SendEmailChangeRequested(ctx context.Context, to, firstName, newEmail, confirmURL, expiresIn string) error
	// The security notices and the owner's connection notices. See account.go.
	SendPasswordChanged(ctx context.Context, to, firstName string) error
	SendEmailChanged(ctx context.Context, to, firstName, newEmail string) error
	SendAccountDeleted(ctx context.Context, to, firstName string) error
	SendOwnerMPConnected(ctx context.Context, to, complexName string) error
	SendOwnerMPDisconnected(ctx context.Context, to, complexName string) error
	SendOwnerMPRefreshFailed(ctx context.Context, to, complexName string) error
	SendOwnerInstagramReconnect(ctx context.Context, to, complexName string) error
	// SendSeriesCreated and SendSeriesCancelled are the two notices of a fixed
	// weekly booking. See series.go.
	SendSeriesCreated(ctx context.Context, to, complexName, courtName, weekdays, hours, from, until, address, mapsURL string, moved []string, contact mailer.Contact) error
	SendSeriesCancelled(ctx context.Context, to, complexName, courtName, weekdays, hours, from string, keptPaid bool, bookURL string, contact mailer.Contact) error
	// SendOwnerSeriesDatesLost tells the owner which dates of a fixed booking
	// the scheduler could not book; lines are one sentence per date.
	SendOwnerSeriesDatesLost(ctx context.Context, to, complexName, courtName, clientName, weekdays, hours string, lines []string) error
	// SendEventRegistered and SendEventCancelled are the two notices of an
	// open-play event sign-up. See events.go.
	SendEventRegistered(ctx context.Context, to, firstName, complexName, eventName, date, hours, address, mapsURL, amount, eventURL string, contact mailer.Contact) error
	SendEventCancelled(ctx context.Context, to, firstName, complexName, eventName, date, hours string, byOwner bool, refundLine, refundAmount, eventsURL string, contact mailer.Contact) error
	// The owner's notices of a paid spot, a participant's partial refund and
	// reminder, and a tournament's draw and match notices. See event_notices.go.
	SendOwnerEventRegistered(ctx context.Context, to string, n mailer.OwnerEventNotice) error
	SendOwnerEventCancelled(ctx context.Context, to string, n mailer.OwnerEventNotice) error
	SendEventRefunded(ctx context.Context, to, firstName, complexName, eventName, date, amount, eventURL string, contact mailer.Contact) error
	SendEventReminder(ctx context.Context, to, firstName, complexName, eventName, when, address, mapsURL, eventURL string, contact mailer.Contact) error
	SendTournamentNotice(ctx context.Context, to string, n mailer.TournamentNotice, contact mailer.Contact) error
	SendWaitlistSlotFreed(ctx context.Context, to, complexName, date, bookURL string) error
	SendWaitlistPlaceFreed(ctx context.Context, to, complexName, placeLabel, bookURL string) error
	SendLifecycleNudge(ctx context.Context, to, kind, firstName, complexName, ctaURL, unsubscribeURL, optOutURL string) error
	SendWelcome(ctx context.Context, to, firstName string) error
	// SendBenefitsCampaign is the one-off benefits email. See campaign.go.
	SendBenefitsCampaign(ctx context.Context, to, firstName, segment, segmentURL, panelURL, unsubscribeURL, optOutURL string) error
	// SendSocialKitCampaign is the one-off social kit email. See campaign.go.
	SendSocialKitCampaign(ctx context.Context, to, firstName, imageURL, kitURL, unsubscribeURL, optOutURL string) error
	// The booking notices of booking_notices.go.
	SendBookingNoShow(ctx context.Context, to, complexName, courtName, date, startTime, bookURL string, contact mailer.Contact) error
	SendPaymentReceived(ctx context.Context, to, complexName, courtName, date, startTime, amount, method, balanceAmount, bookURL string, contact mailer.Contact) error
	SendRefundOwedByVenue(ctx context.Context, to, complexName, courtName, date, startTime, amount, bookURL string, contact mailer.Contact) error
	SendOwnerRefundOwed(ctx context.Context, to, complexName, courtName, clientName, date, startTime, amount string) error
	SendOwnerDuplicatePaymentRefunded(ctx context.Context, to, complexName, courtName, clientName, date, startTime, amount string) error
}

// UserReader resolves the owner an owner-facing email is addressed to.
type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*authstore.User, error)
}

// BookingReader reads a booking back at delivery time, so the reminder worker
// can drop a reminder whose booking was cancelled or has already begun since it
// was queued.
type BookingReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*bookingstore.Booking, error)
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
	queue    Queue
	mailer   Mailer
	users    UserReader
	bookings BookingReader
	logger   *slog.Logger
	// now is the worker's clock, a field so a test can pin it.
	now func() time.Time
}

// NewService returns a Service.
func NewService(queue Queue, mailer Mailer, users UserReader, bookings BookingReader, logger *slog.Logger) *Service {
	return &Service{
		queue:    queue,
		mailer:   mailer,
		users:    users,
		bookings: bookings,
		logger:   logger,
		now:      time.Now,
	}
}

// BookingConfirmed notifies the client by email, when they gave us an address,
// and the owner by email.
//
// Each recipient is independent: a client with no email is skipped, and the
// owner is told either way.
//
// It returns the queue's refusal of the client's confirmation, which is the
// notice a client acts on: the caller has the booking id and the context to
// report it so it can be resent. The owner's notice is still queued after a
// refusal, and its own failure is logged by the queue.
func (s *Service) BookingConfirmed(c BookingConfirmation) error {
	var err error
	if c.Email != "" {
		if qerr := s.queue.TryEnqueue(TaskEmailBookingConfirmation, bookingConfirmationEmail{
			Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
			Date: c.Date, StartTime: c.StartTime,
			Address: c.Address, MapsURL: c.MapsURL,
			CancelURL:     c.CancelURL,
			DepositAmount: c.DepositAmount, BalanceAmount: c.BalanceAmount, CancellationLine: c.CancellationLine,
			PaymentState:  c.PaymentState,
			clientContact: clientContact{ComplexPhone: c.ComplexPhone, OwnerID: c.OwnerID},
		}, dedup(TaskEmailBookingConfirmation, c.Email, c.BookingID)); qerr != nil {
			err = fmt.Errorf("queue booking confirmation email: %w", qerr)
		}
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
	return err
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
// became of their deposit. It returns the queue's refusal of the client's
// notice, for the caller to report with the booking it is about.
func (s *Service) BookingCancelled(c Cancellation) error {
	var err error
	if c.Email != "" {
		if qerr := s.queue.TryEnqueue(TaskEmailBookingCancelled, bookingCancelledEmail{
			Email: c.Email, ComplexName: c.ComplexName, CourtName: c.CourtName,
			Date: c.Date, StartTime: c.StartTime,
			RefundLine: c.RefundLine, RefundAmount: c.RefundAmount, BookURL: c.BookURL,
			clientContact: clientContact{ComplexPhone: c.ComplexPhone, OwnerID: c.OwnerID},
		}, dedup(TaskEmailBookingCancelled, c.Email, c.BookingID)); qerr != nil {
			err = fmt.Errorf("queue booking cancelled email: %w", qerr)
		}
	}

	// The owner is told only when the client cancelled: that is the one way a
	// slot comes back that the owner did not decide on. See SourceClientCancel.
	if c.Source == SourceClientCancel && c.OwnerID != "" {
		s.queue.Enqueue(TaskEmailOwnerBookingCancelled, ownerBookingCancelled{
			OwnerID: c.OwnerID, ComplexName: c.ComplexName, CourtName: c.CourtName,
			ClientName: c.ClientName, Date: c.Date, StartTime: c.StartTime,
		}, dedup(TaskEmailOwnerBookingCancelled, c.OwnerID, c.BookingID))
	}
	return err
}

// OwnerBookingRefunded tells the complex owner that a refund MercadoPago issued
// on its own side cancelled a live booking. It is the owner's only notice for
// that cancellation; the client is told through DepositRefunded, with
// Refund.Cancelled set so their one email says the booking is off as well.
func (s *Service) OwnerBookingRefunded(r OwnerBookingRefund) {
	if r.OwnerID == "" {
		return
	}
	s.queue.Enqueue(TaskEmailOwnerBookingRefunded, ownerBookingRefunded{
		OwnerID: r.OwnerID, ComplexName: r.ComplexName, CourtName: r.CourtName,
		ClientName: r.ClientName, Date: r.Date, StartTime: r.StartTime,
	}, dedup(TaskEmailOwnerBookingRefunded, r.OwnerID, r.BookingID))
}

// DepositRefunded notifies the client that their money is on its way back,
// and, when ref.Cancelled, that the refund cancelled their booking. It returns
// the queue's refusal, for the caller to report with the booking.
func (s *Service) DepositRefunded(ref Refund) error {
	if ref.Email == "" {
		return nil
	}
	if err := s.queue.TryEnqueue(TaskEmailDepositRefunded, depositRefundedEmail{
		Email: ref.Email, ComplexName: ref.ComplexName,
		Amount: ref.Amount, BookURL: ref.BookURL,
		Cancelled: ref.Cancelled, CourtName: ref.CourtName, Date: ref.Date, StartTime: ref.StartTime,
		Reason: ref.Reason, ByHand: ref.ByHand,
		clientContact: clientContact{ComplexPhone: ref.ComplexPhone, OwnerID: ref.OwnerID},
	}, dedup(TaskEmailDepositRefunded, ref.Email, refundSubject(ref))); err != nil {
		return fmt.Errorf("queue refund email: %w", err)
	}
	return nil
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

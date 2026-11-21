// Package notifications decides what to tell a client or an owner, and hands
// the delivery to a durable queue.
//
// Nothing here sends anything synchronously. Every call enqueues a task that an
// independent worker pool picks up, so a slow mail provider or a provider
// outage cannot delay — or fail — the booking that triggered it.
package notifications

import "strings"

// Task types on the durable queue. They are stored in Redis, so renaming one
// strands whatever is already enqueued under the old name.
const (
	TaskEmailBookingConfirmation   = "email:booking_confirmation"
	TaskEmailReminder2h            = "email:reminder_2h"
	TaskEmailBookingCancelled      = "email:booking_cancelled"
	TaskEmailDepositRefunded       = "email:deposit_refunded"
	TaskEmailOwnerNewBooking       = "email:owner_new_booking"
	TaskEmailOwnerBookingCancelled = "email:owner_booking_cancelled"
	TaskEmailOwnerBookingRefunded  = "email:owner_booking_refunded"
	TaskEmailVerification          = "email:verification"
	TaskEmailPasswordReset         = "email:password_reset"
	TaskEmailDuplicateRegistration = "email:duplicate_registration"
	TaskEmailChangeRequested       = "email:change_requested"
)

// ScrubsPayload reports whether a notification task's payload is emptied once
// its job finishes (see jobs.Config.ScrubPayload). Every notification payload
// is a recipient, often with a single-use link (cancel, reset, verification),
// and nothing reads it after the attempt. Every task type above is named
// "email:...", which is the prefix this policy keys on. The "wa:" prefix stays
// for the rows of the removed WhatsApp channel that were still queued when it
// went: they hold phone numbers, and the drain handler that completes them
// (legacy_whatsapp.go) must not leave those behind. Other queues keep their
// payloads: the payments export reads its own back after it is done.
func ScrubsPayload(taskType string) bool {
	return strings.HasPrefix(taskType, "email:") || strings.HasPrefix(taskType, "wa:")
}

// The flows that confirm a booking. Source is a log field for every consumer
// but one: the owner's "Nueva reserva" email is suppressed for the staff route,
// because the owner is the person who just made that booking and does not need
// an email announcing their own click.
//
// The default for an unrecognised source is to send it. A flow added later and
// not listed here is a flow nobody thought about, and a spurious owner email is
// a smaller failure than a sale the owner never hears about.
const (
	// SourceStaffCreate is internal/bookings.Handler.Create — the owner or
	// their staff entering a booking from the dashboard.
	SourceStaffCreate = "create booking"
	// SourceOnlineCheckout is internal/payments' MercadoPago webhook
	// confirming a public client's payment.
	SourceOnlineCheckout = "mp webhook"
)

// The flows that cancel a booking. Only a client's own cancellation reaches the
// owner: the owner's panel cancel is their own action, and an expired unpaid
// booking was never announced to them, so there is nothing to take back.
const (
	// SourceClientCancel is internal/bookings.Service.PublicCancel, the client
	// cancelling from the link in their confirmation.
	SourceClientCancel = "client cancel"
	// SourceOwnerCancel is internal/bookings.Service.Cancel, the owner or their
	// staff cancelling from the dashboard.
	SourceOwnerCancel = "owner cancel"
	// SourceExpiredUnpaid is internal/bookings.Service.ReleaseExpiredPayments.
	SourceExpiredUnpaid = "payment expired"
)

// The payloads below are serialised into Redis, so their JSON tags are a
// storage format: changing one breaks tasks already in the queue.

// BookingConfirmation is everything known about a confirmed booking, from
// which each recipient's payload is derived.
//
// It is a struct rather than a parameter list because the call it replaced took
// fifteen positional arguments, eleven of them strings — a shape where
// transposing two is invisible to the compiler and to the reader. It is an
// argument, never an enqueued payload: see bookingConfirmationEmail.
type BookingConfirmation struct {
	// Email is the address to write to. An empty value skips the client
	// rather than failing.
	Email string `json:"email"`

	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	ClientName  string `json:"client_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`

	// CancelURL is the full link to cancel the booking.
	CancelURL string `json:"cancel_url"`

	// Address and MapsURL are the confirmation email's "Cómo llegar" line —
	// the address as a person reads it, and the absolute Google Maps link
	// built from the coordinate/address fallback.
	Address string `json:"address"`
	MapsURL string `json:"maps_url"`

	// DepositAmount, BalanceAmount and CancellationLine are the values the
	// email quotes verbatim, rendered by PaymentAmounts/CancellationLine in
	// copy.go.
	//
	// They are rendered once, at the enqueue site, rather than carried as the
	// raw numbers the mailer would format for itself, so the queued email
	// cannot describe a cancellation window or a payment that differs from
	// what the caller recorded.
	DepositAmount    string `json:"deposit_amount"`
	BalanceAmount    string `json:"balance_amount"`
	CancellationLine string `json:"cancellation_line"`
	// PaymentState is what was collected (see PaymentState in copy.go), which
	// decides whether the email talks about a seña, a full payment or nothing.
	PaymentState string `json:"payment_state"`

	// ComplexPhone is the complex's phone, which the email offers as the way
	// to reach the venue. Empty leaves the line without a number.
	ComplexPhone string `json:"complex_phone"`

	// OwnerID is the complex owner, who is emailed separately about the sale
	// and whose address the client's email replies to.
	OwnerID string `json:"owner_id"`

	// BookingID identifies the booking this is about. It is the business half
	// of the queue's deduplication key (JOB-04) and never travels into the
	// payload — the workers do not read it, and a durable queue should not
	// store a field nobody consumes.
	BookingID string `json:"-"`

	// Source names the flow that triggered this, for log lines only.
	Source string `json:"-"`
}

// Reminder is the notice sent two hours before a booking starts.
//
// It carries the venue's address and what is still owed there, because two
// hours out those are the only two facts a client acts on, and the
// confirmation that held them went out days ago.
type Reminder struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	// Address is the venue's street address as a person reads it, including
	// the city.
	Address string `json:"address"`
	// BalanceAmount is what is left to pay at the complex, from BalanceAmount
	// in copy.go.
	BalanceAmount string `json:"balance_amount"`
	// CancelURL is the cancel link, absolute.
	CancelURL string `json:"cancel_url"`
	// MapsURL is the absolute Google Maps link the email's "Cómo llegar"
	// line uses.
	MapsURL string `json:"maps_url"`

	// ComplexPhone is the complex's phone, which the email offers as the way
	// to reach the venue. Empty leaves the line without a number.
	ComplexPhone string `json:"complex_phone,omitempty"`
	// OwnerID is the complex owner, whose address the reply goes to.
	OwnerID string `json:"owner_id,omitempty"`

	// BookingID identifies the booking this is about. It is the business half
	// of the queue's deduplication key (JOB-04), and it travels in the payload
	// because the worker reads the booking back before sending: a reminder
	// whose booking was cancelled, or has already begun, is dropped. A reminder
	// queued before this field existed has none and is sent as it was.
	BookingID string `json:"booking_id,omitempty"`
}

// clientContact is the reminder's contact in the shape the other client emails
// carry it.
func (r Reminder) clientContact() clientContact {
	return clientContact{ComplexPhone: r.ComplexPhone, OwnerID: r.OwnerID}
}

// Cancellation is the notice sent when a booking is cancelled.
//
// RefundLine is what makes it worth sending. A cancellation that says nothing
// about the deposit is the message clients write in about, and this one used
// to say nothing at all: the refund decision was made three lines above the
// enqueue call, in the same function, and never travelled with it.
type Cancellation struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	// RefundLine is the one sentence about the client's money, covering all
	// six cancellation outcomes, with the amount when there is one.
	RefundLine string `json:"refund_line"`
	// RefundAmount is that same money, formatted, or empty when none is coming
	// back. It is what the email's preview line quotes; the body quotes
	// RefundLine.
	RefundAmount string `json:"refund_amount"`
	// BookURL is the booking page, absolute.
	BookURL string `json:"book_url"`

	// ClientName and OwnerID are for the owner's email, sent only when Source is
	// SourceClientCancel. OwnerID is the complex owner, resolved at delivery time.
	ClientName string `json:"client_name"`
	OwnerID    string `json:"owner_id"`

	// ComplexPhone is the complex's phone, which the email offers as the way
	// to reach the venue. Empty leaves the line without a number.
	ComplexPhone string `json:"complex_phone"`

	// Source names the flow that cancelled the booking. It decides whether the
	// owner is emailed, and is a log field otherwise.
	Source string `json:"-"`

	// BookingID identifies the booking this is about. It is the business half
	// of the queue's deduplication key (JOB-04) and never travels into the
	// payload — the workers do not read it, and a durable queue should not
	// store a field nobody consumes.
	BookingID string `json:"-"`
}

// OwnerBookingRefund is the owner's notice that a refund MercadoPago issued on
// its own side cancelled a live booking. OwnerID is the complex owner, resolved
// at delivery time.
type OwnerBookingRefund struct {
	OwnerID     string
	ComplexName string
	CourtName   string
	ClientName  string
	Date        string
	StartTime   string
	BookingID   string
}

// Refund is the notice sent when a deposit is returned.
type Refund struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	Amount      string `json:"amount"`
	// BookURL is the booking page, absolute.
	BookURL string `json:"book_url"`

	// Cancelled says this refund also cancelled the booking (MercadoPago
	// refunded a live booking on its own side). The client's one email then
	// says the booking is off too, naming it by CourtName, Date and StartTime.
	Cancelled bool   `json:"-"`
	CourtName string `json:"-"`
	Date      string `json:"-"`
	StartTime string `json:"-"`

	// Reason is the one sentence saying why the money came back, rendered by
	// the caller (a payment that arrived twice, or after the booking was
	// cancelled). Empty says nothing more than the amount.
	Reason string `json:"-"`
	// ByHand says the complex returned the money directly rather than through
	// MercadoPago, so the email does not promise processing days.
	ByHand bool `json:"-"`

	// ComplexPhone is the complex's phone, which the email offers as the way
	// to reach the venue. Empty leaves the line without a number.
	ComplexPhone string `json:"-"`
	// OwnerID is the complex owner, whose address the reply goes to.
	OwnerID string `json:"-"`

	// BookingID identifies the booking this is about. It is the business half
	// of the queue's deduplication key (JOB-04) and never travels into the
	// payload — the workers do not read it, and a durable queue should not
	// store a field nobody consumes.
	BookingID string `json:"-"`

	// RefundID tells one refund of that booking from another. A booking can be
	// refunded more than once, in parts, and each part is its own notice; it is
	// the other half of the deduplication key, so a redelivery of the same notice
	// is dropped while the next partial refund is not.
	RefundID string `json:"-"`
}

// The three transactional account messages each get their own type.
//
// They were briefly merged into one struct with three optional URL fields,
// which compiled a password-reset task carrying a verification link and no
// reset link. Grouping a long parameter list into a struct is worth doing;
// merging distinct payloads into a permissive one is the opposite trade, and
// it moves an error the compiler used to catch into production.

// VerificationEmail asks a new account to confirm its address.
type VerificationEmail struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
	VerifyURL string `json:"verify_url"`
}

// PasswordResetEmail carries the reset link.
type PasswordResetEmail struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
	ResetURL  string `json:"reset_url"`
}

// DuplicateRegistrationEmail tells someone who already has an account how to
// get back into it.
type DuplicateRegistrationEmail struct {
	To        string `json:"to"`
	FirstName string `json:"first_name"`
	LoginURL  string `json:"login_url"`
	ResetURL  string `json:"reset_url"`
}

// EmailChangeRequestedEmail tells the account's CURRENT address that a
// session asked to move it to NewEmail. It is the confirmation step that
// keeps a hijacked session from moving the account's recovery channel
// unnoticed — see auth.Service.UpdateCurrentUser.
type EmailChangeRequestedEmail struct {
	To         string `json:"to"`
	FirstName  string `json:"first_name"`
	NewEmail   string `json:"new_email"`
	ConfirmURL string `json:"confirm_url"`
	// ExpiresIn is the link's TTL, already rendered to Spanish (SpanishDuration)
	// by the enqueue site (auth.Service.requestEmailChange) and carried in the
	// payload rather than recomputed by the mailer — the same "render once at
	// enqueue" rule this file's own copy.go documents for CancellationLine, so
	// a queued-but-not-yet-delivered email cannot end up quoting a TTL the
	// constant has since changed.
	ExpiresIn string `json:"expires_in"`
}

// bookingConfirmationEmail is what the email worker needs, and nothing else.
//
// The queue is durable storage, so every field enqueued is a field persisted:
// sending the whole BookingConfirmation would put the owner id and the
// client's details into the queue on every booking, for a worker that reads
// none of them.
type bookingConfirmationEmail struct {
	Email            string `json:"email"`
	ComplexName      string `json:"complex_name"`
	CourtName        string `json:"court_name"`
	Date             string `json:"date"`
	StartTime        string `json:"start_time"`
	Address          string `json:"address"`
	MapsURL          string `json:"maps_url"`
	CancelURL        string `json:"cancel_url"`
	DepositAmount    string `json:"deposit_amount"`
	BalanceAmount    string `json:"balance_amount"`
	CancellationLine string `json:"cancellation_line"`
	PaymentState     string `json:"payment_state,omitempty"`
	clientContact
}

// clientContact is the complex's contact a client email carries: its phone,
// and the owner whose current address the reply goes to, resolved at delivery
// like every owner address. Both are absent from a task queued before they
// existed, and the email then offers what it has.
type clientContact struct {
	ComplexPhone string `json:"complex_phone,omitempty"`
	OwnerID      string `json:"owner_id,omitempty"`
}

// bookingCancelledEmail and depositRefundedEmail are what their email workers
// need, and nothing else — same rule as bookingConfirmationEmail above. The
// queue is durable storage, so enqueueing the whole Cancellation would write
// fields no worker reads into it on every cancellation.
type bookingCancelledEmail struct {
	Email        string `json:"email"`
	ComplexName  string `json:"complex_name"`
	CourtName    string `json:"court_name"`
	Date         string `json:"date"`
	StartTime    string `json:"start_time"`
	RefundLine   string `json:"refund_line"`
	RefundAmount string `json:"refund_amount"`
	BookURL      string `json:"book_url"`
	clientContact
}

type depositRefundedEmail struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	Amount      string `json:"amount"`
	BookURL     string `json:"book_url"`
	Cancelled   bool   `json:"cancelled,omitempty"`
	CourtName   string `json:"court_name,omitempty"`
	Date        string `json:"date,omitempty"`
	StartTime   string `json:"start_time,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ByHand      bool   `json:"by_hand,omitempty"`
	clientContact
}

// ownerNewBooking tells the complex owner a booking was made. The owner is
// referenced by id and resolved at delivery time, so the queue does not hold a
// stale address.
type ownerNewBooking struct {
	OwnerID     string `json:"owner_id"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	ClientName  string `json:"client_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
}

// ownerBookingCancelled tells the complex owner that a client cancelled a
// booking, which frees its slot. Like ownerNewBooking, the owner is referenced
// by id and resolved at delivery time.
type ownerBookingCancelled struct {
	OwnerID     string `json:"owner_id"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	ClientName  string `json:"client_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
}

// ownerBookingRefunded tells the complex owner that a refund MercadoPago issued
// cancelled a booking. It is a separate task from ownerBookingCancelled so the
// copy can say where the cancellation came from; the owner is referenced by id
// and resolved at delivery time.
type ownerBookingRefunded struct {
	OwnerID     string `json:"owner_id"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	ClientName  string `json:"client_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
}

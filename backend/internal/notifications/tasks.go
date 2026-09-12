// Package notifications decides what to tell a client or an owner, and hands
// the delivery to a durable queue.
//
// Nothing here sends anything synchronously. Every call enqueues a task that an
// independent worker pool picks up, so a slow mail provider or a WhatsApp
// outage cannot delay — or fail — the booking that triggered it.
package notifications

// Task types on the durable queue. They are stored in Redis, so renaming one
// strands whatever is already enqueued under the old name.
const (
	TaskEmailBookingConfirmation   = "email:booking_confirmation"
	TaskEmailReminder2h            = "email:reminder_2h"
	TaskEmailBookingCancelled      = "email:booking_cancelled"
	TaskEmailDepositRefunded       = "email:deposit_refunded"
	TaskEmailOwnerNewBooking       = "email:owner_new_booking"
	TaskEmailVerification          = "email:verification"
	TaskEmailPasswordReset         = "email:password_reset"
	TaskEmailDuplicateRegistration = "email:duplicate_registration"
	TaskWABookingConfirmation      = "wa:booking_confirmation"
	TaskWAReminder2h               = "wa:reminder_2h"
	TaskWABookingCancelled         = "wa:booking_cancelled"
	TaskWADepositRefunded          = "wa:deposit_refunded"
)

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

// The payloads below are serialised into Redis, so their JSON tags are a
// storage format: changing one breaks tasks already in the queue.

// BookingConfirmation is everything known about a confirmed booking, from
// which each channel's payload is derived.
//
// It is a struct rather than a parameter list because the call it replaced took
// fifteen positional arguments, eleven of them strings — a shape where
// transposing two is invisible to the compiler and to the reader. It is an
// argument, never an enqueued payload: see bookingConfirmationEmail.
type BookingConfirmation struct {
	// Email and Phone are the channels to use. An empty value skips that
	// channel rather than failing.
	Email string `json:"email"`
	Phone string `json:"phone"`

	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	ClientName  string `json:"client_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`

	// CancelURL is the full link for email; CancelPath and MapsQuery are the
	// button parameters WhatsApp templates take instead.
	CancelURL  string `json:"cancel_url"`
	CancelPath string `json:"cancel_path"`
	MapsQuery  string `json:"maps_query"`

	// Address and MapsURL are the confirmation email's "Cómo llegar" line —
	// the address as a person reads it, and the absolute Google Maps link
	// built from the same coordinate/address fallback as MapsQuery. WhatsApp
	// has no equivalent: its button binds to a base URL configured once in
	// WhatsApp Manager, so it only ever needs the query suffix.
	Address string `json:"address"`
	MapsURL string `json:"maps_url"`

	// DepositAmount, BalanceAmount and CancellationLine are the three values
	// both channels quote verbatim, rendered by PaymentAmounts/CancellationLine
	// in copy.go.
	//
	// They are rendered once, at the enqueue site, rather than carried as the
	// raw numbers each channel would format for itself. That is the point: the
	// email used to describe a cancellation window the WhatsApp message never
	// mentioned, and neither said what had been paid. One set of values cannot
	// disagree with itself.
	DepositAmount    string `json:"deposit_amount"`
	BalanceAmount    string `json:"balance_amount"`
	CancellationLine string `json:"cancellation_line"`

	// OwnerID is the complex owner, who is emailed separately about the sale.
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
	Phone       string `json:"phone"`
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
	// MapsQuery and CancelPath bind the reminder template's two buttons;
	// CancelURL is the same cancel link, absolute, for the email.
	MapsQuery  string `json:"maps_query"`
	CancelPath string `json:"cancel_path"`
	CancelURL  string `json:"cancel_url"`
	// MapsURL is the absolute Google Maps link the email's "Cómo llegar"
	// line uses instead of WhatsApp's button-suffix MapsQuery.
	MapsURL string `json:"maps_url"`

	// BookingID identifies the booking this is about. It is the business half
	// of the queue's deduplication key (JOB-04) and never travels into the
	// payload — the workers do not read it, and a durable queue should not
	// store a field nobody consumes.
	BookingID string `json:"-"`
}

// Cancellation is the notice sent when a booking is cancelled.
//
// RefundLine is what makes it worth sending. A cancellation that says nothing
// about the deposit is the message clients write in about, and this one used
// to say nothing at all: the refund decision was made three lines above the
// enqueue call, in the same function, and never travelled with it.
type Cancellation struct {
	Email       string `json:"email"`
	Phone       string `json:"phone"`
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
	// BookPath binds the "Nueva reserva" button; BookURL is the same page,
	// absolute, for the email.
	BookPath string `json:"book_path"`
	BookURL  string `json:"book_url"`

	// BookingID identifies the booking this is about. It is the business half
	// of the queue's deduplication key (JOB-04) and never travels into the
	// payload — the workers do not read it, and a durable queue should not
	// store a field nobody consumes.
	BookingID string `json:"-"`
}

// Refund is the notice sent when a deposit is returned.
type Refund struct {
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	ComplexName string `json:"complex_name"`
	Amount      string `json:"amount"`
	// BookPath binds the "Nueva reserva" button; BookURL is the same page,
	// absolute, for the email.
	BookPath string `json:"book_path"`
	BookURL  string `json:"book_url"`

	// BookingID identifies the booking this is about. It is the business half
	// of the queue's deduplication key (JOB-04) and never travels into the
	// payload — the workers do not read it, and a durable queue should not
	// store a field nobody consumes.
	BookingID string `json:"-"`
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

// bookingConfirmationEmail is what the email worker needs, and nothing else.
//
// The queue is durable storage, so every field enqueued is a field persisted:
// sending the whole BookingConfirmation put the client's phone number, the
// owner id and two WhatsApp-only template parameters into Redis on every
// booking, for a worker that reads none of them.
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
}

// bookingCancelledEmail and depositRefundedEmail are what their email workers
// need, and nothing else — same rule as bookingConfirmationEmail above. The
// queue is durable storage, so enqueueing the whole Cancellation would write
// the client's phone number and two WhatsApp button suffixes into Redis on
// every cancellation, for a worker that reads none of them.
type bookingCancelledEmail struct {
	Email        string `json:"email"`
	ComplexName  string `json:"complex_name"`
	CourtName    string `json:"court_name"`
	Date         string `json:"date"`
	StartTime    string `json:"start_time"`
	RefundLine   string `json:"refund_line"`
	RefundAmount string `json:"refund_amount"`
	BookURL      string `json:"book_url"`
}

type depositRefundedEmail struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	Amount      string `json:"amount"`
	BookURL     string `json:"book_url"`
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

// The wa* types are the WhatsApp variants, which carry only the fields their
// templates bind.
type waBookingConfirmation struct {
	Phone            string `json:"phone"`
	ComplexName      string `json:"complex_name"`
	CourtName        string `json:"court_name"`
	Date             string `json:"date"`
	StartTime        string `json:"start_time"`
	DepositAmount    string `json:"deposit_amount"`
	BalanceAmount    string `json:"balance_amount"`
	CancellationLine string `json:"cancellation_line"`
	CancelPath       string `json:"cancel_path"`
	MapsQuery        string `json:"maps_query"`
}

type waReminder struct {
	Phone         string `json:"phone"`
	ComplexName   string `json:"complex_name"`
	CourtName     string `json:"court_name"`
	Date          string `json:"date"`
	StartTime     string `json:"start_time"`
	Address       string `json:"address"`
	BalanceAmount string `json:"balance_amount"`
	MapsQuery     string `json:"maps_query"`
	CancelPath    string `json:"cancel_path"`
}

type waBookingCancelled struct {
	Phone       string `json:"phone"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	RefundLine  string `json:"refund_line"`
	BookPath    string `json:"book_path"`
}

type waDepositRefunded struct {
	Phone       string `json:"phone"`
	ComplexName string `json:"complex_name"`
	Amount      string `json:"amount"`
	BookPath    string `json:"book_path"`
}

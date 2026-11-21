package notifications

import "context"

// Booking notices for the moments a client or an owner used to be told nothing:
// a booking marked as not attended, a payment taken at the counter, money the
// complex owes back by hand, and a repeated payment MercadoPago returned on its
// own. Each is one email, deduplicated on the booking and the event it is about.
const (
	// TaskEmailBookingNoShow tells the client the complex marked their booking
	// as not attended.
	TaskEmailBookingNoShow = "email:booking_no_show"
	// TaskEmailPaymentReceived is the client's receipt for a counter payment.
	TaskEmailPaymentReceived = "email:payment_received"
	// TaskEmailRefundOwedByVenue tells the client the complex returns money to
	// them directly; TaskEmailOwnerRefundOwed tells the owner they owe it.
	TaskEmailRefundOwedByVenue = "email:refund_owed_by_venue"
	TaskEmailOwnerRefundOwed   = "email:owner_refund_owed"
	// TaskEmailOwnerDuplicatePaymentRefunded tells the owner a client paid a
	// live booking twice and the second payment went back on its own.
	TaskEmailOwnerDuplicatePaymentRefunded = "email:owner_duplicate_payment_refunded"
)

// NoShow is the notice that the complex marked a booking as not attended.
type NoShow struct {
	Email        string
	ComplexName  string
	CourtName    string
	Date         string
	StartTime    string
	BookURL      string
	ComplexPhone string
	OwnerID      string
	// BookingID is the deduplication subject and never travels in the payload.
	BookingID string
}

// PaymentReceipt is the client's receipt for a payment taken at the counter.
type PaymentReceipt struct {
	Email       string
	ComplexName string
	CourtName   string
	Date        string
	StartTime   string
	// Amount is what was paid, formatted. Method is how, as a phrase
	// (PaymentMethodPhrase). BalanceAmount is what is still owed, formatted,
	// or empty when the booking is paid.
	Amount        string
	Method        string
	BalanceAmount string
	BookURL       string
	ComplexPhone  string
	OwnerID       string
	// BookingID and PaymentID are the deduplication subject: one receipt per
	// payment row. They never travel in the payload.
	BookingID string
	PaymentID string
}

// RefundOwed is money a complex owes a client back by hand, after the
// automatic refund of the booking could not cover it. The client is told the
// complex returns it, and the owner is told they owe it.
type RefundOwed struct {
	Email        string
	ClientName   string
	ComplexName  string
	CourtName    string
	Date         string
	StartTime    string
	Amount       string
	BookURL      string
	ComplexPhone string
	OwnerID      string
	// BookingID and Subject (what made it owed, such as the refund attempt
	// that ran out of retries) are the deduplication subject.
	BookingID string
	Subject   string
}

// OwnerDuplicatePaymentRefund is the owner's notice that a client paid a live
// booking twice and MercadoPago returned the second payment.
type OwnerDuplicatePaymentRefund struct {
	OwnerID     string
	ComplexName string
	CourtName   string
	ClientName  string
	Date        string
	StartTime   string
	Amount      string
	// BookingID and RefundID are the deduplication subject.
	BookingID string
	RefundID  string
}

type bookingNoShowEmail struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	BookURL     string `json:"book_url"`
	clientContact
}

type paymentReceivedEmail struct {
	Email         string `json:"email"`
	ComplexName   string `json:"complex_name"`
	CourtName     string `json:"court_name"`
	Date          string `json:"date"`
	StartTime     string `json:"start_time"`
	Amount        string `json:"amount"`
	Method        string `json:"method"`
	BalanceAmount string `json:"balance_amount"`
	BookURL       string `json:"book_url"`
	clientContact
}

type refundOwedByVenueEmail struct {
	Email       string `json:"email"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	Amount      string `json:"amount"`
	BookURL     string `json:"book_url"`
	clientContact
}

// ownerBookingMoney is the owner's half of a money notice: the owner is
// referenced by id and resolved at delivery time, like every owner email.
type ownerBookingMoney struct {
	OwnerID     string `json:"owner_id"`
	ComplexName string `json:"complex_name"`
	CourtName   string `json:"court_name"`
	ClientName  string `json:"client_name"`
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	Amount      string `json:"amount"`
}

// paymentMethodPhrases says how a counter payment was made, the way the
// receipt reads it after the amount ("$5.000 en efectivo").
var paymentMethodPhrases = map[string]string{ //nolint:gosec // G101 false positive: payment method labels, not credentials.
	"cash":        "en efectivo",
	"transfer":    "por transferencia",
	"debit_card":  "con tarjeta de débito",
	"credit_card": "con tarjeta de crédito",
	"qr_wallet":   "con QR",
}

// PaymentMethodPhrase renders a counter payment method for the receipt, or the
// empty string for a method it does not know, which the receipt then omits.
func PaymentMethodPhrase(method string) string { return paymentMethodPhrases[method] }

// BookingNoShow tells the client the complex marked their booking as not
// attended. The owner, who marked it, is not told.
func (s *Service) BookingNoShow(n NoShow) {
	if n.Email == "" {
		return
	}
	s.queue.Enqueue(TaskEmailBookingNoShow, bookingNoShowEmail{
		Email: n.Email, ComplexName: n.ComplexName, CourtName: n.CourtName,
		Date: n.Date, StartTime: n.StartTime, BookURL: n.BookURL,
		clientContact: clientContact{ComplexPhone: n.ComplexPhone, OwnerID: n.OwnerID},
	}, dedup(TaskEmailBookingNoShow, n.Email, n.BookingID))
}

// PaymentReceived sends the client a receipt for a counter payment. The owner,
// who took it, is not told.
func (s *Service) PaymentReceived(r PaymentReceipt) {
	if r.Email == "" {
		return
	}
	s.queue.Enqueue(TaskEmailPaymentReceived, paymentReceivedEmail{
		Email: r.Email, ComplexName: r.ComplexName, CourtName: r.CourtName,
		Date: r.Date, StartTime: r.StartTime,
		Amount: r.Amount, Method: r.Method, BalanceAmount: r.BalanceAmount, BookURL: r.BookURL,
		clientContact: clientContact{ComplexPhone: r.ComplexPhone, OwnerID: r.OwnerID},
	}, dedup(TaskEmailPaymentReceived, r.Email, joinSubject(r.BookingID, r.PaymentID)))
}

// RefundOwedByVenue tells the client the complex will return their money
// directly, and the owner that they owe it. Each recipient is independent: a
// client with no address is skipped and the owner is still told.
func (s *Service) RefundOwedByVenue(r RefundOwed) {
	subject := joinSubject(r.BookingID, r.Subject)
	if r.Email != "" {
		s.queue.Enqueue(TaskEmailRefundOwedByVenue, refundOwedByVenueEmail{
			Email: r.Email, ComplexName: r.ComplexName, CourtName: r.CourtName,
			Date: r.Date, StartTime: r.StartTime, Amount: r.Amount, BookURL: r.BookURL,
			clientContact: clientContact{ComplexPhone: r.ComplexPhone, OwnerID: r.OwnerID},
		}, dedup(TaskEmailRefundOwedByVenue, r.Email, subject))
	}
	if r.OwnerID != "" {
		s.queue.Enqueue(TaskEmailOwnerRefundOwed, ownerBookingMoney{
			OwnerID: r.OwnerID, ComplexName: r.ComplexName, CourtName: r.CourtName,
			ClientName: r.ClientName, Date: r.Date, StartTime: r.StartTime, Amount: r.Amount,
		}, dedup(TaskEmailOwnerRefundOwed, r.OwnerID, subject))
	}
}

// OwnerDuplicatePaymentRefunded tells the owner a client's second payment on a
// live booking went back through MercadoPago.
func (s *Service) OwnerDuplicatePaymentRefunded(r OwnerDuplicatePaymentRefund) {
	if r.OwnerID == "" {
		return
	}
	s.queue.Enqueue(TaskEmailOwnerDuplicatePaymentRefunded, ownerBookingMoney{
		OwnerID: r.OwnerID, ComplexName: r.ComplexName, CourtName: r.CourtName,
		ClientName: r.ClientName, Date: r.Date, StartTime: r.StartTime, Amount: r.Amount,
	}, dedup(TaskEmailOwnerDuplicatePaymentRefunded, r.OwnerID, joinSubject(r.BookingID, r.RefundID)))
}

// joinSubject is a deduplication subject made of two halves, or the empty
// string (no key at all) when either is missing: a key built from a blank half
// would collide across every event of that booking. See dedup.
func joinSubject(booking, event string) string {
	if booking == "" || event == "" {
		return ""
	}
	return booking + ":" + event
}

// registerBookingNoticeWorkers wires the delivery of the notices above.
// RegisterWorkers calls it.
func (s *Service) registerBookingNoticeWorkers() {
	s.queue.RegisterHandler(handle(TaskEmailBookingNoShow, func(ctx context.Context, p bookingNoShowEmail) error {
		return s.mailer.SendBookingNoShow(ctx, p.Email, p.ComplexName, p.CourtName, p.Date, p.StartTime,
			p.BookURL, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailPaymentReceived, func(ctx context.Context, p paymentReceivedEmail) error {
		return s.mailer.SendPaymentReceived(ctx, p.Email, p.ComplexName, p.CourtName, p.Date, p.StartTime,
			p.Amount, p.Method, p.BalanceAmount, p.BookURL, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailRefundOwedByVenue, func(ctx context.Context, p refundOwedByVenueEmail) error {
		return s.mailer.SendRefundOwedByVenue(ctx, p.Email, p.ComplexName, p.CourtName, p.Date, p.StartTime,
			p.Amount, p.BookURL, s.contactOf(ctx, p.clientContact))
	}))
	s.queue.RegisterHandler(handle(TaskEmailOwnerRefundOwed, func(ctx context.Context, p ownerBookingMoney) error {
		to, err := s.ownerAddress(ctx, "owner refund owed", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerRefundOwed(ctx, to, p.ComplexName, p.CourtName, p.ClientName, p.Date, p.StartTime, p.Amount)
	}))
	s.queue.RegisterHandler(handle(TaskEmailOwnerDuplicatePaymentRefunded, func(ctx context.Context, p ownerBookingMoney) error {
		to, err := s.ownerAddress(ctx, "owner duplicate payment refunded", p.OwnerID)
		if err != nil {
			return err
		}
		return s.mailer.SendOwnerDuplicatePaymentRefunded(ctx, to, p.ComplexName, p.CourtName, p.ClientName, p.Date, p.StartTime, p.Amount)
	}))
}

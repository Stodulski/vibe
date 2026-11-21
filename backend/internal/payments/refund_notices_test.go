package payments

import (
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/mp"
	"github.com/stodulski/vibe-server/internal/notifications"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
)

// readableNotice makes the client, the complex and the court of booking
// readable, so a notice has something to name.
func readableNotice(f *fixture, booking *bookingstore.Booking) {
	email := "ana@example.com"
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana", LastName: "Diaz", Email: &email}
	f.complexes.complex.Name, f.complexes.complex.Slug = "Vibe", "vibe"
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}
}

// S4: a client who paid a live booking twice gets the second payment back
// automatically, and is told so with the reason; the owner is told too. The
// notice is keyed on the same refund key the retry job would use, so the
// client cannot be told twice about one refund.
func TestADuplicatePaymentRefundTellsTheClientWhyAndTheOwner(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, deposit := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusConfirmed
	recorded := "mp-OLD"
	deposit.MPPaymentID = &recorded
	f.payments.byBooking = deposit
	f.payments.byBookingAll = []*paymentstore.Payment{deposit}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	readableNotice(f, booking)
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	if err := f.service.processPaymentWebhook(t.Context(), "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.provider.refunds) != 1 {
		t.Fatalf("setup: the duplicate must be refunded; got %d refunds", len(f.provider.refunds))
	}
	if len(f.notify.refunds) != 1 {
		t.Fatalf("want one refund notice to the client; got %d", len(f.notify.refunds))
	}
	r := f.notify.refunds[0]
	if r.Reason != duplicatePaymentReason || r.Cancelled || r.Email != "ana@example.com" {
		t.Errorf("the client must be told why, and not that the booking is off; got %+v", r)
	}
	if r.Amount != formatARS(150_000) {
		t.Errorf("want the refunded amount quoted; got %q", r.Amount)
	}
	if r.BookingID != booking.ID.String() || r.RefundID != refundKey("mp-123", 150_000) {
		t.Errorf("the notice must carry the refund's own key; got booking %q refund %q", r.BookingID, r.RefundID)
	}
	if len(f.notify.ownerDuplicates) != 1 {
		t.Fatalf("the owner of a live booking must be told; got %d", len(f.notify.ownerDuplicates))
	}
	o := f.notify.ownerDuplicates[0]
	if o.OwnerID != f.complexes.complex.OwnerID.String() || o.ClientName != "Ana Diaz" || o.Amount != formatARS(150_000) || o.RefundID != r.RefundID {
		t.Errorf("the owner's notice must name the client and the amount; got %+v", o)
	}
	if len(f.notify.ownerRefunds) != 0 {
		t.Error("the booking stands: the owner must not be told it was cancelled")
	}
}

// S4: a duplicate payment whose refund MercadoPago refuses tells nobody yet;
// the retry job tells the client once the money is back.
func TestADuplicatePaymentWhoseRefundIsQueuedTellsNobodyYet(t *testing.T) {
	withCapturedSentryEvents(t)
	f := newFixture(t)
	complexID := uuid.New()
	booking, deposit := paidBooking(complexID)
	recorded := "mp-OLD"
	deposit.MPPaymentID = &recorded
	f.payments.byBooking = deposit
	f.payments.byBookingAll = []*paymentstore.Payment{deposit}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	readableNotice(f, booking)
	f.provider.refundErr = errProvider
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	_ = f.service.processPaymentWebhook(t.Context(), "mp-123")
	if len(f.notify.refunds) != 0 || len(f.notify.ownerDuplicates) != 0 {
		t.Errorf("nothing came back yet, nobody may be told; got %d client and %d owner notices",
			len(f.notify.refunds), len(f.notify.ownerDuplicates))
	}
}

// S4: a payment that arrives for a booking already cancelled goes straight
// back, and the client is told why. The booking is not live, so the owner is
// not emailed.
func TestALatePaymentRefundTellsTheClientWhy(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusCancelled
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	readableNotice(f, booking)

	if err := f.service.processApprovedPayment(t.Context(), booking,
		&mp.Payment{ID: 123, TransactionAmount: 1500, CollectorID: 111111111}, "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != latePaymentReason {
		t.Fatalf("want one notice with the late-payment reason; got %+v", f.notify.refunds)
	}
	if len(f.notify.ownerDuplicates) != 0 {
		t.Error("a cancelled booking is not live: the owner must not be emailed")
	}
}

// S4: a payment that lands after its slot was sold to somebody else is
// refunded, and the client is told the booking could not be confirmed.
func TestASlotTakenRefundTellsTheClientWhy(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, mpPayment := pendingBooking(complexID)
	mpPayment.CollectorID = 111111111
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	readableNotice(f, booking)
	f.payments.slotErr = bookingstore.ErrSlotUnavailable

	if err := f.service.processApprovedPayment(t.Context(), booking, mpPayment, "mp-123"); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(f.notify.refunds) != 1 || f.notify.refunds[0].Reason != slotTakenReason {
		t.Fatalf("want one notice with the slot-taken reason; got %+v", f.notify.refunds)
	}
}

// S5: once a refund runs out of retries and the booking's cash is left owed by
// hand, the owner is told what is owed to whom and the client that the complex
// returns it. Each is keyed on the attempt, so a replay is the same notice.
func TestAnExhaustedRefundTellsTheOwnerAndTheClientWhatIsOwedByHand(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	bookingID := uuid.New()
	mpID := "mp-" + uuid.NewString()
	booking := &bookingstore.Booking{
		ID: bookingID, ComplexID: complexID, ClientID: uuid.New(), CourtID: uuid.New(),
		Status: "cancelled", CollectionStatus: bookingstore.CollectionStatusDepositPaid,
		RefundStatus: bookingstore.RefundStatusNone, Price: 500_000, DepositAmount: 150_000,
	}
	mpRow := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 150_000, Status: "deposit_paid", MPPaymentID: &mpID,
	}
	cash := &paymentstore.Payment{
		ID: uuid.New(), BookingID: bookingID, ComplexID: complexID,
		Amount: 50_000, Status: "deposit_paid", Method: "cash",
	}
	f.bookings.booking = booking
	f.payments.byBookingAll = []*paymentstore.Payment{mpRow, cash}
	f.provider.refundErr = errProvider
	f.payments.exhausted = true
	f.complexes.complex = linkedComplex(complexID, "")
	readableNotice(f, booking)
	withCapturedSentryEvents(t)

	f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.notify.owedByVenue) != 1 {
		t.Fatalf("want one owed-by-hand notice; got %d", len(f.notify.owedByVenue))
	}
	n := f.notify.owedByVenue[0]
	if n.Amount != notifications.FormatARS(50_000) {
		t.Errorf("only the cash balance is owed by hand; got %q", n.Amount)
	}
	if n.Email != "ana@example.com" || n.ClientName != "Ana Diaz" || n.OwnerID != f.complexes.complex.OwnerID.String() {
		t.Errorf("the notice must reach the client and name them to the owner; got %+v", n)
	}
	if n.BookingID != booking.ID.String() || n.Subject == "" {
		t.Errorf("the notice must carry its deduplication subject; got %+v", n)
	}
	if len(f.notify.refunds) != 0 {
		t.Error("nothing came back yet, so the client must not be told a refund was made")
	}
}

// A confirmed booking paid at the counter in cash that receives a second,
// MercadoPago payment whose refund runs out of retries owes nobody anything by
// hand: the cash is the valid payment of a booking that stands. Neither the
// client nor the owner may be told the complex returns it.
func TestAnExhaustedDuplicateRefundOnALiveBookingTellsNobodyCashIsOwed(t *testing.T) {
	withCapturedSentryEvents(t)
	f := newFixture(t)
	complexID := uuid.New()
	booking, _ := paidBooking(complexID)
	booking.Status = bookingstore.BookingStatusConfirmed
	cash := &paymentstore.Payment{
		ID: uuid.New(), BookingID: booking.ID, ComplexID: complexID,
		Amount: 150_000, Status: "deposit_paid", Method: "cash",
	}
	f.payments.byBooking = cash
	f.payments.byBookingAll = []*paymentstore.Payment{cash}
	f.complexes.complex = linkedComplex(complexID, "111111111")
	f.bookings.booking = booking
	readableNotice(f, booking)
	f.provider.refundErr = errProvider
	f.payments.exhausted = true
	f.provider.payment = &mp.Payment{
		ID: 123, Status: "approved", TransactionAmount: 1500,
		CollectorID: 111111111, ExternalReference: booking.ID.String(),
	}

	_ = f.service.processPaymentWebhook(t.Context(), "mp-123")

	if len(f.payments.recordedFailure) != 1 {
		t.Fatalf("setup: the duplicate's refund must be claimed and exhausted; got %d failures", len(f.payments.recordedFailure))
	}
	if len(f.notify.owedByVenue) != 0 {
		t.Errorf("the booking stands, nothing is owed by hand; got %+v", f.notify.owedByVenue)
	}
}

// S5: an exhausted refund with no cash balance owed by hand tells nobody: the
// retry job keeps working on the automatic half.
func TestAnExhaustedRefundWithNothingOwedByHandTellsNobody(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking, payment := paidBooking(complexID)
	booking.Status = "cancelled"
	f.bookings.booking = booking
	f.payments.byBooking = payment
	f.payments.claimAmount = 150_000
	f.provider.refundErr = errProvider
	f.payments.exhausted = true
	f.complexes.complex = linkedComplex(complexID, "")
	readableNotice(f, booking)
	withCapturedSentryEvents(t)

	f.service.AutoRefundIfPaid(t.Context(), booking)

	if len(f.notify.owedByVenue) != 0 {
		t.Errorf("nothing is owed by hand; got %+v", f.notify.owedByVenue)
	}
}

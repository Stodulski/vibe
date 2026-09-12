package bookings

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// A booking already played or marked no-show cannot be cancelled: the court
// time was used, and cancelling would refund it.
func TestCancelRefusesASettledBooking(t *testing.T) {
	for _, status := range []string{"completed", "no_show"} {
		t.Run(status, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := futureBooking(complexID)
			booking.Status = bookingstore.BookingStatus(status)
			f.store.booking = booking
			f.linkResolver.booking = booking

			w := httptest.NewRecorder()
			f.handler.Cancel(w, ownerRequest(t, http.MethodPost, "/", complexID,
				map[string]string{"bookingID": booking.ID.String()}, `{}`))

			if w.Code == http.StatusOK {
				t.Errorf("a %s booking must not be cancellable; got %d", status, w.Code)
			}
			if len(f.refunds.refunded) != 0 {
				t.Error("no refund may be issued for a booking that cannot be cancelled")
			}
			if len(f.store.updated) != 0 {
				t.Error("nothing may be persisted")
			}
		})
	}
}

// Cancelling twice is idempotent rather than an error: a client double-clicking
// or a retried request must not produce a failure, and must not refund twice.
func TestCancellingAnAlreadyCancelledBookingIsIdempotent(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "cancelled"
	f.store.booking = booking
	f.linkResolver.booking = booking

	w := httptest.NewRecorder()
	f.handler.Cancel(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{}`))

	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.refunds.refunded) != 0 {
		t.Error("a second cancellation must not refund again")
	}
	if len(f.store.updated) != 0 {
		t.Error("nothing may be written a second time")
	}
}

// A booking under another complex reads as missing, not forbidden.
func TestCancelHidesOtherComplexesBookings(t *testing.T) {
	f := newFixture(t)
	booking := futureBooking(uuid.New())
	f.store.booking = booking
	f.linkResolver.booking = booking

	w := httptest.NewRecorder()
	f.handler.Cancel(w, ownerRequest(t, http.MethodPost, "/", uuid.New(),
		map[string]string{"bookingID": booking.ID.String()}, `{}`))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.updated) != 0 {
		t.Error("another complex's booking must not be touched")
	}
}

func TestCancelRefundsNotifiesAndAudits(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID, FirstName: "Ana"}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.Cancel(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{"reason":"client called"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Errorf("the booking was not cancelled; got %v", f.store.updated)
	}
	if len(f.refunds.refunded) != 1 {
		t.Error("a paid booking must be refunded when cancelled")
	}
	if len(f.realtime.published) == 0 {
		t.Error("the owner's dashboard must be told")
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != "cancel" {
		t.Errorf("the cancellation was not audited; got %+v", f.audit.entries)
	}
}

// The booking detail response carries the whole payment ledger, not just the
// single MercadoPago-preferred row `payment` still holds for backward
// compatibility — a booking can have a deposit paid online and a balance
// confirmed in cash, and the detail view must show both.
func TestGetIncludesTheWholePaymentLedger(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking

	depositID := "mp-" + uuid.New().String()
	deposit := &paymentstore.Payment{ID: uuid.New(), BookingID: booking.ID, Amount: 150_000, MPPaymentID: &depositID}
	balance := &paymentstore.Payment{ID: uuid.New(), BookingID: booking.ID, Amount: 350_000}
	f.payments.payment = deposit
	f.payments.ledger = []*paymentstore.Payment{deposit, balance}

	w := httptest.NewRecorder()
	f.handler.Get(w, ownerRequest(t, http.MethodGet, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	var response struct {
		Payments []paymentstore.Payment `json:"payments"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Payments) != 2 {
		t.Fatalf("want 2 payments in the ledger; got %d (%s)", len(response.Payments), w.Body.String())
	}
	if response.Payments[0].ID != deposit.ID || response.Payments[1].ID != balance.ID {
		t.Errorf("want the deposit then the balance; got %+v", response.Payments)
	}
}

// An empty ledger renders as "payments": [] rather than null, so a client can
// always range over the field without a nil check.
func TestGetRendersAnEmptyLedgerAsAnEmptyArray(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking

	w := httptest.NewRecorder()
	f.handler.Get(w, ownerRequest(t, http.MethodGet, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"payments":[]`) {
		t.Errorf("want an empty array for payments; got %s", w.Body.String())
	}
}

func TestConfirmPaymentRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unknown method", `{"method":"crypto","amount":100000}`},
		{"zero amount", `{"method":"cash","amount":0}`},
		{"negative amount", `{"method":"cash","amount":-500}`},
		{"absurd amount", `{"method":"cash","amount":100000000}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := futureBooking(complexID)
			booking.Status = "pending"
			f.store.booking = booking
			f.linkResolver.booking = booking

			w := httptest.NewRecorder()
			f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
				map[string]string{"bookingID": booking.ID.String()}, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if len(f.payments.inserted) != 0 && len(f.payments.confirmed) != 0 {
				t.Error("no payment may be recorded from invalid input")
			}
		})
	}
}

// Recording a cash payment on a cancelled booking would resurrect it.
func TestConfirmPaymentRefusesACancelledBooking(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "cancelled"
	f.store.booking = booking
	f.linkResolver.booking = booking

	w := httptest.NewRecorder()
	f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{"method":"cash","amount":100000}`))

	if w.Code == http.StatusOK {
		t.Errorf("a cancelled booking must not accept a payment; got %d", w.Code)
	}
}

// H-15 / R4-payment-sentinel-unmapped: this handler's own "cannot confirm a
// cancelled booking" check above reads booking.Status before
// InsertAndConfirmBooking's transaction opens, so a cancellation committing
// in that gap reaches the store as ErrBookingNotConfirmable rather than being
// caught here. Before this fix nothing named that sentinel, so it fell
// through to the generic 500 the owner got no matter how much cash they had
// already taken at the counter. It must now answer 409, the same family every
// other business-answer conflict in this handler already gets.
func TestConfirmPaymentAnswersConflictWhenTheStoreCatchesAConcurrentCancellation(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "pending"
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.insertAndConfirmErr = bookingstore.ErrBookingNotConfirmable

	w := httptest.NewRecorder()
	f.handler.ConfirmPayment(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{"method":"cash","amount":100000}`))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409 when the store catches a concurrent cancellation; got %d (%s)", w.Code, w.Body.String())
	}
}

// A generic staff edit must not be able to move a partially refunded booking
// anywhere but refund_status 'full' — that transition, and only it, is what the
// manual-refund endpoint itself makes once the owner confirms the cash
// portion came back. Before the equivalent entry existed, the transition map
// had no key for that state at all, so the map lookup's `ok` came back
// false and the whole guard silently let a generic edit move it anywhere,
// including back to 'none' with the manual balance still unpaid.
func TestUpdateGuardsThePartialRefundTransition(t *testing.T) {
	tests := []struct {
		name   string
		target string
		wantOK bool
	}{
		{"to a full refund is the one allowed move", bookingstore.RefundStatusFull, true},
		{"back to no refund at all is refused", bookingstore.RefundStatusNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := futureBooking(complexID)
			booking.Status = "cancelled"
			booking.RefundStatus = bookingstore.RefundStatusPartial
			f.store.booking = booking

			w := httptest.NewRecorder()
			body := fmt.Sprintf(`{"refund_status":%q}`, tt.target)
			f.handler.Update(w, ownerRequest(t, http.MethodPut, "/", complexID,
				map[string]string{"bookingID": booking.ID.String()}, body))

			if tt.wantOK {
				if w.Code != http.StatusOK {
					t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
				}
				if len(f.store.updated) != 1 || f.store.updated[0].RefundStatus != bookingstore.RefundStatus(tt.target) {
					t.Errorf("want the booking written with refund_status %q; got %+v", tt.target, f.store.updated)
				}
			} else {
				if w.Code != http.StatusConflict {
					t.Errorf("want 409; got %d (%s)", w.Code, w.Body.String())
				}
				if len(f.store.updated) != 0 {
					t.Error("a refused transition must not be persisted")
				}
			}
		})
	}
}

// ManualRefund is the write behind the owner confirming, by hand, that they
// returned a partially refunded booking's remaining cash balance. It
// happy-paths only from exactly that state.
func TestManualRefundClosesOutTheCashBalance(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "cancelled"
	booking.RefundStatus = bookingstore.RefundStatusPartial
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.payments.manualRefundAmount = 350_000

	w := httptest.NewRecorder()
	f.handler.ManualRefund(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"bookingID": booking.ID.String()}, `{}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.manualRefundCalls != 1 {
		t.Fatalf("RecordManualRefund must be called exactly once; got %d", f.payments.manualRefundCalls)
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != "manual_refund" {
		t.Errorf("the manual refund was not audited; got %+v", f.audit.entries)
	}
	if len(f.realtime.published) == 0 {
		t.Error("the owner's dashboard must be told")
	}

	body := decode(t, w)
	if body["returned_amount"] != float64(350_000) {
		t.Errorf("want the amount RecordManualRefund reported; got %v", body["returned_amount"])
	}
	if _, ok := body["payments"]; !ok {
		t.Error("the response must carry the ledger after the write")
	}
	bookingBody, _ := body["booking"].(map[string]any)
	if bookingBody["refund_status"] != bookingstore.RefundStatusFull {
		t.Errorf("the response booking must read a full refund; got %v", bookingBody["refund_status"])
	}
}

// A booking that never reached a partial refund — or was never cancelled — has
// no manual portion to close out. This is refused with a clear message
// before RecordManualRefund is ever called, matching this handler set's tone
// for a rejection the owner reads directly (see ConfirmPayment's "esta
// reserva ya tiene pago confirmado").
func TestManualRefundRefusesWhenNothingIsOwed(t *testing.T) {
	tests := []struct {
		name             string
		status           string
		collectionStatus string
		refundStatus     string
	}{
		{"never cancelled", "confirmed", bookingstore.CollectionStatusFullyPaid, bookingstore.RefundStatusPartial},
		{"fully refunded already", "cancelled", bookingstore.CollectionStatusFullyPaid, bookingstore.RefundStatusFull},
		{"never paid", "cancelled", bookingstore.CollectionStatusUnpaid, bookingstore.RefundStatusNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := futureBooking(complexID)
			booking.Status = bookingstore.BookingStatus(tt.status)
			booking.CollectionStatus = bookingstore.CollectionStatus(tt.collectionStatus)
			booking.RefundStatus = bookingstore.RefundStatus(tt.refundStatus)
			f.store.booking = booking
			f.linkResolver.booking = booking

			w := httptest.NewRecorder()
			f.handler.ManualRefund(w, ownerRequest(t, http.MethodPost, "/", complexID,
				map[string]string{"bookingID": booking.ID.String()}, `{}`))

			if w.Code != http.StatusBadRequest {
				t.Errorf("want 400; got %d (%s)", w.Code, w.Body.String())
			}
			if f.payments.manualRefundCalls != 0 {
				t.Error("RecordManualRefund must not be called when nothing is owed")
			}
			if len(f.audit.entries) != 0 {
				t.Error("nothing may be audited when the request is refused")
			}
		})
	}
}

// The owner's list is scoped to a single day, so the date is required rather
// than defaulted — a missing one would silently return today instead of what
// the dashboard asked for.
func TestListRequiresADate(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.List(w, ownerRequest(t, http.MethodGet, "/", uuid.New(), nil, ""))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 without a date; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestListReturnsTheComplexBookings(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	f.store.list = []*bookingstore.Booking{futureBooking(complexID), futureBooking(complexID)}

	w := httptest.NewRecorder()
	f.handler.List(w, ownerRequest(t, http.MethodGet, "/?date="+time.Now().Format("2006-01-02"), complexID, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	list, _ := decode(t, w)["bookings"].([]any)
	if len(list) != 2 {
		t.Errorf("want 2 bookings; got %d", len(list))
	}
}

func TestHandlersRequireTheComplexInContext(t *testing.T) {
	f := newFixture(t)

	for name, call := range map[string]http.HandlerFunc{
		"list":            f.handler.List,
		"get":             f.handler.Get,
		"update":          f.handler.Update,
		"create":          f.handler.Create,
		"cancel":          f.handler.Cancel,
		"confirm payment": f.handler.ConfirmPayment,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			call(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

			if w.Code != http.StatusInternalServerError {
				t.Errorf("want 500 with no complex in context; got %d", w.Code)
			}
		})
	}
}

// publicBookBody builds a valid public booking payload of the given duration
// (60, 90 or 120), starting at 18:00.
func publicBookBody(complexID, courtID uuid.UUID, durationMinutes int) string {
	date := time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02")
	return fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":"18:00",`+
		`"duration_minutes":%d,"client_first_name":"Ana","client_last_name":"Perez",`+
		`"client_phone":"+541100000000","client_email":"ana@example.com"}`,
		complexID, courtID, date, durationMinutes)
}

// openAllWeek opens the complex the same hours every day and prices the whole
// of them, so a test that is not about the schedule or the price bands does not
// have to state either.
func openAllWeek(f *fixture, courtID uuid.UUID, opensAt, closesAt string) {
	f.complexes.schedules = []*complexstore.Schedule{}
	for _, d := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"} {
		f.complexes.schedules = append(f.complexes.schedules, &complexstore.Schedule{Day: d, OpenTime: opensAt, CloseTime: closesAt})
		f.courts.prices = append(f.courts.prices, courtstore.NewCourtPriceForTest(courtID, d, opensAt, closesAt, 500_000))
	}
}

// preparePublicBooking wires a fixture that can complete a public booking.
func preparePublicBooking(f *fixture) (complexID, courtID uuid.UUID) {
	complexID, courtID = uuid.New(), uuid.New()
	token := "seller-token"
	complex := complexstore.NewComplexForTest(complexID, &token, nil)
	complex.Name = "Vibe"
	complex.Slug = "vibe"
	complex.IsActive = true
	complex.DepositPercentage = 30
	complex.CancellationHours = 24
	f.complexes.complex = complex
	// Opening at 09:00 rather than 08:00 is not cosmetic. Both write paths now
	// validate against slots.Grid, and the grid a 90-minute court has from
	// 08:00 offers 08:00, 09:30, 11:00 ... — 18:00 is not on it. Every booking
	// in this suite starts at 18:00, which is exactly the position the
	// storefront never offered and the write path used to accept anyway
	// (finding 41). From 09:00 the grid runs 09:00, 10:30, ... 18:00, 19:30,
	// 21:00, so 18:00 is a real slot and these tests are about what they say
	// they are about rather than about the grid.
	openAllWeek(f, courtID, "09:00", "23:00")
	f.courts.court = &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}
	return complexID, courtID
}

// More consecutive slots than the configured maximum would let one client take
// the whole evening in a single request.
// TestPublicBookRefusesWhenTheComplexHasNoSellerCredential is the
// mutation-verified money-path test (spec requirement 6 / design's blocking
// constraint): no code path may name mp.CreatePreferenceInput.Caller from
// anything other than a value SellerAccessToken() produced, and
// CreatePreference must never be reached when the accessor refuses.
// ErrMPNotConnected is the reachable failure mode in Slice 1;
// ErrMPCredentialUnreadable's arm is completed in Slice 2 Phase 18 once a
// real decrypt failure is constructible.
//
// Mutation-verified: delete the guard at public.go's
// `complex.SellerAccessToken()` check, re-run this test — it must fail
// (booking proceeds, CreatePreference gets called with an empty token).
// Restore the guard afterward. See the PR description for the recorded
// RED/GREEN mutation output.
func TestPublicBookRefusesWhenTheComplexHasNoSellerCredential(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	// Override the complex prepared above with one that has never connected
	// MercadoPago.
	disconnected := complexstore.NewComplexForTest(complexID, nil, nil)
	disconnected.Name = "Vibe"
	disconnected.Slug = "vibe"
	disconnected.IsActive = true
	disconnected.DepositPercentage = 30
	disconnected.CancellationHours = 24
	f.complexes.complex = disconnected

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 when MercadoPago is not connected; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("no booking may be left pending against a preference that was never created")
	}
	if f.checkout.created != 0 {
		t.Errorf("CreatePreference must never be called when the seller credential is unusable; called %d times", f.checkout.created)
	}
	if f.checkout.lastInput.Caller != (mp.Caller{}) {
		t.Error("no caller may be named from a raw stored value when the accessor refused; CreatePreferenceInput.Caller was populated")
	}
}

// TestCheckoutRetryReportsAFailedCredentialPersist covers the branch in
// createMPPreferenceWithRetry that alerts when a freshly refreshed
// MercadoPago credential cannot be stored.
//
// MercadoPago rotates the refresh token on every use: the moment
// RefreshOAuthToken returns, the refresh token in our database is already
// dead. If the write that replaces it is refused, this venue's checkout
// keeps working only until the new access token expires, and then no code
// path can refresh it again — the next owner-facing symptom is checkout
// failing outright with no record of why. The cron refresh path already
// alerted on this; this one only logged, so the same fault was invisible
// depending on which path hit it first.
//
// The complex_id is asserted alongside the alert because an operator who
// cannot tell which venue needs reconnecting cannot act on it.
//
// Mutation: delete the sentry.CaptureMessage call in public.go's
// UpdateMPCredentials error branch, leaving only the log line, and re-run —
// this test must fail.
func TestCheckoutRetryReportsAFailedCredentialPersist(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	accessToken, refreshToken := "expired-seller-token", "seller-refresh-token"
	complex := complexstore.NewComplexForTest(complexID, &accessToken, &refreshToken)
	complex.Name = "Vibe"

	// MercadoPago rejects the stored access token, which is what sends the
	// checkout down the refresh-and-retry path in the first place.
	f.checkout.err = &mp.APIError{StatusCode: http.StatusUnauthorized, Body: "invalid access token"}
	// The refresh succeeds — new tokens in hand — and the write that must
	// store them is refused.
	f.complexes.credentialsErr = errors.New("database unavailable")

	sentryEvents := withCapturedSentryEvents(t)

	_, _ = f.service.createMPPreferenceWithRetry(t.Context(),
		mp.CreatePreferenceInput{Caller: mustSeller(t, accessToken)}, complex)

	if len(f.complexes.credentials) != 1 {
		t.Fatalf("the refreshed credential must be written back exactly once; got %d attempts", len(f.complexes.credentials))
	}

	messages := sentryEvents.messages()
	alert, found := findMessage(messages, "refreshed-credential persist FAILED")
	if !found {
		t.Fatalf("a refreshed MercadoPago credential that could not be stored must alert, "+
			"not just log: the stored refresh token is dead from here on; captured %q", messages)
	}
	if !strings.Contains(alert, complexID.String()) {
		t.Errorf("the alert must name the complex whose credential is now unrefreshable, "+
			"or the operator cannot act on it; want complex_id=%s, got %q", complexID, alert)
	}
}

// TestPublicBookRefusesWhenTheComplexCredentialIsUnreadable completes spec
// requirement 6's second arm, now that Phase 17 makes a real decrypt
// failure constructible: a stored credential that exists but cannot be
// decrypted (wrong/retired key, corruption, tampering) must refuse exactly
// like the not-connected case, and — new in Slice 2 — alert to Sentry
// (public.go's ErrMPCredentialUnreadable branch added in Phase 2.1).
//
// Mutation-verified: delete Phase 2.1's guard at public.go's
// `complex.SellerAccessToken()` check, re-run this test — it must fail
// (booking proceeds, CreatePreference gets called with an empty token, no
// Sentry alert). Restore the guard afterward. See the PR description for
// the recorded RED/GREEN mutation output.
func TestPublicBookRefusesWhenTheComplexCredentialIsUnreadable(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	unreadable := complexstore.NewComplexWithUnreadableCredentialForTest(complexID)
	unreadable.Name = "Vibe"
	unreadable.Slug = "vibe"
	unreadable.IsActive = true
	unreadable.DepositPercentage = 30
	unreadable.CancellationHours = 24
	f.complexes.complex = unreadable

	sentryEvents := withCapturedSentryEvents(t)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 when the credential cannot be decrypted; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("no booking may be left pending against a preference that was never created")
	}
	if f.checkout.created != 0 {
		t.Errorf("CreatePreference must never be called when the seller credential is unusable; called %d times", f.checkout.created)
	}
	if f.checkout.lastInput.Caller != (mp.Caller{}) {
		t.Error("no caller may be named from a raw stored value when the accessor refused; CreatePreferenceInput.Caller was populated")
	}
	if sentryEvents.count() == 0 {
		t.Error("an unreadable credential must alert to Sentry, not fail silently")
	}
}

// A duration that is not one of the grid's permitted lengths (60, 90, 120) is
// refused as input, the same way slot_count once had a hard ceiling — except
// now the set is fixed by slots.PermittedDurations rather than a configurable
// maximum.
func TestPublicBookRefusesADurationThatIsNotPermitted(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 45)))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for a duration that is not 60, 90 or 120; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.locks.acquired) != 0 {
		t.Error("no slot may be held for a rejected request")
	}
}

// The same refusal on the owner's dashboard, which shares no code with
// PublicBook's validator.Check call but must agree on the same permitted set.
func TestStaffCreateRefusesADurationThatIsNotPermitted(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := staffCreate(t, f, complexID, courtID, onGrid)
	if w.Code != http.StatusCreated {
		t.Fatalf("premise: %s must be bookable at all, or this test proves nothing; got %d (%s)", onGrid, w.Code, w.Body.String())
	}

	f2, complexID2, courtID2 := staffFixture(t)
	date, _ := bookableDate()
	body := staffBookBodyOnFor(courtID2, date, onGrid, 45)

	w2 := httptest.NewRecorder()
	f2.handler.Create(w2, ownerRequest(t, http.MethodPost, "/", complexID2,
		map[string]string{"id": complexID2.String()}, body))

	if w2.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for a duration that is not 60, 90 or 120; got %d (%s)", w2.Code, w2.Body.String())
	}
	if len(f2.store.inserted) != 0 {
		t.Error("no booking may be created for an unpermitted duration")
	}
}

// The slot lock is what stops two clients paying for the same time. If the
// lock is already held, the booking must not proceed.
func TestPublicBookStopsWhenTheSlotIsAlreadyHeld(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.locks.acquireErr = bookingstore.ErrSlotLocked

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code == http.StatusCreated {
		t.Errorf("a held slot must not be bookable; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 0 {
		t.Error("no booking may be created for a slot somebody else holds")
	}
	if f.checkout.created != 0 {
		t.Error("no checkout may be created for a slot somebody else holds")
	}
}

// If the booking insert loses the race, the held slots have to be released or
// they stay locked until the TTL expires and nobody can book them.
func TestPublicBookReleasesTheSlotsWhenTheInsertLoses(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.store.insertErr = bookingstore.ErrSlotUnavailable

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 120)))

	if w.Code == http.StatusCreated {
		t.Fatalf("want a rejection; got %d", w.Code)
	}
	if len(f.locks.released) == 0 {
		t.Error("the held slots were not released, so they stay locked until the TTL expires")
	}
}

func TestPublicBookHoldsTheSlotAndCreatesCheckout(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.locks.acquired) != 1 {
		t.Errorf("want the slot held; got %v", f.locks.acquired)
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("want one booking; got %d", len(f.store.inserted))
	}
	// A public booking is not confirmed until the payment webhook lands.
	if got := f.store.inserted[0].Status; got != "pending" {
		t.Errorf("a public booking must start pending; got %q", got)
	}
	if f.checkout.created != 1 {
		t.Errorf("want a checkout preference; got %d", f.checkout.created)
	}
	if len(f.payments.inserted) != 1 {
		t.Errorf("want the pending payment recorded; got %d", len(f.payments.inserted))
	}
}

// TestPublicBookCancelsAndSendsNoConfirmationWhenMPPreferenceFails is the QA
// coverage for the "does an unpaid public booking ever get a confirmation
// email" question: when CreatePreference fails (MercadoPago 503, PolicyAgent
// refusal, anything), the booking this request just inserted must be
// cancelled immediately, its slot lock released, and — the part nothing
// exercised before this test — notify.BookingConfirmed must never be called.
// A client who never got a usable checkout link must never receive an email
// telling them their reservation is confirmed.
//
// Mutation: delete the `booking.Status = "cancelled"` / store.Update block in
// public.go's CreatePreference error branch (or move the notify.BookingConfirmed
// call above the CreatePreference call), re-run — this test must fail.
func TestPublicBookCancelsAndSendsNoConfirmationWhenMPPreferenceFails(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.checkout.err = errors.New("mp: API error 503: service unavailable")

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when the payment link cannot be created; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("want the booking inserted before the MP call; got %d", len(f.store.inserted))
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Fatalf("want the booking cancelled after the preference failure; got updates %v", f.store.updated)
	}
	if len(f.locks.released) != 1 {
		t.Errorf("want the slot lock released so the court is bookable again; got %v", f.locks.released)
	}
	if len(f.notify.confirmed) != 0 {
		t.Errorf("a booking that never got a payment link must not be confirmed by email; got %v", f.notify.confirmed)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("this is a same-request rollback, not a client-facing cancellation notice; want none, got %v", f.notify.cancelled)
	}
}

// TestPublicBookResponseCarriesTheTokenAndNeverTheBookingID pins tasks
// 10.2/11.4 (specs/booking-link-credential's "public response bodies never
// return the booking's primary key" requirement): the "token" field is the
// credential the client carries forward, and no field anywhere in the body —
// including the nested "booking" object — is the booking's UUID.
func TestPublicBookResponseCarriesTheTokenAndNeverTheBookingID(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Booking map[string]any `json:"booking"`
		Token   string         `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if body.Token == "" {
		t.Error("want a non-empty token field in the response")
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("want one booking inserted; got %d", len(f.store.inserted))
	}
	if body.Token != f.store.inserted[0].LinkToken {
		t.Errorf("token = %q, want the minted LinkToken %q", body.Token, f.store.inserted[0].LinkToken)
	}
	if _, present := body.Booking["id"]; present {
		t.Error("the booking's primary key must not appear in the response — replacing it as the " +
			"credential on the way in while still emitting it on the way out would be half a fix")
	}
	for key, value := range body.Booking {
		if value == f.store.inserted[0].ID.String() {
			t.Errorf("field %q carries the booking's UUID; it must not appear anywhere in the response body", key)
		}
	}
}

// The client is created from the booking form, since they have no account.
func TestPublicBookCreatesTheClientFromTheForm(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.clients.created) != 1 || f.clients.created[0] != "+541100000000" {
		t.Errorf("the client was not created from the form; got %v", f.clients.created)
	}
}

// R1-client-name-overwrite: PublicBook needs no account, so it must never be
// able to overwrite an existing client's stored name on a phone match — only
// the authenticated owner path (create.go) is trusted with that correction.
// This pins the value PublicBook passes to GetOrCreate, since the stub always
// succeeds regardless of it and a regression here would otherwise be invisible
// to every other assertion in this file.
func TestPublicBookNeverAllowsOverwritingAnExistingClientsName(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.clients.allowNameUpdateCalls) != 1 || f.clients.allowNameUpdateCalls[0] {
		t.Errorf("the public booking endpoint must call GetOrCreate with allowNameUpdate=false; got %v", f.clients.allowNameUpdateCalls)
	}
}

// The owner-booking counterpart: create.go runs behind requireComplexOwner, so
// the same phone match is trusted to carry the owner's name correction — see
// clientstore.Store.GetOrCreate's comment on allowNameUpdate.
func TestStaffCreateAllowsCorrectingAnExistingClientsName(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := staffCreate(t, f, complexID, courtID, onGrid)
	if w.Code != http.StatusCreated {
		t.Fatalf("premise: %s must be bookable, or this test proves nothing; got %d (%s)", onGrid, w.Code, w.Body.String())
	}
	if len(f.clients.allowNameUpdateCalls) != 1 || !f.clients.allowNameUpdateCalls[0] {
		t.Errorf("the owner booking endpoint must call GetOrCreate with allowNameUpdate=true; got %v", f.clients.allowNameUpdateCalls)
	}
}

// TestPublicStatusRejectsAnAbsentToken is task 13.1's absent-input half:
// PublicStatus keeps its own existing 400 shape when the token query
// parameter is missing entirely — resolveLink never runs.
func TestPublicStatusRejectsAnAbsentToken(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/", ""))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 with no token; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestPublicStatusUnknownTokenIsNotFound is task 13.1: a value never minted —
// including a syntactically valid but unresolvable one, since a token is no
// longer parsed as a UUID — answers 404, never 410.
func TestPublicStatusUnknownTokenIsNotFound(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=never-minted-value", ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for an unknown token; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestABookingIDPresentedAsTokenAuthorizesNothing is task 13.4/17.1: the
// booking's own primary key must not work as a token. The stub's default
// permissiveness (it ignores the token argument entirely) does not prove
// this on its own, so validToken pins the store to reject anything but the
// real minted value — the same shape a real ResolveBooking(hash) lookup
// enforces via token_hash UNIQUE having no row for a bare UUID.
func TestABookingIDPresentedAsTokenAuthorizesNothing(t *testing.T) {
	f := newFixture(t)
	booking := futureBooking(uuid.New())
	f.linkResolver.booking = booking
	f.linkResolver.validToken = "the-real-minted-token"

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token="+booking.ID.String(), ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("a booking id presented as a token must authorize nothing; want 404, got %d (%s)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), booking.ID.String()) {
		t.Errorf("the response must not echo the presented booking id; got %s", w.Body.String())
	}
}

// TestAnExpiredTokenGetsA410 is task 13.2: a real, previously-minted token
// whose expiry has passed answers 410 with a non-empty, distinguishable body
// — never the same body as an unknown token's 404.
//
// Mutation, run and recorded: swap the 410 branch's body for the same message
// as NotFound's ("the requested resource could not be found") — re-run, and
// this test's distinguishability assertion must fail.
func TestAnExpiredTokenGetsA410(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	// outOfWindowBooking (cancel_test.go): starts in 2h against a 24h window,
	// created 24h ago — both CanRefund branches are false, so LinkLive's
	// disjunct cannot rescue this booking from its already-past expiry.
	booking := outOfWindowBooking(complexID)
	f.linkResolver.booking = booking
	f.linkResolver.expiresAt = time.Now().Add(-time.Hour)
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=expired-token", ""))

	if w.Code != http.StatusGone {
		t.Fatalf("want 410 for an expired token; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if msg, _ := body["error"].(string); msg == "" || msg == "the requested resource could not be found" {
		t.Errorf("want a non-empty body distinguishable from the 404 case; got %v", body)
	}
}

// TestAnExpiredTokenOnARefundEligibleBookingIsNot410 is task 13.3: LinkLive's
// invariant exercised through the HTTP layer, not just the pricing matrix — a
// booking still before its CanRefund deadline is never rejected as expired.
func TestAnExpiredTokenOnARefundEligibleBookingIsNot410(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.linkResolver.booking = booking
	f.linkResolver.expiresAt = time.Now().Add(-time.Hour)
	// cancellation_hours = 0 means CanRefund is unconditionally true — the
	// exact configuration design.md's falsified-invariant finding is about.
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 0}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=expired-but-refundable", ""))

	if w.Code == http.StatusGone {
		t.Errorf("a refund-eligible booking must never be rejected for token expiry; got 410 (%s)", w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Errorf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestACancelledBookingsTokenStillAnswersStatus is task 13.5: a still-
// unexpired token on an already-cancelled booking returns its current state,
// not 410/404 — cancellation does not revoke the token.
func TestACancelledBookingsTokenStillAnswersStatus(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "cancelled"
	booking.CollectionStatus = bookingstore.CollectionStatusFullyPaid
	booking.RefundStatus = bookingstore.RefundStatusFull
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=still-valid", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	bookingBody, _ := body["booking"].(map[string]any)
	if bookingBody["status"] != "cancelled" ||
		bookingBody["collection_status"] != bookingstore.CollectionStatusFullyPaid ||
		bookingBody["refund_status"] != bookingstore.RefundStatusFull {
		t.Errorf("want the current cancelled/fully-refunded state; got %v", bookingBody)
	}
}

// TestPublicStatusFullPayloadForAConfirmedBooking is the new PublicStatus
// contract: the public success page renders entirely from this response when
// the browser has no cached copy of the booking, so it must carry the whole
// picture, not just the two status axes.
func TestPublicStatusFullPayloadForAConfirmedBooking(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Price = 14000
	booking.DepositAmount = 4200
	booking.DurationMinutes = 60
	booking.CreatedAt = time.Now().In(timezone.Argentina)
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{
		ID: complexID, Name: "Vibe Padel", Address: "Av. Siempre Viva 742", Phone: "+5491112345678",
		CancellationHours: 24,
	}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1", Sport: "padel", CourtType: "indoor"}
	f.payments.payment = &paymentstore.Payment{BookingID: booking.ID, ServiceFee: 1000}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=full-payload", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	bookingBody, _ := body["booking"].(map[string]any)

	wantString := map[string]string{
		"status": "confirmed", "collection_status": "deposit_paid", "refund_status": "none",
		"complex_name": "Vibe Padel", "complex_address": "Av. Siempre Viva 742", "complex_phone": "+5491112345678",
		"court_name": "Cancha 1", "sport": "padel", "court_type": "indoor",
		"date": booking.Date.Format("2006-01-02"), "start_time": "18:00",
		"starts_at": booking.StartsAt.Format(time.RFC3339), "ends_at": booking.EndsAt.Format(time.RFC3339),
	}
	for field, want := range wantString {
		if got, _ := bookingBody[field].(string); got != want {
			t.Errorf("%s: want %q, got %q (%v)", field, want, got, bookingBody[field])
		}
	}

	wantNumber := map[string]float64{
		"duration_minutes": 60, "price": 14000, "deposit_amount": 4200,
		"service_fee": 1000, "remaining_amount": 9800,
	}
	for field, want := range wantNumber {
		got, ok := bookingBody[field].(float64)
		if !ok || got != want {
			t.Errorf("%s: want %v, got %v", field, want, bookingBody[field])
		}
	}

	cancellation, _ := bookingBody["cancellation"].(map[string]any)
	if cancellation == nil {
		t.Fatalf("want a cancellation object; got %v", bookingBody)
	}
	if cancellation["can_cancel"] != true {
		t.Errorf("want can_cancel true; got %v", cancellation["can_cancel"])
	}
	if cancellation["can_refund_now"] != true {
		t.Errorf("want can_refund_now true; got %v", cancellation["can_refund_now"])
	}
	if got, _ := cancellation["cancellation_hours"].(float64); got != 24 {
		t.Errorf("want cancellation_hours 24; got %v", cancellation["cancellation_hours"])
	}
	wantDeadline := pricing.RefundDeadline(booking, 24, f.service.cfg.GracePeriod).Format(time.RFC3339)
	if got, _ := cancellation["refund_deadline"].(string); got != wantDeadline {
		t.Errorf("want refund_deadline %s; got %v", wantDeadline, cancellation["refund_deadline"])
	}
}

// TestResolveLinkAnswersVenueGoneWhenTheComplexWasSoftDeleted is H-18's first
// call site: a token that still resolves to a real booking, whose complex has
// since been soft-deleted (GetByID reads active_complexes, which excludes
// it). Before this fix resolveLink's complex lookup had no ErrRecordNotFound
// branch at all and fell into ServerError — a bare 500 the client's retry
// button could never get past. leaving f.complexes.complex nil reproduces
// exactly that: the stub answers ErrRecordNotFound the same way the real
// store does for a soft-deleted row.
func TestResolveLinkAnswersVenueGoneWhenTheComplexWasSoftDeleted(t *testing.T) {
	f := newFixture(t)
	booking := futureBooking(uuid.New())
	f.linkResolver.booking = booking
	// f.complexes.complex is left nil on purpose.

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=venue-gone", ""))

	if w.Code != http.StatusGone {
		t.Fatalf("want 410 for a live booking whose venue was deleted; got %d (%s)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "requested resource could not be found") {
		t.Errorf("the venue-gone body must be distinguishable from a bare 404; got %s", w.Body.String())
	}
}

// TestPublicStatusAnswersVenueGoneWhenTheCourtWasDeleted is H-18's second
// call site: the complex is still active but this particular court was
// soft-deleted (an owner retiring it, per courts.Handler.Delete — permitted
// once it has no active bookings). Before this fix the court lookup had no
// ErrRecordNotFound branch and fell into ServerError, exactly matching the
// production evidence: a valid token answering 500 while an unknown one
// correctly answers 404.
func TestPublicStatusAnswersVenueGoneWhenTheCourtWasDeleted(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe Padel", CancellationHours: 24}
	// f.courts.court is left nil on purpose — the court no longer exists.

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=court-gone", ""))

	if w.Code != http.StatusGone {
		t.Fatalf("want 410 for a booking whose court was deleted; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestPublicCancelInfoAnswersVenueGoneWhenTheCourtWasDeleted is the same
// scenario against PublicCancelInfo — the sibling the finding's table showed
// making the identical mistake as PublicStatus.
func TestPublicCancelInfoAnswersVenueGoneWhenTheCourtWasDeleted(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe Padel", CancellationHours: 24}
	// f.courts.court is left nil on purpose — the court no longer exists.

	w := httptest.NewRecorder()
	f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet, "/?token=court-gone", ""))

	if w.Code != http.StatusGone {
		t.Fatalf("want 410 for a cancel-info request whose court was deleted; got %d (%s)", w.Code, w.Body.String())
	}
}

// TestPublicStatusZeroCancellationHoursDeadlineIsBookingStart covers the
// cancellation_hours <= 0 branch of pricing.RefundDeadline through the HTTP
// layer: the deadline reported is the booking's own start.
func TestPublicStatusZeroCancellationHoursDeadlineIsBookingStart(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 0}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=zero-hours", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	bookingBody, _ := body["booking"].(map[string]any)
	cancellation, _ := bookingBody["cancellation"].(map[string]any)

	startTimeObj, _ := time.Parse("15:04", booking.StartTime)
	wantStart := time.Date(
		booking.Date.Year(), booking.Date.Month(), booking.Date.Day(),
		startTimeObj.Hour(), startTimeObj.Minute(), 0, 0, timezone.Argentina,
	)
	if got, _ := cancellation["refund_deadline"].(string); got != wantStart.Format(time.RFC3339) {
		t.Errorf("want refund_deadline %s (booking start); got %v", wantStart.Format(time.RFC3339), got)
	}
}

// TestPublicStatusGraceStillOpenAfterWindowClosed is the scenario RefundDeadline
// exists for: a booking made a minute ago whose standard window has already
// closed, but the grace period from creation has not — the deadline reported
// is the grace end, and can_refund_now is still true.
func TestPublicStatusGraceStillOpenAfterWindowClosed(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	now := time.Now().In(timezone.Argentina)
	booking := futureBooking(complexID)
	// Starts in 2 minutes against a 24h window: the standard window closed
	// long before this booking was even made.
	booking.Date = now
	booking.StartTime = now.Add(2 * time.Minute).Format("15:04")
	booking.CreatedAt = now.Add(-time.Minute)
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=grace-window", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	bookingBody, _ := body["booking"].(map[string]any)
	cancellation, _ := bookingBody["cancellation"].(map[string]any)

	if cancellation["can_refund_now"] != true {
		t.Errorf("want can_refund_now true while the grace period is still open; got %v", cancellation["can_refund_now"])
	}
	wantDeadline := booking.CreatedAt.In(timezone.Argentina).Add(f.service.cfg.GracePeriod).Format(time.RFC3339)
	if got, _ := cancellation["refund_deadline"].(string); got != wantDeadline {
		t.Errorf("want refund_deadline %s (grace end); got %v", wantDeadline, got)
	}
}

// TestPublicStatusCancelledBookingCannotCancelAgain covers the terminal-status
// branch: can_cancel is false and refund_deadline is not reported.
func TestPublicStatusCancelledBookingCannotCancelAgain(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "cancelled"
	booking.CollectionStatus = bookingstore.CollectionStatusFullyPaid
	booking.RefundStatus = bookingstore.RefundStatusFull
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=cancelled-booking", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	bookingBody, _ := body["booking"].(map[string]any)
	cancellation, _ := bookingBody["cancellation"].(map[string]any)

	if cancellation["can_cancel"] != false {
		t.Errorf("want can_cancel false for a cancelled booking; got %v", cancellation["can_cancel"])
	}
	if cancellation["can_refund_now"] != false {
		t.Errorf("want can_refund_now false for a cancelled booking; got %v", cancellation["can_refund_now"])
	}
	if cancellation["refund_deadline"] != nil {
		t.Errorf("want refund_deadline null for a cancelled booking; got %v", cancellation["refund_deadline"])
	}
}

// TestPublicCancelOnATerminalBookingKeepsItsExisting400 is task 13.6: the
// token swap does not touch PublicCancel's terminal-booking guard.
func TestPublicCancelOnATerminalBookingKeepsItsExisting400(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	booking.Status = "completed"
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for an already-terminal booking; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestPublicCancelRefusesASettledBooking(t *testing.T) {
	for _, status := range []string{"cancelled", "completed", "no_show"} {
		t.Run(status, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := futureBooking(complexID)
			booking.Status = bookingstore.BookingStatus(status)
			f.store.booking = booking
			f.linkResolver.booking = booking
			f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}

			w := httptest.NewRecorder()
			f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
				`{"token":"test-token"}`))

			if w.Code != http.StatusBadRequest {
				t.Errorf("a %s booking must not be cancellable; want 400, got %d (%s)", status, w.Code, w.Body.String())
			}
		})
	}
}

func TestPublicCancelRefundsWithinTheWindow(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.refunds.refunded) != 1 {
		t.Error("a cancellation inside the window must refund")
	}
}

// Outside the window the booking is still cancelled — the client should not be
// forced to show up — but the deposit is not returned.
func TestPublicCancelOutsideTheWindowDoesNotRefund(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	// Two hours from now, against a 24-hour window, and created long enough ago
	// that the grace period has passed.
	booking.Date = time.Now().In(timezone.Argentina)
	booking.StartTime = time.Now().In(timezone.Argentina).Add(2 * time.Hour).Format("15:04")
	booking.CreatedAt = time.Now().Add(-24 * time.Hour)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: booking.ClientID}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/",
		`{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 — the booking is still cancelled; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.refunds.refunded) != 0 {
		t.Error("a cancellation outside the window must not refund")
	}
	if len(f.store.updated) == 0 || f.store.updated[0].Status != "cancelled" {
		t.Error("the booking must still be cancelled")
	}
}

func TestWhatsAppVerifyEchoesTheChallenge(t *testing.T) {
	f := newFixture(t)
	f.whatsapp.challenge = "challenge-123"

	w := httptest.NewRecorder()
	f.handler.WhatsAppVerify(w, publicRequest(t, http.MethodGet, "/", ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "challenge-123") {
		t.Errorf("Meta's handshake needs the challenge echoed back; got %q", w.Body.String())
	}
}

// An unsigned webhook must not be processed: anyone can POST to this endpoint.
func TestWhatsAppWebhookRejectsABadSignature(t *testing.T) {
	f := newFixture(t)
	f.whatsapp.signatureErr = data.ErrRecordNotFound // any non-nil error

	w := httptest.NewRecorder()
	f.handler.WhatsAppWebhook(w, publicRequest(t, http.MethodPost, "/", `{"entry":[]}`))

	if len(f.store.updated) != 0 {
		t.Error("an unverified webhook must not change a booking")
	}
	// Unlike the MercadoPago webhook, which answers 200 to stop the provider
	// retrying a forgery, this one rejects outright. Both are defensible; the
	// difference is recorded here so it is a decision rather than a drift.
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401; got %d", w.Code)
	}
}

func TestMaskPhone(t *testing.T) {
	tests := map[string]string{
		"+541199887766": "***7766",
		"1234":          "****",
		"12":            "****",
		"":              "****",
	}

	for in, want := range tests {
		if got := maskPhone(in); got != want {
			t.Errorf("maskPhone(%q) = %q; want %q", in, got, want)
		}
	}
}

// publicBookBodyOn is publicBookBody with the date named rather than assumed,
// for the cases that are about the date itself.
func publicBookBodyOn(complexID, courtID uuid.UUID, date string) string {
	return fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":"18:00",`+
		`"duration_minutes":90,"client_first_name":"Ana","client_last_name":"Perez",`+
		`"client_phone":"+541100000000","client_email":"ana@example.com"}`,
		complexID, courtID, date)
}

// H-08: PublicBook had no upper bound on the date, so an anonymous caller
// could book 9999-12-31 — a row that holds a slot, shows in the owner's list,
// and is never reaped, because cron complete_bookings only completes bookings
// whose end time has passed and that one's never will.
//
// The boundary is asserted from both sides on purpose. A test that only proves
// a date far past the horizon is refused would still pass with the comparison
// off by a day, or with the check accidentally rejecting everything; pinning
// the last accepted day and the first refused one is what makes an off-by-one
// visible.
func TestPublicBookRefusesADateBeyondTheBookingHorizon(t *testing.T) {
	today := time.Now().In(timezone.Argentina)

	tests := []struct {
		name       string
		daysAhead  int
		wantStatus int
	}{
		{"the last day inside the horizon is accepted", bookingstore.MaxBookingHorizonDays, http.StatusCreated},
		{"one day past the horizon is refused", bookingstore.MaxBookingHorizonDays + 1, http.StatusConflict},
		{"far past the horizon is refused", bookingstore.MaxBookingHorizonDays + 5000, http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID, courtID := preparePublicBooking(f)
			openAllWeek(f, courtID, "08:00", "23:00")

			date := today.AddDate(0, 0, tt.daysAhead).Format("2006-01-02")
			w := httptest.NewRecorder()
			f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBodyOn(complexID, courtID, date)))

			if w.Code != tt.wantStatus {
				t.Errorf("date %s (%+d days): want %d; got %d (%s)",
					date, tt.daysAhead, tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantStatus != http.StatusCreated && len(f.store.inserted) != 0 {
				t.Error("a booking beyond the horizon must not be persisted")
			}
		})
	}
}

// H-22 gave InsertSafe a new failure of its own: it reads the court under a
// row lock now, so a court deleted while the booking transaction is in flight
// comes back as data.ErrRecordNotFound rather than succeeding onto a court
// that is gone. These two cases pin what each handler does with it.
//
// They exist because the branch is externally observable and nothing else
// proves it. The data-layer race tests assert the error InsertSafe returns;
// they say nothing about the status it becomes, and the difference that
// matters is 404 against the 500 this would otherwise fall through to —
// turning H-22's fix into H-18's defect. The ordering is asserted too: the
// EditConflict branch is tested above and runs first, so a mis-ordered check
// would answer 409 here.

func TestStaffCreateAnswersNotFoundWhenTheCourtIsDeletedMidTransaction(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.store.insertErr = data.ErrRecordNotFound

	w := staffCreate(t, f, complexID, courtID, onGrid)

	if w.Code != http.StatusNotFound {
		t.Fatalf("a court deleted between the check and the commit must answer 404, not %d — "+
			"falling through to 500 is the defect H-18 exists for", w.Code)
	}
}

func TestPublicBookAnswersNotFoundWhenTheCourtIsDeletedMidTransaction(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.store.insertErr = data.ErrRecordNotFound

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 120)))

	if w.Code != http.StatusNotFound {
		t.Fatalf("a court deleted between the check and the commit must answer 404, not %d", w.Code)
	}
	// Not the slot-conflict message: that tells the visitor to pick another
	// hour on a court that no longer exists.
	if body := w.Body.String(); strings.Contains(body, "no longer available, please choose another") {
		t.Errorf("a deleted court must not be reported as a taken slot; got %s", body)
	}
	// The held slot is released on every insert failure, and this new branch
	// must not be the one that forgets.
	if len(f.locks.released) == 0 {
		t.Error("the held slots were not released, so they stay locked until the TTL expires")
	}
}

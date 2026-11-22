package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/mp"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/timezone"
)

// clock is how a test spells an instant on the venue's wall clock.
func clock(t time.Time) string { return t.In(timezone.Argentina).Format("15:04") }

// S2: a hold refused by the cooldown answers its own kind, and tells the
// visitor in Spanish when the hours can be picked again.
func TestTakeHoldRefusedByTheCooldown(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	until := time.Now().Add(7 * time.Minute)
	f.store.holdErr = &bookingstore.HoldCooldownError{Until: until}

	w := httptest.NewRecorder()
	f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if problemKind(body) != string(KindHoldCooldown) {
		t.Errorf("want kind %q; got %q", KindHoldCooldown, problemKind(body))
	}
	if title, _ := body["title"].(string); title == "" {
		t.Error("a Problem must carry a title; the cooldown refusal had none")
	}
	detail, _ := body["detail"].(string)
	if !strings.Contains(detail, clock(until)) || !strings.Contains(detail, "elegir otro horario") {
		t.Errorf("want a Spanish detail naming %s; got %q", clock(until), detail)
	}
}

// S3: an owner booking refused by a visitor's live hold keeps the
// slot-unavailable contract and says, in Spanish, when the slot frees.
func TestStaffCreateOverALiveHoldSaysWhenItFrees(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	freesAt := time.Now().Add(9 * time.Minute)
	f.store.insertErr = &bookingstore.HoldBlockedError{FreesAt: freesAt}
	date, _ := bookableDate()
	body := fmt.Sprintf(`{"court_id":%q,"date":%q,"start_time":%q,"duration_minutes":90,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000","payment_option":"unpaid"}`,
		courtID, date.Format("2006-01-02"), onGrid)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, map[string]string{"id": complexID.String()}, body))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	got := decode(t, w)
	if problemKind(got) != "slot-unavailable" {
		t.Errorf("the kind must stay slot-unavailable; got %q", problemKind(got))
	}
	detail, _ := got["detail"].(string)
	if !strings.Contains(detail, "reservando online") || !strings.Contains(detail, clock(freesAt)) {
		t.Errorf("want the online hold and %s in the detail; got %q", clock(freesAt), detail)
	}
}

// S3, previous behavior: any other slot refusal is the sentence it always was.
func TestStaffCreateOverABookingKeepsItsRefusal(t *testing.T) {
	f, complexID, courtID := staffFixture(t)
	f.store.insertErr = bookingstore.ErrSlotUnavailable
	date, _ := bookableDate()
	body := fmt.Sprintf(`{"court_id":%q,"date":%q,"start_time":%q,"duration_minutes":90,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000","payment_option":"unpaid"}`,
		courtID, date.Format("2006-01-02"), onGrid)

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID, map[string]string{"id": complexID.String()}, body))

	got := decode(t, w)
	if w.Code != http.StatusConflict || problemKind(got) != "slot-unavailable" || got["detail"] != slotTakenMessage {
		t.Errorf("want the plain slot-unavailable refusal; got %d %v", w.Code, got)
	}
}

// S3: the owner's day read carries the day's online holds, and nothing that
// names the visitor.
func TestListCarriesTheDaysOnlineHolds(t *testing.T) {
	f := newFixture(t)
	courtID := uuid.New()
	day := time.Date(2027, 3, 20, 0, 0, 0, 0, time.UTC)
	freesAt := time.Date(2027, 3, 18, 22, 10, 0, 0, time.UTC)
	f.store.liveHolds = []bookingstore.LiveHold{{
		CourtID: courtID, Date: day, StartTime: "23:00", DurationMinutes: 90, FreesAt: freesAt,
	}}

	w := httptest.NewRecorder()
	f.handler.List(w, ownerRequest(t, http.MethodGet, "/?date=2027-03-20", uuid.New(), nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.liveHoldDays) != 1 || !f.store.liveHoldDays[0].Equal(day) {
		t.Errorf("want the holds of the day asked; asked %v", f.store.liveHoldDays)
	}
	holds, _ := decode(t, w)["online_holds"].([]any)
	if len(holds) != 1 {
		t.Fatalf("want one online hold; got %v", holds)
	}
	got, _ := holds[0].(map[string]any)
	want := map[string]any{
		"court_id": courtID.String(), "date": "2027-03-20", "start_time": "23:00",
		"end_time": "00:30", "duration_minutes": float64(90), "frees_at": "2027-03-18T22:10:00Z",
	}
	if len(got) != len(want) {
		t.Errorf("want exactly %v; got %v", want, got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

// S3, previous behavior: a day with no holds still lists its bookings, and the
// holds are an empty list rather than missing.
func TestListWithNoOnlineHolds(t *testing.T) {
	f := newFixture(t)
	f.store.list = []*bookingstore.Booking{futureBooking(uuid.New())}

	w := httptest.NewRecorder()
	f.handler.List(w, ownerRequest(t, http.MethodGet, "/?date=2027-03-20", uuid.New(), nil, ""))

	body := decode(t, w)
	bookings, _ := body["bookings"].([]any)
	holds, ok := body["online_holds"].([]any)
	if w.Code != http.StatusOK || len(bookings) != 1 || !ok || len(holds) != 0 {
		t.Errorf("want one booking and an empty hold list; got %d %v", w.Code, body)
	}
}

// failedCheckout is a converted storefront booking whose payment was
// rejected: still pending, nothing collected, a checkout on file, opened
// minutesAgo of its window ago.
func failedCheckout(f *fixture, minutesAgo int) *bookingstore.Booking {
	complexID, courtID := preparePublicBooking(f)
	b := futureBooking(complexID)
	b.CourtID = courtID
	b.ClientID = uuid.Nil
	b.Status = "pending"
	b.CollectionStatus = bookingstore.CollectionStatusUnpaid
	b.CreatedAt = time.Now().Add(-time.Duration(minutesAgo) * time.Minute)
	b.ClientSnapshot = &bookingstore.ClientSnapshot{FirstName: "Ana", LastName: "Perez", Phone: "+5491100000077", Email: "ana@example.com"}
	f.linkResolver.booking = b
	f.store.booking = b
	f.linkResolver.validToken = "link-token"
	pref := "pref-77"
	f.payments.payment = &paymentstore.Payment{BookingID: b.ID, ComplexID: complexID, Method: "mercadopago", Status: "unpaid", MPPreferenceID: &pref}
	return b
}

const checkoutBody = `{"token":"link-token"}`

// confirmedOnHandOver has the processing behind the reconciler confirm b as
// a hand-over returns, the way the webhook's processing does when it runs
// inline and wins.
func confirmedOnHandOver(f *fixture, b *bookingstore.Booking) {
	f.reconciler.processed = func(string) {
		b.Status = "confirmed"
		b.CollectionStatus = bookingstore.CollectionStatusDepositPaid
	}
}

// S4: a visitor whose card was rejected pays again on the same checkout while
// the window lasts.
func TestRetryCheckoutHandsBackTheSameCheckout(t *testing.T) {
	f := newFixture(t)
	b := failedCheckout(f, 5)

	w := httptest.NewRecorder()
	f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	checkout, _ := decode(t, w)["checkout"].(map[string]any)
	if checkout["mp_init_point"] != "https://mp.test/checkout?pref_id=pref-77" {
		t.Errorf("want the booking's own checkout link; got %v", checkout["mp_init_point"])
	}
	if want := b.CreatedAt.Add(15 * time.Minute).UTC().Format(time.RFC3339); checkout["expires_at"] != want {
		t.Errorf("expires_at = %v, want the end of the window %s", checkout["expires_at"], want)
	}
	if len(f.checkout.reads) != 1 || f.checkout.created != 0 || len(f.store.updated) != 0 {
		t.Errorf("a retry reads the checkout and writes nothing: reads=%v created=%d updated=%d",
			f.checkout.reads, f.checkout.created, len(f.store.updated))
	}
	if f.checkout.readCallers[0] != mustSeller(t, "seller-token") {
		t.Errorf("the checkout must be read as the seller; got %+v", f.checkout.readCallers[0])
	}
}

// S4: no retry once the booking cannot be paid any more.
func TestRetryCheckoutRefusesWhatCannotBePaid(t *testing.T) {
	for name, mutate := range map[string]func(*bookingstore.Booking){
		"the window is over":  func(b *bookingstore.Booking) { b.CreatedAt = time.Now().Add(-16 * time.Minute) },
		"it was released":     func(b *bookingstore.Booking) { b.Status = "cancelled" },
		"it was paid already": func(b *bookingstore.Booking) { b.CollectionStatus = bookingstore.CollectionStatusDepositPaid },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			mutate(failedCheckout(f, 5))

			w := httptest.NewRecorder()
			f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

			if w.Code != http.StatusConflict || len(f.checkout.reads) != 0 {
				t.Errorf("want 409 and no checkout read; got %d reads=%v (%s)", w.Code, f.checkout.reads, w.Body.String())
			}
		})
	}
}

// S4: "Elegir otro horario" closes the checkout, gives the slot back to the
// grid and the waitlist, and tells nobody by email.
func TestReleaseCheckoutGivesTheSlotBack(t *testing.T) {
	f := newFixture(t)
	b := failedCheckout(f, 5)

	w := httptest.NewRecorder()
	f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" || f.store.updated[0].ID != b.ID {
		t.Fatalf("want the booking cancelled; got %+v", f.store.updated)
	}
	if len(f.checkout.expired) != 1 || f.checkout.expired[0] != "pref-77" {
		t.Errorf("the checkout must be closed so it cannot be paid; expired %v", f.checkout.expired)
	}
	if len(f.realtime.published) != 1 || len(f.waitlist.freed) != 1 {
		t.Errorf("the grid and the waitlist must hear the slot is free; published=%d freed=%d", len(f.realtime.published), len(f.waitlist.freed))
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("no email for a release the visitor asked for on screen; got %d", len(f.notify.cancelled))
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != "public_release" {
		t.Errorf("want one public_release audit entry; got %+v", f.audit.entries)
	}
}

// T12 (1): MercadoPago's payment search can lag an approval, and a release
// asks it once. The release is recorded with the cancellation, so the expiry
// sweep asks once more later and a payment its webhook never delivered is
// still found; it is recorded as a release, never as an expiry, so that late
// payment is refunded rather than reopening the booking.
func TestReleaseCheckoutRecordsTheReleaseForASecondAsk(t *testing.T) {
	f := newFixture(t)
	b := failedCheckout(f, 5)

	w := httptest.NewRecorder()
	f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204; got %d (%s)", w.Code, w.Body.String())
	}
	if !slices.Equal(f.store.checkoutReleased, []uuid.UUID{b.ID}) {
		t.Errorf("want the release recorded with the cancellation; got %v", f.store.checkoutReleased)
	}
	if len(f.store.holdExpired) != 0 {
		t.Errorf("a visitor's release is not an expiry a late payment may reopen; got %v", f.store.holdExpired)
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Errorf("want the booking cancelled once; got %+v", f.store.updated)
	}
}

// S4: a release of a booking that was paid after all is refused and changes
// nothing; one already released is a no-op.
func TestReleaseCheckoutLeavesPaidAndReleasedBookingsAlone(t *testing.T) {
	t.Run("paid", func(t *testing.T) {
		f := newFixture(t)
		b := failedCheckout(f, 5)
		b.Status = "confirmed"
		b.CollectionStatus = bookingstore.CollectionStatusDepositPaid

		w := httptest.NewRecorder()
		f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

		if w.Code != http.StatusConflict || len(f.store.updated) != 0 || len(f.checkout.expired) != 0 {
			t.Errorf("want 409 and nothing touched; got %d updated=%d expired=%v", w.Code, len(f.store.updated), f.checkout.expired)
		}
		if kind := problemKind(decode(t, w)); kind != string(httpx.KindBookingPaid) {
			t.Errorf("a paid booking's refusal says so (%s); got %q", httpx.KindBookingPaid, kind)
		}
	})
	t.Run("already released", func(t *testing.T) {
		f := newFixture(t)
		failedCheckout(f, 5).Status = "cancelled"

		w := httptest.NewRecorder()
		f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

		if w.Code != http.StatusNoContent || len(f.store.updated) != 0 || len(f.realtime.published) != 0 {
			t.Errorf("want 204 and nothing written; got %d updated=%d", w.Code, len(f.store.updated))
		}
	})
}

// S4: the checkout's failure redirect carries the booking's link token, so the
// storefront can offer both; it carries nothing about the visitor.
func TestTheCheckoutFailureRedirectCarriesTheLinkToken(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}

	failure, err := url.Parse(f.checkout.lastInput.BackURLs.Failure)
	if err != nil {
		t.Fatalf("failure URL %q: %v", f.checkout.lastInput.BackURLs.Failure, err)
	}
	q := failure.Query()
	if q.Get("token") != f.store.inserted[0].LinkToken || q.Get("error") != "payment_failed" || len(q) != 2 {
		t.Errorf("want only the error and the booking's link token; got %q", failure.String())
	}
}

// S4/S8, finding 5: a retry trusts nothing but MercadoPago. A payment still in
// process is not paid a second time, a link the sweep already closed is not
// handed back, and an approved payment whose notice was lost is processed.
func TestRetryCheckoutAsksMercadoPagoFirst(t *testing.T) {
	t.Run("a payment in process", func(t *testing.T) {
		f := newFixture(t)
		b := failedCheckout(f, 5)
		f.checkout.searchResults = []mp.Payment{{ID: 71, Status: "in_process", ExternalReference: b.ID.String(), DateCreated: time.Now()}}

		w := httptest.NewRecorder()
		f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

		if w.Code != http.StatusConflict || problemKind(decode(t, w)) != string(httpx.KindPaymentInProcess) {
			t.Fatalf("want 409 %s; got %d (%s)", httpx.KindPaymentInProcess, w.Code, w.Body.String())
		}
		if len(f.checkout.reads) != 0 {
			t.Errorf("no checkout link may be handed back while a payment is in process; reads %v", f.checkout.reads)
		}
	})
	t.Run("the link was closed", func(t *testing.T) {
		f := newFixture(t)
		failedCheckout(f, 5)
		f.checkout.readClosed = true

		w := httptest.NewRecorder()
		f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

		if w.Code != http.StatusConflict {
			t.Fatalf("want 409 for a closed link; got %d (%s)", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "pref_id") {
			t.Errorf("a closed link must not be handed back; got %s", w.Body.String())
		}
	})
	t.Run("an approved payment nobody recorded", func(t *testing.T) {
		f := newFixture(t)
		b := failedCheckout(f, 5)
		f.checkout.searchResults = []mp.Payment{{ID: 72, Status: "approved", ExternalReference: b.ID.String()}}
		confirmedOnHandOver(f, b)

		w := httptest.NewRecorder()
		f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

		if w.Code != http.StatusConflict || problemKind(decode(t, w)) != string(httpx.KindBookingPaid) {
			t.Fatalf("want 409 %s; got %d (%s)", httpx.KindBookingPaid, w.Code, w.Body.String())
		}
		if !slices.Equal(f.reconciler.reconciled, []string{"72"}) {
			t.Errorf("want the approved payment processed; got %v", f.reconciler.reconciled)
		}
	})
}

// S4, finding 4: a release asks MercadoPago before it cancels anything. An
// approved payment whose notice was lost is processed and the booking kept; a
// payment in process keeps it too; and when MercadoPago cannot be asked the
// booking is not released blind. The link is closed first in every case (L18).
func TestReleaseCheckoutAsksMercadoPagoFirst(t *testing.T) {
	cases := []struct {
		name       string
		found      []mp.Payment
		searchErr  error
		status     int
		kind       httpx.Kind
		reconciled []string
	}{
		{name: "approved", found: []mp.Payment{{ID: 81, Status: "approved"}}, status: http.StatusConflict, kind: httpx.KindBookingPaid, reconciled: []string{"81"}},
		{name: "in process", found: []mp.Payment{{ID: 82, Status: "pending", DateCreated: time.Now()}}, status: http.StatusConflict, kind: httpx.KindPaymentInProcess},
		{name: "unreachable", searchErr: errors.New("mercadopago down"), status: http.StatusServiceUnavailable, kind: httpx.KindUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			b := failedCheckout(f, 5)
			for i := range tc.found {
				tc.found[i].ExternalReference = b.ID.String()
			}
			f.checkout.searchResults = tc.found
			f.checkout.searchErr = tc.searchErr
			confirmedOnHandOver(f, b)

			w := httptest.NewRecorder()
			f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

			if w.Code != tc.status || problemKind(decode(t, w)) != string(tc.kind) {
				t.Fatalf("want %d %s; got %d (%s)", tc.status, tc.kind, w.Code, w.Body.String())
			}
			if len(f.store.updated) != 0 || len(f.realtime.published) != 0 {
				t.Errorf("nothing may be released; updated=%d published=%d", len(f.store.updated), len(f.realtime.published))
			}
			if !slices.Equal(f.checkout.expired, []string{"pref-77"}) {
				t.Errorf("the link is closed before MercadoPago is asked, whatever it answers; closed %v", f.checkout.expired)
			}
			if !slices.Equal(f.reconciler.reconciled, tc.reconciled) {
				t.Errorf("reconciled %v, want %v", f.reconciler.reconciled, tc.reconciled)
			}
			if len(f.checkout.searchCallers) != 1 || f.checkout.searchCallers[0] != mustSeller(t, "seller-token") {
				t.Errorf("MercadoPago must be asked once, as the seller; got %+v", f.checkout.searchCallers)
			}
		})
	}
}

// Finding 7: "paid" is only said of a booking that really is paid. A booking
// that is neither pending nor paid (the owner confirmed it without a payment)
// is refused as a plain conflict, not as paid.
func TestReleaseCheckoutSaysPaidOnlyOfAPaidBooking(t *testing.T) {
	f := newFixture(t)
	b := failedCheckout(f, 5)
	b.Status = "confirmed"

	w := httptest.NewRecorder()
	f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

	if w.Code != http.StatusConflict || problemKind(decode(t, w)) == string(httpx.KindBookingPaid) {
		t.Fatalf("want a 409 that does not say paid; got %d (%s)", w.Code, w.Body.String())
	}
}

// L18 major: a release closes the checkout link before it asks MercadoPago,
// so no payment can be approved between the answer and the cancellation, and
// only then cancels the booking.
func TestReleaseCheckoutClosesTheLinkBeforeAskingMercadoPago(t *testing.T) {
	f := newFixture(t)
	b := failedCheckout(f, 5)

	w := httptest.NewRecorder()
	f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204; got %d (%s)", w.Code, w.Body.String())
	}
	if !slices.Equal(f.checkout.calls, []string{"close", "search"}) {
		t.Errorf("want the link closed, then MercadoPago asked; got %v", f.checkout.calls)
	}
	if len(f.store.updated) != 1 || f.store.updated[0].ID != b.ID || f.store.updated[0].Status != "cancelled" {
		t.Errorf("want the booking cancelled after both; got %+v", f.store.updated)
	}
}

// L18 major: a link that will not close is not released over. The visitor is
// told to try again (503), MercadoPago is not asked and the booking keeps its
// slot: cancelling it would leave a payable link on a booking that is gone.
func TestReleaseCheckoutRefusesWhenTheLinkWillNotClose(t *testing.T) {
	f := newFixture(t)
	failedCheckout(f, 5)
	f.checkout.expireErr = errors.New("mercadopago refused")

	w := httptest.NewRecorder()
	f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

	if w.Code != http.StatusServiceUnavailable || problemKind(decode(t, w)) != string(httpx.KindUnavailable) {
		t.Fatalf("want 503 %s; got %d (%s)", httpx.KindUnavailable, w.Code, w.Body.String())
	}
	if len(f.store.updated) != 0 || len(f.realtime.published) != 0 || len(f.waitlist.freed) != 0 {
		t.Errorf("nothing may be released; updated=%d published=%d freed=%d", len(f.store.updated), len(f.realtime.published), len(f.waitlist.freed))
	}
	if len(f.checkout.searches) != 0 {
		t.Errorf("MercadoPago is asked only once the link is closed; searched %v", f.checkout.searches)
	}
}

// L18 major: the cancellation is conditional, and when the booking changed
// since it was read the answer comes from what it is now, never a bare
// conflict over a booking whose link was just closed.
func TestReleaseCheckoutAnswersFromTheCurrentBookingOnARace(t *testing.T) {
	cases := []struct {
		name   string
		now    func(b *bookingstore.Booking)
		status int
		kind   httpx.Kind
	}{
		{name: "paid meanwhile", now: func(b *bookingstore.Booking) {
			b.Status, b.CollectionStatus = "confirmed", bookingstore.CollectionStatusDepositPaid
		}, status: http.StatusConflict, kind: httpx.KindBookingPaid},
		{name: "released meanwhile", now: func(b *bookingstore.Booking) { b.Status = "cancelled" }, status: http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			b := failedCheckout(f, 5)
			current := *b
			tc.now(&current)
			f.store.booking = &current
			f.store.updateErrFor = func(*bookingstore.Booking) error { return data.ErrEditConflict }

			w := httptest.NewRecorder()
			f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

			if w.Code != tc.status {
				t.Fatalf("want %d; got %d (%s)", tc.status, w.Code, w.Body.String())
			}
			if tc.kind != "" && problemKind(decode(t, w)) != string(tc.kind) {
				t.Errorf("want kind %s; got %s", tc.kind, w.Body.String())
			}
			if len(f.realtime.published) != 0 || len(f.waitlist.freed) != 0 || len(f.audit.entries) != 0 {
				t.Errorf("a release that did not write frees nothing; published=%d freed=%d audit=%d",
					len(f.realtime.published), len(f.waitlist.freed), len(f.audit.entries))
			}
		})
	}
}

// L18 minor: every approved payment on the checkout is handed over, one the
// ledger already holds is not replayed, and "paid" is said only once the
// processing took them; a hand-over that failed is a retry-later, not paid.
func TestSettlingACheckoutHandsOverEveryApprovedPayment(t *testing.T) {
	t.Run("two approved, one already recorded", func(t *testing.T) {
		f := newFixture(t)
		b := failedCheckout(f, 5)
		recorded := "91"
		f.payments.ledger = []*paymentstore.Payment{{BookingID: b.ID, Status: "deposit_paid", MPPaymentID: &recorded}}
		f.checkout.searchResults = []mp.Payment{
			{ID: 91, Status: "approved", ExternalReference: b.ID.String()},
			{ID: 92, Status: "approved", ExternalReference: b.ID.String()},
			{ID: 93, Status: "approved", ExternalReference: b.ID.String()},
		}
		confirmedOnHandOver(f, b)

		w := httptest.NewRecorder()
		f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

		if w.Code != http.StatusConflict || problemKind(decode(t, w)) != string(httpx.KindBookingPaid) {
			t.Fatalf("want 409 %s; got %d (%s)", httpx.KindBookingPaid, w.Code, w.Body.String())
		}
		if !slices.Equal(f.reconciler.reconciled, []string{"92", "93"}) {
			t.Errorf("want every approved payment not yet recorded handed over; got %v", f.reconciler.reconciled)
		}
	})
	t.Run("the hand-over failed", func(t *testing.T) {
		f := newFixture(t)
		b := failedCheckout(f, 5)
		f.reconciler.err = errors.New("database down")
		f.checkout.searchResults = []mp.Payment{{ID: 94, Status: "approved", ExternalReference: b.ID.String()}}

		w := httptest.NewRecorder()
		f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

		if w.Code != http.StatusServiceUnavailable || problemKind(decode(t, w)) != string(httpx.KindUnavailable) {
			t.Fatalf("want 503 %s; got %d (%s)", httpx.KindUnavailable, w.Code, w.Body.String())
		}
		if len(f.checkout.reads) != 0 {
			t.Errorf("no checkout link is handed back over an approved payment; reads %v", f.checkout.reads)
		}
	})
}

// L18 minor: a retry on a booking that is paid already sends the visitor to
// it (booking-paid), rather than saying it can no longer be paid.
func TestRetryCheckoutOnAPaidBookingSaysPaid(t *testing.T) {
	f := newFixture(t)
	b := failedCheckout(f, 5)
	b.Status = "confirmed"
	b.CollectionStatus = bookingstore.CollectionStatusDepositPaid

	w := httptest.NewRecorder()
	f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))

	if w.Code != http.StatusConflict || problemKind(decode(t, w)) != string(httpx.KindBookingPaid) {
		t.Fatalf("want 409 %s; got %d (%s)", httpx.KindBookingPaid, w.Code, w.Body.String())
	}
	if len(f.checkout.reads) != 0 || len(f.checkout.searches) != 0 {
		t.Errorf("a paid booking needs no MercadoPago call; reads %v searches %v", f.checkout.reads, f.checkout.searches)
	}
}

// L19 (3): an address already holding its share of slots is refused with its
// own kind, hold-cap, still 429, and a Spanish detail that says what to do.
// The generic rate-limited kind made the storefront say "Hiciste muchos
// intentos seguidos", which is not what happened.
func TestTheAddressHoldCapAnswersItsOwnKind(t *testing.T) {
	for name, call := range map[string]func(f *fixture, complexID, courtID uuid.UUID, w *httptest.ResponseRecorder){
		"a hold": func(f *fixture, complexID, courtID uuid.UUID, w *httptest.ResponseRecorder) {
			f.store.holdErr = bookingstore.ErrHoldLimit
			f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))
		},
		"a booking without a hold": func(f *fixture, complexID, courtID uuid.UUID, w *httptest.ResponseRecorder) {
			f.store.publicCapErr = bookingstore.ErrHoldLimit
			f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			complexID, courtID := preparePublicBooking(f)
			w := httptest.NewRecorder()
			call(f, complexID, courtID, w)

			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("want 429; got %d (%s)", w.Code, w.Body.String())
			}
			body := decode(t, w)
			if problemKind(body) != string(httpx.KindHoldCap) {
				t.Errorf("want kind %q; got %q", httpx.KindHoldCap, problemKind(body))
			}
			if title, _ := body["title"].(string); title == "" {
				t.Error("a Problem must carry a title; the hold-cap refusal had none")
			}
			if detail, _ := body["detail"].(string); !strings.Contains(detail, "desde esta conexión") {
				t.Errorf("want the Spanish detail; got %q", detail)
			}
		})
	}
}

// T12 (3): handing an approved payment over records it for processing; it
// does not mean the booking is paid yet. The release and the retry answer
// booking-paid only once the booking reads paid, and payment-in-process,
// saying the payment is being confirmed, while it is still pending.
func TestAnApprovedPaymentIsCalledPaidOnlyOnceTheBookingIs(t *testing.T) {
	routes := map[string]func(f *fixture, w *httptest.ResponseRecorder){
		"release": func(f *fixture, w *httptest.ResponseRecorder) {
			f.handler.ReleaseCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))
		},
		"retry": func(f *fixture, w *httptest.ResponseRecorder) {
			f.handler.RetryCheckout(w, publicRequest(t, http.MethodPost, "/", checkoutBody))
		},
	}
	for route, call := range routes {
		t.Run(route+": still being confirmed", func(t *testing.T) {
			f := newFixture(t)
			b := failedCheckout(f, 5)
			f.checkout.searchResults = []mp.Payment{{ID: 95, Status: "approved", ExternalReference: b.ID.String()}}

			w := httptest.NewRecorder()
			call(f, w)

			body := decode(t, w)
			if w.Code != http.StatusConflict || problemKind(body) != string(httpx.KindPaymentInProcess) {
				t.Fatalf("want 409 %s; got %d (%s)", httpx.KindPaymentInProcess, w.Code, w.Body.String())
			}
			if body["detail"] != paymentConfirmingMessage {
				t.Errorf("want the being-confirmed detail; got %v", body["detail"])
			}
			if !slices.Equal(f.reconciler.reconciled, []string{"95"}) {
				t.Errorf("want the payment handed over; got %v", f.reconciler.reconciled)
			}
			if len(f.store.updated) != 0 || len(f.checkout.reads) != 0 {
				t.Errorf("nothing released nor handed back; updated=%d reads=%v", len(f.store.updated), f.checkout.reads)
			}
		})
		t.Run(route+": confirmed by the processing", func(t *testing.T) {
			f := newFixture(t)
			b := failedCheckout(f, 5)
			f.checkout.searchResults = []mp.Payment{{ID: 96, Status: "approved", ExternalReference: b.ID.String()}}
			confirmedOnHandOver(f, b)

			w := httptest.NewRecorder()
			call(f, w)

			if w.Code != http.StatusConflict || problemKind(decode(t, w)) != string(httpx.KindBookingPaid) {
				t.Fatalf("want 409 %s; got %d (%s)", httpx.KindBookingPaid, w.Code, w.Body.String())
			}
			if len(f.store.updated) != 0 {
				t.Errorf("a paid booking is not released; updated=%d", len(f.store.updated))
			}
		})
	}
}

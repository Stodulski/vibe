package bookings

import (
	"bytes"
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
	"github.com/stodulski/vibe-server/internal/slots"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/turnstile"
)

// holdDate is the grid day every hold in this file is taken on.
func holdDate() string {
	return time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02")
}

func holdBody(complexID, courtID uuid.UUID, extra string) string {
	return fmt.Sprintf(`{"complex_id":%q,"court_id":%q,"date":%q,"start_time":"18:00","duration_minutes":90%s}`,
		complexID, courtID, holdDate(), extra)
}

// holdBookBody is publicBookBody carrying a hold token.
func holdBookBody(complexID, courtID uuid.UUID, token string) string {
	return strings.TrimSuffix(publicBookBody(complexID, courtID, 90), "}") + `,"hold_token":"` + token + `"}`
}

// problemKind is the last path segment of a Problem's type URI.
func problemKind(body map[string]any) string {
	typ, _ := body["type"].(string)
	return typ[strings.LastIndex(typ, "/")+1:]
}

func TestTakeHold(t *testing.T) {
	t.Run("holds the slot with no contact and hands back its token", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))

		if w.Code != http.StatusCreated {
			t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
		}
		hold, _ := decode(t, w)["hold"].(map[string]any)
		token, _ := hold["token"].(string)
		if len(token) < 32 {
			t.Fatalf("want an opaque token; got %q", token)
		}
		if len(f.store.holds) != 1 {
			t.Fatalf("want one hold stored; got %d", len(f.store.holds))
		}
		held := f.store.holds[0]
		if held.ClientSnapshot != nil || held.HasClient() || held.Status != "pending" || held.CreatedBy != nil {
			t.Errorf("a hold is a pending visitor booking naming nobody; got %+v", held)
		}
		if !bytes.Equal(f.store.holdHashes[0], bookingstore.HashHoldToken(token)) {
			t.Error("only the token's hash may be stored")
		}
		if f.store.holdIPs[0] != "192.0.2.1" || f.store.holdCaps[0] != maxActiveHoldsPerIP {
			t.Errorf("want the caller's address and the cap; got %q %d", f.store.holdIPs[0], f.store.holdCaps[0])
		}
		expiresAt, err := time.Parse(time.RFC3339, hold["expires_at"].(string))
		if err != nil || !expiresAt.Equal(held.CreatedAt.Add(15*time.Minute).Truncate(time.Second)) {
			t.Errorf("expires_at = %v, want created_at + payment expiry (%v)", hold["expires_at"], held.CreatedAt.Add(15*time.Minute))
		}
		if f.checkout.created != 0 || len(f.locks.acquired) != 0 || len(f.audit.entries) != 0 {
			t.Errorf("a hold creates no checkout, no slot lock and no audit entry: checkouts=%d locks=%d audit=%d",
				f.checkout.created, len(f.locks.acquired), len(f.audit.entries))
		}
		if len(f.realtime.published) != 1 {
			t.Errorf("the grid must be told; got %d broadcasts", len(f.realtime.published))
		}
	})

	t.Run("replaces releases the previous hold first", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, `,"replaces":"old-token"`)))

		if w.Code != http.StatusCreated {
			t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.store.releasedHolds) != 1 || !bytes.Equal(f.store.releasedHolds[0], bookingstore.HashHoldToken("old-token")) {
			t.Fatalf("want the previous hold released by its hash; got %v", f.store.releasedHolds)
		}
		if strings.Join(f.store.holdCalls, ",") != "release,insert" {
			t.Errorf("the previous hold must be released before the new one is taken; order %v", f.store.holdCalls)
		}
	})

	t.Run("the per-address cap answers 429", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		f.store.holdErr = bookingstore.ErrHoldLimit

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))

		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("want 429; got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("a slot already held answers 409 slot-unavailable", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		f.store.holdErr = bookingstore.ErrSlotUnavailable

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))

		if w.Code != http.StatusConflict || problemKind(decode(t, w)) != "slot-unavailable" {
			t.Fatalf("want 409 slot-unavailable; got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("a blocked slot is refused like POST /book", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		day, _ := time.Parse("2006-01-02", holdDate())
		f.courts.block(courtID, day, "18:00", "19:30")

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))

		if w.Code != http.StatusConflict || len(f.store.holds) != 0 {
			t.Fatalf("want 409 and no hold; got %d holds=%d (%s)", w.Code, len(f.store.holds), w.Body.String())
		}
	})
}

func TestTakeHoldTurnstile(t *testing.T) {
	t.Run("enforced: a missing token is refused and nothing is held", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		withTurnstile(f, &stubTurnstile{enabled: true}, true)

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))

		if w.Code != http.StatusUnprocessableEntity || bookingFieldError(decode(t, w), "turnstile_token") != "required" {
			t.Fatalf("want 422 turnstile_token required; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.store.holds) != 0 {
			t.Error("a refused token must hold nothing")
		}
	})

	t.Run("enforced: an invalid token is refused", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		withTurnstile(f, &stubTurnstile{enabled: true, err: turnstile.ErrInvalidToken}, true)

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, `,"turnstile_token":"bad"`)))

		if w.Code != http.StatusUnprocessableEntity || bookingFieldError(decode(t, w), "turnstile_token") != "invalid" {
			t.Fatalf("want 422 turnstile_token invalid; got %d (%s)", w.Code, w.Body.String())
		}
	})

	t.Run("not enforced: held without asking Cloudflare", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		verifier := &stubTurnstile{enabled: true, err: turnstile.ErrInvalidToken}
		withTurnstile(f, verifier, false)

		w := httptest.NewRecorder()
		f.handler.TakeHold(w, publicRequest(t, http.MethodPost, "/", holdBody(complexID, courtID, "")))

		if w.Code != http.StatusCreated || len(verifier.calls) != 0 {
			t.Fatalf("want 201 with no Verify call; got %d calls=%d", w.Code, len(verifier.calls))
		}
	})
}

func TestReleaseHoldAlwaysAnswers204(t *testing.T) {
	for _, known := range []bool{true, false} {
		f := newFixture(t)
		f.store.releaseFinds = known

		w := httptest.NewRecorder()
		f.handler.ReleaseHold(w, publicRequest(t, http.MethodPost, "/", ""), "some-token")

		if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
			t.Fatalf("known=%v: want an empty 204; got %d (%s)", known, w.Code, w.Body.String())
		}
		if len(f.store.releasedHolds) != 1 || !bytes.Equal(f.store.releasedHolds[0], bookingstore.HashHoldToken("some-token")) {
			t.Errorf("known=%v: want the release asked by hash", known)
		}
		if got := len(f.realtime.published); (got == 1) != known {
			t.Errorf("known=%v: the grid is told only when a hold went away; got %d broadcasts", known, got)
		}
	}
}

// seedHold gives the fixture's store a live hold for publicBookBody's slot,
// taken age ago, and returns its token.
func seedHold(t *testing.T, f *fixture, complexID, courtID uuid.UUID, age time.Duration) string {
	t.Helper()
	day, err := time.Parse("2006-01-02", time.Now().In(timezone.Argentina).AddDate(0, 0, 7).Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	b := &bookingstore.Booking{
		ID: uuid.New(), ComplexID: complexID, CourtID: courtID, Date: day, StartTime: "18:00",
		DurationMinutes: 90, Price: 750_000, DepositAmount: 225_000, Status: "pending",
		CollectionStatus: bookingstore.CollectionStatusUnpaid, RefundStatus: bookingstore.RefundStatusNone,
		CreatedAt: time.Now().Add(-age),
	}
	b.StartsAt = slots.At(timezone.Day(day), "18:00")
	b.EndsAt = b.StartsAt.Add(90 * time.Minute)
	const token = "hold-token-0123456789abcdefghijklmnopqrstu"
	f.store.held = b
	f.store.heldHash = bookingstore.HashHoldToken(token)
	return token
}

func TestPublicBookConvertsAHold(t *testing.T) {
	t.Run("converts with no Turnstile, the snapshot and the remaining time", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		token := seedHold(t, f, complexID, courtID, 5*time.Minute)
		verifier := &stubTurnstile{enabled: true}
		withTurnstile(f, verifier, true)

		w := httptest.NewRecorder()
		f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", holdBookBody(complexID, courtID, token)))

		if w.Code != http.StatusCreated {
			t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
		}
		if len(verifier.calls) != 0 {
			t.Error("converting a hold must not ask Cloudflare again")
		}
		if len(f.store.converted) != 1 {
			t.Fatalf("want one conversion; got %d", len(f.store.converted))
		}
		snap := f.store.converted[0].ClientSnapshot
		if snap == nil || snap.FirstName != "Ana" || snap.LastName != "Perez" || snap.Phone != "+541100000000" || snap.Email != "ana@example.com" {
			t.Errorf("want the visitor's snapshot written; got %+v", snap)
		}
		if len(f.store.inserted) != 0 || len(f.locks.acquired) != 0 {
			t.Error("a conversion inserts no booking and takes no slot lock")
		}
		got := f.checkout.lastInput.ExpiresIn
		if got > 10*time.Minute || got < 9*time.Minute+50*time.Second {
			t.Errorf("the checkout must expire with the hold (~10m left); got %v", got)
		}
		if f.checkout.lastInput.BookingID != f.store.held.ID {
			t.Error("the checkout is for the held booking")
		}
		if body := decode(t, w); body["token"] != "stub-converted-link-token" {
			t.Errorf("want the converted booking's link token; got %v", body["token"])
		}
		if len(f.audit.entries) != 1 {
			t.Errorf("the conversion is the public_book audit entry; got %d", len(f.audit.entries))
		}
	})

	t.Run("a hold with seconds left gets the floor, not a fresh window", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		token := seedHold(t, f, complexID, courtID, 14*time.Minute+45*time.Second)

		w := httptest.NewRecorder()
		f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", holdBookBody(complexID, courtID, token)))

		if w.Code != http.StatusCreated || f.checkout.lastInput.ExpiresIn != minCheckoutWindow {
			t.Fatalf("want 201 with ExpiresIn %v; got %d %v", minCheckoutWindow, w.Code, f.checkout.lastInput.ExpiresIn)
		}
	})

	t.Run("a blocked phone is refused and nothing is converted", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		token := seedHold(t, f, complexID, courtID, time.Minute)
		f.clients.byPhone = &clientstore.Client{ID: uuid.New(), IsBlocked: true}

		w := httptest.NewRecorder()
		f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", holdBookBody(complexID, courtID, token)))

		if w.Code != http.StatusForbidden || len(f.store.converted) != 0 || f.checkout.created != 0 {
			t.Fatalf("want 403 and nothing converted; got %d converted=%d", w.Code, len(f.store.converted))
		}
	})

	t.Run("a request for other hours than the hold's is 409", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		token := seedHold(t, f, complexID, courtID, time.Minute)
		f.store.held.StartTime = "19:30"

		w := httptest.NewRecorder()
		f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", holdBookBody(complexID, courtID, token)))

		if w.Code != http.StatusConflict || problemKind(decode(t, w)) != "conflict" || len(f.store.converted) != 0 {
			t.Fatalf("want 409 conflict and nothing converted; got %d (%s)", w.Code, w.Body.String())
		}
	})

	expired := []struct {
		name  string
		setup func(f *fixture) string
	}{
		{"an unknown token", func(*fixture) string { return "nobody-holds-this-token" }},
		{"an expired hold", func(f *fixture) string {
			return seedHold(t, f, f.complexes.complex.ID, f.courts.court.ID, 16*time.Minute)
		}},
		{"a hold converted by a concurrent request", func(f *fixture) string {
			token := seedHold(t, f, f.complexes.complex.ID, f.courts.court.ID, time.Minute)
			f.store.convertErr = bookingstore.ErrHoldNotFound
			return token
		}},
	}
	for _, tc := range expired {
		t.Run(tc.name+" is 409 hold-expired", func(t *testing.T) {
			f := newFixture(t)
			complexID, courtID := preparePublicBooking(f)
			token := tc.setup(f)

			w := httptest.NewRecorder()
			f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", holdBookBody(complexID, courtID, token)))

			if w.Code != http.StatusConflict || problemKind(decode(t, w)) != "hold-expired" {
				t.Fatalf("want 409 hold-expired; got %d (%s)", w.Code, w.Body.String())
			}
			if f.checkout.created != 0 {
				t.Error("no checkout may be created for a hold that is gone")
			}
		})
	}

	t.Run("MercadoPago failing cancels the converted booking", func(t *testing.T) {
		f := newFixture(t)
		complexID, courtID := preparePublicBooking(f)
		token := seedHold(t, f, complexID, courtID, time.Minute)
		f.checkout.err = errors.New("mercadopago down")

		w := httptest.NewRecorder()
		f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", holdBookBody(complexID, courtID, token)))

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("want 503; got %d", w.Code)
		}
		if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
			t.Errorf("want the booking cancelled so the slot frees at once; got %v", f.store.updated)
		}
	})
}

func TestTheExpirySweepDeletesUntouchedHolds(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	f.store.expiredHoldComplexes = []uuid.UUID{complexID, complexID}
	nobody := cronCandidate("pending")
	nobody.ClientEmail, nobody.ClientPhone = "", ""
	f.store.expiredPending = []*bookingstore.CronBooking{nobody}

	f.service.ReleaseExpiredPayments(t.Context())

	if len(f.store.expiredHoldWindows) != 1 || f.store.expiredHoldWindows[0] != 15*time.Minute {
		t.Fatalf("want the hold sweep run with the payment expiry; got %v", f.store.expiredHoldWindows)
	}
	told := 0
	for _, id := range f.realtime.published {
		if id == complexID {
			told++
		}
	}
	if told != 1 {
		t.Errorf("want the hold's complex told once; got %d", told)
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Errorf("the booking with no contact is still released; got %v", f.store.updated)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("a booking with no contact gets no notice; got %d", len(f.notify.cancelled))
	}
}

func TestDisconnectingMercadoPagoDeletesTheComplexsHolds(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()

	if err := f.service.ReleaseUnpaidHolds(t.Context(), complexID); err != nil {
		t.Fatalf("ReleaseUnpaidHolds: %v", err)
	}
	if len(f.store.complexHoldsDeleted) != 1 || f.store.complexHoldsDeleted[0] != complexID {
		t.Errorf("want the complex's holds deleted; got %v", f.store.complexHoldsDeleted)
	}
}

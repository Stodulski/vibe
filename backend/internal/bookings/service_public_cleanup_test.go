package bookings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A client closing the tab while MercadoPago is refusing the checkout cancels
// the request context. The booking this request just inserted is cancelled with
// that same context, and a cancelled context fails the write before it reaches
// PostgreSQL: the booking stays pending and the slot stays taken until the
// expiry sweep runs, which is exactly the wait the cancellation exists to avoid.
//
// The slot lock is released on a detached context and so survives the same
// disconnect; the booking's own cancellation must survive it too.
func TestPublicBookCancelsTheBookingEvenWhenTheClientLeftDuringTheMPFailure(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	ctx, disconnect := context.WithCancel(t.Context())
	// The tab closes while MercadoPago is being asked for the checkout, and the
	// checkout then fails, so the handler reaches the cancellation.
	f.checkout.onCreate = disconnect
	f.checkout.err = errCheckoutUnavailable

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/",
		strings.NewReader(publicBookBody(complexID, courtID, 90)))
	req.Header.Set("Content-Type", "application/json")
	f.handler.PublicBook(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when the checkout cannot be created; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.abandonedUpdates != 0 {
		t.Errorf("the booking cancellation was handed a finished context and never reached the store (%d write(s) abandoned)",
			f.store.abandonedUpdates)
	}
	if len(f.store.updated) != 1 || f.store.updated[0].Status != "cancelled" {
		t.Fatalf("want the booking cancelled although the client left; got updates %v", f.store.updated)
	}
	if len(f.locks.released) != 1 {
		t.Errorf("want the slot lock released; released %v, abandoned %v", f.locks.released, f.locks.abandoned)
	}
}

// A checkout frees the slot lock it took, and it names its own holder token when
// it does. The cancel path still frees whichever lock is there; the checkout does
// not get that choice, because its lock may by now belong to someone else.
func TestCheckoutFreesItsSlotLockWithItsOwnHolderToken(t *testing.T) {
	cases := []struct {
		name string
		fail func(f *fixture)
	}{
		{"checkout refused", func(f *fixture) { f.checkout.err = errCheckoutUnavailable }},
		{"booking insert failed", func(f *fixture) { f.store.insertErr = errCheckoutUnavailable }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			complexID, courtID := preparePublicBooking(f)
			tc.fail(f)

			w := httptest.NewRecorder()
			f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

			if len(f.locks.acquiredBy) != 1 {
				t.Fatalf("want the checkout to take one slot lock; got holders %v", f.locks.acquiredBy)
			}
			if len(f.locks.releasedWith) != 1 || f.locks.releasedWith[0] != f.locks.acquiredBy[0] {
				t.Errorf("want the lock freed with the holder token it was taken with (%q); freed with %v",
					f.locks.acquiredBy[0], f.locks.releasedWith)
			}
			if len(f.locks.released) != 1 {
				t.Errorf("want the slot freed; released %v, refused %v", f.locks.released, f.locks.refusedHeld)
			}
		})
	}
}

// A checkout whose lock expired while MercadoPago was slow can find the slot
// already taken over by another checkout. Freeing it from here would free that
// checkout's live lock, and a second buyer could then book a slot somebody is
// paying for. The release names this checkout's holder, which no longer matches,
// so the other checkout's lock stays.
func TestCheckoutLeavesALockTakenOverByAnotherCheckoutAlone(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.checkout.err = errCheckoutUnavailable
	f.checkout.onCreate = f.locks.takeOver

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if len(f.locks.released) != 0 {
		t.Errorf("the other checkout's live lock was freed; released %v", f.locks.released)
	}
	if len(f.locks.refusedHeld) != 1 {
		t.Errorf("want the release refused because the lock names another holder; refused %v, released %v",
			f.locks.refusedHeld, f.locks.released)
	}
}

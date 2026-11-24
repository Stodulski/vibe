package bookings

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// Leaving the confirm page lets the hold go a grace later instead of at once
// (owner decision 2026-11-24, "liberar con un margen de 1 minuto"), so a reload
// or a discarded background tab can take it back.

func TestLeaveHoldAsksForADeferredReleaseAndFreesNothingYet(t *testing.T) {
	f := newFixture(t)
	f.store.releaseFinds = true

	w := httptest.NewRecorder()
	f.handler.LeaveHold(w, publicRequest(t, http.MethodPost, "/", ""), "some-token")

	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("want an empty 204; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.leftHolds) != 1 || !bytes.Equal(f.store.leftHolds[0], bookingstore.HashHoldToken("some-token")) {
		t.Fatalf("want the release asked for by hash; got %v", f.store.leftHolds)
	}
	if len(f.store.releasedHolds) != 0 {
		t.Errorf("leaving must not release the hold at once; got %d releases", len(f.store.releasedHolds))
	}
	if len(f.realtime.published) != 0 || len(f.waitlist.freed) != 0 {
		t.Errorf("nothing is freed yet; got %d broadcasts, %d waitlist hand-overs", len(f.realtime.published), len(f.waitlist.freed))
	}
}

func TestReclaimHold(t *testing.T) {
	t.Run("hands the hold back and answers an empty 204", func(t *testing.T) {
		f := newFixture(t)
		f.store.heldHash = bookingstore.HashHoldToken("some-token")
		f.store.reclaimed = time.Now().Add(-4 * time.Minute)

		w := httptest.NewRecorder()
		f.handler.ReclaimHold(w, publicRequest(t, http.MethodPost, "/", ""), "some-token")

		// The page keeps the deadline it already has: a reclaim never extends
		// the hold, so there is nothing new to tell it.
		if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
			t.Fatalf("want an empty 204; got %d (%s)", w.Code, w.Body.String())
		}
		if len(f.store.reclaims) != 1 || !bytes.Equal(f.store.reclaims[0], f.store.heldHash) {
			t.Fatalf("want the hold reclaimed by hash; got %v", f.store.reclaims)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("want Cache-Control no-store; got %q", w.Header().Get("Cache-Control"))
		}
	})

	t.Run("answers 409 hold-expired when the hold is gone", func(t *testing.T) {
		f := newFixture(t)
		f.store.heldHash = bookingstore.HashHoldToken("some-token")
		f.store.reclaimGone = true

		w := httptest.NewRecorder()
		f.handler.ReclaimHold(w, publicRequest(t, http.MethodPost, "/", ""), "some-token")

		if w.Code != http.StatusConflict {
			t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
		}
		if kind := problemKind(decode(t, w)); kind != "hold-expired" {
			t.Errorf("want kind hold-expired; got %q", kind)
		}
	})

	t.Run("refuses an over-long token without reading the store", func(t *testing.T) {
		f := newFixture(t)

		w := httptest.NewRecorder()
		f.handler.ReclaimHold(w, publicRequest(t, http.MethodPost, "/", ""), string(make([]byte, 257)))

		if w.Code != http.StatusConflict || len(f.store.reclaims) != 0 {
			t.Fatalf("want 409 with no store read; got %d, %d reads", w.Code, len(f.store.reclaims))
		}
	})
}

func TestReleaseLeftHoldsFreesEachSlotAfterTheGrace(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	f.store.leftReleased = []bookingstore.ReleasedHold{
		{ComplexID: complexID, ComplexName: "Vibe", ComplexSlug: "vibe", Day: day},
		{ComplexID: complexID, ComplexName: "Vibe", ComplexSlug: "vibe", Day: day},
		{ComplexID: complexID, ComplexName: "Vibe", ComplexSlug: "vibe", Day: day.AddDate(0, 0, 1)},
	}

	f.service.ReleaseLeftHolds(t.Context())

	if len(f.store.leftGraces) != 1 || f.store.leftGraces[0] != time.Minute {
		t.Fatalf("want the sweep run with a one-minute grace; got %v", f.store.leftGraces)
	}
	if len(f.realtime.published) != 1 || f.realtime.published[0] != complexID {
		t.Errorf("want the complex's grid told once; got %v", f.realtime.published)
	}
	var handed []string
	for _, freed := range f.waitlist.freed {
		if freed.ComplexID == complexID && freed.ComplexSlug == "vibe" {
			handed = append(handed, freed.Day.Format(time.DateOnly))
		}
	}
	if len(handed) != 2 || handed[0] == handed[1] {
		t.Errorf("want each freed day handed to the waitlist once; got %v", handed)
	}
}

func TestChangingTheTimeStillReleasesAtOnce(t *testing.T) {
	// "Cambiar horario" calls the immediate release route, and taking a new
	// hold releases the one it replaces: the visitor chose to give it back.
	f := newFixture(t)
	f.store.releaseFinds = true

	w := httptest.NewRecorder()
	f.handler.ReleaseHold(w, publicRequest(t, http.MethodPost, "/", ""), "some-token")

	if len(f.store.releasedHolds) != 1 || len(f.store.leftHolds) != 0 {
		t.Fatalf("want an immediate release and no deferred one; got %d releases, %d deferred",
			len(f.store.releasedHolds), len(f.store.leftHolds))
	}
	if len(f.realtime.published) != 1 || len(f.waitlist.freed) != 1 {
		t.Errorf("want the slot freed now; got %d broadcasts, %d waitlist hand-overs", len(f.realtime.published), len(f.waitlist.freed))
	}
}

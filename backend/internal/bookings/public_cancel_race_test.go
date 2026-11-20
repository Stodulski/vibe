package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// The link is resolved before the cancellation is written, and something else
// can write the booking in between: the owner cancelling it (and refunding it)
// from the panel, or the payment webhook confirming it. The public cancel used
// to write its whole stale snapshot over that change. It now writes only while
// the row is the one it read, and otherwise answers 409 with nothing done.
func TestPublicCancelDoesNotWriteOverAConcurrentChange(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	row := futureBooking(complexID)
	row.UpdatedAt = time.Now().Add(-time.Minute)

	// What the link resolved: the booking before the owner's cancellation.
	snapshot := *row
	f.linkResolver.booking = &snapshot
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
	f.clients.client = &clientstore.Client{ID: row.ClientID}
	f.courts.court = &courtstore.Court{ID: row.CourtID, Name: "Court 1"}

	// The owner's cancellation commits after the link was read.
	ownerNote := "Cancelado por el complejo"
	row.Status = bookingstore.BookingStatusCancelled
	row.Notes = &ownerNote
	row.UpdatedAt = time.Now()
	f.service.store = &racingStore{stubStore: f.store, row: row}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("a cancellation over a concurrent change must be answered 409; got %d (%s)", w.Code, w.Body.String())
	}
	if row.Notes == nil || *row.Notes != ownerNote {
		t.Errorf("the owner's cancellation was overwritten; notes now %v", row.Notes)
	}
	if len(f.refunds.refunded) != 0 {
		t.Errorf("a refused cancellation must not dispatch a refund; got %v", f.refunds.refunded)
	}
	if len(f.notify.cancelled) != 0 {
		t.Errorf("a refused cancellation must not notify; got %d", len(f.notify.cancelled))
	}
}

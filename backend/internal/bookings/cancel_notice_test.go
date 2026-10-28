package bookings

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// The cancellation itself is committed before any notice is composed, so a read
// that fails while composing the notice must not fail the cancel. But the client
// was told nothing, and before this the skip left no trace at all: no log line
// and no alert. The skip has to be loud, and it must not name the booking in
// Sentry (the booking-link-credential spec keeps resolved booking ids out of it).
func TestAFailedNoticeReadIsReportedInsteadOfSilentlySkipped(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := futureBooking(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", Slug: "vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1"}
	f.clients.err = errDatabase
	sentryEvents := withCapturedSentryEvents(t)

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("the cancellation is committed and must answer 200 even though the notice could not be built; got %d (%s)",
			w.Code, w.Body.String())
	}
	if len(f.notify.cancelled) != 0 {
		t.Fatalf("with the client unreadable there is nothing to name, so no notice may be sent; got %d", len(f.notify.cancelled))
	}

	messages := sentryEvents.messages()
	if _, found := findMessage(messages, "CANCELLATION NOTICE NOT SENT"); !found {
		t.Fatalf("a client left without their cancellation notice must alert; captured %q", messages)
	}
	for _, m := range messages {
		if strings.Contains(m, booking.ID.String()) {
			t.Errorf("the Sentry message must not name the booking: %q", m)
		}
	}
}

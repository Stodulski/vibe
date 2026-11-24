package bookings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// The checkout ticket, the confirmed ticket and the cancel page show a court the
// way the court picker did: roof, surface and lighting. Both public reads of a
// booking therefore carry the court's surface and lighting next to its roof.
func TestPublicBookingReadsCarryTheCourtSurfaceAndLighting(t *testing.T) {
	clay := "clay"
	cases := []struct {
		name               string
		surface            *string
		hasLighting        bool
		wantSurface        any
		wantSurfacePresent bool
	}{
		{name: "surface set, lights on", surface: &clay, hasLighting: true, wantSurface: "clay", wantSurfacePresent: true},
		{name: "surface unknown, no lights", surface: nil, hasLighting: false, wantSurfacePresent: false},
	}

	reads := map[string]func(f *fixture, w *httptest.ResponseRecorder) map[string]any{
		"status": func(f *fixture, w *httptest.ResponseRecorder) map[string]any {
			f.handler.PublicStatus(w, publicRequest(t, http.MethodGet, "/?token=facts", ""))
			booking, _ := decode(t, w)["booking"].(map[string]any)
			return booking
		},
		"cancel-info": func(f *fixture, w *httptest.ResponseRecorder) map[string]any {
			f.handler.PublicCancelInfo(w, publicRequest(t, http.MethodGet, "/?token=facts", ""))
			booking, _ := decode(t, w)["booking"].(map[string]any)
			return booking
		},
	}

	for readName, read := range reads {
		for _, tc := range cases {
			t.Run(readName+"/"+tc.name, func(t *testing.T) {
				f := newFixture(t)
				complexID := uuid.New()
				booking := futureBooking(complexID)
				f.store.booking = booking
				f.linkResolver.booking = booking
				f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
				f.courts.court = &courtstore.Court{
					ID: booking.CourtID, Name: "Cancha 1", Sport: "padel", CourtType: "indoor",
					Surface: tc.surface, HasLighting: tc.hasLighting,
				}

				w := httptest.NewRecorder()
				body := read(f, w)
				if w.Code != http.StatusOK {
					t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
				}
				if body["court_type"] != "indoor" {
					t.Errorf("court_type: want indoor; got %v", body["court_type"])
				}
				if got, ok := body["has_lighting"].(bool); !ok || got != tc.hasLighting {
					t.Errorf("has_lighting: want %v; got %v", tc.hasLighting, body["has_lighting"])
				}
				got, present := body["surface"]
				if present != tc.wantSurfacePresent || (present && got != tc.wantSurface) {
					t.Errorf("surface: want present=%v %v; got present=%v %v", tc.wantSurfacePresent, tc.wantSurface, present, got)
				}
			})
		}
	}
}

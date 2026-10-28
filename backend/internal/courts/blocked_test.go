package courts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/slots"
)

// R3-blocked-slots-silent-truncation: no test in the candidate that introduced
// the default limit=50 exercised the new limit, cursor or metadata envelope at
// all — the truncation itself would have failed a test that did. These three
// pin the fixed contract: unbounded by default, paginated only when the
// caller opts in, and the envelope the opt-in path returns.

type blockedSlotsResponse struct {
	BlockedSlots []*courtstore.BlockedSlot `json:"blocked_slots"`
	Metadata     data.Metadata             `json:"metadata"`
}

// threeBlockedSlots returns three slots already in the (date, start_time, id)
// order GetBlockedSlotsByComplex's real ORDER BY produces, which is what the
// handler's cursor-trim logic assumes of whatever the store hands it.
func threeBlockedSlots(courtID uuid.UUID) []*courtstore.BlockedSlot {
	base := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)
	out := make([]*courtstore.BlockedSlot, 3)
	for i := range out {
		out[i] = &courtstore.BlockedSlot{
			ID:        uuid.New(),
			CourtID:   courtID,
			Date:      base.AddDate(0, 0, i),
			StartTime: "10:00",
			EndTime:   "12:00",
		}
	}
	return out
}

func decodeBlockedSlots(t *testing.T, w *httptest.ResponseRecorder) blockedSlotsResponse {
	t.Helper()
	var resp blockedSlotsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return resp
}

// A caller that sends neither "limit" nor "cursor" gets everything in the
// range, exactly like every consumer that predates H-09.
func TestListBlockedSlotsIsUnboundedWithNoLimitOrCursor(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{blockedSlots: threeBlockedSlots(courtID)}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.ListBlockedSlots(w, ownerRequest(t, http.MethodGet,
		"/?date_from=2026-01-01&date_to=2026-01-31", complexID, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	resp := decodeBlockedSlots(t, w)
	if len(resp.BlockedSlots) != 3 {
		t.Errorf("want all 3 slots with no limit/cursor; got %d", len(resp.BlockedSlots))
	}
	if resp.Metadata.HasMore || resp.Metadata.NextCursor != "" {
		t.Errorf("an unbounded response must report no more pages; got %+v", resp.Metadata)
	}
}

// A caller that opts in with "limit" gets exactly that many rows and a cursor
// that reaches the rest.
func TestListBlockedSlotsAppliesLimitAndCursorWhenRequested(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	slots := threeBlockedSlots(courtID)
	store := &stubStore{blockedSlots: slots}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})

	w := httptest.NewRecorder()
	h.ListBlockedSlots(w, ownerRequest(t, http.MethodGet,
		"/?date_from=2026-01-01&date_to=2026-01-31&limit=2", complexID, nil, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	page1 := decodeBlockedSlots(t, w)
	if len(page1.BlockedSlots) != 2 {
		t.Fatalf("want 2 slots for limit=2; got %d", len(page1.BlockedSlots))
	}
	if !page1.Metadata.HasMore || page1.Metadata.NextCursor == "" {
		t.Fatalf("a truncated page must report more pages and a cursor; got %+v", page1.Metadata)
	}
	if page1.BlockedSlots[0].ID != slots[0].ID || page1.BlockedSlots[1].ID != slots[1].ID {
		t.Errorf("the page must keep the store's (date, start_time, id) order")
	}

	w2 := httptest.NewRecorder()
	h.ListBlockedSlots(w2, ownerRequest(t, http.MethodGet,
		"/?date_from=2026-01-01&date_to=2026-01-31&limit=2&cursor="+page1.Metadata.NextCursor, complexID, nil, ""))
	if w2.Code != http.StatusOK {
		t.Fatalf("want 200 on the second page; got %d (%s)", w2.Code, w2.Body.String())
	}
	page2 := decodeBlockedSlots(t, w2)
	if len(page2.BlockedSlots) != 1 || page2.BlockedSlots[0].ID != slots[2].ID {
		t.Errorf("the cursor must resume exactly where the first page stopped; got %+v", page2.BlockedSlots)
	}
	if page2.Metadata.HasMore {
		t.Errorf("the last page must not claim more pages remain")
	}
}

// recordingBroadcaster stands in for the realtime hub and keeps every complex
// it was asked to announce.
type recordingBroadcaster struct{ complexIDs []uuid.UUID }

func (r *recordingBroadcaster) PublishBookingChanged(complexID uuid.UUID) {
	r.complexIDs = append(r.complexIDs, complexID)
}

// An owner's open calendar refreshes from the realtime hub, and taking hours off
// sale or putting them back changes what that calendar draws. Each committed
// block change has to announce its complex the way a booking change does; a
// block refused for a booking it would overlap commits nothing and announces
// nothing.
func TestBlockChangesAnnounceTheComplexOnTheRealtimeHub(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	date := mustParseDate(t, futureDate())
	court := &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}
	input := BlockSlotInput{Date: date, StartTime: "10:00", EndTime: "11:00", CreatedBy: uuid.New()}

	hub := &recordingBroadcaster{}
	store := &stubStore{court: court}
	svc := NewService(store, &stubBookings{}, &stubComplexes{}, &stubRecorder{}, nil, WithRealtime(hub))

	slot, err := svc.BlockSlot(t.Context(), complexID, Actor{}, courtID, input)
	if err != nil {
		t.Fatalf("BlockSlot: %v", err)
	}
	if len(hub.complexIDs) != 1 || hub.complexIDs[0] != complexID {
		t.Fatalf("a committed block must announce its complex once; got %v", hub.complexIDs)
	}

	store.blockedSlot = slot
	if err := svc.DeleteBlockedSlot(t.Context(), complexID, Actor{}, slot.ID); err != nil {
		t.Fatalf("DeleteBlockedSlot: %v", err)
	}
	if len(hub.complexIDs) != 2 || hub.complexIDs[1] != complexID {
		t.Fatalf("a committed delete must announce its complex too; got %v", hub.complexIDs)
	}

	// A block the owner's own bookings refuse is never written, so there is
	// nothing for the calendar to refresh for.
	refused := &recordingBroadcaster{}
	booked := &stubBookings{booked: []bookingstore.BookedSpan{{
		CourtID:  courtID,
		StartsAt: slots.At(date, "10:30"),
		EndsAt:   slots.At(date, "11:30"),
	}}}
	svc = NewService(&stubStore{court: court}, booked, &stubComplexes{}, &stubRecorder{}, nil, WithRealtime(refused))
	if _, err := svc.BlockSlot(t.Context(), complexID, Actor{}, courtID, input); err == nil {
		t.Fatal("a block overlapping a booking must be refused")
	}
	if len(refused.complexIDs) != 0 {
		t.Errorf("a refused block commits nothing and must announce nothing; got %v", refused.complexIDs)
	}
}

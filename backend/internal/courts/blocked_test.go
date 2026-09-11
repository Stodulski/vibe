package courts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// R3-blocked-slots-silent-truncation: no test in the candidate that introduced
// the default limit=50 exercised the new limit, cursor or metadata envelope at
// all — the truncation itself would have failed a test that did. These three
// pin the fixed contract: unbounded by default, paginated only when the
// caller opts in, and the envelope the opt-in path returns.

type blockedSlotsResponse struct {
	BlockedSlots []*data.BlockedSlot `json:"blocked_slots"`
	Metadata     data.Metadata       `json:"metadata"`
}

// threeBlockedSlots returns three slots already in the (date, start_time, id)
// order GetBlockedSlotsByComplex's real ORDER BY produces, which is what the
// handler's cursor-trim logic assumes of whatever the store hands it.
func threeBlockedSlots(courtID uuid.UUID) []*data.BlockedSlot {
	base := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)
	out := make([]*data.BlockedSlot, 3)
	for i := range out {
		out[i] = &data.BlockedSlot{
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

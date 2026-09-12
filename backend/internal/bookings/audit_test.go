package bookings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/audit"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// auditEntry returns the single entry recorded under action, failing if the
// trail holds none or more than one.
//
// Asserting on the entry rather than on a call count is the whole point: a
// double that only counts cannot tell a correct entry from an empty one, and
// an empty entry is what a handler that records the wrong scope, the wrong
// entity or a nil value would produce.
func auditEntry(t *testing.T, f *fixture, action string) audit.Entry {
	t.Helper()
	var found []audit.Entry
	for _, e := range f.audit.entries {
		if e.Action == action {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %q audit entry; got %d (all: %v)", action, len(found), actions(f))
	}
	return found[0]
}

// actions lists every action recorded, for a failure message that says what
// the trail actually holds.
func actions(f *fixture) []string {
	var names []string
	for _, e := range f.audit.entries {
		names = append(names, e.Action)
	}
	return names
}

// auditValue re-encodes an entry's value the way audit.Recorder.Record does
// and decodes it back, so the assertions below are about the JSON that would
// reach the audit_log row rather than about the struct the handler built.
func auditValue(t *testing.T, e audit.Entry) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(e.NewValue)
	if err != nil {
		t.Fatalf("audit value does not encode: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("audit value is not a JSON object: %v", err)
	}
	return decoded
}

// The two actions an unauthenticated caller can take were the two with no
// trail: everything an owner did was recorded and everything a stranger did
// was not.
//
// Mutation-verified: delete the h.record call in PublicBook and this test
// fails on the missing entry.
func TestPublicBookIsRecordedWithTheClientAsActor(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("want one booking inserted; got %d", len(f.store.inserted))
	}
	booking := f.store.inserted[0]

	entry := auditEntry(t, f, "public_book")
	if entry.EntityType != "booking" {
		t.Errorf("want entity type %q; got %q", "booking", entry.EntityType)
	}
	if entry.EntityID == nil || *entry.EntityID != booking.ID {
		t.Errorf("the entry must name the booking it is about; got %v, want %v", entry.EntityID, booking.ID)
	}
	if entry.ComplexID == nil || *entry.ComplexID != complexID {
		t.Errorf("the entry must be scoped to the complex that owns the booking; got %v", entry.ComplexID)
	}
	// The trail is read scoped to a complex by its owner, so an unscoped entry
	// is one nobody can ever see.
	if entry.UserID != nil {
		t.Errorf("a public booking has no authenticated user; got user id %v", *entry.UserID)
	}
	// The address is the only handle anyone has on this caller.
	if entry.IPAddress != "192.0.2.1" {
		t.Errorf("want the caller's address recorded; got %q", entry.IPAddress)
	}

	value := auditValue(t, entry)
	if value["actor"] != publicActor {
		t.Errorf("a nil user id also means a system action, so the actor must be named; got %v", value["actor"])
	}
	if value["client_id"] != booking.ClientID.String() {
		t.Errorf("want the client the booking was made for; got %v", value["client_id"])
	}
	recorded, ok := value["booking"].(map[string]any)
	if !ok {
		t.Fatalf("the entry must carry the booking; got %v", value["booking"])
	}
	if recorded["status"] != "pending" {
		t.Errorf("want the booking as it committed; got status %v", recorded["status"])
	}
	if recorded["id"] != booking.ID.String() {
		t.Errorf("want the recorded booking to be this one; got %v", recorded["id"])
	}
}

// specs/booking-link-credential's token is the credential that authorizes the
// three public routes. data.Booking keeps it out of anything encoded by
// tagging LinkToken `json:"-"`, exactly as data.Complex's tags keep
// MercadoPago credentials out of the trail — and the audit value hands the
// whole booking over, so that tag is what stands between the token and a
// database row the platform can read.
//
// Mutation-verified: change LinkToken's tag in internal/data/bookings.go to
// `json:"link_token"` and this test fails.
func TestPublicBookAuditEntryCarriesNoLinkToken(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	booking := f.store.inserted[0]
	if booking.LinkToken == "" {
		t.Fatal("the fixture must mint a token, or this test proves nothing")
	}

	entry := auditEntry(t, f, "public_book")
	encoded, err := json.Marshal(entry.NewValue)
	if err != nil {
		t.Fatalf("audit value does not encode: %v", err)
	}
	if strings.Contains(string(encoded), booking.LinkToken) {
		t.Errorf("the booking's access token must never reach the audit trail; got %s", encoded)
	}
}

// A cancellation inside the window returns the deposit; one outside it keeps
// the deposit and never reaches internal/payments at all. That decision is
// made here, out of the complex's own policy, and if this entry did not carry
// it nothing would: the booking row afterwards looks the same either way apart
// from a note in free text.
//
// Mutation-verified: replace WithinRefundWindow/OwesRefund in PublicCancel's
// h.record call with `true, true` and the out-of-window case fails.
func TestPublicCancelRecordsTheRefundWindowDecision(t *testing.T) {
	tests := []struct {
		name             string
		booking          func(uuid.UUID) *data.Booking
		collectionStatus string
		wantWithinWindow bool
		wantOwesRefund   bool
	}{
		{
			name:             "inside the window, on a paid booking",
			booking:          futureBooking,
			collectionStatus: data.CollectionStatusDepositPaid,
			wantWithinWindow: true,
			wantOwesRefund:   true,
		},
		{
			// Two hours from its start, created a day ago: past the grace
			// period and inside the 24-hour window. The shape of a late
			// cancellation, and the one case where the venue keeps the money.
			name:             "outside the window",
			booking:          outOfWindowBooking,
			collectionStatus: data.CollectionStatusDepositPaid,
			wantWithinWindow: false,
			wantOwesRefund:   false,
		},
		{
			name:             "inside the window but never paid",
			booking:          futureBooking,
			collectionStatus: data.CollectionStatusUnpaid,
			wantWithinWindow: true,
			wantOwesRefund:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			complexID := uuid.New()
			booking := tt.booking(complexID)
			booking.CollectionStatus = tt.collectionStatus
			f.store.booking = booking
			f.linkResolver.booking = booking
			f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", CancellationHours: 24}
			f.clients.client = &clientstore.Client{ID: booking.ClientID}
			f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Court 1"}

			w := httptest.NewRecorder()
			f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
			}

			entry := auditEntry(t, f, "public_cancel")
			if entry.EntityID == nil || *entry.EntityID != booking.ID {
				t.Errorf("the entry must name the booking it is about; got %v", entry.EntityID)
			}
			if entry.ComplexID == nil || *entry.ComplexID != complexID {
				t.Errorf("the entry must be scoped to the complex; got %v", entry.ComplexID)
			}
			if entry.UserID != nil {
				t.Errorf("a public cancellation has no authenticated user; got %v", *entry.UserID)
			}
			if entry.IPAddress != "192.0.2.1" {
				t.Errorf("want the caller's address recorded; got %q", entry.IPAddress)
			}

			value := auditValue(t, entry)
			if value["actor"] != publicActor {
				t.Errorf("want the actor named; got %v", value["actor"])
			}
			if value["client_id"] != booking.ClientID.String() {
				t.Errorf("want the client the booking belongs to; got %v", value["client_id"])
			}
			if value["within_refund_window"] != tt.wantWithinWindow {
				t.Errorf("want within_refund_window %v; got %v", tt.wantWithinWindow, value["within_refund_window"])
			}
			if value["owes_refund"] != tt.wantOwesRefund {
				t.Errorf("want owes_refund %v; got %v", tt.wantOwesRefund, value["owes_refund"])
			}
			recorded, ok := value["booking"].(map[string]any)
			if !ok {
				t.Fatalf("the entry must carry the booking; got %v", value["booking"])
			}
			if recorded["status"] != "cancelled" {
				t.Errorf("want the booking as it committed; got status %v", recorded["status"])
			}
		})
	}
}

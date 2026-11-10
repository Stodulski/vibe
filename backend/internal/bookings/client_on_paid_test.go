package bookings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	clientstore "github.com/stodulski/vibe-server/internal/clients/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
)

// Owner rule (2026-11-10): "no deberian de crearse clientes si no hay una
// reserva exitosa". A storefront booking creates no clients row; it carries who
// it is for as a snapshot until its payment is approved, and the transaction
// that records the payment attaches the client (internal/bookings/store
// AttachClientFromSnapshot).

func TestPublicBookCreatesNoClientAndStoresTheSnapshot(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.clients.created) != 0 {
		t.Errorf("a booking that is not paid yet must not create a client; GetOrCreate ran for %v", f.clients.created)
	}
	if len(f.store.inserted) != 1 {
		t.Fatalf("want one booking inserted; got %d", len(f.store.inserted))
	}
	b := f.store.inserted[0]
	if b.ClientID != uuid.Nil {
		t.Errorf("want no client on the pending booking; got %v", b.ClientID)
	}
	want := bookingstore.ClientSnapshot{FirstName: "Ana", LastName: "Perez", Phone: "+541100000000", Email: "ana@example.com"}
	if b.ClientSnapshot == nil || *b.ClientSnapshot != want {
		t.Errorf("want the snapshot %+v on the booking; got %+v", want, b.ClientSnapshot)
	}
}

func TestPublicBookRefusesABlockedPhoneWithoutCreatingAnything(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.clients.byPhone = &clientstore.Client{ID: uuid.New(), ComplexID: complexID, Phone: "+541100000000", IsBlocked: true}

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 for a blocked phone; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.clients.phoneLookups) != 1 || f.clients.phoneLookups[0] != "+541100000000" {
		t.Errorf("want the phone looked up once; got %v", f.clients.phoneLookups)
	}
	if len(f.clients.created) != 0 || len(f.store.inserted) != 0 {
		t.Errorf("a refused booking writes nothing; clients %v, bookings %d", f.clients.created, len(f.store.inserted))
	}
}

func TestPublicBookLeavesAKnownUnblockedPhoneUnattachedUntilPaid(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)
	f.clients.byPhone = &clientstore.Client{ID: uuid.New(), ComplexID: complexID, Phone: "+541100000000"}

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.clients.created) != 0 {
		t.Errorf("want no GetOrCreate at booking time; got %v", f.clients.created)
	}
	if b := f.store.inserted[0]; b.ClientID != uuid.Nil || b.ClientSnapshot == nil {
		t.Errorf("want the booking held under its snapshot until paid; client %v, snapshot %+v", b.ClientID, b.ClientSnapshot)
	}
}

func TestPublicBookAuditEntryRecordsNoClientID(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := preparePublicBooking(f)

	w := httptest.NewRecorder()
	f.handler.PublicBook(w, publicRequest(t, http.MethodPost, "/", publicBookBody(complexID, courtID, 90)))
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}

	value := auditValue(t, auditEntry(t, f, "public_book"))
	if v, ok := value["client_id"]; ok {
		t.Errorf("a booking with no client records no client id; got %v", v)
	}
	recorded, _ := value["booking"].(map[string]any)
	if v, ok := recorded["client_id"]; ok {
		t.Errorf("the recorded booking must not carry the zero client id; got %v", v)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, pii := range []string{"+541100000000", "ana@example.com", "Perez"} {
		if strings.Contains(string(encoded), pii) {
			t.Errorf("the snapshot is personal data and stays out of the trail; found %q in %s", pii, encoded)
		}
	}
}

// snapshotHold is an unpaid storefront booking with no client, the way POST
// /book leaves it.
func snapshotHold(complexID uuid.UUID) *bookingstore.Booking {
	b := futureBooking(complexID)
	b.ClientID = uuid.Nil
	b.Status = "pending"
	b.CollectionStatus = bookingstore.CollectionStatusUnpaid
	b.ClientSnapshot = &bookingstore.ClientSnapshot{FirstName: "Ana", LastName: "Perez", Phone: "+541100000000", Email: "ana@example.com"}
	return b
}

// A visitor cancelling their own unpaid booking is told so, from the snapshot:
// there is no client row to read.
func TestPublicCancelOfABookingWithNoClientNotifiesFromTheSnapshot(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	booking := snapshotHold(complexID)
	f.store.booking = booking
	f.linkResolver.booking = booking
	f.complexes.complex = &complexstore.Complex{ID: complexID, Name: "Vibe", Slug: "vibe", CancellationHours: 24}
	f.courts.court = &courtstore.Court{ID: booking.CourtID, Name: "Cancha 1"}

	w := httptest.NewRecorder()
	f.handler.PublicCancel(w, publicRequest(t, http.MethodPost, "/", `{"token":"test-token"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.notify.cancelled) != 1 {
		t.Fatalf("want the cancellation notice sent; got %d", len(f.notify.cancelled))
	}
	n := f.notify.cancelled[0]
	if n.Email != "ana@example.com" || n.Phone != "+541100000000" || n.ClientName != "Ana Perez" {
		t.Errorf("want the notice addressed from the snapshot; got %q %q %q", n.Email, n.Phone, n.ClientName)
	}
	value := auditValue(t, auditEntry(t, f, "public_cancel"))
	if v, ok := value["client_id"]; ok {
		t.Errorf("a booking with no client records no client id; got %v", v)
	}
}

// The wire never hands out the zero UUID as somebody's id.
func TestTheWireCarriesNoClientIDForABookingWithNoClient(t *testing.T) {
	b := snapshotHold(uuid.New())
	if got := toGenBooking(b).ClientId; got != nil {
		t.Errorf("want client_id null for a booking with no client; got %v", *got)
	}
	b.ClientID = uuid.New()
	if got := toGenBooking(b).ClientId; got == nil || *got != b.ClientID {
		t.Errorf("want the client id on the wire; got %v", got)
	}
}

package complexes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mp"
)

const validComplex = `{"name":"Vibe Palermo","slug":"vibe-palermo","address":"Av. Santa Fe 1234",` +
	`"city":"CABA","province":"Buenos Aires","phone":"+541100000000",` +
	`"deposit_percentage":30,"cancellation_hours":24}`

func TestCreatePersistsAndAudits(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", owner, nil, nil, validComplex))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.inserted == nil {
		t.Fatal("the complex was not persisted")
	}
	if f.store.inserted.OwnerID != owner {
		t.Errorf("the complex was filed under the wrong owner; got %s", f.store.inserted.OwnerID)
	}
	// amenities is NOT NULL in the database and a nil slice binds as NULL, so
	// a nil here is a 500 for every new venue.
	if f.store.inserted.Amenities == nil {
		t.Error("a new complex must carry an empty amenities list, not nil")
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != "create" {
		t.Errorf("the create was not audited; got %+v", f.audit.entries)
	}
}

// A slug is the public URL of the venue, so a collision would send one
// complex's clients to another's booking page.
// The slug is normalised rather than rejected: an owner typing "Vibe Palermo"
// gets vibe-palermo, which is the URL their clients will see.
func TestCreateNormalisesTheSlug(t *testing.T) {
	tests := map[string]string{
		"Vibe Palermo":  "vibe-palermo",
		"VIBE":          "vibe",
		"  vibe  ":      "vibe",
		"vibe!!palermo": "vibepalermo",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			f := newFixture(t)
			body := `{"name":"Vibe","slug":"` + input + `","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`

			w := httptest.NewRecorder()
			f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

			if w.Code != http.StatusCreated {
				t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
			}
			if f.store.inserted.Slug != want {
				t.Errorf("want slug %q; got %q", want, f.store.inserted.Slug)
			}
		})
	}
}

func TestCreateRejectsATakenSlug(t *testing.T) {
	f := newFixture(t)
	f.store.slugTaken = true

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, validComplex))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.inserted != nil {
		t.Error("a complex must not be created on a taken slug")
	}
}

// H-13 / CPX-09: "../admin" slugified to the bare "admin" and was accepted
// with 201 — the venue was created, paid for, and its own storefront URL was
// permanently unreachable, because frontend's router matches its own
// static /admin route before the public /{slug} page ever gets a look.
// slugify runs before validation, so this is the same code path a client
// typing "admin" directly would hit; the traversal-looking input is here to
// document exactly the payload CPX-09 used, not because slugify needs a
// separate defense against it.
func TestCreateRejectsASlugThatCollidesWithAClientRoute(t *testing.T) {
	f := newFixture(t)
	body := `{"name":"Admin Club","slug":"../admin","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.inserted != nil {
		t.Error("a complex must not be created on a slug reserved by the client's own router")
	}
}

func TestCreateEnforcesThePerAccountLimit(t *testing.T) {
	f := newFixture(t)
	f.store.owned = make([]*complexstore.Complex, 4) // the configured maximum

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, validComplex))

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.inserted != nil {
		t.Error("the limit must stop the insert")
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"no name", `{"name":"","slug":"vibe","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`},
		{"empty slug", `{"name":"V","slug":"","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`},
		{"slug of only punctuation", `{"name":"V","slug":"!!!","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`},
		{"deposit over 100", `{"name":"V","slug":"vibe","address":"a","city":"c","province":"p","phone":"1","deposit_percentage":150,"cancellation_hours":24}`},
		{"cancellation over a week", `{"name":"V","slug":"vibe","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":200}`},
		{"malformed email", `{"name":"V","slug":"vibe","address":"a","city":"c","province":"p","phone":"1","email":"not-an-email","cancellation_hours":24}`},
		{"reserved slug", `{"name":"V","slug":"admin","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`},
		{"slug over the length bound", `{"name":"V","slug":"` + strings.Repeat("a", maxSlugLength+1) + `","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)

			w := httptest.NewRecorder()
			f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if f.store.inserted != nil {
				t.Error("an invalid complex must not be persisted")
			}
		})
	}
}

// A cancellation window of zero is not "no window", it is "always refundable":
// pricing.CanRefund returns true unconditionally on cancellationHours <= 0, and
// pricing.LinkLive is `now.Before(expiresAt) || CanRefund(...)`, so a complex at
// zero hands out booking links that never expire.
//
// Zero has its own test rather than a row in TestCreateRejectsInvalidInput
// because it is the only value in that table whose rejection is new, and
// because of how it got written in the first place: `CancellationHours int` is
// a value and not a pointer, so a request that simply omits the field submits
// zero. The "omitted" case below is therefore not a redundant restatement of
// the "explicit" one — it is the path that actually produced the two
// cancellation_hours = 0 rows this project has.
func TestCreateRejectsAZeroCancellationWindow(t *testing.T) {
	bodies := map[string]string{
		"explicit": `{"name":"V","slug":"vibe","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":0}`,
		"omitted":  `{"name":"V","slug":"vibe","address":"a","city":"c","province":"p","phone":"1"}`,
		"negative": `{"name":"V","slug":"vibe","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":-1}`,
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)

			w := httptest.NewRecorder()
			f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if f.store.inserted != nil {
				t.Errorf("a zero cancellation window must not be persisted; got %d hours",
					f.store.inserted.CancellationHours)
			}
			if !strings.Contains(w.Body.String(), "cancellation_hours") {
				t.Errorf("the rejection must name cancellation_hours, or the owner cannot fix it: %s", w.Body.String())
			}
		})
	}
}

// One hour is the floor, and this is what stops the floor drifting upward
// unnoticed. Without it a check of `>= 2` — or a copy-paste of the 24-hour
// default into the bound — would pass every other test in this file.
func TestCreateAcceptsTheSmallestLegalCancellationWindow(t *testing.T) {
	for _, hours := range []int{1, 24, 168} {
		t.Run(fmt.Sprintf("hours=%d", hours), func(t *testing.T) {
			f := newFixture(t)
			body := fmt.Sprintf(
				`{"name":"V","slug":"vibe","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":%d}`,
				hours)

			w := httptest.NewRecorder()
			f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

			if w.Code != http.StatusCreated {
				t.Fatalf("want 201 for a %d-hour window; got %d (%s)", hours, w.Code, w.Body.String())
			}
			if f.store.inserted == nil || f.store.inserted.CancellationHours != hours {
				t.Errorf("the window must reach the store unchanged; got %+v", f.store.inserted)
			}
		})
	}
}

// The same floor on the other write path. Update takes *int, so omission there
// means "leave it alone" and only an explicit 0 is a request to remove the
// window — which is exactly the request that must now fail. Create and Update
// carry two separate v.Check calls, so a test of one proves nothing about the
// other.
func TestUpdateRejectsAZeroCancellationWindow(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), CancellationHours: 24}

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"cancellation_hours":0}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated != nil {
		t.Errorf("a refused update must not reach the store; got %+v", f.store.updated)
	}
	// The in-memory complex is mutated before validation runs, so this asserts
	// what the client can observe: nothing was persisted, and the stored window
	// is still whatever it was.
	if !strings.Contains(w.Body.String(), "cancellation_hours") {
		t.Errorf("the rejection must name cancellation_hours: %s", w.Body.String())
	}
}

// And the floor from above, on Update. 1 is legal; 0 is not.
func TestUpdateAcceptsAOneHourCancellationWindow(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), CancellationHours: 24}

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"cancellation_hours":1}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated == nil || f.store.updated.CancellationHours != 1 {
		t.Errorf("a one-hour window must be persisted; got %+v", f.store.updated)
	}
}

// H-14: a lost update must be refused, not silently applied. The store maps
// two concurrent edits to ErrRecordNotFound (the row's updated_at moved
// between this request's read and its write — see complexstore.Store.Update); this
// pins that the handler in turn tells the caller "conflict, retry" rather
// than either a 500 or a false 200.
func TestUpdateReportsAnEditConflictOnALostUpdate(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo"}
	f.store.updateErr = data.ErrRecordNotFound

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"name":"Renamed Mid-Race"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated != nil {
		t.Errorf("a refused update must not be recorded as applied; got %+v", f.store.updated)
	}
}

// A stale If-Match/version returns data.ErrEditConflict directly; before this
// it fell through to ServerError and answered 500 instead of 409.
func TestUpdateReportsAnEditConflictOnAStaleVersion(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), Name: "Vibe Palermo"}
	f.store.updateErr = data.ErrEditConflict

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"name":"Renamed Under Someone Else","version":3}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated != nil {
		t.Errorf("a refused update must not be recorded as applied; got %+v", f.store.updated)
	}
}

// Deleting a complex cancels every future booking on it, so it is refused
// while any is still live rather than silently taking clients' reservations
// with it.
// H-13: Create already refuses a reserved slug, but a rename through Update
// is the same collision reached by a second write path — an owner renaming
// their venue could talk themselves into it just as easily as one creating a
// new one, and the fix direction only asked for Create originally.
func TestUpdateRejectsASlugThatCollidesWithAClientRoute(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), Slug: "vibe-palermo"}

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"slug":"settings"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated != nil {
		t.Errorf("a rename onto a reserved slug must not reach the store; got %+v", f.store.updated)
	}
}

func TestUpdateRejectsASlugOverTheLengthBound(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New(), Slug: "vibe-palermo"}

	w := httptest.NewRecorder()
	f.handler.Update(w, ownerRequest(t, http.MethodPatch, "/", uuid.New(), complex, nil,
		`{"slug":"`+strings.Repeat("a", maxSlugLength+1)+`"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.updated != nil {
		t.Errorf("an over-length rename must not reach the store; got %+v", f.store.updated)
	}
}

// H-13: SlugAvailable is also the suggestion path — the fix direction is
// explicit that it must not offer a reserved slug either, even though no
// complex has ever claimed it in the database.
func TestSlugAvailableReportsAReservedSlugAsUnavailable(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.SlugAvailable(w, ownerRequest(t, http.MethodGet, "/?slug=admin", uuid.New(), nil, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if available, _ := body["available"].(bool); available {
		t.Errorf("a reserved slug must report available: false; got %v", body)
	}
	// slugsWithPrefix is empty and slugTaken is false, so the only thing
	// that can be making this unavailable is the reserved list — proving the
	// check is not accidentally piggybacking on the "taken in the database"
	// path it shares a map with.
	if f.store.slugTaken {
		t.Fatal("test setup: the database must not report this slug as taken")
	}
}

func TestSlugAvailableReportsAFreeSlugAsAvailable(t *testing.T) {
	f := newFixture(t)

	w := httptest.NewRecorder()
	f.handler.SlugAvailable(w, ownerRequest(t, http.MethodGet, "/?slug=vibe-palermo", uuid.New(), nil, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if available, _ := body["available"].(bool); !available {
		t.Errorf("an unreserved, unclaimed slug must report available: true; got %v", body)
	}
}

func TestDeleteIsRefusedWhileBookingsAreLive(t *testing.T) {
	f := newFixture(t)
	f.bookings.hasActive = true
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.Delete(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(), complex, nil, ""))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.softDeleted != nil {
		t.Error("the complex must not be deleted while bookings are live")
	}
}

func TestDeleteCascadesToCourtsAndFutureBookings(t *testing.T) {
	f := newFixture(t)
	f.store.courtsDeactivated = 3
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.Delete(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(), complex, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.softDeleted == nil || *f.store.softDeleted != complex.ID {
		t.Error("the complex was not deleted")
	}
	if f.bookings.cancelled == nil || *f.bookings.cancelled != complex.ID {
		t.Error("future bookings were left live on a deleted complex")
	}
	if len(f.audit.entries) != 1 || f.audit.entries[0].Action != "delete" {
		t.Errorf("the delete was not audited; got %+v", f.audit.entries)
	}

	// How many courts went down with the venue, in both places an owner or an
	// auditor can read it. The count travels from the one transaction that
	// closed them; a handler that dropped it would answer "complex deleted"
	// and leave nobody able to say what else stopped working.
	var body struct {
		Message           string `json:"message"`
		CourtsDeactivated int    `json:"courts_deactivated"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.CourtsDeactivated != 3 {
		t.Errorf("the response must say how many courts were deactivated; want 3, got %d (%s)",
			body.CourtsDeactivated, w.Body.String())
	}

	outcome, ok := f.audit.entries[0].NewValue.(deletionOutcome)
	if !ok {
		t.Fatalf("the audit entry must carry the cascade outcome; got %T", f.audit.entries[0].NewValue)
	}
	if outcome.CourtsDeactivated != 3 {
		t.Errorf("the audit entry must say how many courts were deactivated; want 3, got %d", outcome.CourtsDeactivated)
	}
}

func TestGetPublicReturnsTheVenueWithItsCourts(t *testing.T) {
	f := newFixture(t)
	complexID, courtID := uuid.New(), uuid.New()
	f.store.complex = &complexstore.Complex{ID: complexID, Slug: "vibe", Name: "Vibe", IsActive: true}
	f.store.schedules = []*complexstore.Schedule{{Day: "monday", OpenTime: "08:00", CloseTime: "22:00"}}
	f.courts.courts = []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	f.handler.GetPublic(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Court 1") {
		t.Errorf("the public page must list the courts; got %s", w.Body.String())
	}
}

func TestGetPublicReportsAnUnknownSlugAsNotFound(t *testing.T) {
	f := newFixture(t)
	f.store.getErr = data.ErrRecordNotFound

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = withSlug(r, "missing")

	w := httptest.NewRecorder()
	f.handler.GetPublic(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestConnectMercadoPagoStoresTheOwnersCredentials(t *testing.T) {
	f := newFixture(t)
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 12345}
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil,
		`{"code":"auth-code","redirect_uri":"https://vibe.test/mp/callback","code_verifier":"verifier"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.mpCredentials == nil {
		t.Fatal("the credentials were not stored")
	}
	if f.store.mpCredentials.access != "AT" || f.store.mpCredentials.refresh != "RT" {
		t.Errorf("the wrong tokens were stored; got %+v", f.store.mpCredentials)
	}
}

// A failed exchange must not leave the complex looking connected.
func TestConnectMercadoPagoStoresNothingOnAFailedExchange(t *testing.T) {
	f := newFixture(t)
	f.payments.err = errors.New("invalid_grant")
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil,
		`{"code":"bad","redirect_uri":"https://vibe.test/mp/callback","code_verifier":"verifier"}`))

	if w.Code == http.StatusOK {
		t.Errorf("a failed exchange must not report success; got %d", w.Code)
	}
	if f.store.mpCredentials != nil {
		t.Error("nothing must be stored when the exchange fails")
	}
}

func TestDisconnectMercadoPagoClearsTheCredentials(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.DisconnectMercadoPago(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(), complex, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.store.mpCleared == nil || *f.store.mpCleared != complex.ID {
		t.Error("the credentials were not cleared")
	}
}

// The presigned URL is what lets the browser upload directly; the key must be
// namespaced to the complex so one owner cannot overwrite another's images.
func TestPresignUploadNamespacesTheKeyToTheComplex(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.PresignUpload(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil,
		`{"type":"logo","content_type":"image/png","file_size":1024}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decode(t, w)
	if body["upload_url"] == nil || body["public_url"] == nil {
		t.Errorf("the response must carry both URLs; got %v", body)
	}
}

// TestPresignUploadSignsTheTypeItValidated pins the type the client declared
// through to the signature and the key's extension.
//
// The handler validated three content types and then hardcoded image/webp into
// both, so a client legitimately declaring image/jpeg received a URL that only
// accepts image/webp — R2 signs the content type, so the upload either failed
// there or succeeded by lying about what it was.
//
// TestPresignUploadNamespacesTheKeyToTheComplex posts image/png and passed the
// whole time, because it only asserted that two URLs came back.
func TestPresignUploadSignsTheTypeItValidated(t *testing.T) {
	for _, c := range []struct{ contentType, ext string }{
		{"image/jpeg", "jpg"},
		{"image/png", "png"},
		{"image/webp", "webp"},
	} {
		t.Run(c.contentType, func(t *testing.T) {
			f := newFixture(t)
			complex := &complexstore.Complex{ID: uuid.New()}

			w := httptest.NewRecorder()
			f.handler.PresignUpload(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil,
				`{"type":"logo","content_type":"`+c.contentType+`","file_size":1024}`))

			if w.Code != http.StatusOK {
				t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
			}
			if f.storage.signedContentType != c.contentType {
				t.Errorf("the signed content type must be the one the client declared and this "+
					"handler validated; want %q, signed %q", c.contentType, f.storage.signedContentType)
			}
			if !strings.HasSuffix(f.storage.signedKey, "."+c.ext) {
				t.Errorf("the key must carry the declared type's extension, or the stored object "+
					"is named as something it is not; want a .%s suffix, got %q", c.ext, f.storage.signedKey)
			}
		})
	}
}

func TestPresignUploadRejectsANonImage(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.PresignUpload(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil,
		`{"type":"logo","content_type":"application/x-sh","file_size":1024}`))

	if w.Code == http.StatusOK {
		t.Errorf("an executable content type must not be presigned; got %d (%s)", w.Code, w.Body.String())
	}
}

// An owner must not be able to delete an object outside their own storage
// namespace by passing an arbitrary URL.
func TestDeleteUploadRejectsAForeignURL(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.DeleteUpload(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(), complex, nil,
		`{"url":"https://someone-else.example/private.png"}`))

	if w.Code == http.StatusOK {
		t.Errorf("a URL outside our storage must not be accepted; got %d", w.Code)
	}
	if len(f.storage.deletedKeys) != 0 {
		t.Errorf("nothing outside our storage may be deleted; got %v", f.storage.deletedKeys)
	}
}

func TestHandlersRequireTheComplexInContext(t *testing.T) {
	f := newFixture(t)

	for name, call := range map[string]http.HandlerFunc{
		"get":        f.handler.Get,
		"update":     f.handler.Update,
		"delete":     f.handler.Delete,
		"schedules":  f.handler.UpdateSchedules,
		"mp status":  f.handler.MercadoPagoStatus,
		"mp connect": f.handler.ConnectMercadoPago,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			call(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

			if w.Code != http.StatusInternalServerError {
				t.Errorf("want 500 with no complex in context; got %d", w.Code)
			}
		})
	}
}

// An owner of one complex must not be able to delete another complex's images.
// The route guard proves the caller owns the complex in the path, never the
// object in the body, and every venue's logo_url and cover_url are public — the
// sitemap enumerates the slugs and the public complex endpoint hands out both
// URLs — so the attacker needs nothing this API has not already given them.
func TestDeleteUploadRejectsAnotherComplexsKey(t *testing.T) {
	f := newFixture(t)
	mine := &complexstore.Complex{ID: uuid.New()}
	theirs := uuid.New()

	w := httptest.NewRecorder()
	f.handler.DeleteUpload(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(), mine, nil,
		fmt.Sprintf(`{"url":"https://cdn.example/complexes/%s/logo/%s.webp"}`, theirs, uuid.New())))

	// 404 rather than 403: a 403 would confirm the object exists and turn this
	// endpoint into a probe for another tenant's storage keys.
	if w.Code != http.StatusNotFound {
		t.Errorf("another tenant's key must answer 404; got %d (%s)", w.Code, w.Body.String())
	}
	// The status alone proves nothing — a handler that answers 404 and deletes
	// anyway is still the same vulnerability. Assert the call never happened.
	if len(f.storage.deletedKeys) != 0 {
		t.Errorf("no object outside the caller's own complex may be deleted; got %v", f.storage.deletedKeys)
	}
}

// H-12 / CPX-04: a key of complexes/{A}/../{B}/logo/x.jpg lexically starts
// with A's prefix, so a plain strings.HasPrefix comparison let it straight
// through to DeleteObject. In production the only thing that stopped the
// cross-tenant delete was the storage SDK normalizing ".." out of the path it
// sent to R2 while the signature was computed over the raw key — an accident
// of URL normalization, not the guard, and it surfaced as a caller-triggered
// 500 rather than a deliberate refusal. This must now be rejected outright,
// before it is ever compared against a prefix.
func TestDeleteUploadRejectsATraversalKey(t *testing.T) {
	f := newFixture(t)
	mine := &complexstore.Complex{ID: uuid.New()}
	theirs := uuid.New()

	w := httptest.NewRecorder()
	f.handler.DeleteUpload(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(), mine, nil,
		fmt.Sprintf(`{"url":"https://cdn.example/complexes/%s/../%s/logo/x.jpg"}`, mine.ID, theirs)))

	if w.Code != http.StatusBadRequest {
		t.Errorf("a key carrying a \"..\" segment must be refused outright with a 4xx, "+
			"not resolved and compared; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.storage.deletedKeys) != 0 {
		t.Errorf("a traversal key must never reach DeleteObject; got %v", f.storage.deletedKeys)
	}
}

// The namespace check must not cost the owner their own images: the key the
// presign step hands out has to survive the delete guard unchanged.
func TestDeleteUploadAcceptsTheComplexsOwnKey(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New()}
	key := fmt.Sprintf("complexes/%s/logo/%s.webp", complex.ID, uuid.New())

	w := httptest.NewRecorder()
	f.handler.DeleteUpload(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(), complex, nil,
		fmt.Sprintf(`{"url":"https://cdn.example/%s"}`, key)))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.storage.deletedKeys) != 1 || f.storage.deletedKeys[0] != key {
		t.Errorf("the complex's own object must be deleted exactly once; got %v", f.storage.deletedKeys)
	}
}

// FINDING 4. A venue that switched itself off kept serving its address, phone,
// courts and prices from the public page, while every booking attempt against
// it answered a bare 404 the page could not explain.
func TestGetPublicIsClosedForADeactivatedComplex(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	f.store.complex = &complexstore.Complex{ID: complexID, Slug: "vibe", Name: "Vibe", IsActive: false}
	f.courts.courts = []*courtstore.Court{{ID: uuid.New(), ComplexID: complexID, Name: "Court 1", IsActive: true}}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	f.handler.GetPublic(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("a deactivated venue must not be browsable; got %d (%s)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Court 1") {
		t.Errorf("nothing about the venue may leak in the refusal; got %s", w.Body.String())
	}
}

// The public page listed every court the complex ever had while the
// availability grid filtered to active ones, so a retired court appeared with a
// price list and no slots to book it in.
func TestGetPublicOmitsInactiveCourts(t *testing.T) {
	f := newFixture(t)
	complexID := uuid.New()
	f.store.complex = &complexstore.Complex{ID: complexID, Slug: "vibe", Name: "Vibe", IsActive: true}
	f.courts.courts = []*courtstore.Court{
		{ID: uuid.New(), ComplexID: complexID, Name: "Court 1", IsActive: true},
		{ID: uuid.New(), ComplexID: complexID, Name: "Retired Court", IsActive: false},
	}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	f.handler.GetPublic(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Retired Court") {
		t.Errorf("a court the grid will not offer must not be listed; got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Court 1") {
		t.Errorf("the active court must still be listed; got %s", w.Body.String())
	}
}

// mp_user_id is the MercadoPago collector id the payment path compares an
// incoming payment against to decide the money reached the right seller.
// Publishing it on an unauthenticated endpoint hands that check's expected
// value to anyone who asks; owner_id names a platform account a booking
// decision never needs.
func TestGetPublicWithholdsTheOwnerAndCollectorIdentifiers(t *testing.T) {
	f := newFixture(t)
	ownerID, mpUserID, token := uuid.New(), "1234567890", "APP_USR-secret"
	complex := complexstore.NewComplexForTest(uuid.New(), &token, nil)
	complex.OwnerID = ownerID
	complex.Slug = "vibe"
	complex.Name = "Vibe"
	complex.IsActive = true
	complex.MPUserID = &mpUserID
	f.store.complex = complex

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	f.handler.GetPublic(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, leaked := range []string{"owner_id", ownerID.String(), "mp_user_id", mpUserID, token} {
		if strings.Contains(body, leaked) {
			t.Errorf("the public page must not carry %q; got %s", leaked, body)
		}
	}
	if !strings.Contains(body, `"payments_enabled":true`) {
		t.Errorf("the page still needs to know the venue can take a payment; got %s", body)
	}
}

// The availability check and the uniqueness constraint have to be asking the
// same question. They were not: SlugExists ignored soft-deleted complexes and
// the constraint does not, so a deleted venue's slug reported free and then
// failed on the insert — a 500 for input the owner could have corrected.
//
// The constraint is the authority, so the handler translates its refusal into
// the same field error the pre-check produces. That also covers the case the
// pre-check can never catch: another request taking the slug in between.
func TestCreateReportsADuplicateSlugAsAFieldErrorNotACrash(t *testing.T) {
	f := newFixture(t)
	f.store.slugTaken = false // the availability check says the slug is free
	f.store.insertErr = complexstore.ErrDuplicateSlug

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, validComplex))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 naming the field; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "slug") {
		t.Errorf("the response must name the slug as the problem; got %s", w.Body.String())
	}
	// The frontend renders from the code, not from prose: a 422 that does not
	// carry it leaves the owner with an empty error.
	if !strings.Contains(w.Body.String(), "slug_taken") {
		t.Errorf("want the slug_taken code the frontend maps; got %s", w.Body.String())
	}
}

// The translation must be to that one constraint, not to every insert failure:
// a database that is down is not the owner's slug being wrong.
func TestCreateStillReportsOtherInsertFailuresAsServerErrors(t *testing.T) {
	f := newFixture(t)
	f.store.insertErr = errors.New("connection refused")

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, validComplex))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 for an unrelated insert failure; got %d (%s)", w.Code, w.Body.String())
	}
}

// The pre-check has to be asked about the slug that will actually be inserted,
// which is the normalised one — otherwise it clears a name nobody is storing.
func TestCreateChecksAvailabilityOfTheNormalisedSlug(t *testing.T) {
	f := newFixture(t)
	body := `{"name":"Vibe","slug":"Vibe Palermo","address":"a","city":"c","province":"p","phone":"1","cancellation_hours":24}`

	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, nil, body))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if len(f.store.slugsChecked) != 1 {
		t.Fatalf("want exactly one availability check; got %v", f.store.slugsChecked)
	}
	if f.store.slugsChecked[0] != "vibe-palermo" {
		t.Errorf("availability was checked for %q; the insert uses %q",
			f.store.slugsChecked[0], f.store.inserted.Slug)
	}
}

// TestMercadoPagoStatusServesTheAppID pins the single source of truth for the
// MercadoPago application id: the client builds the authorize URL from what
// mp/status returns, so a client built with a different id can no longer send
// the seller through an app the API cannot exchange the code with ("the
// client_id does not match the original").
func TestMercadoPagoStatusServesTheAppID(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New(), OwnerID: owner}

	w := httptest.NewRecorder()
	f.handler.MercadoPagoStatus(w, ownerRequest(t, http.MethodGet, "/", owner, complex, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body["app_id"] != "app-123" {
		t.Errorf("app_id = %v, want app-123", body["app_id"])
	}
	if body["connected"] != false {
		t.Errorf("connected = %v, want false for a complex without credentials", body["connected"])
	}
}

package courts

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/slots"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

func TestListReturnsTheComplexCourts(t *testing.T) {
	complexID := uuid.New()
	store := &stubStore{courts: []*courtstore.Court{
		{ID: uuid.New(), ComplexID: complexID, Name: "Court 1"},
		{ID: uuid.New(), ComplexID: complexID, Name: "Court 2"},
	}}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.List(w, ownerRequest(t, http.MethodGet, "/", complexID, nil, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	courts, _ := decode(t, w)["courts"].([]any)
	if len(courts) != 2 {
		t.Errorf("want 2 courts; got %d", len(courts))
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"no name", `{"name":"","sport":"padel","court_type":"indoor"}`},
		{"unknown sport", `{"name":"C1","sport":"cricket","court_type":"indoor"}`},
		{"unknown surface", `{"name":"C1","sport":"padel","court_type":"rooftop"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{}
			h, rec := newTestHandler(store, &stubBookings{}, &stubComplexes{})

			w := httptest.NewRecorder()
			h.Create(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), nil, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if store.inserted != nil {
				t.Error("an invalid court must not be persisted")
			}
			if len(rec.entries) != 0 {
				t.Error("a rejected create must not be audited")
			}
		})
	}
}

func TestCreatePersistsAndAudits(t *testing.T) {
	complexID := uuid.New()
	store := &stubStore{}
	h, rec := newTestHandler(store, &stubBookings{}, &stubComplexes{})

	w := httptest.NewRecorder()
	h.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		nil, `{"name":"Court 1","sport":"padel","court_type":"indoor"}`))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if store.inserted == nil {
		t.Fatal("the court was not persisted")
	}
	if store.inserted.ComplexID != complexID {
		t.Errorf("the court was filed under the wrong complex; got %s", store.inserted.ComplexID)
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "create" || rec.entries[0].EntityType != "court" {
		t.Errorf("the create was not audited correctly; got %+v", rec.entries)
	}
	if rec.entries[0].ComplexID == nil || *rec.entries[0].ComplexID != complexID {
		t.Error("the audit entry must be scoped to the complex")
	}
}

// A court belonging to another complex reads as missing, not forbidden — a 403
// would confirm the id exists.
func TestCourtsOfOtherComplexesAreInvisible(t *testing.T) {
	courtID := uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: uuid.New()}}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})

	for name, call := range map[string]http.HandlerFunc{
		"update": h.Update,
		"delete": h.Delete,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			call(w, ownerRequest(t, http.MethodPut, "/", uuid.New(),
				map[string]string{"courtID": courtID.String()}, `{"name":"Renamed"}`))

			if w.Code != http.StatusNotFound {
				t.Errorf("want 404; got %d", w.Code)
			}
		})
	}
	if store.updated != nil || store.softDeleted != nil {
		t.Error("nothing must be written for another complex's court")
	}
}

// Deleting a court with live bookings would strand the clients who hold them,
// so it is refused rather than cascading.
func TestDeleteIsRefusedWhileBookingsAreLive(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	// The precondition moved with the behaviour (H-02): "this court still has
	// live bookings" is now the store's answer, given inside the same statement
	// that would delete, rather than a separate question the handler asked
	// first. The assertions below are unchanged — a refusal, no delete, no
	// audit entry — only where the setup states the precondition has moved.
	store := &stubStore{
		court:             &courtstore.Court{ID: courtID, ComplexID: complexID},
		hasActiveBookings: true,
	}
	bookings := &stubBookings{hasActive: true}

	h, rec := newTestHandler(store, bookings, &stubComplexes{})
	w := httptest.NewRecorder()
	h.Delete(w, ownerRequest(t, http.MethodDelete, "/", complexID,
		map[string]string{"courtID": courtID.String()}, ""))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if store.softDeleted != nil {
		t.Error("the court must not be deleted while bookings are live")
	}
	if len(rec.entries) != 0 {
		t.Error("a refused delete must not be audited")
	}
}

func TestDeleteSucceedsWithNoLiveBookings(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

	h, rec := newTestHandler(store, &stubBookings{hasActive: false}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.Delete(w, ownerRequest(t, http.MethodDelete, "/", complexID,
		map[string]string{"courtID": courtID.String()}, ""))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if store.softDeleted == nil || *store.softDeleted != courtID {
		t.Error("the court was not deleted")
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "delete" {
		t.Errorf("the delete was not audited; got %+v", rec.entries)
	}
}

// R3-softdelete-conflates-notfound: SoftDelete's WHERE clause used to answer
// every zero-row UPDATE with ErrCourtHasActiveBookings, so a retried or
// concurrent delete of an already-deleted court answered 409 claiming bookings
// that do not exist instead of succeeding as the idempotent no-op it is. The
// store now tells the two apart; this pins the handler side of that: a nil
// error from SoftDelete is always a 200, whether or not anything actually
// changed underneath it.
func TestDeleteOfAnAlreadyDeletedCourtIsIdempotent(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

	h, rec := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.Delete(w, ownerRequest(t, http.MethodDelete, "/", complexID,
		map[string]string{"courtID": courtID.String()}, ""))

	if w.Code != http.StatusOK {
		t.Errorf("a retried delete of an already-deleted court must still answer 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(rec.entries) != 1 {
		t.Errorf("a successful (even idempotent) delete must still be audited; got %+v", rec.entries)
	}
}

// The other zero-row cause SoftDelete now distinguishes: the row disappeared
// between GetByID and the delete itself (a hard-delete race, or a differently
// scoped store), and that is ErrRecordNotFound, not the has-bookings conflict.
func TestDeleteAnswersNotFoundWhenTheCourtDisappearsBeforeTheDelete(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{
		court:         &courtstore.Court{ID: courtID, ComplexID: complexID},
		softDeleteErr: data.ErrRecordNotFound,
	}

	h, rec := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.Delete(w, ownerRequest(t, http.MethodDelete, "/", complexID,
		map[string]string{"courtID": courtID.String()}, ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
	if len(rec.entries) != 0 {
		t.Error("a delete that found nothing must not be audited")
	}
}

// Prices are replaced wholesale, so the old band must be cleared before the new
// one is written or the two would both apply.
func TestUpdatePricesReplacesTheWholeBand(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

	h, rec := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.UpdatePrices(w, ownerRequest(t, http.MethodPut, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"18:00","price":500000}]}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if store.pricesCleared == nil || *store.pricesCleared != courtID {
		t.Error("the previous prices were not cleared before writing the new ones")
	}
	if len(store.insertedPrices) != 1 {
		t.Fatalf("want 1 price written; got %d", len(store.insertedPrices))
	}
	if len(rec.entries) != 1 || rec.entries[0].Action != "update_prices" {
		t.Errorf("the price change was not audited; got %+v", rec.entries)
	}
}

func TestUpdatePricesRejectsInvalidBands(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty list", `{"prices":[]}`},
		{"zero price", `{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"18:00","price":0}]}`},
		{"unknown day", `{"prices":[{"day_type":"caturday","time_from":"08:00","time_to":"18:00","price":1000}]}`},
		{"equal start and end", `{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"08:00","price":1000}]}`},
		{"malformed time", `{"prices":[{"day_type":"monday","time_from":"8am","time_to":"18:00","price":1000}]}`},
		{"negative price", `{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"18:00","price":-500}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexID, courtID := uuid.New(), uuid.New()
			store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

			h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
			w := httptest.NewRecorder()
			h.UpdatePrices(w, ownerRequest(t, http.MethodPut, "/", complexID,
				map[string]string{"courtID": courtID.String()}, tt.body))

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if store.pricesCleared != nil {
				t.Error("an invalid band must not clear the existing prices")
			}
		})
	}
}

// A stale version turns a zero-row ReplacePrices into courts.ErrEditConflict,
// which wraps data.ErrEditConflict; before this it fell through to
// ServerError and answered 500 instead of 409. It answers with its own
// stale-version kind rather than the generic conflict kind.
func TestUpdatePricesReportsAnEditConflictOnAStaleVersion(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{
		court:              &courtstore.Court{ID: courtID, ComplexID: complexID},
		replacePricesErr:   data.ErrRecordNotFound,
		replaceFailedIndex: -1,
	}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.UpdatePrices(w, ownerRequest(t, http.MethodPut, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"version":3,"prices":[{"day_type":"monday","time_from":"08:00","time_to":"18:00","price":500000}]}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if len(store.insertedPrices) != 0 {
		t.Error("a refused update must not clear or write prices")
	}

	var body httpx.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if want := httpx.KindStaleVersion.URI(); body.Type != want {
		t.Errorf("want type %q; got %q", want, body.Type)
	}
}

// A complex may close after midnight (complex_schedules reads close_time <=
// open_time as "closes after midnight" since complex_schedules_open_window_not_empty), and a court's
// price band for that closing window is legitimately "18:00" to "08:00" — the
// bug this guards against rejected every such band with a 422 and a silent
// dialog, because the handler used to require time_from < time_to.
func TestUpdatePricesAcceptsABandCrossingMidnight(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.UpdatePrices(w, ownerRequest(t, http.MethodPut, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"prices":[{"day_type":"thursday","time_from":"08:00","time_to":"01:30","price":120000}]}`))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if len(store.insertedPrices) != 1 {
		t.Fatalf("want 1 price written; got %d", len(store.insertedPrices))
	}
}

// Two bands for the same day_type that share any minute must be refused with
// a field error naming the later offending index, never a 500 — the DB's
// exclusion constraint would otherwise surface as translateCourtWrite passing
// a raw pgconn error straight to ServerError.
func TestUpdatePricesRejectsOverlappingBandsForTheSameDay(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.UpdatePrices(w, ownerRequest(t, http.MethodPut, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"prices":[
			{"day_type":"monday","time_from":"08:00","time_to":"18:00","price":500000},
			{"day_type":"monday","time_from":"12:00","time_to":"20:00","price":600000}
		]}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "prices[1].time_from") {
		t.Errorf("want the error keyed on the offending index prices[1]; got %s", w.Body.String())
	}
	if store.pricesCleared != nil {
		t.Error("an overlapping band must not clear the existing prices")
	}
}

// Blocking a slot in the past cannot do anything useful and would corrupt the
// availability grid for a day already played.
func TestBlockSlotRejectsAPastDate(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.BlockSlot(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"date":"2020-01-01","start_time":"18:00","end_time":"19:30"}`))

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if store.insertedBlocked != nil {
		t.Error("a past slot must not be blocked")
	}
}

// A block must not be able to erase a slot somebody already booked.
func TestBlockSlotRefusesToOverlapABooking(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}
	day := mustParseDate(t, futureDate())
	bookings := &stubBookings{booked: []bookingstore.BookedSpan{{
		StartsAt: slots.At(day, "18:00"), EndsAt: slots.At(day, "19:30"),
	}}}

	h, _ := newTestHandler(store, bookings, &stubComplexes{})
	w := httptest.NewRecorder()
	h.BlockSlot(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"date":"`+futureDate()+`","start_time":"18:30","end_time":"20:00"}`))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if store.insertedBlocked != nil {
		t.Error("a slot overlapping a booking must not be blocked")
	}
}

// Adjacent is not overlapping: a block starting exactly when a booking ends is
// allowed.
func TestBlockSlotAllowsAnAdjacentRange(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}
	day := mustParseDate(t, futureDate())
	bookings := &stubBookings{booked: []bookingstore.BookedSpan{{
		StartsAt: slots.At(day, "18:00"), EndsAt: slots.At(day, "19:30"),
	}}}

	h, _ := newTestHandler(store, bookings, &stubComplexes{})
	w := httptest.NewRecorder()
	h.BlockSlot(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"date":"`+futureDate()+`","start_time":"19:30","end_time":"21:00"}`))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if store.insertedBlocked == nil {
		t.Error("an adjacent range should be blockable")
	}
}

func TestBlockSlotReportsAnAlreadyBlockedRange(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{
		court:    &courtstore.Court{ID: courtID, ComplexID: complexID},
		blockErr: courtstore.ErrSlotAlreadyBlocked,
	}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.BlockSlot(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"date":"`+futureDate()+`","start_time":"18:00","end_time":"19:30"}`))

	if w.Code != http.StatusConflict {
		t.Errorf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
}

// The pre-check above this handler's store call cannot see a booking that
// commits while the request is in flight; InsertBlockedSlot asks again under
// the court-day lock and answers ErrSlotHasBooking. The owner must read the
// same sentence either way — which of the two checks caught it is not
// something they can act on.
func TestBlockSlotReportsARangeARaceSold(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{
		court:    &courtstore.Court{ID: courtID, ComplexID: complexID},
		blockErr: courtstore.ErrSlotHasBooking,
	}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.BlockSlot(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"date":"`+futureDate()+`","start_time":"18:00","end_time":"19:30"}`))

	if w.Code != http.StatusConflict {
		t.Fatalf("want 409; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), blockedSlotHasBookingMessage) {
		t.Errorf("the refusal must say the hours are booked, not that they are already blocked; got %s", w.Body.String())
	}
}

func TestBlockSlotRecordsWhoBlockedIt(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID}}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.BlockSlot(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"date":"`+futureDate()+`","start_time":"18:00","end_time":"19:30","reason":"maintenance"}`))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	if store.insertedBlocked.CreatedBy == nil {
		t.Error("the blocking user must be recorded")
	}
	if store.insertedBlocked.Reason == nil || *store.insertedBlocked.Reason != "maintenance" {
		t.Error("the reason was not stored")
	}
}

// The audit trail is the only record of who took a court off sale, and it has
// to say which court. The blocked slot carries the court's name for exactly
// that, but the handler used to fill it in on the line *after* the entry was
// recorded, so every audit row for a block landed with the name missing — and
// the row carries no court id either, so the trail could not name the court at
// all.
func TestBlockSlotAuditsTheCourtName(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	store := &stubStore{court: &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Cancha Bloqueo"}}

	h, table, runAudits := newTestHandlerWithAuditTrail(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.BlockSlot(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"courtID": courtID.String()},
		`{"date":"`+futureDate()+`","start_time":"18:00","end_time":"19:30","reason":"maintenance"}`))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}

	runAudits()
	if len(table.rows) != 1 {
		t.Fatalf("want 1 audit row; got %d", len(table.rows))
	}

	row := table.rows[0]
	if row.action != "create" || row.entityType != "blocked_slot" {
		t.Errorf("want create/blocked_slot; got %s/%s", row.action, row.entityType)
	}
	if !strings.Contains(string(row.newJSON), `"court_name":"Cancha Bloqueo"`) {
		t.Errorf("the audited slot must name its court; got %s", row.newJSON)
	}
}

func TestDeleteBlockedSlotHidesOtherComplexes(t *testing.T) {
	slotID, courtID := uuid.New(), uuid.New()
	store := &stubStore{
		blockedSlot: &courtstore.BlockedSlot{ID: slotID, CourtID: courtID},
		court:       &courtstore.Court{ID: courtID, ComplexID: uuid.New()},
	}

	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})
	w := httptest.NewRecorder()
	h.DeleteBlockedSlot(w, ownerRequest(t, http.MethodDelete, "/", uuid.New(),
		map[string]string{"slotID": slotID.String()}, ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d (%s)", w.Code, w.Body.String())
	}
	if store.deletedBlocked != nil {
		t.Error("another complex's blocked slot must not be deletable")
	}
}

func TestHandlersRequireTheComplexInContext(t *testing.T) {
	h, _ := newTestHandler(&stubStore{}, &stubBookings{}, &stubComplexes{})

	for name, call := range map[string]http.HandlerFunc{
		"list":           h.List,
		"create":         h.Create,
		"update":         h.Update,
		"delete":         h.Delete,
		"prices":         h.UpdatePrices,
		"block":          h.BlockSlot,
		"blocked list":   h.ListBlockedSlots,
		"blocked delete": h.DeleteBlockedSlot,
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

func TestStoreFailuresBecomeServerErrors(t *testing.T) {
	store := &stubStore{getErr: errors.New("connection refused")}
	h, _ := newTestHandler(store, &stubBookings{}, &stubComplexes{})

	w := httptest.NewRecorder()
	h.List(w, ownerRequest(t, http.MethodGet, "/", uuid.New(), nil, ""))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500; got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "connection refused") {
		t.Error("the underlying cause must not reach the client")
	}
}

func TestAvailabilityIsPublicAndBuildsTheGrid(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	date := futureDate()

	complexes := &stubComplexes{
		complex:   &complexstore.Complex{ID: complexID, Name: "Vibe", Slug: "vibe", IsActive: true},
		schedules: openEveryDay(),
	}
	store := &stubStore{
		courts: []*courtstore.Court{{ID: courtID, ComplexID: complexID, Name: "Court 1", IsActive: true}},
		prices: pricedEveryDay(courtID),
	}

	h, _ := newTestHandler(store, &stubBookings{}, complexes)

	// No session and no complex in context: this route is reached by clients
	// who have no account.
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+date, nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}

	grid, _ := decode(t, w)["availability"].(map[string]any)
	if grid["is_open"] != true {
		t.Errorf("the complex is open every day in this fixture; got is_open=%v", grid["is_open"])
	}

	courtsOut, _ := grid["courts"].([]any)
	if len(courtsOut) != 1 {
		t.Fatalf("want one court in the grid; got %d (%s)", len(courtsOut), w.Body.String())
	}
	first, _ := courtsOut[0].(map[string]any)
	slotsOut, _ := first["slots"].([]any)
	if len(slotsOut) == 0 {
		t.Error("the day is open and priced, so the grid must have slots")
	}
}

func TestAvailabilityReportsAnUnknownComplexAsNotFound(t *testing.T) {
	complexes := &stubComplexes{err: data.ErrRecordNotFound}
	h, _ := newTestHandler(&stubStore{}, &stubBookings{}, complexes)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+futureDate(), nil)
	r = withSlug(r, "missing")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404; got %d", w.Code)
	}
}

func TestAvailabilityRejectsAMalformedDate(t *testing.T) {
	complexes := &stubComplexes{complex: &complexstore.Complex{ID: uuid.New(), Slug: "vibe", IsActive: true}}
	h, _ := newTestHandler(&stubStore{}, &stubBookings{}, complexes)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date=next-tuesday", nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code == http.StatusOK {
		t.Errorf("an unparseable date must not produce a grid; got %d (%s)", w.Code, w.Body.String())
	}
}

// A deactivated court is not bookable, so it must not appear in the grid at
// all — showing it with an empty slot list would read as "fully booked".
func TestAvailabilityOmitsInactiveCourts(t *testing.T) {
	complexID := uuid.New()
	complexes := &stubComplexes{
		complex:   &complexstore.Complex{ID: complexID, Slug: "vibe", IsActive: true},
		schedules: openEveryDay(),
	}
	store := &stubStore{courts: []*courtstore.Court{
		{ID: uuid.New(), ComplexID: complexID, Name: "Retired", IsActive: false},
	}}

	h, _ := newTestHandler(store, &stubBookings{}, complexes)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?date="+futureDate(), nil)
	r = withSlug(r, "vibe")

	w := httptest.NewRecorder()
	h.Availability(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	grid, _ := decode(t, w)["availability"].(map[string]any)
	courtsOut, _ := grid["courts"].([]any)
	if len(courtsOut) != 0 {
		t.Errorf("an inactive court must not appear in the grid; got %v", courtsOut)
	}
}

// FINDING 6. The past-date guard on blocking a slot used to truncate against
// the UTC epoch, so for the three hours between 21:00 and midnight in Buenos
// Aires the server had already rolled over to tomorrow and refused an owner
// blocking off the rest of tonight — the one date decision in the product not
// taken in Argentina time.
func TestBlockSlotDateIsJudgedOnTheArgentineCalendar(t *testing.T) {
	// 02:00 UTC on the 15th is 23:00 on the 14th in Buenos Aires.
	lateAtNight := time.Date(2026, 3, 15, 2, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		date string
		want bool
	}{
		{"the rest of tonight", "2026-03-14", true},
		{"tomorrow", "2026-03-15", true},
		{"yesterday", "2026-03-13", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, err := time.Parse("2006-01-02", tt.date)
			if err != nil {
				t.Fatalf("bad fixture: %v", err)
			}
			if got := onOrAfterToday(date, lateAtNight); got != tt.want {
				t.Errorf("at %s (23:00 on the 14th in Buenos Aires), %s bookable = %v; want %v",
					lateAtNight.Format(time.RFC3339), tt.date, got, tt.want)
			}
		})
	}
}

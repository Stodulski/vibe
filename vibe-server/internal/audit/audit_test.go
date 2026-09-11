package audit

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type recordedCall struct {
	userID, complexID, entityID *uuid.UUID
	action, entityType, ipAddr  string
	oldJSON, newJSON            []byte
}

type stubStore struct {
	calls []recordedCall
	err   error
}

func (s *stubStore) InsertAuditLog(_ context.Context, userID, complexID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldJSON, newJSON []byte, ipAddr string) error {
	s.calls = append(s.calls, recordedCall{
		userID: userID, complexID: complexID, entityID: entityID,
		action: action, entityType: entityType, ipAddr: ipAddr,
		oldJSON: oldJSON, newJSON: newJSON,
	})
	return s.err
}

// runInline stands in for the application's background helper, running the task
// synchronously so tests observe the write without waiting.
func runInline(fn func()) { fn() }

func TestRecordPassesEveryFieldThrough(t *testing.T) {
	store := &stubStore{}
	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), runInline)

	userID, complexID, entityID := uuid.New(), uuid.New(), uuid.New()
	rec.Record(Entry{
		UserID:     &userID,
		ComplexID:  &complexID,
		Action:     "cancel",
		EntityType: "booking",
		EntityID:   &entityID,
		OldValue:   map[string]any{"status": "confirmed"},
		NewValue:   map[string]any{"status": "cancelled"},
		IPAddress:  "203.0.113.7",
	})

	if len(store.calls) != 1 {
		t.Fatalf("want 1 insert; got %d", len(store.calls))
	}

	got := store.calls[0]
	if got.userID == nil || *got.userID != userID {
		t.Errorf("user id was not passed through; got %v", got.userID)
	}
	if got.complexID == nil || *got.complexID != complexID {
		t.Errorf("complex id was not passed through; got %v", got.complexID)
	}
	if got.entityID == nil || *got.entityID != entityID {
		t.Errorf("entity id was not passed through; got %v", got.entityID)
	}
	if got.action != "cancel" || got.entityType != "booking" {
		t.Errorf("want cancel/booking; got %s/%s", got.action, got.entityType)
	}
	if got.ipAddr != "203.0.113.7" {
		t.Errorf("want ip 203.0.113.7; got %s", got.ipAddr)
	}
	if string(got.oldJSON) != `{"status":"confirmed"}` {
		t.Errorf("the old value must reach the store encoded; got %q", got.oldJSON)
	}
	if string(got.newJSON) != `{"status":"cancelled"}` {
		t.Errorf("the new value must reach the store encoded; got %q", got.newJSON)
	}
}

// An absent value is not the JSON literal `null`: it must stay nil so the store
// writes SQL NULL, which is how "a creation has no old value" is distinguished
// from "the old value was recorded and it was null".
func TestRecordLeavesAnAbsentValueNil(t *testing.T) {
	store := &stubStore{}
	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), runInline)

	rec.Record(Entry{Action: "create", EntityType: "court", NewValue: map[string]any{"name": "Cancha 1"}})

	got := store.calls[0]
	if got.oldJSON != nil {
		t.Errorf("a creation has no old value; got %q", got.oldJSON)
	}
	if string(got.newJSON) != `{"name":"Cancha 1"}` {
		t.Errorf("want the encoded new value; got %q", got.newJSON)
	}
}

// The whole point of the encoding living in Record: the caller keeps its struct
// and may write to it the moment Record returns. What lands in the audit trail
// must be the object as it was when Record was called, and — more importantly —
// the background write must never read that struct at all.
//
// This is asserted without the race detector, with a runner that defers the
// task, so it fails deterministically rather than only when the scheduler
// cooperates.
func TestRecordSnapshotsTheValueBeforeSchedulingTheWrite(t *testing.T) {
	store := &stubStore{}
	var scheduled []func()
	deferRun := func(fn func()) { scheduled = append(scheduled, fn) }

	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), deferRun)

	slot := &struct {
		Reason    string `json:"reason"`
		CourtName string `json:"court_name"`
	}{Reason: "maintenance", CourtName: "Cancha 1"}

	rec.Record(Entry{Action: "create", EntityType: "blocked_slot", NewValue: slot})

	// The caller goes on owning its struct.
	slot.CourtName = "mutated after the fact"

	scheduled[0]()

	const want = `{"reason":"maintenance","court_name":"Cancha 1"}`
	if got := string(store.calls[0].newJSON); got != want {
		t.Errorf("the entry must hold the value as it was at Record time\n want %s\n  got %s", want, got)
	}
}

// The same guarantee under the race detector, with the write on a real
// goroutine and the caller mutating concurrently — which is exactly the shape
// every handler that calls Record has.
func TestRecordDoesNotRaceWithTheCallersLaterWrites(t *testing.T) {
	store := &stubStore{}
	var wg sync.WaitGroup
	goRun := func(fn func()) {
		wg.Add(1)
		go func() { defer wg.Done(); fn() }()
	}

	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), goRun)

	slot := &struct {
		Reason    string `json:"reason"`
		CourtName string `json:"court_name"`
	}{Reason: "maintenance"}

	rec.Record(Entry{Action: "create", EntityType: "blocked_slot", NewValue: slot})
	slot.CourtName = "Cancha 1"

	wg.Wait()
	if len(store.calls) != 1 {
		t.Fatalf("want 1 insert; got %d", len(store.calls))
	}
}

// unencodable fails json.Marshal, which is now the caller's problem rather than
// the background write's.
type unencodable struct{}

func (unencodable) MarshalJSON() ([]byte, error) { return nil, errors.New("cannot encode this") }

// Encoding moved onto the request goroutine, so it can now fail in front of the
// caller. The contract does not change: the operation being described already
// happened, so nothing is returned and nothing is failed. Only the payload is
// dropped — the entry itself is still written, because losing the actor is a
// worse audit failure than losing the diff.
func TestRecordDropsAnUnencodableValueButStillWritesTheEntry(t *testing.T) {
	var logBuf bytes.Buffer
	store := &stubStore{}
	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&logBuf, nil)), runInline)

	userID := uuid.New()
	rec.Record(Entry{
		UserID:     &userID,
		Action:     "update",
		EntityType: "court",
		NewValue:   unencodable{},
	})

	if len(store.calls) != 1 {
		t.Fatalf("the entry must still be recorded; got %d inserts", len(store.calls))
	}
	got := store.calls[0]
	if got.newJSON != nil {
		t.Errorf("the unencodable value must be dropped; got %q", got.newJSON)
	}
	if got.userID == nil || *got.userID != userID || got.action != "update" || got.entityType != "court" {
		t.Error("who did what must survive a value that could not be encoded")
	}

	logged := logBuf.String()
	if !strings.Contains(logged, "audit value could not be encoded") {
		t.Errorf("the encoding failure must be logged; got %q", logged)
	}
	if !strings.Contains(logged, "new_value") {
		t.Errorf("the log must name the field that was dropped; got %q", logged)
	}
}

// A system action has no acting user and no complex; both must stay nil rather
// than being substituted with a zero UUID.
func TestRecordAllowsNilActorAndScope(t *testing.T) {
	store := &stubStore{}
	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), runInline)

	rec.Record(Entry{Action: "auto_cancel", EntityType: "booking"})

	got := store.calls[0]
	if got.userID != nil {
		t.Errorf("want a nil user for a system action; got %v", *got.userID)
	}
	if got.complexID != nil {
		t.Errorf("want a nil complex for a platform action; got %v", *got.complexID)
	}
}

// Auditing describes something that already happened, so a failed write is
// logged and swallowed — it must never surface to the caller.
func TestRecordLogsButSwallowsStoreFailure(t *testing.T) {
	var logBuf bytes.Buffer
	store := &stubStore{err: errors.New("connection refused")}
	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&logBuf, nil)), runInline)

	rec.Record(Entry{Action: "create", EntityType: "court"})

	logged := logBuf.String()
	if !strings.Contains(logged, "failed to insert audit log") {
		t.Errorf("the failure must be logged; got %q", logged)
	}
	if !strings.Contains(logged, "create") || !strings.Contains(logged, "court") {
		t.Errorf("the log must identify the action and entity; got %q", logged)
	}
}

// The write is handed to the runner rather than executed inline, so it stays
// off the request path.
func TestRecordDelegatesToTheRunner(t *testing.T) {
	store := &stubStore{}
	var scheduled []func()
	deferRun := func(fn func()) { scheduled = append(scheduled, fn) }

	rec := NewRecorder(store, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), deferRun)
	rec.Record(Entry{Action: "create", EntityType: "court"})

	if len(store.calls) != 0 {
		t.Fatal("Record must not touch the store on the calling goroutine")
	}
	if len(scheduled) != 1 {
		t.Fatalf("want 1 scheduled task; got %d", len(scheduled))
	}

	scheduled[0]()
	if len(store.calls) != 1 {
		t.Errorf("want the insert once the task runs; got %d calls", len(store.calls))
	}
}

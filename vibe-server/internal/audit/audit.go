// Package audit records who changed what, when, and from where.
//
// It lived inside the admin handlers, but the bookings, courts and complexes
// modules all write to it: auditing is a cross-cutting service, not an admin
// feature. Recorder knows nothing about HTTP — callers supply the acting user
// and the client address.
//
// The package also serves the trail back to the tenant it belongs to (see
// handler.go), because a record of who changed a venue's data that only the
// platform can read is only half an audit trail.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/data"
)

// writeTimeout bounds a single audit insert. The write happens off the request
// path, so it must not be able to hold a goroutine open indefinitely.
const writeTimeout = 5 * time.Second

// Store is the persistence this package needs. It is declared here, by the
// consumer, so that adding a method to the admin store does not widen what
// auditing depends on.
//
// oldVal and newVal arrive already encoded. The store persists bytes it is
// handed rather than objects it must encode itself, because encoding is the one
// step that reads the caller's live struct and it therefore cannot happen on the
// background goroutine — see Record.
type Store interface {
	InsertAuditLog(ctx context.Context, userID, complexID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldVal, newVal []byte, ipAddr string) error
}

// Entry is one recorded action. It is a struct rather than a parameter list
// because the underlying insert takes nine arguments, five of which are
// pointers or `any` — positional calls at that width are unreadable and easy to
// transpose silently.
type Entry struct {
	// UserID is the authenticated actor, or nil for a system action.
	UserID *uuid.UUID
	// ComplexID scopes the entry to a complex, or nil for platform-wide actions.
	ComplexID *uuid.UUID
	// Action is the verb, e.g. "create", "cancel", "mp_disconnect".
	Action string
	// EntityType is the kind of record affected, e.g. "booking", "court".
	EntityType string
	// EntityID identifies the affected record.
	EntityID *uuid.UUID
	// OldValue and NewValue are JSON-encoded by Record, on the caller's own
	// goroutine, for later diffing. Either may be nil: a creation has no old
	// value, a deletion has no new one.
	//
	// They are `any` because callers hand over whatever domain struct they were
	// already holding. Record turns them into bytes immediately, so the caller
	// keeps sole ownership of that struct and may go on mutating it.
	OldValue any
	NewValue any
	// IPAddress is the client address the action came from.
	IPAddress string
}

// Recorder writes audit entries without blocking the caller.
type Recorder struct {
	store  Store
	logger *slog.Logger
	run    func(func())
}

// NewRecorder returns a Recorder that persists through store and schedules its
// writes with run — the application's tracked-goroutine helper, so that a
// pending audit write still completes during graceful shutdown.
func NewRecorder(store Store, logger *slog.Logger, run func(func())) *Recorder {
	return &Recorder{store: store, logger: logger, run: run}
}

// Record persists e in the background.
//
// The values are encoded here, synchronously, before anything is scheduled.
// They must be: OldValue and NewValue are `any`, so they are almost always
// pointers into a struct the calling handler still owns and often still writes
// to — its next line may fill in a display field, or the response encoder may
// touch it. Encoding on the background goroutine read that live struct
// concurrently with its owner, which is a data race in every handler that calls
// this, not only the ones where it happened to be observed. Marshalling costs
// microseconds on the request path and buys a snapshot nothing else can reach.
//
// A failed audit write is logged and dropped, never returned: auditing must not
// be able to fail the operation it is describing, which has already happened by
// the time Record is called.
func (rec *Recorder) Record(e Entry) {
	oldJSON := rec.encode(e, "old_value", e.OldValue)
	newJSON := rec.encode(e, "new_value", e.NewValue)

	rec.run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()

		// The write is detached from the request, so it carries none of the
		// request's tenant scope, and audit_log is under row-level security
		// (the tenant row-level security policies). The entry names its own complex, which is the
		// scope this insert needs; an entry with none is a platform row —
		// every auth event is one — and those are written under the bypass,
		// because a policy comparing NULL to a uuid matches nothing and would
		// silently drop the row instead of storing it.
		if e.ComplexID != nil {
			ctx = data.ContextWithTenant(ctx, *e.ComplexID)
		} else {
			ctx = data.ContextWithTenantBypass(ctx)
		}

		err := rec.store.InsertAuditLog(ctx, e.UserID, e.ComplexID, e.Action, e.EntityType, e.EntityID, oldJSON, newJSON, e.IPAddress)
		if err != nil {
			rec.logger.Error("failed to insert audit log",
				"error", err,
				"action", e.Action,
				"entity_type", e.EntityType,
			)
		}
	})
}

// encode turns one audit value into the bytes the store will persist, or nil
// where there is no value to record.
//
// Encoding can now fail in front of the caller, which raises the question the
// background version never had to answer. The answer is the same contract:
// auditing does not fail the operation it describes. That operation is already
// committed by the time Record is called, so an error returned from here would
// reach a handler with nothing left to undo and no honest way to report it.
//
// What changes is what gets dropped. Only the payload is lost — the entry is
// still written, so who acted, on what, from where and when all survive. An
// audit trail that forgets a diff is diminished; one that forgets an actor
// because their court had an unencodable field is broken, and the second is the
// far worse failure for the one job this table has. The error is logged at the
// call site, on the request goroutine, instead of vanishing into a background
// insert's error return.
func (rec *Recorder) encode(e Entry, field string, v any) []byte {
	if v == nil {
		return nil
	}

	encoded, err := json.Marshal(v)
	if err != nil {
		rec.logger.Error("audit value could not be encoded; recording the entry without it",
			"error", err,
			"field", field,
			"action", e.Action,
			"entity_type", e.EntityType,
		)
		return nil
	}
	return encoded
}

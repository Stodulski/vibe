package audit

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	auditstore "github.com/stodulski/vibe-server/internal/audit/store"
	"github.com/stodulski/vibe-server/internal/data"
)

// TestServiceList covers the two rules that moved out of the handler: the trail
// is always scoped to the complex the caller owns, and reading it is itself an
// event on it — recorded after the query, so a failed read is never reported as
// a completed one.
func TestServiceList(t *testing.T) {
	boom := errors.New("store is down")

	tests := []struct {
		name       string
		logs       []*auditstore.AuditLogRow
		readErr    error
		entityType string
		wantErr    error
		wantScope  string
	}{
		{
			name:      "an unfiltered read records how much came back",
			logs:      []*auditstore.AuditLogRow{{}, {}, {}},
			wantScope: `{"returned":3}`,
		},
		{
			name:       "a filtered read records the filter too",
			logs:       []*auditstore.AuditLogRow{{}},
			entityType: "booking",
			wantScope:  `{"entity_type":"booking","returned":1}`,
		},
		{
			name:    "a failed read is not recorded as a completed one",
			readErr: boom,
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTrailFixture(t)
			f.reader.logs = tt.logs
			f.reader.err = tt.readErr

			complexID := uuid.New()
			userID := uuid.New()
			logs, _, err := f.service.List(t.Context(), Actor{UserID: &userID, IP: "1.2.3.4"}, complexID, tt.entityType, data.Filters{Limit: 50})

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if len(f.store.calls) != 0 {
					t.Error("a failed read still wrote an entry to the trail")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(logs) != len(tt.logs) {
				t.Errorf("got %d rows, want %d", len(logs), len(tt.logs))
			}
			// The scope must come from the complex the guard resolved, never
			// from anything the caller named.
			if f.reader.gotComplex == nil || *f.reader.gotComplex != complexID {
				t.Error("the read was not scoped to the caller's own complex")
			}
			if len(f.store.calls) != 1 {
				t.Fatalf("got %d trail entries for this read, want 1", len(f.store.calls))
			}
			// The entry names what was read, never the rows themselves.
			if got := string(f.store.calls[0].newJSON); got != tt.wantScope {
				t.Errorf("recorded scope %s, want %s", got, tt.wantScope)
			}
		})
	}
}

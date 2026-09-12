package courts

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/slots"
)

// TestServiceBlockSlot covers the rule that moved out of the handler: an
// owner may take hours off sale only on a court of their own complex, and only
// where nothing is already sold or already blocked.
func TestServiceBlockSlot(t *testing.T) {
	complexID := uuid.New()
	courtID := uuid.New()
	date := time.Now().AddDate(0, 0, 7).Truncate(24 * time.Hour)

	ownCourt := &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Court 1"}
	foreignCourt := &courtstore.Court{ID: courtID, ComplexID: uuid.New(), Name: "Someone else's"}

	booked := func(from, to string) []bookingstore.BookedSpan {
		return []bookingstore.BookedSpan{{
			CourtID:  courtID,
			StartsAt: slots.At(date, from),
			EndsAt:   slots.At(date, to),
		}}
	}

	tests := []struct {
		name      string
		court     *courtstore.Court
		getErr    error
		booked    []bookingstore.BookedSpan
		blockErr  error
		start     string
		end       string
		wantErr   error
		wantAudit bool
	}{
		{
			name:      "free range is blocked and audited",
			court:     ownCourt,
			start:     "10:00",
			end:       "11:00",
			wantAudit: true,
		},
		{
			name:    "a court of another complex is invisible",
			court:   foreignCourt,
			start:   "10:00",
			end:     "11:00",
			wantErr: data.ErrRecordNotFound,
		},
		{
			name:    "a court that does not exist is not found",
			getErr:  data.ErrRecordNotFound,
			start:   "10:00",
			end:     "11:00",
			wantErr: data.ErrRecordNotFound,
		},
		{
			name:    "an overlapping booking refuses the block",
			court:   ownCourt,
			booked:  booked("10:30", "12:00"),
			start:   "10:00",
			end:     "11:00",
			wantErr: courtstore.ErrSlotHasBooking,
		},
		{
			name:   "an adjacent booking does not",
			court:  ownCourt,
			booked: booked("11:00", "12:00"),
			start:  "10:00",
			end:    "11:00",

			wantAudit: true,
		},
		{
			name:     "a range the store already holds is refused",
			court:    ownCourt,
			blockErr: courtstore.ErrSlotAlreadyBlocked,
			start:    "10:00",
			end:      "11:00",
			wantErr:  courtstore.ErrSlotAlreadyBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &stubStore{court: tt.court, getErr: tt.getErr, blockErr: tt.blockErr}
			bookings := &stubBookings{booked: tt.booked}
			rec := &stubRecorder{}
			svc := NewService(store, bookings, &stubComplexes{}, rec)

			userID := uuid.New()
			slot, err := svc.BlockSlot(t.Context(), complexID, Actor{UserID: &userID, IP: "1.2.3.4"}, courtID, BlockSlotInput{
				Date:      date,
				StartTime: tt.start,
				EndTime:   tt.end,
				CreatedBy: userID,
			})

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if store.insertedBlocked != nil {
					t.Fatalf("a refused block still wrote a row")
				}
				if len(rec.entries) != 0 {
					t.Fatalf("a refused block still recorded an audit entry")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if slot == nil || store.insertedBlocked == nil {
				t.Fatalf("the block was not written")
			}
			// The court name is on the slot before the entry is recorded, so
			// the audit row names the court rather than only its id.
			if slot.CourtName != tt.court.Name {
				t.Errorf("slot names court %q, want %q", slot.CourtName, tt.court.Name)
			}
			if tt.wantAudit {
				if len(rec.entries) != 1 {
					t.Fatalf("got %d audit entries, want 1", len(rec.entries))
				}
				e := rec.entries[0]
				if e.Action != "create" || e.EntityType != "blocked_slot" {
					t.Errorf("audit entry is %s/%s, want create/blocked_slot", e.Action, e.EntityType)
				}
				if e.IPAddress != "1.2.3.4" || e.UserID == nil || *e.UserID != userID {
					t.Errorf("audit entry does not carry the actor the handler read")
				}
			}
		})
	}
}

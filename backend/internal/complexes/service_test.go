package complexes

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// TestServiceCreate covers the rules that moved out of the handler: a venue may
// not take a public URL another venue holds, an account may not own more than
// one live complex, and a venue that is created starts with a full week of
// opening hours.
func TestServiceCreate(t *testing.T) {
	valid := CreateInput{
		Name:              "Club Norte",
		Slug:              "club-norte",
		Address:           "Av. Siempreviva 742",
		City:              "Rosario",
		Province:          "Santa Fe",
		Phone:             "+5493411234567",
		DepositPercentage: 50,
		CancellationHours: 24,
	}

	tests := []struct {
		name      string
		slugTaken bool
		insertErr error
		owned     int
		wantErr   error
	}{
		{name: "an account that owns nothing yet is created"},
		{name: "a slug another venue holds is refused", slugTaken: true, wantErr: ErrSlugTaken},
		{
			name:      "a slug taken between the check and the insert is still refused as a slug",
			insertErr: complexstore.ErrDuplicateSlug,
			wantErr:   ErrSlugTaken,
		},
		{name: "an account that already owns a complex is refused", owned: 1, wantErr: ErrAlreadyOwnsComplex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.store.slugTaken = tt.slugTaken
			f.store.insertErr = tt.insertErr
			f.store.owned = make([]*complexstore.Complex, tt.owned)

			ownerID := uuid.New()
			actorID := ownerID
			complex, err := f.service.Create(t.Context(), ownerID, Actor{UserID: &actorID, IP: "1.2.3.4"}, valid)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				if len(f.store.upsertedSchedule) != 0 {
					t.Errorf("a refused creation still wrote %d schedules", len(f.store.upsertedSchedule))
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if complex == nil || f.store.inserted == nil {
				t.Fatal("the venue was not written")
			}
			if f.store.inserted.CountryCode != "AR" || f.store.inserted.Currency != "ARS" {
				t.Errorf("a new venue must default to AR/ARS; got %s/%s", f.store.inserted.CountryCode, f.store.inserted.Currency)
			}
			// The column is NOT NULL and an explicit NULL parameter does not
			// fall back to the column default, so this must be an empty list
			// rather than nil.
			if f.store.inserted.Amenities == nil {
				t.Error("a new venue must carry an empty amenity list, not nil")
			}
			if len(f.store.upsertedSchedule) != 7 {
				t.Fatalf("got %d default schedules, want 7", len(f.store.upsertedSchedule))
			}
			for _, sc := range f.store.upsertedSchedule {
				if sc.OpenTime != "08:00" || sc.CloseTime != "23:00" || sc.IsClosed {
					t.Errorf("default schedule for %s is %s-%s (closed=%v), want 08:00-23:00 open", sc.Day, sc.OpenTime, sc.CloseTime, sc.IsClosed)
				}
			}
			if len(f.audit.entries) != 1 || f.audit.entries[0].Action != "create" {
				t.Errorf("want one create audit entry; got %d", len(f.audit.entries))
			}
		})
	}
}

// TestThePublicProfileRefusesToServeWithNoCourtPort pins the one thing the
// deferred court port must never do: answer quietly.
//
// courts and complexes read each other, so this service is built with no court
// port at all and cmd/api closes the loop with SetCourts before the router
// exists. If that call is ever moved, deleted, or ordered after the first
// request, the failure has to be loud — a venue page that renders with no
// courts on it looks exactly like a venue that has none, and nothing in the
// response says otherwise.
func TestThePublicProfileRefusesToServeWithNoCourtPort(t *testing.T) {
	f := newFixture(t)
	// The court port is the one dependency this service can be built without,
	// so the fixture's is taken back off to reach the state a missing SetCourts
	// leaves behind.
	f.service.SetCourts(nil)
	f.store.complex = &complexstore.Complex{ID: uuid.New(), Slug: "vibe-palermo", IsActive: true}

	defer func() {
		if recover() == nil {
			t.Error("a public profile served with no court port must panic, not answer a venue page with no courts on it")
		}
	}()

	_, _ = f.service.GetPublic(t.Context(), "vibe-palermo")
}

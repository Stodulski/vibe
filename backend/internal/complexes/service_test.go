package complexes

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
)

// TestServiceCreate covers the rules that moved out of the handler: a venue may
// not take a public URL another venue holds, an account may not exceed its cap,
// and a venue that is created starts with a full week of opening hours.
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
		maxOwned  int
		wantErr   error
	}{
		{name: "a free slug under the cap is created", maxOwned: 4},
		{name: "a slug another venue holds is refused", slugTaken: true, maxOwned: 4, wantErr: ErrSlugTaken},
		{
			name:      "a slug taken between the check and the insert is still refused as a slug",
			insertErr: complexstore.ErrDuplicateSlug,
			maxOwned:  4,
			wantErr:   ErrSlugTaken,
		},
		{name: "an account at its cap is refused", owned: 4, maxOwned: 4, wantErr: ErrMaxComplexes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.service.cfg.MaxComplexes = tt.maxOwned
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

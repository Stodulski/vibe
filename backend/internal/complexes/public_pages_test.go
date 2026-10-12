package complexes

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/indexnow"
)

// recordingIndexer keeps every change the service reports, in order.
type recordingIndexer struct {
	changes []indexnow.Change
}

func (r *recordingIndexer) ComplexChanged(c indexnow.Change) {
	r.changes = append(r.changes, c)
}

func publicComplex(slug, city string) *complexstore.Complex {
	return &complexstore.Complex{ID: uuid.New(), OwnerID: uuid.New(), Name: "Club Norte", Slug: slug, City: city, IsActive: true}
}

func TestCreatePublishesTheNewPage(t *testing.T) {
	f := newFixture(t)
	rec := &recordingIndexer{}
	f.service.pages = rec

	valid := CreateInput{Name: "Club Norte", Slug: "club-norte", Address: "Av. 1", City: "Rosario", Province: "Santa Fe", Phone: "+5493411234567", DepositPercentage: 50, CancellationHours: 24}
	complex, err := f.service.Create(t.Context(), uuid.New(), Actor{}, valid)
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	want := []indexnow.Change{{
		ComplexID: complex.ID,
		After:     indexnow.Snapshot{Slug: "club-norte", City: "Rosario", Visible: true},
	}}
	if !reflect.DeepEqual(rec.changes, want) {
		t.Fatalf("changes = %+v, want %+v", rec.changes, want)
	}
}

func TestUpdateReportsTheSlugBeforeAndAfter(t *testing.T) {
	f := newFixture(t)
	rec := &recordingIndexer{}
	f.service.pages = rec
	complex := publicComplex("club-viejo", "Rosario")
	newSlug := "club-norte"

	if _, err := f.service.Update(t.Context(), complex, Actor{}, UpdateInput{Slug: &newSlug}); err != nil {
		t.Fatalf("Update() = %v", err)
	}

	want := []indexnow.Change{{
		ComplexID: complex.ID,
		Before:    indexnow.Snapshot{Slug: "club-viejo", City: "Rosario", Visible: true},
		After:     indexnow.Snapshot{Slug: "club-norte", City: "Rosario", Visible: true},
	}}
	if !reflect.DeepEqual(rec.changes, want) {
		t.Fatalf("changes = %+v, want %+v", rec.changes, want)
	}
}

func TestSwitchingOffIsReportedAsLeavingThePublicSet(t *testing.T) {
	f := newFixture(t)
	rec := &recordingIndexer{}
	f.service.pages = rec
	complex := publicComplex("club-norte", "Rosario")
	off := false

	if _, err := f.service.Update(t.Context(), complex, Actor{}, UpdateInput{IsActive: &off}); err != nil {
		t.Fatalf("Update() = %v", err)
	}

	if len(rec.changes) != 1 {
		t.Fatalf("reported %d changes, want 1", len(rec.changes))
	}
	if got := rec.changes[0].After; got.Visible {
		t.Fatalf("after switching off the page is still reported visible: %+v", got)
	}
	if got := rec.changes[0].Before; !got.Visible {
		t.Fatalf("before switching off the page was visible, got %+v", got)
	}
}

func TestDeleteReportsThePageThatNowAnswersGone(t *testing.T) {
	f := newFixture(t)
	rec := &recordingIndexer{}
	f.service.pages = rec
	complex := publicComplex("club-norte", "Rosario")

	if _, err := f.service.Delete(t.Context(), complex, Actor{}); err != nil {
		t.Fatalf("Delete() = %v", err)
	}

	want := []indexnow.Change{{
		ComplexID: complex.ID,
		Before:    indexnow.Snapshot{Slug: "club-norte", City: "Rosario", Visible: true},
	}}
	if !reflect.DeepEqual(rec.changes, want) {
		t.Fatalf("changes = %+v, want %+v", rec.changes, want)
	}
}

func TestAFailedWriteAnnouncesNothing(t *testing.T) {
	f := newFixture(t)
	rec := &recordingIndexer{}
	f.service.pages = rec
	f.store.updateErr = errors.New("database unavailable")
	newSlug := "club-norte"

	if _, err := f.service.Update(t.Context(), publicComplex("club-viejo", "Rosario"), Actor{}, UpdateInput{Slug: &newSlug}); err == nil {
		t.Fatal("Update() = nil, want the write error")
	}
	if len(rec.changes) != 0 {
		t.Fatalf("a write that did not commit was announced: %+v", rec.changes)
	}
}

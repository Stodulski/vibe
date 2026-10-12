package courts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	courtstore "github.com/stodulski/vibe-server/internal/courts/store"
	"github.com/stodulski/vibe-server/internal/indexnow"
)

// recordingPages keeps every change the courts service reports, in order.
type recordingPages struct{ changes []indexnow.Change }

func (r *recordingPages) ComplexChanged(c indexnow.Change) { r.changes = append(r.changes, c) }

// recordingQueue stands in for the durable queue under a real indexnow.Service.
type recordingQueue struct{ taskTypes []string }

func (q *recordingQueue) EnqueueAt(taskType string, _ any, _ time.Time, _ string) {
	q.taskTypes = append(q.taskTypes, taskType)
}

var errBoom = errors.New("database unavailable")

// courtWrite is one court write that changes what the public complex page shows.
type courtWrite struct {
	name string
	run  func(ctx context.Context, svc *Service, complexID, courtID uuid.UUID) error
	// fail makes the store refuse this write.
	fail func(s *stubStore)
}

func courtWrites() []courtWrite {
	inactive := false
	sport := "tennis"
	prices := []PriceInput{{Price: 9000, DayType: "weekday", TimeFrom: "08:00", TimeTo: "23:00"}}
	return []courtWrite{
		{
			name: "create",
			run: func(ctx context.Context, svc *Service, complexID, _ uuid.UUID) error {
				_, err := svc.Create(ctx, complexID, Actor{}, CreateInput{Name: "Court 2", Sport: "padel", CourtType: "indoor"})
				return err
			},
			fail: func(s *stubStore) { s.insertErr = errBoom },
		},
		{
			name: "deactivate",
			run: func(ctx context.Context, svc *Service, complexID, courtID uuid.UUID) error {
				_, err := svc.Update(ctx, complexID, Actor{}, courtID, UpdateInput{IsActive: &inactive})
				return err
			},
			fail: func(s *stubStore) { s.updateErr = errBoom },
		},
		{
			name: "change sport",
			run: func(ctx context.Context, svc *Service, complexID, courtID uuid.UUID) error {
				_, err := svc.Update(ctx, complexID, Actor{}, courtID, UpdateInput{Sport: &sport})
				return err
			},
			fail: func(s *stubStore) { s.updateErr = errBoom },
		},
		{
			name: "delete",
			run: func(ctx context.Context, svc *Service, complexID, courtID uuid.UUID) error {
				return svc.Delete(ctx, complexID, Actor{}, courtID)
			},
			fail: func(s *stubStore) { s.softDeleteErr = errBoom },
		},
		{
			name: "update prices",
			run: func(ctx context.Context, svc *Service, complexID, courtID uuid.UUID) error {
				_, _, err := svc.UpdatePrices(ctx, complexID, Actor{}, courtID, prices, nil)
				return err
			},
			fail: func(s *stubStore) { s.replacePricesErr = errBoom },
		},
	}
}

// pagesStore holds one court of complexID, which every non-create write acts on.
func pagesStore(complexID, courtID uuid.UUID) *stubStore {
	return &stubStore{
		court: &courtstore.Court{ID: courtID, ComplexID: complexID, Name: "Court 1", Sport: "padel", CourtType: "indoor", IsActive: true},
	}
}

func servedComplex(id uuid.UUID) *complexstore.Complex {
	return &complexstore.Complex{ID: id, Slug: "club-norte", City: "Rosario", IsActive: true}
}

func TestCourtWritesAnnounceTheComplexPage(t *testing.T) {
	for _, w := range courtWrites() {
		t.Run(w.name, func(t *testing.T) {
			complexID, courtID := uuid.New(), uuid.New()
			pages := &recordingPages{}
			svc := NewService(pagesStore(complexID, courtID), &stubBookings{},
				&stubComplexes{complex: servedComplex(complexID)}, &stubRecorder{}, pages)

			if err := w.run(t.Context(), svc, complexID, courtID); err != nil {
				t.Fatalf("write failed: %v", err)
			}

			page := indexnow.Snapshot{Slug: "club-norte", City: "Rosario", Visible: true}
			want := []indexnow.Change{{ComplexID: complexID, Before: page, After: page}}
			if !reflect.DeepEqual(pages.changes, want) {
				t.Fatalf("changes = %+v, want %+v", pages.changes, want)
			}
		})
	}
}

func TestCourtWritesAnnounceNothingWhenTheStoreRefuses(t *testing.T) {
	for _, w := range courtWrites() {
		t.Run(w.name, func(t *testing.T) {
			complexID, courtID := uuid.New(), uuid.New()
			store := pagesStore(complexID, courtID)
			w.fail(store)
			pages := &recordingPages{}
			svc := NewService(store, &stubBookings{},
				&stubComplexes{complex: servedComplex(complexID)}, &stubRecorder{}, pages)

			if err := w.run(t.Context(), svc, complexID, courtID); !errors.Is(err, errBoom) {
				t.Fatalf("got error %v, want the store's refusal", err)
			}
			if len(pages.changes) != 0 {
				t.Fatalf("a refused write announced %d changes", len(pages.changes))
			}
		})
	}
}

func TestCourtWritesWithoutANotifierStillWrite(t *testing.T) {
	for _, w := range courtWrites() {
		t.Run(w.name, func(t *testing.T) {
			complexID, courtID := uuid.New(), uuid.New()
			svc := NewService(pagesStore(complexID, courtID), &stubBookings{},
				&stubComplexes{complex: servedComplex(complexID)}, &stubRecorder{}, nil)

			if err := w.run(t.Context(), svc, complexID, courtID); err != nil {
				t.Fatalf("write failed without a notifier: %v", err)
			}
		})
	}
}

func TestCourtWriteStandsWhenTheComplexCannotBeRead(t *testing.T) {
	complexID, courtID := uuid.New(), uuid.New()
	pages := &recordingPages{}
	svc := NewService(pagesStore(complexID, courtID), &stubBookings{},
		&stubComplexes{err: errBoom}, &stubRecorder{}, pages)

	sport := "tennis"
	if _, err := svc.Update(t.Context(), complexID, Actor{}, courtID, UpdateInput{Sport: &sport}); err != nil {
		t.Fatalf("committed write reported an error: %v", err)
	}
	if len(pages.changes) != 0 {
		t.Fatalf("an unreadable complex still announced %d changes", len(pages.changes))
	}
}

// TestCourtWriteSubmitsOnlyForAServedComplex runs the real indexnow service
// under the courts service: a switched-on complex is submitted, a switched-off
// one is not.
func TestCourtWriteSubmitsOnlyForAServedComplex(t *testing.T) {
	tests := []struct {
		name     string
		complex  func(id uuid.UUID) *complexstore.Complex
		wantJobs []string
	}{
		{
			name:     "a served complex is submitted",
			complex:  servedComplex,
			wantJobs: []string{indexnow.TaskSubmit},
		},
		{
			name: "a switched-off complex is not",
			complex: func(id uuid.UUID) *complexstore.Complex {
				c := servedComplex(id)
				c.IsActive = false
				return c
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			complexID, courtID := uuid.New(), uuid.New()
			queue := &recordingQueue{}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			notifier := indexnow.NewService("production", indexnow.ProductionOrigin, queue, nil, logger)
			svc := NewService(pagesStore(complexID, courtID), &stubBookings{},
				&stubComplexes{complex: tt.complex(complexID)}, &stubRecorder{}, notifier)

			sport := "tennis"
			if _, err := svc.Update(t.Context(), complexID, Actor{}, courtID, UpdateInput{Sport: &sport}); err != nil {
				t.Fatalf("Update() = %v", err)
			}
			if !reflect.DeepEqual(queue.taskTypes, tt.wantJobs) {
				t.Fatalf("queued %v, want %v", queue.taskTypes, tt.wantJobs)
			}
		})
	}
}

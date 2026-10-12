//go:build integration

package store_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist"
	"github.com/stodulski/vibe-server/internal/waitlist/store"
)

func newWaitlistStore(f *datatest.Fixture) *store.Store {
	return &store.Store{Q: db.New(f.DB)}
}

func day(d int) time.Time {
	return time.Date(2026, time.November, d, 0, 0, 0, 0, time.UTC)
}

func countEntries(t *testing.T, f *datatest.Fixture, complexID uuid.UUID) int {
	t.Helper()
	var n int
	if err := f.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM waitlist_entries WHERE complex_id = $1`, complexID).Scan(&n); err != nil {
		t.Fatalf("counting waitlist entries: %v", err)
	}
	return n
}

// Joining the same day twice leaves one row: the second join is a no-op, which
// is what lets the endpoint answer 204 both times.
func TestInsertIsIdempotentForTheSameAddressAndDay(t *testing.T) {
	f := datatest.Isolated(t)
	st := newWaitlistStore(f)
	ctx := f.Scoped(context.Background())

	for i := 0; i < 2; i++ {
		if err := st.Insert(ctx, f.ComplexID, day(20), "ana@example.com"); err != nil {
			t.Fatalf("insert %d: %v", i+1, err)
		}
	}

	if got := countEntries(t, f, f.ComplexID); got != 1 {
		t.Fatalf("want one row for a repeated join; got %d", got)
	}
}

// The schema refuses an address that was not trimmed and lowercased first, so
// "Ana@x.com" can never sit beside "ana@x.com" as two people.
func TestTheSchemaRefusesAnUnnormalisedEmail(t *testing.T) {
	f := datatest.Isolated(t)
	st := newWaitlistStore(f)
	ctx := f.Scoped(context.Background())

	if err := st.Insert(ctx, f.ComplexID, day(20), "Ana@Example.com"); err == nil {
		t.Fatal("want the CHECK constraint to refuse an address that is not lowercase")
	}
}

// Only the requested day is claimed, and a claimed row is never claimed again.
func TestClaimNotifiesEachWaitingEntryOnceAndOnlyForThatDay(t *testing.T) {
	f := datatest.Isolated(t)
	st := newWaitlistStore(f)
	ctx := f.Scoped(context.Background())
	for _, email := range []string{"ana@example.com", "bruno@example.com"} {
		if err := st.Insert(ctx, f.ComplexID, day(20), email); err != nil {
			t.Fatalf("insert %s: %v", email, err)
		}
	}
	if err := st.Insert(ctx, f.ComplexID, day(21), "carla@example.com"); err != nil {
		t.Fatalf("insert other day: %v", err)
	}

	first, err := st.ClaimUnnotified(ctx, f.ComplexID, day(20))
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("want the two entries for day 20; got %d", len(first))
	}
	for _, c := range first {
		if c.Email == "carla@example.com" {
			t.Fatalf("the claim for day 20 took an entry for day 21")
		}
	}

	second, err := st.ClaimUnnotified(ctx, f.ComplexID, day(20))
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("a claimed entry must not be claimed again; got %d", len(second))
	}
}

// The daily cleanup removes only days before the cutoff.
func TestDeleteBeforeRemovesOnlyPastDays(t *testing.T) {
	f := datatest.Isolated(t)
	st := newWaitlistStore(f)
	ctx := f.Scoped(context.Background())
	if err := st.Insert(ctx, f.ComplexID, day(10), "past@example.com"); err != nil {
		t.Fatalf("insert past: %v", err)
	}
	if err := st.Insert(ctx, f.ComplexID, day(20), "ahead@example.com"); err != nil {
		t.Fatalf("insert ahead: %v", err)
	}

	removed, err := st.DeleteBefore(context.Background(), day(15))

	if err != nil {
		t.Fatalf("delete before: %v", err)
	}
	if removed != 1 {
		t.Fatalf("want one past row removed; got %d", removed)
	}
	if got := countEntries(t, f, f.ComplexID); got != 1 {
		t.Fatalf("want the future row kept; got %d rows", got)
	}
}

// A store scoped to another complex refuses to write this complex's rows.
func TestInsertRefusesAnotherTenantsComplex(t *testing.T) {
	f := datatest.Isolated(t)
	st := newWaitlistStore(f)
	otherTenant := data.ContextWithTenant(context.Background(), uuid.New())

	err := st.Insert(otherTenant, f.ComplexID, day(20), "ana@example.com")

	if err == nil {
		t.Fatal("want a write refused when the context is scoped to another complex")
	}
}

type fixedLookup struct{ complex *complexstore.Complex }

func (l fixedLookup) GetBySlug(context.Context, string) (*complexstore.Complex, error) {
	return l.complex, nil
}

type recordingNotifier struct {
	sent []notifications.WaitlistSlotFreedEmail
}

func (n *recordingNotifier) WaitlistSlotFreed(e notifications.WaitlistSlotFreedEmail) {
	n.sent = append(n.sent, e)
}

// A slot freed twice on the same day, the way two cancellations or a cancel and
// the expiry sweep can race, emails each waiting person exactly once.
func TestASlotFreedTwiceEmailsEachEntryOnce(t *testing.T) {
	f := datatest.Isolated(t)
	st := newWaitlistStore(f)
	notifier := &recordingNotifier{}
	complex := &complexstore.Complex{ID: f.ComplexID, Name: "Vibe Padel", Slug: "vibe-padel", IsActive: true}
	svc := waitlist.NewService(st, fixedLookup{complex: complex}, notifier, "https://vibe.test",
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	waitingDay := timezone.Today().AddDate(0, 0, 5)
	for _, email := range []string{"ana@example.com", "bruno@example.com"} {
		if err := svc.Join(ctx, "vibe-padel", waitingDay, email); err != nil {
			t.Fatalf("join %s: %v", email, err)
		}
	}

	freed := waitlist.Freed{ComplexID: f.ComplexID, ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel", Day: waitingDay}
	svc.SlotFreed(ctx, freed)
	svc.SlotFreed(ctx, freed)

	if len(notifier.sent) != 2 {
		t.Fatalf("want one email per entry, two in total; got %d", len(notifier.sent))
	}
}

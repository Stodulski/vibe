package waitlist

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/notifications"
	"github.com/stodulski/vibe-server/internal/timezone"
	"github.com/stodulski/vibe-server/internal/waitlist/store"
)

type insertCall struct {
	complexID uuid.UUID
	day       time.Time
	email     string
}

// fakeStore records what the service asks of it and answers claims with the
// rows configured in claimed.
type fakeStore struct {
	inserted   []insertCall
	claimed    []store.Claimed
	claimCalls []insertCall
	claimErr   error
	purgedDays []time.Time
	purgeRows  int64
}

func (f *fakeStore) Insert(_ context.Context, complexID uuid.UUID, day time.Time, email string) error {
	f.inserted = append(f.inserted, insertCall{complexID: complexID, day: day, email: email})
	return nil
}

func (f *fakeStore) ClaimUnnotified(_ context.Context, complexID uuid.UUID, day time.Time) ([]store.Claimed, error) {
	f.claimCalls = append(f.claimCalls, insertCall{complexID: complexID, day: day})
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	return f.claimed, nil
}

func (f *fakeStore) DeleteBefore(_ context.Context, today time.Time) (int64, error) {
	f.purgedDays = append(f.purgedDays, today)
	return f.purgeRows, nil
}

// fakeComplexes answers the slug lookup with one complex, or with err.
type fakeComplexes struct {
	complex *complexstore.Complex
	err     error
}

func (f *fakeComplexes) GetBySlug(context.Context, string) (*complexstore.Complex, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.complex, nil
}

type fakeNotifier struct {
	sent []notifications.WaitlistSlotFreedEmail
}

func (f *fakeNotifier) WaitlistSlotFreed(e notifications.WaitlistSlotFreedEmail) {
	f.sent = append(f.sent, e)
}

func newTestService(st *fakeStore, cx *fakeComplexes, n *fakeNotifier) *Service {
	return NewService(st, cx, n, "https://vibe.test", slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func activeComplex(id uuid.UUID) *fakeComplexes {
	return &fakeComplexes{complex: &complexstore.Complex{ID: id, Name: "Vibe Padel", Slug: "vibe-padel", IsActive: true}}
}

func TestJoinStoresTheEntryUnderTheComplexOfTheSlug(t *testing.T) {
	complexID := uuid.New()
	st := &fakeStore{}
	svc := newTestService(st, activeComplex(complexID), &fakeNotifier{})
	day := timezone.Today().AddDate(0, 0, 3)

	if err := svc.Join(t.Context(), "vibe-padel", day, "ana@example.com"); err != nil {
		t.Fatalf("join: %v", err)
	}

	if len(st.inserted) != 1 {
		t.Fatalf("want one insert; got %d", len(st.inserted))
	}
	got := st.inserted[0]
	if got.complexID != complexID || got.email != "ana@example.com" || !got.day.Equal(day) {
		t.Fatalf("insert = %+v; want complex %s, day %s, ana@example.com", got, complexID, day)
	}
}

func TestJoinRefusesAnInactiveComplexAsNotFound(t *testing.T) {
	st := &fakeStore{}
	cx := activeComplex(uuid.New())
	cx.complex.IsActive = false
	svc := newTestService(st, cx, &fakeNotifier{})

	err := svc.Join(t.Context(), "vibe-padel", timezone.Today(), "ana@example.com")

	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound for a deactivated complex; got %v", err)
	}
	if len(st.inserted) != 0 {
		t.Fatalf("nothing may be stored for a deactivated complex; got %d inserts", len(st.inserted))
	}
}

func TestJoinForAnUnknownSlugIsNotFound(t *testing.T) {
	st := &fakeStore{}
	svc := newTestService(st, &fakeComplexes{err: data.ErrRecordNotFound}, &fakeNotifier{})

	err := svc.Join(t.Context(), "nope", timezone.Today(), "ana@example.com")

	if !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound for an unknown slug; got %v", err)
	}
	if len(st.inserted) != 0 {
		t.Fatalf("nothing may be stored for an unknown slug; got %d inserts", len(st.inserted))
	}
}

func TestSlotFreedEmailsEveryoneStillWaitingForTheDay(t *testing.T) {
	complexID := uuid.New()
	day := timezone.Today().AddDate(0, 0, 3)
	first, second := uuid.New(), uuid.New()
	st := &fakeStore{claimed: []store.Claimed{{ID: first, Email: "ana@example.com"}, {ID: second, Email: "bruno@example.com"}}}
	n := &fakeNotifier{}
	svc := newTestService(st, &fakeComplexes{}, n)

	svc.SlotFreed(t.Context(), Freed{ComplexID: complexID, ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel", Day: day})

	if len(st.claimCalls) != 1 || st.claimCalls[0].complexID != complexID || !st.claimCalls[0].day.Equal(day) {
		t.Fatalf("want one claim for the complex and day; got %+v", st.claimCalls)
	}
	if len(n.sent) != 2 {
		t.Fatalf("want one email per entry; got %d", len(n.sent))
	}
	wantBookURL := "https://vibe.test/c/vibe-padel?date=" + day.Format("2006-01-02")
	for i, want := range []struct {
		entry uuid.UUID
		email string
	}{{first, "ana@example.com"}, {second, "bruno@example.com"}} {
		got := n.sent[i]
		if got.EntryID != want.entry.String() || got.Email != want.email {
			t.Errorf("email %d = entry %q to %q; want entry %s to %s", i, got.EntryID, got.Email, want.entry, want.email)
		}
		if got.ComplexName != "Vibe Padel" || got.DateLabel != day.Format("02/01") || got.BookURL != wantBookURL {
			t.Errorf("email %d = %+v; want complex, dd/mm and the storefront link", i, got)
		}
	}
}

func TestSlotFreedSkipsADayThatHasPassed(t *testing.T) {
	st := &fakeStore{claimed: []store.Claimed{{ID: uuid.New(), Email: "ana@example.com"}}}
	n := &fakeNotifier{}
	svc := newTestService(st, &fakeComplexes{}, n)

	svc.SlotFreed(t.Context(), Freed{ComplexID: uuid.New(), ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel", Day: timezone.Today().AddDate(0, 0, -1)})

	if len(st.claimCalls) != 0 || len(n.sent) != 0 {
		t.Fatalf("a day that has passed frees nothing; claims %d, emails %d", len(st.claimCalls), len(n.sent))
	}
}

func TestSlotFreedWithNobodyWaitingSendsNothing(t *testing.T) {
	n := &fakeNotifier{}
	svc := newTestService(&fakeStore{}, &fakeComplexes{}, n)

	svc.SlotFreed(t.Context(), Freed{ComplexID: uuid.New(), ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel", Day: timezone.Today().AddDate(0, 0, 3)})

	if len(n.sent) != 0 {
		t.Fatalf("no one is waiting; want no email, got %d", len(n.sent))
	}
}

func TestSlotFreedSurvivesAClaimFailure(t *testing.T) {
	n := &fakeNotifier{}
	svc := newTestService(&fakeStore{claimErr: errors.New("db down")}, &fakeComplexes{}, n)

	// The cancellation that freed the slot has already happened; a failed claim
	// must not panic the caller or send anything half-formed.
	svc.SlotFreed(t.Context(), Freed{ComplexID: uuid.New(), ComplexName: "Vibe Padel", ComplexSlug: "vibe-padel", Day: timezone.Today().AddDate(0, 0, 3)})

	if len(n.sent) != 0 {
		t.Fatalf("a failed claim sends nothing; got %d emails", len(n.sent))
	}
}

func TestPurgeDeletesEveryDayBeforeToday(t *testing.T) {
	st := &fakeStore{purgeRows: 4}
	svc := newTestService(st, &fakeComplexes{}, &fakeNotifier{})

	n, err := svc.Purge(t.Context())

	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 4 {
		t.Fatalf("want 4 rows deleted; got %d", n)
	}
	if len(st.purgedDays) != 1 || !st.purgedDays[0].Equal(timezone.Today()) {
		t.Fatalf("want the cutoff to be today (ART); got %v", st.purgedDays)
	}
}

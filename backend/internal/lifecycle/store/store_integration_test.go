//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
	"github.com/stodulski/vibe-server/internal/lifecycle"
	"github.com/stodulski/vibe-server/internal/lifecycle/store"
)

func newLifecycleStore(f *datatest.Fixture) *store.Store {
	return &store.Store{Q: db.New(f.DB)}
}

// insertUser adds an owner. verified sets email_verified; createdAgo backdates
// created_at relative to the fixture's transaction clock, which NOW() is held at.
func insertUser(t *testing.T, f *datatest.Fixture, verified bool, createdAgo time.Duration) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := f.DB.QueryRow(context.Background(), `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified, created_at)
		VALUES ($1, $2, 'Ana', 'Test', '+5491100000009', 'owner', $3, NOW() - make_interval(secs => $4))
		RETURNING id`,
		"nudge-"+uuid.NewString()+"@example.test", []byte("not-a-real-hash"), verified, createdAgo.Seconds(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("inserting owner: %v", err)
	}
	return id
}

func emailOf(t *testing.T, f *datatest.Fixture, id uuid.UUID) string {
	t.Helper()
	var e string
	if err := f.DB.QueryRow(context.Background(), `SELECT email::text FROM users WHERE id = $1`, id).Scan(&e); err != nil {
		t.Fatalf("reading email: %v", err)
	}
	return e
}

// insertComplex adds a live complex for owner, created createdAgo before the transaction.
func insertComplex(t *testing.T, f *datatest.Fixture, owner uuid.UUID, name string, createdAgo time.Duration) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := f.DB.QueryRow(context.Background(), `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone, created_at)
		VALUES ($1, $2, $3, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000008',
		        NOW() - make_interval(secs => $4))
		RETURNING id`,
		owner, name, "nudge-"+uuid.NewString(), createdAgo.Seconds(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("inserting complex: %v", err)
	}
	return id
}

func candidateIDs(cs []lifecycle.Candidate) map[uuid.UUID]lifecycle.Candidate {
	out := make(map[uuid.UUID]lifecycle.Candidate, len(cs))
	for _, c := range cs {
		out[c.UserID] = c
	}
	return out
}

func listFor(t *testing.T, st *store.Store, ctx context.Context, kind lifecycle.Kind) map[uuid.UUID]lifecycle.Candidate {
	t.Helper()
	cs, err := st.ListCandidates(ctx, kind, 1000)
	if err != nil {
		t.Fatalf("ListCandidates(%s): %v", kind, err)
	}
	return candidateIDs(cs)
}

const day = 24 * time.Hour

// An unverified account is due its verification nudge between one and seven days
// after it was made. Younger accounts are still being filled in, older ones are on
// their way to deletion, and a suppressed address never hears from us again.
func TestVerifyEmailIsDueForUnverifiedAccountsBetween1And7Days(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	due := insertUser(t, f, false, 25*time.Hour)
	tooNew := insertUser(t, f, false, time.Hour)
	tooOld := insertUser(t, f, false, 8*day)
	verified := insertUser(t, f, true, 25*time.Hour)
	suppressed := insertUser(t, f, false, 25*time.Hour)
	if err := st.Suppress(ctx, emailOf(t, f, suppressed), lifecycle.ReasonUnsubscribe); err != nil {
		t.Fatalf("Suppress: %v", err)
	}

	got := listFor(t, st, ctx, lifecycle.KindVerifyEmail)
	if _, ok := got[due]; !ok {
		t.Fatal("an unverified account 25 hours old was not due a verification nudge")
	}
	for name, id := range map[string]uuid.UUID{"too new": tooNew, "too old": tooOld, "verified": verified, "suppressed": suppressed} {
		if _, ok := got[id]; ok {
			t.Errorf("%s account was listed as due", name)
		}
	}
}

// A verified owner with no live complex is due create_complex after a day.
func TestCreateComplexIsDueForVerifiedOwnersWithoutAComplex(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	due := insertUser(t, f, true, 25*time.Hour)
	tooNew := insertUser(t, f, true, time.Hour)
	withComplex := insertUser(t, f, true, 25*time.Hour)
	insertComplex(t, f, withComplex, "Ya Tiene", 25*time.Hour)

	got := listFor(t, st, ctx, lifecycle.KindCreateComplex)
	if _, ok := got[due]; !ok {
		t.Fatal("a verified owner with no complex, 25 hours old, was not due")
	}
	if _, ok := got[tooNew]; ok {
		t.Error("an owner created an hour ago was listed")
	}
	if _, ok := got[withComplex]; ok {
		t.Error("an owner with a live complex was listed for create_complex")
	}
	if _, ok := got[f.UserID]; ok {
		t.Error("the fixture owner, who has a complex, was listed")
	}
}

// A complex that is 48 hours old with no live court is due add_courts, and carries
// its name for the email.
func TestAddCourtsIsDueForAComplexWithNoLiveCourtAfter48Hours(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	owner := insertUser(t, f, true, 50*time.Hour)
	complexID := insertComplex(t, f, owner, "Padel Norte", 49*time.Hour)

	c, ok := listFor(t, st, ctx, lifecycle.KindAddCourts)[owner]
	if !ok {
		t.Fatalf("a complex 49 hours old with no courts was not due add_courts (complex %s)", complexID)
	}
	if c.ComplexName != "Padel Norte" {
		t.Fatalf("complex name = %q, want the complex's name for the email", c.ComplexName)
	}

	// The fixture complex has a court, so its owner is not due.
	if _, ok := listFor(t, st, ctx, lifecycle.KindAddCourts)[f.UserID]; ok {
		t.Error("the fixture owner, whose complex has a court, was listed for add_courts")
	}
}

// Inactive: a live complex, no activity for three days, and no booking from any source
// in the same window. The fixture complex is used because it can take a real booking.
func TestInactiveIsDueUntilABookingOrARecentRequestSaysOtherwise(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	// The fixture owner: created long ago, never active, no bookings yet.
	if _, err := f.DB.Exec(ctx, `UPDATE users SET created_at = NOW() - interval '4 days' WHERE id = $1`, f.UserID); err != nil {
		t.Fatalf("backdating the owner: %v", err)
	}
	if _, ok := listFor(t, st, ctx, lifecycle.KindInactive)[f.UserID]; !ok {
		t.Fatal("an owner with a complex, idle for four days and no bookings, was not due inactive")
	}

	// A booking for the complex in the last three days means the complex is alive.
	f.CreateBooking(t, datatest.BookingOptions{StartTime: "10:00", EndTime: "11:30", Public: true})
	if _, ok := listFor(t, st, ctx, lifecycle.KindInactive)[f.UserID]; ok {
		t.Fatal("a complex with a booking this week was still listed as inactive")
	}
}

// A recent request from the owner counts as activity, even with an old account.
func TestInactiveIsNotDueWhenTheOwnerWasActiveRecently(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	if _, err := f.DB.Exec(ctx, `UPDATE users SET created_at = NOW() - interval '30 days', last_active_at = NOW() - interval '1 day' WHERE id = $1`, f.UserID); err != nil {
		t.Fatalf("setting activity: %v", err)
	}
	if _, ok := listFor(t, st, ctx, lifecycle.KindInactive)[f.UserID]; ok {
		t.Fatal("an owner active yesterday was listed as inactive")
	}
}

// Each nudge is once-only: the first claim wins, a second is refused, and a
// released claim can be taken again. This is what makes "once per kind" a fact of
// the data rather than of the cron's timing.
func TestClaimingANudgeIsOnceOnlyAndReleasable(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())
	owner := insertUser(t, f, true, 25*time.Hour)

	first, err := st.ClaimOwnerNudge(ctx, owner, lifecycle.KindCreateComplex)
	if err != nil || !first {
		t.Fatalf("first claim = %v, %v; want true, nil", first, err)
	}
	second, err := st.ClaimOwnerNudge(ctx, owner, lifecycle.KindCreateComplex)
	if err != nil || second {
		t.Fatalf("second claim = %v, %v; want false, nil", second, err)
	}
	if _, ok := listFor(t, st, ctx, lifecycle.KindCreateComplex)[owner]; ok {
		t.Fatal("an owner already nudged for this kind is still listed")
	}

	if err := st.ReleaseOwnerNudge(ctx, owner, lifecycle.KindCreateComplex); err != nil {
		t.Fatalf("release: %v", err)
	}
	again, err := st.ClaimOwnerNudge(ctx, owner, lifecycle.KindCreateComplex)
	if err != nil || !again {
		t.Fatalf("claim after release = %v, %v; want true, nil", again, err)
	}
}

// The launch baseline: an account that already holds a nudge row is not listed for
// that kind, whatever its state. The migration writes those rows for every existing
// account, so this is the same rule the cron relies on at launch.
func TestAnAccountWithAnExistingNudgeRowIsNeverListedForThatKind(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())
	old := insertUser(t, f, true, 25*time.Hour)

	if _, err := f.DB.Exec(ctx, `INSERT INTO owner_nudges (user_id, kind) VALUES ($1, 'create_complex')`, old); err != nil {
		t.Fatalf("seeding the baseline row: %v", err)
	}
	if _, ok := listFor(t, st, ctx, lifecycle.KindCreateComplex)[old]; ok {
		t.Fatal("an existing account with a baseline row was nudged")
	}
}

// Activity is written at most once an hour, by the database's own condition, so the
// throttle holds across instances and not only within one process.
func TestOwnerActivityIsWrittenAtMostOncePerHour(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	wrote, err := st.TouchOwnerLastActive(ctx, f.UserID)
	if err != nil || !wrote {
		t.Fatalf("first touch = %v, %v; want a write", wrote, err)
	}
	wrote, err = st.TouchOwnerLastActive(ctx, f.UserID)
	if err != nil || wrote {
		t.Fatalf("a touch inside the hour = %v, %v; want no write", wrote, err)
	}

	if _, err := f.DB.Exec(ctx, `UPDATE users SET last_active_at = NOW() - interval '61 minutes' WHERE id = $1`, f.UserID); err != nil {
		t.Fatalf("backdating activity: %v", err)
	}
	wrote, err = st.TouchOwnerLastActive(ctx, f.UserID)
	if err != nil || !wrote {
		t.Fatalf("a touch after the hour = %v, %v; want a write", wrote, err)
	}
}

// Only owners carry an activity stamp. A superadmin's request is not an owner
// being active, so the write is refused for a non-owner.
func TestOwnerActivityIsNotWrittenForANonOwner(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	var admin uuid.UUID
	if err := f.DB.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, $2, 'Admin', 'Test', '+5491100000007', 'superadmin', true) RETURNING id`,
		"admin-"+uuid.NewString()+"@example.test", []byte("x"),
	).Scan(&admin); err != nil {
		t.Fatalf("inserting superadmin: %v", err)
	}
	wrote, err := st.TouchOwnerLastActive(ctx, admin)
	if err != nil || wrote {
		t.Fatalf("a superadmin's activity = %v, %v; want no write", wrote, err)
	}
}

// Suppression is idempotent: the second opt-out of an address is a no-op.
func TestSuppressionIsIdempotent(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	for range 2 {
		if err := st.Suppress(ctx, "ana@example.com", lifecycle.ReasonUnsubscribe); err != nil {
			t.Fatalf("Suppress: %v", err)
		}
	}
	var n int
	if err := f.DB.QueryRow(ctx, `SELECT count(*) FROM email_suppressions WHERE email = 'ana@example.com'`).Scan(&n); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if n != 1 {
		t.Fatalf("suppression rows = %d, want 1", n)
	}
}

// A lead is due its nudge two hours after capture, once, and only while no account
// has that address and it has not opted out.
func TestLeadFinishSignupIsDueOnceAfterTwoHours(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	insertLead := func(email string, ago time.Duration) {
		t.Helper()
		if _, err := f.DB.Exec(ctx, `
			INSERT INTO registration_leads (email, first_name, created_at) VALUES ($1, 'Lead', NOW() - make_interval(secs => $2))`,
			email, ago.Seconds()); err != nil {
			t.Fatalf("inserting lead %s: %v", email, err)
		}
	}
	insertLead("due@example.com", 3*time.Hour)
	insertLead("fresh@example.com", time.Hour)
	insertLead("signed-up@example.com", 3*time.Hour)
	insertLead("opted-out@example.com", 3*time.Hour)
	if _, err := f.DB.Exec(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ('signed-up@example.com', 'x', 'A', 'B', '+5491100000006', 'owner', false)`); err != nil {
		t.Fatalf("inserting the account: %v", err)
	}
	if err := st.Suppress(ctx, "opted-out@example.com", lifecycle.ReasonUnsubscribe); err != nil {
		t.Fatalf("Suppress: %v", err)
	}

	cs, err := st.ListFinishSignupCandidates(ctx, 1000)
	if err != nil {
		t.Fatalf("ListFinishSignupCandidates: %v", err)
	}
	if len(cs) != 1 || cs[0].Email != "due@example.com" {
		t.Fatalf("due leads = %+v, want only due@example.com", cs)
	}

	claimed, err := st.ClaimLeadNudge(ctx, "due@example.com")
	if err != nil || !claimed {
		t.Fatalf("first lead claim = %v, %v", claimed, err)
	}
	again, err := st.ClaimLeadNudge(ctx, "due@example.com")
	if err != nil || again {
		t.Fatalf("second lead claim = %v, %v; want refused", again, err)
	}
	cs, _ = st.ListFinishSignupCandidates(ctx, 1000)
	if len(cs) != 0 {
		t.Fatalf("a nudged lead is still due: %+v", cs)
	}
}

// connect_mp is due for a complex with a live court and no MercadoPago account, a
// day after the complex was created, and carries its name for the email. It is not
// due once MercadoPago is connected, before the day is up, or with no court yet.
func TestConnectMPIsDueForAComplexWithCourtsAndNoMercadoPagoAfterADay(t *testing.T) {
	f := datatest.Isolated(t)
	st := newLifecycleStore(f)
	ctx := f.Scoped(context.Background())

	// The fixture complex has a court and no MercadoPago account.
	if _, err := f.DB.Exec(ctx, `UPDATE complexes SET created_at = NOW() - interval '25 hours' WHERE id = $1`, f.ComplexID); err != nil {
		t.Fatalf("setup: ageing the fixture complex: %v", err)
	}
	c, ok := listFor(t, st, ctx, lifecycle.KindConnectMP)[f.UserID]
	if !ok {
		t.Fatal("a complex a day old with a court and no MercadoPago was not due connect_mp")
	}
	var name string
	if err := f.DB.QueryRow(ctx, `SELECT name FROM complexes WHERE id = $1`, f.ComplexID).Scan(&name); err != nil {
		t.Fatalf("reading the complex name: %v", err)
	}
	if c.ComplexName != name {
		t.Errorf("complex name = %q, want %q for the email", c.ComplexName, name)
	}

	// A newer complex is not due yet, and one with no court gets add_courts instead.
	young := insertUser(t, f, true, 30*time.Hour)
	insertComplex(t, f, young, "Recién Creado", 2*time.Hour)
	noCourts := insertUser(t, f, true, 50*time.Hour)
	insertComplex(t, f, noCourts, "Sin Canchas", 49*time.Hour)
	due := listFor(t, st, ctx, lifecycle.KindConnectMP)
	if _, ok := due[young]; ok {
		t.Error("a complex created two hours ago was listed for connect_mp")
	}
	if _, ok := due[noCourts]; ok {
		t.Error("a complex with no court was listed for connect_mp")
	}

	// Connecting MercadoPago takes the complex off the list.
	if _, err := f.DB.Exec(ctx, `UPDATE complexes SET mp_access_token = 'connected', mp_user_id = '123' WHERE id = $1`, f.ComplexID); err != nil {
		t.Fatalf("setup: connecting MercadoPago: %v", err)
	}
	if _, ok := listFor(t, st, ctx, lifecycle.KindConnectMP)[f.UserID]; ok {
		t.Error("a complex with MercadoPago connected was listed for connect_mp")
	}
}

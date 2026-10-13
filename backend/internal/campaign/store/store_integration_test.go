//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/campaign"
	"github.com/stodulski/vibe-server/internal/campaign/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
)

func newBenefitsStore(f *datatest.Fixture) *store.Store {
	return &store.Store{Q: db.New(f.DB)}
}

// insertAccount adds one account in the given role and state and returns its id and address.
func insertAccount(t *testing.T, f *datatest.Fixture, role string, verified, active bool) (uuid.UUID, string) {
	t.Helper()
	var id uuid.UUID
	email := "benefits-" + uuid.NewString() + "@example.test"
	err := f.DB.QueryRow(context.Background(), `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified, is_active)
		VALUES ($1, $2, 'Ana', 'Test', '+5491100000009', $3::user_role, $4, $5)
		RETURNING id`,
		email, []byte("not-a-real-hash"), role, verified, active,
	).Scan(&id)
	if err != nil {
		t.Fatalf("inserting %s account: %v", role, err)
	}
	return id, email
}

// insertComplex adds a complex for owner. mpUserID is empty for a venue that is not
// connected to MercadoPago; deleted soft-deletes it; createdAgo orders it among the
// owner's complexes.
func insertComplex(t *testing.T, f *datatest.Fixture, owner uuid.UUID, slug, mpUserID string, deleted bool, createdAgo time.Duration) {
	t.Helper()
	var mp *string
	if mpUserID != "" {
		mp = &mpUserID
	}
	_, err := f.DB.Exec(context.Background(), `
		INSERT INTO complexes (owner_id, name, slug, address, city, province, phone, mp_user_id, deleted_at, created_at)
		VALUES ($1, 'Cancha Norte', $2, 'Av. Siempreviva 742', 'Rosario', 'Santa Fe', '+5491100000008', $3,
		        CASE WHEN $4::bool THEN NOW() ELSE NULL END, NOW() - make_interval(secs => $5))`,
		owner, slug, mp, deleted, createdAgo.Seconds(),
	)
	if err != nil {
		t.Fatalf("inserting complex %s: %v", slug, err)
	}
}

func recipientsOf(t *testing.T, st *store.Store, ctx context.Context) map[uuid.UUID]campaign.Recipient {
	t.Helper()
	rows, err := st.ListRecipients(ctx, 1000)
	if err != nil {
		t.Fatalf("ListRecipients: %v", err)
	}
	out := make(map[uuid.UUID]campaign.Recipient, len(rows))
	for _, r := range rows {
		out[r.UserID] = r
	}
	return out
}

// Only a verified, active owner who has not opted out is a recipient. A superadmin,
// a client, an unverified account and a suppressed address are never listed.
func TestBenefitsRecipientsAreVerifiedActiveOwnersOnly(t *testing.T) {
	f := datatest.Isolated(t)
	st := newBenefitsStore(f)
	ctx := f.Scoped(context.Background())

	want, _ := insertAccount(t, f, "owner", true, true)
	unverified, _ := insertAccount(t, f, "owner", false, true)
	inactive, _ := insertAccount(t, f, "owner", true, false)
	superadmin, _ := insertAccount(t, f, "superadmin", true, true)
	client, _ := insertAccount(t, f, "client", true, true)
	suppressed, suppressedEmail := insertAccount(t, f, "owner", true, true)
	if _, err := f.DB.Exec(ctx, `INSERT INTO email_suppressions (email, reason) VALUES ($1, 'unsubscribe')`, suppressedEmail); err != nil {
		t.Fatalf("suppressing: %v", err)
	}

	got := recipientsOf(t, st, ctx)
	if _, ok := got[want]; !ok {
		t.Fatal("a verified, active owner was not a recipient")
	}
	for name, id := range map[string]uuid.UUID{
		"unverified": unverified, "inactive": inactive, "superadmin": superadmin,
		"client": client, "suppressed": suppressed,
	} {
		if _, ok := got[id]; ok {
			t.Errorf("the %s account was listed as a recipient", name)
		}
	}
}

// The last line depends on the owner's live complexes: none means create one; a live
// complex with no MercadoPago means connect it; a connected one means share the page.
func TestBenefitsSegmentFollowsTheOwnersLiveComplexes(t *testing.T) {
	f := datatest.Isolated(t)
	st := newBenefitsStore(f)
	ctx := f.Scoped(context.Background())
	suffix := uuid.NewString()[:8]

	none, _ := insertAccount(t, f, "owner", true, true)
	deletedOnly, _ := insertAccount(t, f, "owner", true, true)
	unconnected, _ := insertAccount(t, f, "owner", true, true)
	connected, _ := insertAccount(t, f, "owner", true, true)
	mixed, _ := insertAccount(t, f, "owner", true, true)

	insertComplex(t, f, deletedOnly, "borrada-"+suffix, "", true, 2*time.Hour)
	insertComplex(t, f, unconnected, "sin-mp-"+suffix, "", false, 2*time.Hour)
	insertComplex(t, f, connected, "con-mp-"+suffix, "mp-100", false, 2*time.Hour)
	// A soft-deleted complex next to a live one: only the live one decides the segment.
	insertComplex(t, f, mixed, "vieja-borrada-"+suffix, "mp-300", true, 3*time.Hour)
	insertComplex(t, f, mixed, "nueva-sin-mp-"+suffix, "", false, time.Hour)

	got := recipientsOf(t, st, ctx)
	cases := []struct {
		name    string
		owner   uuid.UUID
		segment string
		slug    string
	}{
		{"no complex", none, campaign.SegmentCreateComplex, ""},
		{"only a deleted complex", deletedOnly, campaign.SegmentCreateComplex, ""},
		{"live complex without MercadoPago", unconnected, campaign.SegmentConnectMP, "sin-mp-" + suffix},
		{"live complex connected", connected, campaign.SegmentSharePage, "con-mp-" + suffix},
		{"a deleted connected complex does not count", mixed, campaign.SegmentConnectMP, "nueva-sin-mp-" + suffix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := got[tc.owner]
			if !ok {
				t.Fatal("the owner is not a recipient")
			}
			if r.Segment != tc.segment || r.Slug != tc.slug {
				t.Errorf("segment/slug = %q/%q; want %q/%q", r.Segment, r.Slug, tc.segment, tc.slug)
			}
		})
	}
}

// A claim is once-only per owner, and a claimed owner leaves the recipient list, so
// a later pass cannot enqueue the email again even after its job row has been swept.
func TestBenefitsClaimIsOnceOnlyAndLeavesTheRecipientList(t *testing.T) {
	f := datatest.Isolated(t)
	st := newBenefitsStore(f)
	ctx := f.Scoped(context.Background())
	owner, _ := insertAccount(t, f, "owner", true, true)

	first, err := st.Claim(ctx, owner)
	if err != nil || !first {
		t.Fatalf("first claim = %v, %v; want true, nil", first, err)
	}
	second, err := st.Claim(ctx, owner)
	if err != nil || second {
		t.Fatalf("second claim = %v, %v; want false, nil", second, err)
	}
	if _, ok := recipientsOf(t, st, ctx)[owner]; ok {
		t.Fatal("a claimed owner is still a recipient")
	}
}

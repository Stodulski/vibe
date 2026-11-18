//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/campaign/store"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
	"github.com/stodulski/vibe-server/internal/db"
)

// One-off campaign (2026-11). Remove with the campaign.

func kitRecipients(t *testing.T, st *store.KitStore, ctx context.Context) map[uuid.UUID]string {
	t.Helper()
	rows, err := st.ListRecipients(ctx, 1000)
	if err != nil {
		t.Fatalf("ListRecipients: %v", err)
	}
	out := make(map[uuid.UUID]string, len(rows))
	for _, r := range rows {
		out[r.UserID] = r.Slug
	}
	return out
}

// Only a verified, active, not suppressed owner whose complex is live and switched
// on gets the kit email, and it carries that complex's slug.
func TestKitRecipientsAreOwnersWithALiveComplex(t *testing.T) {
	f := datatest.Isolated(t)
	st := &store.KitStore{Q: db.New(f.DB)}
	ctx := f.Scoped(context.Background())
	suffix := uuid.NewString()[:8]

	want, _ := insertAccount(t, f, "owner", true, true)
	insertComplex(t, f, want, "kit-live-"+suffix, "", false, 0)

	noComplex, _ := insertAccount(t, f, "owner", true, true)
	deleted, _ := insertAccount(t, f, "owner", true, true)
	insertComplex(t, f, deleted, "kit-deleted-"+suffix, "", true, 0)
	switchedOff, _ := insertAccount(t, f, "owner", true, true)
	insertComplex(t, f, switchedOff, "kit-off-"+suffix, "", false, 0)
	if _, err := f.DB.Exec(ctx, `UPDATE complexes SET is_active = false WHERE slug = $1`, "kit-off-"+suffix); err != nil {
		t.Fatalf("switching the complex off: %v", err)
	}
	unverified, _ := insertAccount(t, f, "owner", false, true)
	insertComplex(t, f, unverified, "kit-unverified-"+suffix, "", false, 0)
	suppressed, suppressedEmail := insertAccount(t, f, "owner", true, true)
	insertComplex(t, f, suppressed, "kit-suppressed-"+suffix, "", false, 0)
	if _, err := f.DB.Exec(ctx, `INSERT INTO email_suppressions (email, reason) VALUES ($1, 'unsubscribe')`, suppressedEmail); err != nil {
		t.Fatalf("suppressing: %v", err)
	}

	got := kitRecipients(t, st, ctx)
	if got[want] != "kit-live-"+suffix {
		t.Fatalf("the owner with a live complex is listed with slug %q; want kit-live-%s", got[want], suffix)
	}
	for name, id := range map[string]uuid.UUID{
		"no complex": noComplex, "deleted complex": deleted, "switched-off complex": switchedOff,
		"unverified": unverified, "suppressed": suppressed,
	} {
		if _, ok := got[id]; ok {
			t.Errorf("the owner with %s was listed as a recipient", name)
		}
	}
}

// The kit claim is once-only and separate from the benefits claim: an owner who got
// the benefits email still gets the kit email, exactly once.
func TestKitClaimIsOnceOnlyAndIndependentOfTheBenefitsClaim(t *testing.T) {
	f := datatest.Isolated(t)
	st := &store.KitStore{Q: db.New(f.DB)}
	ctx := f.Scoped(context.Background())
	owner, _ := insertAccount(t, f, "owner", true, true)
	insertComplex(t, f, owner, "kit-claim-"+uuid.NewString()[:8], "", false, 0)

	if ok, err := newBenefitsStore(f).Claim(ctx, owner); err != nil || !ok {
		t.Fatalf("benefits claim = %v, %v; want true, nil", ok, err)
	}
	if _, ok := kitRecipients(t, st, ctx)[owner]; !ok {
		t.Fatal("an owner who got the benefits email is not a kit recipient")
	}
	first, err := st.Claim(ctx, owner)
	if err != nil || !first {
		t.Fatalf("first kit claim = %v, %v; want true, nil", first, err)
	}
	second, err := st.Claim(ctx, owner)
	if err != nil || second {
		t.Fatalf("second kit claim = %v, %v; want false, nil", second, err)
	}
	if _, ok := kitRecipients(t, st, ctx)[owner]; ok {
		t.Fatal("a claimed owner is still a kit recipient")
	}
}

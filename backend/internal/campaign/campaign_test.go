package campaign

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/jobs"
	"github.com/stodulski/vibe-server/internal/lifecycle"
	"github.com/stodulski/vibe-server/internal/notifications"
)

var testConfig = Config{
	Env:           "production",
	FrontendURL:   "https://app.vibe.test",
	PublicBaseURL: "https://vibe.test",
	BackendURL:    "https://api.vibe.test",
}

// fakeStore behaves like the campaign_claims table: ListRecipients returns the
// owners not yet claimed, and Claim is once-only per owner.
type fakeStore struct {
	mu        sync.Mutex
	rows      []Recipient
	claimed   map[uuid.UUID]bool
	claimErr  error
	listCalls int
}

func newFakeStore(rows ...Recipient) *fakeStore {
	return &fakeStore{rows: rows, claimed: map[uuid.UUID]bool{}}
}

func (f *fakeStore) ListRecipients(_ context.Context, limit int32) ([]Recipient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	var out []Recipient
	for _, r := range f.rows {
		if f.claimed[r.UserID] {
			continue
		}
		out = append(out, r)
		if len(out) == int(limit) {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) Claim(_ context.Context, userID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return false, f.claimErr
	}
	if f.claimed[userID] {
		return false, nil
	}
	f.claimed[userID] = true
	return true, nil
}

// staleStore keeps offering one row whose claim another pass already won.
type staleStore struct {
	row       Recipient
	listCalls int
}

func (s *staleStore) ListRecipients(context.Context, int32) ([]Recipient, error) {
	s.listCalls++
	return []Recipient{s.row}, nil
}

func (s *staleStore) Claim(context.Context, uuid.UUID) (bool, error) { return false, nil }

type scheduled struct {
	jobType  string
	payload  any
	runAt    time.Time
	dedupKey string
}

type fakeScheduler struct {
	mu  sync.Mutex
	got []scheduled
}

func (s *fakeScheduler) EnqueueAt(jobType string, payload any, runAt time.Time, dedupKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, scheduled{jobType: jobType, payload: payload, runAt: runAt, dedupKey: dedupKey})
}

func recipient(segment, slug string) Recipient {
	return Recipient{
		UserID:    uuid.New(),
		Email:     uuid.NewString() + "@example.test",
		FirstName: "Ana",
		Slug:      slug,
		Segment:   segment,
	}
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func payloadOf(t *testing.T, s scheduled) notifications.BenefitsCampaignEmail {
	t.Helper()
	p, ok := s.payload.(notifications.BenefitsCampaignEmail)
	if !ok {
		t.Fatalf("payload is %T; want notifications.BenefitsCampaignEmail", s.payload)
	}
	return p
}

// assertTokenFor checks that the token in a link verifies to the address it was
// minted for, so an opt-out link can never unsubscribe somebody else.
func assertTokenFor(t *testing.T, signer *lifecycle.Signer, rawURL, email string) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing %q: %v", rawURL, err)
	}
	got, err := signer.Verify(u.Query().Get("token"))
	if err != nil || got != email {
		t.Errorf("token in %s verifies to %q (%v); want %q", rawURL, got, err, email)
	}
}

func TestEnabledOnlyInProduction(t *testing.T) {
	for env, want := range map[string]bool{"production": true, "staging": false, "development": false, "": false} {
		if got := Enabled(env); got != want {
			t.Errorf("Enabled(%q) = %v; want %v", env, got, want)
		}
	}
}

func TestOutsideProductionNothingIsReadOrSent(t *testing.T) {
	store := newFakeStore(recipient(SegmentCreateComplex, ""))
	sched := &fakeScheduler{}
	cfg := testConfig
	cfg.Env = "staging"
	c := New(store, sched, lifecycle.NewSigner("secret", ""), cfg, discard())

	if n := c.Run(context.Background()); n != 0 {
		t.Fatalf("Run = %d outside production; want 0", n)
	}
	if store.listCalls != 0 {
		t.Error("the recipients were read outside production")
	}
	if len(sched.got) != 0 {
		t.Errorf("%d emails were scheduled outside production", len(sched.got))
	}
}

func TestEveryRecipientGetsOneEmailUnderItsOwnDedupKey(t *testing.T) {
	a := recipient(SegmentCreateComplex, "")
	b := recipient(SegmentSharePage, "cancha-norte")
	store := newFakeStore(a, b)
	sched := &fakeScheduler{}
	signer := lifecycle.NewSigner("secret", "")
	c := New(store, sched, signer, testConfig, discard())

	if n := c.Run(context.Background()); n != 2 {
		t.Fatalf("Run = %d; want 2", n)
	}
	if len(sched.got) != 2 {
		t.Fatalf("scheduled %d emails; want 2", len(sched.got))
	}
	for i, r := range []Recipient{a, b} {
		got := sched.got[i]
		if got.jobType != notifications.TaskEmailBenefitsCampaign {
			t.Errorf("job type = %q; want %q", got.jobType, notifications.TaskEmailBenefitsCampaign)
		}
		wantKey := jobs.DedupKey(notifications.TaskEmailBenefitsCampaign, "campaign:benefits-2026-10:"+r.UserID.String())
		if got.dedupKey != wantKey {
			t.Errorf("dedup key for owner %d is not the campaign key for that owner", i)
		}
		p := payloadOf(t, got)
		if p.Email != r.Email || p.FirstName != "Ana" || p.Segment != r.Segment {
			t.Errorf("payload = %+v; want the recipient's address, name and segment", p)
		}
		assertTokenFor(t, signer, p.UnsubscribeURL, r.Email)
		assertTokenFor(t, signer, p.OptOutURL, r.Email)
	}
}

func TestASecondPassEnqueuesNothingForOwnersAlreadyClaimed(t *testing.T) {
	store := newFakeStore(recipient(SegmentCreateComplex, ""), recipient(SegmentConnectMP, "x"))
	sched := &fakeScheduler{}
	c := New(store, sched, lifecycle.NewSigner("secret", ""), testConfig, discard())

	c.Run(context.Background())
	if n := c.Run(context.Background()); n != 0 {
		t.Fatalf("second Run = %d; want 0", n)
	}
	if len(sched.got) != 2 {
		t.Fatalf("scheduled %d emails across both passes; want 2", len(sched.got))
	}
}

func TestSendsAreStaggeredOneSecondApart(t *testing.T) {
	store := newFakeStore(recipient(SegmentCreateComplex, ""), recipient(SegmentCreateComplex, ""), recipient(SegmentCreateComplex, ""))
	sched := &fakeScheduler{}
	c := New(store, sched, lifecycle.NewSigner("secret", ""), testConfig, discard())

	c.Run(context.Background())
	if len(sched.got) != 3 {
		t.Fatalf("scheduled %d emails; want 3", len(sched.got))
	}
	for i := 1; i < len(sched.got); i++ {
		if gap := sched.got[i].runAt.Sub(sched.got[i-1].runAt); gap != time.Second {
			t.Errorf("gap before email %d = %v; want 1s", i, gap)
		}
	}
}

func TestEveryBatchIsEnqueuedInOnePass(t *testing.T) {
	rows := make([]Recipient, 2*batchSize+7)
	for i := range rows {
		rows[i] = recipient(SegmentCreateComplex, "")
	}
	store := newFakeStore(rows...)
	sched := &fakeScheduler{}
	c := New(store, sched, lifecycle.NewSigner("secret", ""), testConfig, discard())

	if n := c.Run(context.Background()); n != len(rows) {
		t.Fatalf("Run = %d; want %d", n, len(rows))
	}
	if len(sched.got) != len(rows) {
		t.Fatalf("scheduled %d emails; want %d", len(sched.got), len(rows))
	}
}

func TestTheSegmentDecidesTheLastLineAndItsLink(t *testing.T) {
	cases := []struct {
		name, segment, slug, want string
	}{
		{"no live complex", SegmentCreateComplex, "", testConfig.FrontendURL + "/onboarding"},
		{"live complex without MercadoPago", SegmentConnectMP, "cancha-norte", testConfig.FrontendURL + "/settings?tab=billing"},
		{"live complex connected to MercadoPago", SegmentSharePage, "cancha-norte", testConfig.PublicBaseURL + "/c/cancha-norte"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore(recipient(tc.segment, tc.slug))
			sched := &fakeScheduler{}
			c := New(store, sched, lifecycle.NewSigner("secret", ""), testConfig, discard())
			c.Run(context.Background())
			if len(sched.got) != 1 {
				t.Fatalf("scheduled %d emails; want 1", len(sched.got))
			}
			p := payloadOf(t, sched.got[0])
			if p.Segment != tc.segment || p.SegmentURL != tc.want {
				t.Errorf("segment %q -> %q; want %q", p.Segment, p.SegmentURL, tc.want)
			}
			if p.PanelURL != testConfig.FrontendURL+"/dashboard" {
				t.Errorf("panel link = %q; want the dashboard", p.PanelURL)
			}
		})
	}
}

func TestAClaimErrorEndsThePassInsteadOfSpinning(t *testing.T) {
	store := newFakeStore(recipient(SegmentCreateComplex, ""), recipient(SegmentCreateComplex, ""))
	store.claimErr = errors.New("database unavailable")
	sched := &fakeScheduler{}
	c := New(store, sched, lifecycle.NewSigner("secret", ""), testConfig, discard())

	if n := c.Run(context.Background()); n != 0 {
		t.Fatalf("Run = %d after a claim error; want 0", n)
	}
	if store.listCalls != 1 {
		t.Errorf("the pass read the recipients %d times; want 1", store.listCalls)
	}
	if len(sched.got) != 0 {
		t.Errorf("%d emails were scheduled without a durable claim", len(sched.got))
	}
}

func TestAPassThatClaimsNothingEndsInsteadOfRereading(t *testing.T) {
	store := &staleStore{row: recipient(SegmentCreateComplex, "")}
	c := New(store, &fakeScheduler{}, lifecycle.NewSigner("secret", ""), testConfig, discard())

	if n := c.Run(context.Background()); n != 0 {
		t.Fatalf("Run = %d; want 0", n)
	}
	if store.listCalls != 1 {
		t.Errorf("the pass read the recipients %d times; want 1", store.listCalls)
	}
}

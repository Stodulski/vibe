package lifecycle

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/stodulski/vibe-server/internal/notifications"
)

type fakeOwners struct {
	mu         sync.Mutex
	candidates map[Kind][]Candidate
	claimed    map[string]bool
	released   []string
	listLimits []int32
	claimErr   error
}

func newFakeOwners() *fakeOwners {
	return &fakeOwners{candidates: map[Kind][]Candidate{}, claimed: map[string]bool{}}
}

func (f *fakeOwners) ListCandidates(_ context.Context, kind Kind, limit int32) ([]Candidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listLimits = append(f.listLimits, limit)
	return f.candidates[kind], nil
}

func (f *fakeOwners) ClaimOwnerNudge(_ context.Context, userID uuid.UUID, kind Kind) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return false, f.claimErr
	}
	key := userID.String() + "/" + string(kind)
	if f.claimed[key] {
		return false, nil
	}
	f.claimed[key] = true
	return true, nil
}

func (f *fakeOwners) ReleaseOwnerNudge(_ context.Context, userID uuid.UUID, kind Kind) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := userID.String() + "/" + string(kind)
	delete(f.claimed, key)
	f.released = append(f.released, key)
	return nil
}

type fakeLeads struct {
	mu         sync.Mutex
	candidates []LeadCandidate
	claimed    map[string]bool
}

func (f *fakeLeads) ListFinishSignupCandidates(context.Context, int32) ([]LeadCandidate, error) {
	return f.candidates, nil
}

func (f *fakeLeads) ClaimLeadNudge(_ context.Context, email string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	if f.claimed[email] {
		return false, nil
	}
	f.claimed[email] = true
	return true, nil
}

func (f *fakeLeads) ReleaseLeadNudge(_ context.Context, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.claimed, email)
	return nil
}

type fakeNudgeSender struct {
	mu    sync.Mutex
	sent  []notifications.LifecycleNudgeEmail
	subjs []string
}

func (f *fakeNudgeSender) LifecycleNudge(e notifications.LifecycleNudgeEmail, subjectID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, e)
	f.subjs = append(f.subjs, subjectID)
}

type fakeIssuer struct {
	err   error
	calls int
}

func (f *fakeIssuer) IssueVerificationToken(context.Context, uuid.UUID) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return "verify-plain-token", nil
}

const testSecretForNudges = "nudge-secret-0123456789abcdef0123456789"

func newTestNudger(owners OwnerNudgeStore, leads LeadNudgeStore, sender NudgeSender, issuer VerificationIssuer) *Nudger {
	return NewNudger(owners, leads, sender, issuer, NewSigner(testSecretForNudges, ""), NudgeConfig{
		FrontendURL:   "https://app.vibe.test",
		PublicBaseURL: "https://vibe.test",
		BackendURL:    "https://api.vibe.test",
	}, discardLogger())
}

func TestNudgePassSendsAtMostOneNudgePerOwner(t *testing.T) {
	owners := newFakeOwners()
	u := uuid.New()
	// Due for add_courts and for inactive at once: precedence picks add_courts.
	owners.candidates[KindAddCourts] = []Candidate{{UserID: u, Email: "ana@example.com", FirstName: "Ana", ComplexName: "Padel Norte"}}
	owners.candidates[KindInactive] = []Candidate{{UserID: u, Email: "ana@example.com", FirstName: "Ana", ComplexName: "Padel Norte"}}
	sender := &fakeNudgeSender{}

	sent := newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{}).Run(context.Background())

	if sent != 1 || len(sender.sent) != 1 {
		t.Fatalf("sent %d (%d emails), want exactly one", sent, len(sender.sent))
	}
	if got := sender.sent[0].Kind; got != string(KindAddCourts) {
		t.Fatalf("kind = %q, want add_courts by precedence", got)
	}
	if owners.claimed[u.String()+"/inactive"] {
		t.Fatal("the lower-precedence inactive nudge was claimed for an owner who got add_courts")
	}
}

func TestNudgePrecedenceOrderIsVerifyCreateAddInactive(t *testing.T) {
	owners := newFakeOwners()
	u := uuid.New()
	for _, k := range []Kind{KindInactive, KindAddCourts, KindCreateComplex, KindVerifyEmail} {
		owners.candidates[k] = []Candidate{{UserID: u, Email: "ana@example.com"}}
	}
	sender := &fakeNudgeSender{}

	newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{}).Run(context.Background())

	if len(sender.sent) != 1 || sender.sent[0].Kind != string(KindVerifyEmail) {
		t.Fatalf("sent %+v, want only the verify_email nudge", sender.sent)
	}
}

func TestNudgeClaimsBeforeSendingSoASecondPassSendsNothingAgain(t *testing.T) {
	owners := newFakeOwners()
	owners.candidates[KindCreateComplex] = []Candidate{{UserID: uuid.New(), Email: "ana@example.com"}}
	sender := &fakeNudgeSender{}
	n := newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{})

	n.Run(context.Background())
	n.Run(context.Background())

	if len(sender.sent) != 1 {
		t.Fatalf("two passes sent %d emails for one owner, want 1", len(sender.sent))
	}
}

func TestNudgeVerifyEmailCarriesAFreshVerificationLink(t *testing.T) {
	owners := newFakeOwners()
	owners.candidates[KindVerifyEmail] = []Candidate{{UserID: uuid.New(), Email: "ana@example.com", FirstName: "Ana"}}
	sender := &fakeNudgeSender{}
	issuer := &fakeIssuer{}

	newTestNudger(owners, &fakeLeads{}, sender, issuer).Run(context.Background())

	if issuer.calls != 1 {
		t.Fatalf("issued %d verification tokens, want 1", issuer.calls)
	}
	want := "https://app.vibe.test/verify-email?token=verify-plain-token"
	if got := sender.sent[0].CTAURL; got != want {
		t.Fatalf("CTA = %q, want %q", got, want)
	}
}

func TestNudgeReleasesTheClaimWhenNoLinkCanBeIssued(t *testing.T) {
	owners := newFakeOwners()
	u := uuid.New()
	owners.candidates[KindVerifyEmail] = []Candidate{{UserID: u, Email: "ana@example.com"}}
	sender := &fakeNudgeSender{}

	sent := newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{err: errors.New("cooldown")}).Run(context.Background())

	if sent != 0 || len(sender.sent) != 0 {
		t.Fatalf("sent %d after a failed token; want nothing sent", sent)
	}
	if owners.claimed[u.String()+"/verify_email"] {
		t.Fatal("the claim survived a failure, so the owner would never be retried")
	}
}

func TestNudgeLeadFinishSignupSendsOnceToTheRegisterPage(t *testing.T) {
	leads := &fakeLeads{candidates: []LeadCandidate{{Email: "lead@example.com", FirstName: ""}}}
	sender := &fakeNudgeSender{}
	n := newTestNudger(newFakeOwners(), leads, sender, &fakeIssuer{})

	n.Run(context.Background())
	n.Run(context.Background())

	if len(sender.sent) != 1 {
		t.Fatalf("lead nudged %d times, want 1", len(sender.sent))
	}
	got := sender.sent[0]
	if got.Kind != string(KindFinishSignup) || got.CTAURL != "https://app.vibe.test/register" {
		t.Fatalf("lead nudge = %+v, want finish_signup pointing at /register", got)
	}
	if sender.subjs[0] != "lead@example.com" {
		t.Fatalf("lead dedup subject = %q, want the address", sender.subjs[0])
	}
}

func TestNudgeOptOutLinksVerifyForTheRecipient(t *testing.T) {
	owners := newFakeOwners()
	owners.candidates[KindCreateComplex] = []Candidate{{UserID: uuid.New(), Email: "Ana@Example.com"}}
	sender := &fakeNudgeSender{}

	newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{}).Run(context.Background())

	e := sender.sent[0]
	signer := NewSigner(testSecretForNudges, "")

	listURL, err := url.Parse(e.UnsubscribeURL)
	if err != nil {
		t.Fatalf("unsubscribe URL %q: %v", e.UnsubscribeURL, err)
	}
	if listURL.Path != "/api/v1/public/email/unsubscribe" || !strings.HasPrefix(e.UnsubscribeURL, "https://api.vibe.test/") {
		t.Fatalf("unsubscribe URL = %q, want the API origin's one-click endpoint", e.UnsubscribeURL)
	}
	if got, err := signer.Verify(listURL.Query().Get("token")); err != nil || got != "ana@example.com" {
		t.Fatalf("unsubscribe token verifies to %q, %v; want the recipient's address", got, err)
	}

	optURL, err := url.Parse(e.OptOutURL)
	if err != nil {
		t.Fatalf("opt-out URL %q: %v", e.OptOutURL, err)
	}
	if optURL.Path != "/baja" || !strings.HasPrefix(e.OptOutURL, "https://vibe.test/") {
		t.Fatalf("opt-out URL = %q, want the public /baja page", e.OptOutURL)
	}
	if got, err := signer.Verify(optURL.Query().Get("token")); err != nil || got != "ana@example.com" {
		t.Fatalf("opt-out token verifies to %q, %v; want the recipient's address", got, err)
	}
}

func TestNudgeCallsTheStoreWithThePassLimit(t *testing.T) {
	owners := newFakeOwners()
	sender := &fakeNudgeSender{}
	newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{}).Run(context.Background())

	if len(owners.listLimits) != len(ownerKinds) {
		t.Fatalf("listed %d owner kinds, want %d", len(owners.listLimits), len(ownerKinds))
	}
	for _, l := range owners.listLimits {
		if l != perKindLimit {
			t.Fatalf("limit = %d, want %d", l, perKindLimit)
		}
	}
}

func TestNudgeOwnerClaimFailureSendsNothingAndDoesNotPanic(t *testing.T) {
	owners := newFakeOwners()
	owners.claimErr = errors.New("db down")
	owners.candidates[KindCreateComplex] = []Candidate{{UserID: uuid.New(), Email: "ana@example.com"}}
	sender := &fakeNudgeSender{}

	if sent := newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{}).Run(context.Background()); sent != 0 {
		t.Fatalf("sent %d with a failing claim", sent)
	}
}

// connect_mp sends the owner to the payments tab of the settings, where the
// MercadoPago connection lives.
func TestNudgeConnectMPLinksToThePaymentsSettings(t *testing.T) {
	owners := newFakeOwners()
	owners.candidates[KindConnectMP] = []Candidate{{UserID: uuid.New(), Email: "ana@example.com", FirstName: "Ana", ComplexName: "Padel Norte"}}
	sender := &fakeNudgeSender{}

	newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{}).Run(context.Background())

	if len(sender.sent) != 1 || sender.sent[0].Kind != string(KindConnectMP) {
		t.Fatalf("sent %+v, want one connect_mp nudge", sender.sent)
	}
	if got, want := sender.sent[0].CTAURL, "https://app.vibe.test/settings?tab=billing"; got != want {
		t.Fatalf("CTA = %q, want %q", got, want)
	}
}

// connect_mp comes after add_courts and before inactive: an owner due both of the
// first two hears about the courts first, and one due connect_mp and inactive hears
// about MercadoPago.
func TestNudgeConnectMPRanksBetweenAddCourtsAndInactive(t *testing.T) {
	for _, tc := range []struct {
		due  []Kind
		want Kind
	}{
		{[]Kind{KindConnectMP, KindAddCourts}, KindAddCourts},
		{[]Kind{KindInactive, KindConnectMP}, KindConnectMP},
	} {
		owners := newFakeOwners()
		u := uuid.New()
		for _, k := range tc.due {
			owners.candidates[k] = []Candidate{{UserID: u, Email: "ana@example.com"}}
		}
		sender := &fakeNudgeSender{}

		newTestNudger(owners, &fakeLeads{}, sender, &fakeIssuer{}).Run(context.Background())

		if len(sender.sent) != 1 || sender.sent[0].Kind != string(tc.want) {
			t.Errorf("due %v: sent %+v, want only %s", tc.due, sender.sent, tc.want)
		}
	}
}

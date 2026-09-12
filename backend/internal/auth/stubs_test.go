package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/stodulski/vibe-server/internal/audit"
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/googleid"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/notifications"
)

type stubUsers struct {
	mu sync.Mutex

	byEmail map[string]*authstore.User
	byID    map[uuid.UUID]*authstore.User

	getErr    error
	insertErr error
	// updatePasswordErr fails the credential reset a Google claim of an
	// unverified account must persist before any session starts.
	updatePasswordErr error
	// deleteErr fails the account delete, the way a foreign key that still
	// points at one of the owner's complexes did in production.
	deleteErr error

	inserted *authstore.User
	// updated is the account as Update was asked to persist it. The stub used to
	// take the argument and drop it, which made every field this handler writes
	// unobservable — a phone number could be mangled on the way through and
	// every test still passed.
	updated         *authstore.User
	deleted         *uuid.UUID
	verified        *uuid.UUID
	passwordUpdated *uuid.UUID
	failedIncrement int
	failedReset     int
}

func newStubUsers() *stubUsers {
	return &stubUsers{byEmail: map[string]*authstore.User{}, byID: map[uuid.UUID]*authstore.User{}}
}

func (s *stubUsers) add(u *authstore.User) *stubUsers {
	s.byEmail[strings.ToLower(u.Email)] = u
	s.byID[u.ID] = u
	return s
}

func (s *stubUsers) GetByEmail(_ context.Context, email string) (*authstore.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if u, ok := s.byEmail[strings.ToLower(email)]; ok {
		return u, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubUsers) GetByID(_ context.Context, id uuid.UUID) (*authstore.User, error) {
	if u, ok := s.byID[id]; ok {
		return u, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubUsers) Insert(_ context.Context, u *authstore.User) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	// The handler detects a taken address from the unique-constraint error the
	// database raises, not from a pre-read — which is what makes registration
	// race-free. The stub has to behave the same way.
	if _, taken := s.byEmail[strings.ToLower(u.Email)]; taken {
		return authstore.ErrDuplicateEmail
	}
	u.ID = uuid.New()
	s.inserted = u
	s.add(u)
	return nil
}

func (s *stubUsers) Update(_ context.Context, u *authstore.User) error {
	// Copy, so a later mutation by the handler cannot rewrite what the test
	// observes as having been persisted.
	stored := *u
	s.updated = &stored
	s.add(u)
	return nil
}

func (s *stubUsers) Delete(_ context.Context, id uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = &id
	return nil
}

func (s *stubUsers) IncrementFailedAttempts(context.Context, uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedIncrement++
	return nil
}

func (s *stubUsers) ResetFailedAttempts(context.Context, uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedReset++
	return nil
}

func (s *stubUsers) SetEmailVerified(_ context.Context, id uuid.UUID) error {
	s.verified = &id
	return nil
}

func (s *stubUsers) UpdatePassword(_ context.Context, id uuid.UUID, _ []byte) error {
	if s.updatePasswordErr != nil {
		return s.updatePasswordErr
	}
	s.passwordUpdated = &id
	return nil
}

type stubTokens struct {
	stored    map[string]*authstore.RefreshToken
	used      map[string]*authstore.RefreshToken
	deleted   [][]byte
	allWiped  []uuid.UUID
	markedUse [][]byte
	// insertErr fails the write of a replacement refresh token, so a test can
	// drive the transient store failure that used to leave the client holding a
	// token already marked spent.
	insertErr error
}

func newStubTokens() *stubTokens {
	return &stubTokens{stored: map[string]*authstore.RefreshToken{}, used: map[string]*authstore.RefreshToken{}}
}

func (s *stubTokens) InsertRefreshToken(_ context.Context, userID uuid.UUID, hash []byte, ttl time.Duration) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.stored[string(hash)] = &authstore.RefreshToken{UserID: userID, ExpiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *stubTokens) GetRefreshToken(_ context.Context, hash []byte) (*authstore.RefreshToken, error) {
	if t, ok := s.stored[string(hash)]; ok {
		return t, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubTokens) GetUsedRefreshToken(_ context.Context, hash []byte) (*authstore.RefreshToken, error) {
	if t, ok := s.used[string(hash)]; ok {
		return t, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubTokens) MarkRefreshTokenUsed(_ context.Context, hash []byte) error {
	s.markedUse = append(s.markedUse, hash)
	if t, ok := s.stored[string(hash)]; ok {
		t.UsedAt = time.Now()
		s.used[string(hash)] = t
		delete(s.stored, string(hash))
	}
	return nil
}

// ageUsedTokens backdates every spent token by d, so a test can present one
// again from outside the concurrent-refresh grace window.
func (s *stubTokens) ageUsedTokens(d time.Duration) {
	for _, t := range s.used {
		t.UsedAt = t.UsedAt.Add(-d)
	}
}

func (s *stubTokens) DeleteRefreshToken(_ context.Context, hash []byte) error {
	s.deleted = append(s.deleted, hash)
	delete(s.stored, string(hash))
	return nil
}

func (s *stubTokens) DeleteAllForUser(_ context.Context, userID uuid.UUID) error {
	s.allWiped = append(s.allWiped, userID)
	return nil
}

type stubVerifications struct {
	tokens    map[string]*authstore.EmailVerificationToken
	inserted  int
	cooldowns int
	deleted   []uuid.UUID
	insertErr error
}

func newStubVerifications() *stubVerifications {
	return &stubVerifications{tokens: map[string]*authstore.EmailVerificationToken{}}
}

func (s *stubVerifications) Insert(_ context.Context, userID uuid.UUID, hash []byte) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.inserted++
	s.tokens[string(hash)] = &authstore.EmailVerificationToken{UserID: userID, ExpiresAt: time.Now().Add(time.Hour)}
	return nil
}

func (s *stubVerifications) InsertWithCooldown(ctx context.Context, userID uuid.UUID, hash []byte) error {
	s.cooldowns++
	return s.Insert(ctx, userID, hash)
}

func (s *stubVerifications) GetByHash(_ context.Context, hash []byte) (*authstore.EmailVerificationToken, error) {
	if t, ok := s.tokens[string(hash)]; ok {
		return t, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubVerifications) DeleteByUser(_ context.Context, userID uuid.UUID) error {
	s.deleted = append(s.deleted, userID)
	return nil
}

type stubResets struct {
	tokens  map[string]*authstore.PasswordResetToken
	deleted []uuid.UUID
	err     error
	// getErr is what GetByHash returns, so a test can drive a store failure
	// on the lookup rather than only on the insert. Without it no test could
	// tell the "this link is invalid" answer apart from "the database is
	// down", because the handler gave both the same one.
	getErr error
}

func newStubResets() *stubResets {
	return &stubResets{tokens: map[string]*authstore.PasswordResetToken{}}
}

func (s *stubResets) InsertWithCooldown(_ context.Context, userID uuid.UUID, hash []byte) error {
	if s.err != nil {
		return s.err
	}
	s.tokens[string(hash)] = &authstore.PasswordResetToken{UserID: userID, ExpiresAt: time.Now().Add(time.Hour)}
	return nil
}

func (s *stubResets) GetByHash(_ context.Context, hash []byte) (*authstore.PasswordResetToken, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if t, ok := s.tokens[string(hash)]; ok {
		return t, nil
	}
	return nil, data.ErrRecordNotFound
}

func (s *stubResets) DeleteByUser(_ context.Context, userID uuid.UUID) error {
	s.deleted = append(s.deleted, userID)
	// The real store removes the rows, which is what makes a reset link
	// single-use; a stub that only records the call cannot show that.
	for hash, tok := range s.tokens {
		if tok.UserID == userID {
			delete(s.tokens, hash)
		}
	}
	return nil
}

type stubComplexes struct{ owned []*data.Complex }

func (s *stubComplexes) GetByOwner(context.Context, uuid.UUID) ([]*data.Complex, error) {
	return s.owned, nil
}

type stubBookings struct{ hasActive bool }

func (s *stubBookings) HasActiveBookings(context.Context, uuid.UUID) (bool, error) {
	return s.hasActive, nil
}

type stubBlacklist struct {
	blacklisted []string
	invalidated []uuid.UUID
	// err is what both writes return, so a handler test can drive the
	// degraded path where the revocation held locally but was not shared.
	err error
}

func (s *stubBlacklist) BlacklistToken(_ context.Context, raw string, _ time.Time) error {
	s.blacklisted = append(s.blacklisted, raw)
	return s.err
}

func (s *stubBlacklist) InvalidateUserTokens(_ context.Context, userID uuid.UUID) error {
	s.invalidated = append(s.invalidated, userID)
	return s.err
}

type stubNotifier struct {
	verifications []notifications.VerificationEmail
	resets        []notifications.PasswordResetEmail
	duplicates    []notifications.DuplicateRegistrationEmail
}

func (s *stubNotifier) EmailVerification(e notifications.VerificationEmail) {
	s.verifications = append(s.verifications, e)
}

func (s *stubNotifier) PasswordReset(e notifications.PasswordResetEmail) {
	s.resets = append(s.resets, e)
}

func (s *stubNotifier) DuplicateRegistration(e notifications.DuplicateRegistrationEmail) {
	s.duplicates = append(s.duplicates, e)
}

// stubTurnstile is a TurnstileVerifier double. A nil err from Verify (the
// zero value) means an enabled verifier that accepts every token; set err to
// drive a rejection, or enabled to false to exercise the disabled path
// without depending on NewHandler's own nil-to-disabled substitution.
type stubTurnstile struct {
	enabled bool
	err     error
	// calls records every token/remoteIP pair Verify was asked to check, so a
	// test can assert the client IP the handler forwarded and that an empty
	// token never reaches Verify at all.
	calls []struct{ token, remoteIP string }
}

func (s *stubTurnstile) Enabled() bool { return s.enabled }

func (s *stubTurnstile) Verify(_ context.Context, token, remoteIP string) error {
	s.calls = append(s.calls, struct{ token, remoteIP string }{token, remoteIP})
	return s.err
}

// stubGoogleVerifier is a GoogleVerifier double. A nil err from Verify (the
// zero value) means an enabled verifier that accepts every credential with
// claims; set err to drive a rejection, or enabled to false to exercise the
// disabled (503) path without depending on NewHandler's own nil-to-disabled
// substitution.
type stubGoogleVerifier struct {
	enabled bool
	claims  *googleid.Claims
	err     error
	// calls records every credential Verify was asked to check, so a test can
	// assert the exact token the handler forwarded.
	calls []string
}

func (s *stubGoogleVerifier) Enabled() bool { return s.enabled }

func (s *stubGoogleVerifier) Verify(_ context.Context, credential string) (*googleid.Claims, error) {
	s.calls = append(s.calls, credential)
	if s.err != nil {
		return nil, s.err
	}
	return s.claims, nil
}

// stubIdentities is an IdentityStore double, recording every link it was
// asked to write.
type stubIdentities struct {
	inserted []*authstore.UserIdentity
	err      error
}

func (s *stubIdentities) Insert(_ context.Context, identity *authstore.UserIdentity) error {
	if s.err != nil {
		return s.err
	}
	stored := *identity
	s.inserted = append(s.inserted, &stored)
	return nil
}

type stubCache struct{ invalidated []uuid.UUID }

func (s *stubCache) InvalidateUser(_ context.Context, id uuid.UUID) {
	s.invalidated = append(s.invalidated, id)
}

// stubRecorder keeps every entry whole.
//
// It counts nothing and summarises nothing on purpose: an audit test asserts
// what the entry contains — which account, which action, whose id in the actor
// column, and what did and did not reach the value — and a double that only
// counted calls could not tell a correct entry from an empty one.
type stubRecorder struct{ entries []audit.Entry }

func (s *stubRecorder) Record(e audit.Entry) { s.entries = append(s.entries, e) }

// only returns the single entry the handler under test wrote, failing loudly
// when the count is anything but one — a second, unnoticed entry is as much a
// defect as a missing one.
func (s *stubRecorder) only(t *testing.T) audit.Entry {
	t.Helper()
	if len(s.entries) != 1 {
		t.Fatalf("want exactly 1 audit entry, got %d: %+v", len(s.entries), s.entries)
	}
	return s.entries[0]
}

type fixture struct {
	handler       *Handler
	users         *stubUsers
	tokens        *stubTokens
	verifications *stubVerifications
	resets        *stubResets
	complexes     *stubComplexes
	bookings      *stubBookings
	blacklist     *stubBlacklist
	notify        *stubNotifier
	cache         *stubCache
	audit         *stubRecorder
	turnstile     *stubTurnstile
	google        *stubGoogleVerifier
	identities    *stubIdentities
	// logs is everything the handler wrote to its logger. Some failures are
	// deliberately invisible to the caller — the generic answer is what stops
	// account enumeration — so the log is the only place their absence or
	// presence can be asserted.
	logs *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{
		users:         newStubUsers(),
		tokens:        newStubTokens(),
		verifications: newStubVerifications(),
		resets:        newStubResets(),
		complexes:     &stubComplexes{},
		bookings:      &stubBookings{},
		blacklist:     &stubBlacklist{},
		notify:        &stubNotifier{},
		cache:         &stubCache{},
		audit:         &stubRecorder{},
		// Disabled by default, matching a deployment with no
		// TURNSTILE_SECRET_KEY: every existing test keeps exercising the
		// unprotected path unchanged. Tests for the feature itself opt in
		// via newFixtureWithTurnstile.
		turnstile: &stubTurnstile{enabled: false},
		// Disabled by default, matching a deployment with no
		// GOOGLE_OAUTH_CLIENT_ID: every existing test keeps exercising the
		// unconfigured (503) path unchanged. Tests for the feature itself
		// opt in via newFixtureWithGoogle.
		google:     &stubGoogleVerifier{enabled: false},
		identities: &stubIdentities{},
		logs:       &bytes.Buffer{},
	}
	logger := slog.New(slog.NewTextHandler(f.logs, nil))
	f.handler = NewHandler(Dependencies{
		Users:         f.users,
		Tokens:        f.tokens,
		Verifications: f.verifications,
		Resets:        f.resets,
		Complexes:     f.complexes,
		Bookings:      f.bookings,
		Blacklist:     f.blacklist,
		Notify:        f.notify,
		Cache:         f.cache,
		Audit:         f.audit,
		Turnstile:     f.turnstile,
		Google:        f.google,
		Identities:    f.identities,
		Respond:       httpx.NewResponder(logger),
		Logger:        logger,
	}, Config{
		JWTSecret:   testJWTSecret,
		Environment: "test",
		FrontendURL: "https://vibe.test",
		// bcrypt.MinCost, not the production cost: this suite registers and
		// signs in hundreds of users, and a cost-12 hash under the race
		// detector takes seconds each. It used to be a package variable a
		// TestMain wrote; it is configuration now.
		PasswordHashCost: bcrypt.MinCost,
	})
	return f
}

// newFixtureWithTurnstile is newFixture with Turnstile verification enabled,
// for the tests that exercise checkTurnstile itself.
func newFixtureWithTurnstile(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.turnstile.enabled = true
	return f
}

// newFixtureWithGoogle is newFixture with Google sign-in enabled and its stub
// verifier returning a fixed set of claims for every credential, for the
// tests that exercise GoogleSignIn and GoogleComplete themselves. A test that
// needs different claims overwrites f.google.claims before calling the
// handler.
func newFixtureWithGoogle(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.google.enabled = true
	f.google.claims = &googleid.Claims{
		Subject:    "10769150350006150715113082367",
		Email:      "ana@example.com",
		GivenName:  "Ana",
		FamilyName: "Perez",
		Name:       "Ana Perez",
	}
	return f
}

func postJSON(t *testing.T, body string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return out
}

// withUser puts an authenticated account in the request context, as the
// authentication middleware does.
func withUser(r *http.Request, u *authstore.User) *http.Request {
	return httpx.ContextSetUser(r, u)
}

// tokenFromURL pulls the single-use token out of an emailed link.
func tokenFromURL(t *testing.T, link string) string {
	t.Helper()
	_, query, found := strings.Cut(link, "token=")
	if !found {
		t.Fatalf("no token in %q", link)
	}
	token, _, _ := strings.Cut(query, "&")
	return token
}

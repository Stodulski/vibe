package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// The user cache is the other half of the package that had no Redis test at
// all, and it is where the password-change 500 lived: caching *authstore.User
// marshalled it through the API response tags, so PasswordHash came back nil.

func newCacheFixture(t *testing.T) (*fixture, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	return newFixtureWith(t, Config{}, rdb), mr
}

// newFailingCacheFixture builds the chain against a Redis that answers every
// command with an error.
//
// Closing the miniredis is the obvious way to break the cache and the wrong
// one here: the client then waits out redisCacheTimeout on every operation, so
// a test that drives fifty requests through a broken cache spends ten seconds
// proving something about a log line. A server that answers with an error is
// the same failure as far as this code is concerned — an operation it could
// not perform — without the wait.
func newFailingCacheFixture(t *testing.T) *fixture {
	t.Helper()

	f, mr := newCacheFixture(t)
	mr.SetError("the cache is not answering")
	return f
}

// testPassword and its hash. bcrypt is deliberately slow, so the hash is
// computed once for the whole package rather than per test, and at the
// library default rather than the production cost of 12 — what these tests
// turn on is a valid hash against a nil one, not the work factor.
const testPassword = "correct-horse-battery"

var testPasswordHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
})

// testUser returns an account with a real bcrypt hash, because the defect is
// only visible through bcrypt: a nil hash is not a mismatch, it is
// ErrHashTooShort, and the handler turns that into a 500 rather than a 401.
func testUser(t *testing.T) *authstore.User {
	t.Helper()

	lockedUntil := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	user := &authstore.User{
		ID:                  uuid.New(),
		Email:               "ana@example.com",
		FirstName:           "Ana",
		LastName:            "Diaz",
		Phone:               "+541100000000",
		Role:                "owner",
		IsActive:            true,
		EmailVerified:       true,
		CreatedAt:           time.Now().UTC().Truncate(time.Second),
		UpdatedAt:           time.Now().UTC().Truncate(time.Second),
		FailedLoginAttempts: 4,
		LockedUntil:         &lockedUntil,
		LastFailedLogin:     &lockedUntil,
	}
	user.PasswordHash = testPasswordHash()
	return user
}

// The reproduction. Nobody could change their password while the cache was
// warm, which is from every user's second request onwards.
func TestACachedAccountCanStillVerifyItsPassword(t *testing.T) {
	f, _ := newCacheFixture(t)
	user := testUser(t)

	f.mw.cacheUser(t.Context(), user)

	cached := f.mw.getCachedUser(t.Context(), user.ID)
	if cached == nil {
		t.Fatal("the account was just cached and must come back")
	}

	match, err := cached.PasswordMatches(testPassword)
	if err != nil {
		// This is the defect verbatim: bcrypt.ErrHashTooShort, which the
		// handler reports as a 500 rather than as a wrong password.
		t.Fatalf("the cached account could not verify its own password: %v", err)
	}
	if !match {
		t.Error("the correct password must match the cached account")
	}

	match, err = cached.PasswordMatches("wrong")
	if err != nil {
		t.Fatalf("a wrong password must be a mismatch, not an error: %v", err)
	}
	if match {
		t.Error("a wrong password must not match")
	}
}

// The lockout fields were stripped by the same tags. Nothing reads them off
// the context user today; the cache must still return the row it claims to be
// returning, because a zero FailedLoginAttempts and a nil LockedUntil both
// read as "this account is fine".
func TestTheCachedAccountIsTheRowNotASubsetOfIt(t *testing.T) {
	f, _ := newCacheFixture(t)
	user := testUser(t)

	f.mw.cacheUser(t.Context(), user)
	cached := f.mw.getCachedUser(t.Context(), user.ID)
	if cached == nil {
		t.Fatal("the account was just cached and must come back")
	}

	if !reflect.DeepEqual(user, cached) {
		t.Errorf("the cached account must equal the one that was cached\n got: %+v\nwant: %+v", cached, user)
	}
}

// The guard that keeps this fixed. cachedUser is a hand-written mirror of
// authstore.User, so a field added to authstore.User and forgotten here becomes another
// silently-zero value on every cache hit — which is exactly the shape of the
// original defect.
func TestCachedUserMirrorsEveryFieldOfDataUser(t *testing.T) {
	fields := func(v any) []string {
		typ := reflect.TypeOf(v)
		names := make([]string, 0, typ.NumField())
		for i := range typ.NumField() {
			name := typ.Field(i).Name
			if name == "Schema" {
				continue // bookkeeping, not part of the row
			}
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	got, want := fields(cachedUser{}), fields(authstore.User{})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cachedUser and data.User must carry the same fields\n"+
			"cachedUser: %v\n data.User: %v\n"+
			"a field on data.User that is missing here is read as its zero value on every cache hit",
			got, want)
	}
}

// A record written by an earlier build has a different meaning, and decoding
// it into the current shape is how a half-populated account gets served. A
// miss costs one database read; a wrong hit costs a 500 or worse.
func TestAStaleSchemaRecordIsReadAsAMiss(t *testing.T) {
	f, mr := newCacheFixture(t)
	user := testUser(t)

	// Exactly what the previous build wrote: authstore.User through its API tags,
	// so no password hash and no schema stamp.
	raw, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := mr.Set(f.mw.userCacheKey(user.ID), string(raw)); err != nil {
		t.Fatalf("seeding the cache: %v", err)
	}

	if cached := f.mw.getCachedUser(t.Context(), user.ID); cached != nil {
		t.Errorf("a record from the previous cache format must be a miss, not an account with a nil hash; got %+v", cached)
	}
}

// Authenticate must serve the cached account without touching the store, and
// the account it serves must be usable — the same reproduction, one layer up.
func TestAuthenticateServesACacheHitWithoutADatabaseRead(t *testing.T) {
	f, _ := newCacheFixture(t)
	user := testUser(t)

	f.tokens.claims = validClaims(user.ID)
	f.mw.cacheUser(t.Context(), user)
	// Any store read from here is a bug: the entry is warm.
	f.users.err = errUnexpectedStoreRead

	var got *authstore.User
	handler := f.mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = httpx.ContextGetAuthenticatedUser(r)
	}))

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if got == nil {
		t.Fatalf("the cached account must be put in the context; response was %d %s", w.Code, w.Body.String())
	}
	if _, err := got.PasswordMatches(testPassword); err != nil {
		t.Errorf("the account handlers receive from the cache must be usable: %v", err)
	}
}

// InvalidateUser is called by handlers that return immediately afterwards. On
// the request's own context, a client that disconnects as it logs out
// cancelled the delete it had just asked for — and the entry carries is_active
// and the role, so the revoked account keeps working for the full TTL.
func TestInvalidateUserSurvivesACancelledRequestContext(t *testing.T) {
	f, mr := newCacheFixture(t)
	user := testUser(t)

	f.mw.cacheUser(t.Context(), user)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // the client is gone before the delete is attempted
	f.mw.InvalidateUser(ctx, user.ID)

	if mr.Exists(f.mw.userCacheKey(user.ID)) {
		t.Error("the cached account must be dropped even though the request context was cancelled")
	}
}

// A failed delete leaves a revoked account live for ten minutes, so it must at
// least be reported. The signature cannot return the error — see the comment
// on InvalidateUser — and a silent failure is the worst of both.
func TestInvalidateUserReportsADeleteItCouldNotMake(t *testing.T) {
	f, mr := newCacheFixture(t)
	mr.Close()

	f.mw.InvalidateUser(t.Context(), uuid.New())

	if f.logs.Len() == 0 {
		t.Error("a failed cache eviction must be logged: the account it should have revoked is still usable")
	}
}

// A cache read had one answer for four different things: an expired entry, a
// connection refused, a timeout, and bytes nothing can decode. All four
// returned nil and wrote nothing anywhere, so a Redis that had stopped
// answering was a database silently taking the whole authenticated request
// load, with latency as the only symptom and nothing to attribute it to.
func TestACacheReadFailureIsReported(t *testing.T) {
	f := newFailingCacheFixture(t) // the cache is broken, not empty

	if got := f.mw.getCachedUser(t.Context(), uuid.New()); got != nil {
		t.Errorf("a failed read is still a miss; got %+v", got)
	}
	if got := f.logs.String(); !strings.Contains(got, "GET") {
		t.Errorf("a cache that is not answering must say so; the log held %q", got)
	}
}

// The other half of the same policy, and the reason it is not "log every
// error": redis.Nil is the cache working. An entry expired, or this account
// has not been seen for ten minutes. Reporting that as a failure makes the
// report useless on the day something really is wrong.
func TestAnOrdinaryCacheMissIsNotReportedAsAFailure(t *testing.T) {
	f, _ := newCacheFixture(t)

	if got := f.mw.getCachedUser(t.Context(), uuid.New()); got != nil {
		t.Errorf("an account that was never cached must be a miss; got %+v", got)
	}
	if got := f.logs.String(); got != "" {
		t.Errorf("an expired or absent entry is the cache working, not a failure; the log held %q", got)
	}
}

// A record nothing can decode is not the stale-format case — a record written
// by an older build still decodes and is turned away on its schema stamp. Bytes
// that do not decode at all mean something other than this code is writing to
// the key, which no redeploy repairs.
func TestARecordThatCannotBeDecodedIsReported(t *testing.T) {
	f, mr := newCacheFixture(t)
	id := uuid.New()

	if err := mr.Set(f.mw.userCacheKey(id), "not json at all"); err != nil {
		t.Fatalf("seeding the cache: %v", err)
	}

	if got := f.mw.getCachedUser(t.Context(), id); got != nil {
		t.Errorf("an undecodable record must be a miss; got %+v", got)
	}
	if got := f.logs.String(); !strings.Contains(got, "decode") {
		t.Errorf("bytes this code did not write must be reported; the log held %q", got)
	}
}

// A record from an earlier schema is expected across a deploy, self-heals on
// the next write, and arrives once per request for the length of the TTL.
// Reporting it would be a flood describing a cache that is working.
func TestAStaleSchemaRecordIsNotReportedAsAFailure(t *testing.T) {
	f, mr := newCacheFixture(t)
	user := testUser(t)

	raw, err := json.Marshal(user) // the previous build's format
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := mr.Set(f.mw.userCacheKey(user.ID), string(raw)); err != nil {
		t.Fatalf("seeding the cache: %v", err)
	}

	if got := f.mw.getCachedUser(t.Context(), user.ID); got != nil {
		t.Fatalf("a previous-format record must be a miss; got %+v", got)
	}
	if got := f.logs.String(); got != "" {
		t.Errorf("a record the next write replaces is not an incident; the log held %q", got)
	}
}

// The cache fails once per request, so reporting every occurrence writes an
// error line per authenticated request for as long as the incident lasts. That
// does not report the incident — it buries it, and everything else in the log
// with it. One line per operation per interval, saying how many occurrences it
// stands for, carries the same information and leaves the log readable.
func TestABrokenCacheDoesNotWriteALinePerRequest(t *testing.T) {
	f := newFailingCacheFixture(t)

	clock := time.Now()
	f.mw.now = func() time.Time { return clock }

	user := testUser(t)
	f.tokens.claims = validClaims(user.ID)
	f.users.user = user

	const requests = 50
	for range requests {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer some-token")
		f.mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
			ServeHTTP(httptest.NewRecorder(), r)
	}

	// One line for the read that failed and one for the write that failed —
	// two operations, one line each, however many requests went through them.
	if got := strings.Count(f.logs.String(), "user cache:"); got != 2 {
		t.Fatalf("%d requests against a broken cache must leave 2 lines, one per operation; left %d\n%s",
			requests, got, f.logs.String())
	}

	// The suppression is a delay, not a mute: the next interval reports again,
	// and says how many occurrences it is standing in for.
	f.logs.Reset()
	clock = clock.Add(cacheReportInterval)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer some-token")
	f.mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(httptest.NewRecorder(), r)

	if got := f.logs.String(); !strings.Contains(got, "suppressed_since_last_line="+strconv.Itoa(requests-1)) {
		t.Errorf("the line must account for the %d occurrences it stands for; got %s", requests-1, got)
	}
}

// TestTheUserCacheKeyCarriesTheEnvironment is RED-01 for the cache. Two
// deployments on one Redis would otherwise serve each other's accounts, and
// the symptom is not an error: it is a user reading a stale copy of their own
// row from the wrong environment, with the role and the is_active flag it had
// there.
func TestTheUserCacheKeyCarriesTheEnvironment(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	staging := newFixtureWith(t, Config{Env: "staging"}, rdb)
	production := newFixtureWith(t, Config{Env: "production"}, rdb)

	id := uuid.New()
	if staging.mw.userCacheKey(id) == production.mw.userCacheKey(id) {
		t.Fatal("two environments cache one account under the same key")
	}
	if !strings.HasPrefix(staging.mw.userCacheKey(id), "vibe:staging:") {
		t.Errorf("key %q is not namespaced by application and environment", staging.mw.userCacheKey(id))
	}

	user := testUser(t)
	staging.mw.cacheUser(t.Context(), user)
	if cached := production.mw.getCachedUser(t.Context(), user.ID); cached != nil {
		t.Error("an account cached by staging was served to production")
	}
}

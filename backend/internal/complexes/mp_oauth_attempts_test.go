package complexes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/mp"
	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
)

const (
	testState    = "state-0123456789abcdef"
	testVerifier = "verifier-0123456789abcdefghijklmnopqrstuvwxyz-._~"
)

func registerBody(state, verifier string) string {
	b, _ := json.Marshal(map[string]string{"state": state, "code_verifier": verifier})
	return string(b)
}

func connectWithState(state string) string {
	return `{"code":"auth-code","redirect_uri":"https://vibe.test/mp/callback","state":"` + state + `"}`
}

// register stores an attempt for owner on complex through the handler, the
// way the client does before it sends the owner to MercadoPago.
func register(t *testing.T, f *fixture, owner uuid.UUID, complex *complexstore.Complex, state, verifier string) {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.RegisterMercadoPagoOAuthAttempt(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, registerBody(state, verifier)))
	if w.Code != http.StatusNoContent {
		t.Fatalf("registering the attempt: want 204; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRegisterMercadoPagoOAuthAttemptStoresItAndAnswers204(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.RegisterMercadoPagoOAuthAttempt(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, registerBody(testState, testVerifier)))

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204; got %d (%s)", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Errorf("a 204 carries no body; got %q", w.Body.String())
	}

	raw, err := f.attempts.Consume(t.Context(), mpOAuthAttemptScope(complex.ID, owner), testState)
	if err != nil {
		t.Fatalf("the attempt was not stored: %v", err)
	}
	var got struct {
		ComplexID    uuid.UUID `json:"complex_id"`
		UserID       uuid.UUID `json:"user_id"`
		CodeVerifier string    `json:"code_verifier"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("the stored attempt is not JSON: %v (%s)", err, raw)
	}
	if got.ComplexID != complex.ID || got.UserID != owner || got.CodeVerifier != testVerifier {
		t.Errorf("stored the wrong attempt; got %+v", got)
	}
}

func TestRegisterMercadoPagoOAuthAttemptTwiceOverwrites(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 1}

	other := strings.Repeat("b", 43)
	register(t, f, owner, complex, testState, testVerifier)
	register(t, f, owner, complex, testState, other)

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.gotVerifier != other {
		t.Errorf("the second registration must win; exchanged with %q", f.payments.gotVerifier)
	}
}

func TestRegisterMercadoPagoOAuthAttemptValidates(t *testing.T) {
	cases := map[string]struct {
		state, verifier string
		field           string
	}{
		"missing state":           {"", testVerifier, "state"},
		"state too short":         {strings.Repeat("s", 15), testVerifier, "state"},
		"state too long":          {strings.Repeat("s", 129), testVerifier, "state"},
		"missing verifier":        {testState, "", "code_verifier"},
		"verifier too short":      {testState, strings.Repeat("v", 42), "code_verifier"},
		"verifier too long":       {testState, strings.Repeat("v", 129), "code_verifier"},
		"verifier reserved chars": {testState, strings.Repeat("v", 42) + "+", "code_verifier"},
		"verifier with a space":   {testState, strings.Repeat("v", 42) + " ", "code_verifier"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			owner := uuid.New()
			complex := &complexstore.Complex{ID: uuid.New()}

			w := httptest.NewRecorder()
			f.handler.RegisterMercadoPagoOAuthAttempt(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, registerBody(tc.state, tc.verifier)))

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"`+tc.field+`"`) {
				t.Errorf("the error must name %q; got %s", tc.field, w.Body.String())
			}
			if tc.state != "" {
				if _, err := f.attempts.Consume(t.Context(), mpOAuthAttemptScope(complex.ID, owner), tc.state); !errors.Is(err, ErrOAuthAttemptInvalid) {
					t.Errorf("a rejected registration must store nothing; got %v", err)
				}
			}
		})
	}
}

func TestRegisterMercadoPagoOAuthAttemptAcceptsTheBoundaries(t *testing.T) {
	for _, tc := range []struct{ state, verifier string }{
		{strings.Repeat("s", 16), strings.Repeat("v", 43)},
		{strings.Repeat("s", 128), strings.Repeat("v", 128)},
	} {
		f := newFixture(t)
		register(t, f, uuid.New(), &complexstore.Complex{ID: uuid.New()}, tc.state, tc.verifier)
	}
}

func TestConnectMercadoPagoWithStateUsesTheStoredVerifier(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 12345}
	register(t, f, owner, complex, testState, testVerifier)

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.gotVerifier != testVerifier {
		t.Errorf("the exchange must use the stored verifier; got %q", f.payments.gotVerifier)
	}
	if f.store.mpCredentials == nil || f.store.mpCredentials.access != "AT" {
		t.Errorf("the credentials were not stored; got %+v", f.store.mpCredentials)
	}
}

func TestConnectMercadoPagoStateIsSingleUse(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 12345}
	register(t, f, owner, complex, testState, testVerifier)

	first := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(first, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))
	if first.Code != http.StatusOK {
		t.Fatalf("first use: want 200; got %d (%s)", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(second, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))
	assertStateRejected(t, second)
	if f.payments.calls != 1 {
		t.Errorf("a spent state must not reach MercadoPago; exchanges = %d", f.payments.calls)
	}
}

func TestConnectMercadoPagoRejectsAnUnknownState(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil, connectWithState(testState)))

	assertStateRejected(t, w)
	if f.payments.calls != 0 {
		t.Errorf("an unknown state must not reach MercadoPago; exchanges = %d", f.payments.calls)
	}
}

func TestConnectMercadoPagoRejectsAnotherOwnersAttempt(t *testing.T) {
	f := newFixture(t)
	complex := &complexstore.Complex{ID: uuid.New()}
	register(t, f, uuid.New(), complex, testState, testVerifier)

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil, connectWithState(testState)))

	assertStateRejected(t, w)
	if f.payments.calls != 0 {
		t.Errorf("another owner's attempt must not reach MercadoPago; exchanges = %d", f.payments.calls)
	}
	if f.store.mpCredentials != nil {
		t.Error("nothing must be stored")
	}
}

func TestConnectMercadoPagoRejectsAnAttemptForAnotherComplex(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	register(t, f, owner, &complexstore.Complex{ID: uuid.New()}, testState, testVerifier)

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", owner, &complexstore.Complex{ID: uuid.New()}, nil, connectWithState(testState)))

	assertStateRejected(t, w)
	if f.payments.calls != 0 {
		t.Errorf("an attempt for another complex must not reach MercadoPago; exchanges = %d", f.payments.calls)
	}
}

// Old cached clients still send their own verifier and no state.
func TestConnectMercadoPagoWithAVerifierIsUnchanged(t *testing.T) {
	f := newFixture(t)
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 1}
	complex := &complexstore.Complex{ID: uuid.New()}

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil, connectBody))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.gotVerifier != "verifier" {
		t.Errorf("the client's verifier must be used; got %q", f.payments.gotVerifier)
	}
}

// A client verifier wins over the stored one, but the attempt is still spent;
// an unknown state beside a verifier is not an error.
func TestConnectMercadoPagoWithAVerifierAndStateSpendsTheAttempt(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 1}
	register(t, f, owner, complex, testState, testVerifier)

	body := `{"code":"auth-code","redirect_uri":"https://vibe.test/mp/callback","code_verifier":"verifier","state":"` + testState + `"}`
	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, body))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.gotVerifier != "verifier" {
		t.Errorf("the client's verifier must be used; got %q", f.payments.gotVerifier)
	}
	if _, err := f.attempts.Consume(t.Context(), mpOAuthAttemptScope(complex.ID, owner), testState); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("the attempt must be spent; got %v", err)
	}

	again := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(again, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, body))
	if again.Code != http.StatusOK {
		t.Errorf("an unknown state beside a verifier is ignored; got %d (%s)", again.Code, again.Body.String())
	}
}

// Someone else's request naming the state must not cost its owner the attempt.
func TestConnectMercadoPagoMismatchLeavesTheAttemptToItsOwner(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 1}
	register(t, f, owner, complex, testState, testVerifier)

	stranger := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(stranger, ownerRequest(t, http.MethodPost, "/", uuid.New(), complex, nil, connectWithState(testState)))
	assertStateRejected(t, stranger)

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))
	if w.Code != http.StatusOK {
		t.Fatalf("the owner must still be able to use the attempt; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.gotVerifier != testVerifier {
		t.Errorf("exchanged with %q", f.payments.gotVerifier)
	}
}

// MercadoPago failing on its side is not the owner's doing: reloading the
// callback must be able to try the same attempt again.
func TestConnectMercadoPagoUnavailableKeepsTheAttempt(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	register(t, f, owner, complex, testState, testVerifier)
	f.payments.err = fmt.Errorf("%w: status 503", mp.ErrOAuthUnavailable)

	failed := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(failed, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))
	if failed.Code < 500 {
		t.Fatalf("want a 5xx while MercadoPago is down; got %d (%s)", failed.Code, failed.Body.String())
	}

	f.payments.err = nil
	f.payments.tokens = &mp.OAuthTokens{AccessToken: "AT", RefreshToken: "RT", UserID: 1}
	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))
	if w.Code != http.StatusOK {
		t.Fatalf("the retry must find the attempt; got %d (%s)", w.Code, w.Body.String())
	}
	if f.payments.gotVerifier != testVerifier {
		t.Errorf("exchanged with %q", f.payments.gotVerifier)
	}
}

// A code MercadoPago refused is spent at MercadoPago too: so is the attempt.
func TestConnectMercadoPagoRejectedCodeSpendsTheAttempt(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	register(t, f, owner, complex, testState, testVerifier)
	f.payments.err = fmt.Errorf("%w: invalid_grant", mp.ErrOAuthRejected)

	w := httptest.NewRecorder()
	f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", owner, complex, nil, connectWithState(testState)))

	if _, err := f.attempts.Consume(t.Context(), mpOAuthAttemptScope(complex.ID, owner), testState); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("a rejected code must spend the attempt; got %v", err)
	}
}

// Connect and register agree on what a state looks like.
func TestConnectMercadoPagoValidatesTheState(t *testing.T) {
	for name, state := range map[string]string{
		"state too short": strings.Repeat("s", 15),
		"state too long":  strings.Repeat("s", 129),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			w := httptest.NewRecorder()
			f.handler.ConnectMercadoPago(w, ownerRequest(t, http.MethodPost, "/", uuid.New(), &complexstore.Complex{ID: uuid.New()}, nil, connectWithState(state)))
			assertStateRejected(t, w)
			if !strings.Contains(w.Body.String(), "between 16 and 128 characters") {
				t.Errorf("want the register endpoint's message; got %s", w.Body.String())
			}
			if f.payments.calls != 0 {
				t.Errorf("an invalid state must not reach MercadoPago; exchanges = %d", f.payments.calls)
			}
		})
	}
}

// One owner keeps at most mpOAuthAttemptsPerOwner live attempts; registering
// one more drops the oldest.
func TestRegisterMercadoPagoOAuthAttemptKeepsTheNewestPerOwner(t *testing.T) {
	f := newFixture(t)
	owner := uuid.New()
	complex := &complexstore.Complex{ID: uuid.New()}
	states := make([]string, mpOAuthAttemptsPerOwner+1)
	for i := range states {
		states[i] = fmt.Sprintf("state-%02d-0123456789", i)
		register(t, f, owner, complex, states[i], testVerifier)
	}

	scope := mpOAuthAttemptScope(complex.ID, owner)
	if _, err := f.attempts.Consume(t.Context(), scope, states[0]); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("the oldest attempt must be dropped; got %v", err)
	}
	for _, state := range states[1:] {
		if _, err := f.attempts.Consume(t.Context(), scope, state); err != nil {
			t.Errorf("attempt %s must be kept; got %v", state, err)
		}
	}
}

func assertStateRejected(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422; got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"state"`) {
		t.Errorf("the error must name state; got %s", w.Body.String())
	}
}

// --- store ---

func TestMPOAuthAttemptsInMemoryIsSingleUse(t *testing.T) {
	s := NewMPOAuthAttempts(nil, "test")
	if err := s.Store(t.Context(), "sc", "k", []byte("v"), time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := s.Consume(t.Context(), "sc", "k")
	if err != nil || string(got) != "v" {
		t.Fatalf("first consume: got %q, %v", got, err)
	}
	if _, err := s.Consume(t.Context(), "sc", "k"); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("second consume: want ErrOAuthAttemptInvalid; got %v", err)
	}
}

func TestMPOAuthAttemptsInMemoryExpires(t *testing.T) {
	s := NewMPOAuthAttempts(nil, "test")
	if err := s.Store(t.Context(), "sc", "k", []byte("v"), time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, err := s.Consume(t.Context(), "sc", "k"); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("an expired attempt: want ErrOAuthAttemptInvalid; got %v", err)
	}
}

func TestMPOAuthAttemptsInMemorySweepsExpiredEntries(t *testing.T) {
	s := NewMPOAuthAttempts(nil, "test")
	_ = s.Store(t.Context(), "sc", "old", []byte("v"), time.Nanosecond)
	time.Sleep(time.Millisecond)
	_ = s.Store(t.Context(), "sc", "new", []byte("v"), time.Minute)
	if _, ok := s.entries[s.key("sc", "old")]; ok {
		t.Error("storing must sweep expired attempts")
	}
}

func TestMPOAuthAttemptsRedisIsNamespacedSingleUseAndExpires(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := platformredis.NewClient(&platformredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewMPOAuthAttempts(rdb, "test")

	if err := s.Store(t.Context(), "sc", "k", []byte("v"), time.Hour); err != nil {
		t.Fatal(err)
	}
	const key = "vibe:test:mpoauth:sc:k"
	if !mr.Exists(key) {
		t.Fatalf("want key %q; have %v", key, mr.Keys())
	}
	if ttl := mr.TTL(key); ttl != time.Hour {
		t.Errorf("want a 1h TTL; got %v", ttl)
	}
	got, err := s.Consume(t.Context(), "sc", "k")
	if err != nil || string(got) != "v" {
		t.Fatalf("consume: got %q, %v", got, err)
	}
	if _, err := s.Consume(t.Context(), "sc", "k"); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("second consume: want ErrOAuthAttemptInvalid; got %v", err)
	}

	_ = s.Store(t.Context(), "sc", "e", []byte("v"), time.Hour)
	mr.FastForward(time.Hour + time.Second)
	if _, err := s.Consume(t.Context(), "sc", "e"); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("expired: want ErrOAuthAttemptInvalid; got %v", err)
	}
}

func TestMPOAuthAttemptsInMemoryCapsEachScope(t *testing.T) {
	s := NewMPOAuthAttempts(nil, "test")
	for i := 0; i <= mpOAuthAttemptsPerOwner; i++ {
		_ = s.Store(t.Context(), "sc", fmt.Sprint(i), []byte("v"), time.Minute)
	}
	_ = s.Store(t.Context(), "other", "x", []byte("v"), time.Minute)
	// Registering a kept state again refreshes it rather than taking a second slot.
	_ = s.Store(t.Context(), "sc", "1", []byte("v2"), time.Minute)

	if _, err := s.Consume(t.Context(), "sc", "0"); !errors.Is(err, ErrOAuthAttemptInvalid) {
		t.Errorf("the oldest must be dropped; got %v", err)
	}
	for i := 1; i <= mpOAuthAttemptsPerOwner; i++ {
		if _, err := s.Consume(t.Context(), "sc", fmt.Sprint(i)); err != nil {
			t.Errorf("attempt %d must be kept; got %v", i, err)
		}
	}
	if _, err := s.Consume(t.Context(), "other", "x"); err != nil {
		t.Errorf("another scope is not affected; got %v", err)
	}
}

func TestMPOAuthAttemptsRedisCapsEachScope(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := platformredis.NewClient(&platformredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewMPOAuthAttempts(rdb, "test")

	for i := 0; i <= mpOAuthAttemptsPerOwner; i++ {
		if err := s.Store(t.Context(), "sc", fmt.Sprint(i), []byte("v"), time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	_ = s.Store(t.Context(), "sc", "1", []byte("v2"), time.Hour)

	if mr.Exists("vibe:test:mpoauth:sc:0") {
		t.Error("the oldest attempt must be deleted")
	}
	if ttl := mr.TTL("vibe:test:mpoauthidx:sc"); ttl <= 0 || ttl > time.Hour {
		t.Errorf("the index must expire with its attempts; TTL %v", ttl)
	}
	for i := 1; i <= mpOAuthAttemptsPerOwner; i++ {
		if _, err := s.Consume(t.Context(), "sc", fmt.Sprint(i)); err != nil {
			t.Errorf("attempt %d must be kept; got %v", i, err)
		}
	}
}

// The service stores attempts for one hour.
func TestRegisterMercadoPagoOAuthAttemptUsesAOneHourTTL(t *testing.T) {
	if mpOAuthAttemptTTL != time.Hour {
		t.Errorf("want 1h; got %v", mpOAuthAttemptTTL)
	}
}

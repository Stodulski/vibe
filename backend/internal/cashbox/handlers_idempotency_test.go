package cashbox

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/middleware"
)

// newIdempotencyGuard is the same harness internal/middleware/idempotency_test.go
// uses: a live miniredis, because the SET NX claim, the TTLs and the replay
// path are most of what this middleware is, and a stub client would only
// prove the stub agrees with the code.
func newIdempotencyGuard(t *testing.T) *middleware.Idempotency {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return middleware.NewIdempotency(rdb, "test", respond)
}

// movementRequest builds a POST .../movements request from ownerID (the same
// caller must send the same actor on a retry, or the idempotency guard reads
// it as a different request reusing the key — see requestFingerprint,
// internal/middleware/idempotency.go), with sessionID bound the way the
// router binds it.
func movementRequest(t *testing.T, ownerID, complexID, sessionID uuid.UUID, body, idempotencyKey string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/complexes/"+complexID.String()+"/cash-sessions/"+sessionID.String()+"/movements",
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		r.Header.Set("Idempotency-Key", idempotencyKey)
	}
	r = httpx.ContextSetUser(r, &authstore.User{ID: ownerID, Role: "owner"})
	r = httpx.ContextSetComplex(r, &complexstore.Complex{ID: complexID})
	r.SetPathValue("sessionID", sessionID.String())
	return r
}

// TestCreateMovementReplayedWithTheSameKeyInsertsOnlyOneRow is the point of
// wiring cashbox-movement into routeGuards' idempotentKey table (owner
// correction, pos-cashbox T2 review): a double tap at the counter — the same
// Idempotency-Key sent twice — must record the movement once and replay the
// first answer for the retry, not create two rows.
func TestCreateMovementReplayedWithTheSameKeyInsertsOnlyOneRow(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store, &stubPayments{})
	idem := newIdempotencyGuard(t)
	guarded := idem.Guard("cashbox-movement")(h.CreateMovement)

	ownerID, complexID, sessionID := uuid.New(), uuid.New(), uuid.New()
	body := `{"kind":"income","category":"other_income","method":"cash","amount":1000}`
	const key = "counter-tap-0001"

	first := httptest.NewRecorder()
	guarded(first, movementRequest(t, ownerID, complexID, sessionID, body, key))

	second := httptest.NewRecorder()
	guarded(second, movementRequest(t, ownerID, complexID, sessionID, body, key))

	if len(store.insertedMovements) != 1 {
		t.Fatalf("want exactly one movement inserted; got %d", len(store.insertedMovements))
	}
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("statuses = %d and %d, want 201 twice", first.Code, second.Code)
	}
	if first.Body.String() != second.Body.String() {
		t.Errorf("replay returned %q, want the first answer %q", second.Body.String(), first.Body.String())
	}
	if second.Header().Get("Idempotent-Replay") != "true" {
		t.Error("the second request must be marked as a replay")
	}
	if first.Header().Get("Idempotent-Replay") != "" {
		t.Error("the first request must not be marked as a replay")
	}
}

// TestCreateMovementWithDifferentKeysInsertsTwoRows is
// TestCreateMovementReplayedWithTheSameKeyInsertsOnlyOneRow's control: two
// genuinely different requests (different keys) must both run.
func TestCreateMovementWithDifferentKeysInsertsTwoRows(t *testing.T) {
	store := &stubStore{}
	h, _ := newTestHandler(store, &stubPayments{})
	idem := newIdempotencyGuard(t)
	guarded := idem.Guard("cashbox-movement")(h.CreateMovement)

	ownerID, complexID, sessionID := uuid.New(), uuid.New(), uuid.New()
	body := `{"kind":"income","category":"other_income","method":"cash","amount":1000}`

	guarded(httptest.NewRecorder(), movementRequest(t, ownerID, complexID, sessionID, body, "key-a"))
	guarded(httptest.NewRecorder(), movementRequest(t, ownerID, complexID, sessionID, body, "key-b"))

	if len(store.insertedMovements) != 2 {
		t.Fatalf("want two movements inserted for two different keys; got %d", len(store.insertedMovements))
	}
}

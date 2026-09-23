package sales

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
	salestore "github.com/stodulski/vibe-server/internal/sales/store"
)

// newIdempotencyGuard is the same harness internal/products' and
// internal/cashbox's own copies use: a live miniredis, because the SET NX
// claim, the TTLs and the replay path are most of what this middleware is.
func newIdempotencyGuard(t *testing.T) *middleware.Idempotency {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return middleware.NewIdempotency(rdb, "test", respond)
}

// createRequest builds a POST .../sales request from ownerID (the same
// caller must send the same actor on a retry, or the idempotency guard reads
// it as a different request reusing the key — see requestFingerprint,
// internal/middleware/idempotency.go).
func createRequest(t *testing.T, ownerID, complexID uuid.UUID, body, idempotencyKey string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/complexes/"+complexID.String()+"/sales", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		r.Header.Set("Idempotency-Key", idempotencyKey)
	}
	r = httpx.ContextSetUser(r, &authstore.User{ID: ownerID, Role: "owner"})
	r = httpx.ContextSetComplex(r, &complexstore.Complex{ID: complexID})
	return r
}

// TestCreateReplayedWithTheSameKeyRecordsOnlyOneSale is the point of wiring
// sales-create into routeGuards' idempotentKey table: a double tap at the
// counter — the same Idempotency-Key sent twice — must record the sale once
// and replay the first answer for the retry, not sell twice.
func TestCreateReplayedWithTheSameKeyRecordsOnlyOneSale(t *testing.T) {
	store := &stubStore{createSale: &salestore.Sale{ID: uuid.New(), Total: 800}}
	h, _ := newTestHandler(store)
	idem := newIdempotencyGuard(t)
	guarded := idem.Guard("sales-create")(h.Create)

	ownerID, complexID, productID := uuid.New(), uuid.New(), uuid.New()
	body := `{"items":[{"product_id":"` + productID.String() + `","quantity":1}],"method":"cash"}`
	const key = "counter-tap-0001"

	first := httptest.NewRecorder()
	guarded(first, createRequest(t, ownerID, complexID, body, key))

	second := httptest.NewRecorder()
	guarded(second, createRequest(t, ownerID, complexID, body, key))

	if store.createCalls != 1 {
		t.Fatalf("want exactly one sale recorded; got %d", store.createCalls)
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

// TestCreateWithDifferentKeysRunsTwice is
// TestCreateReplayedWithTheSameKeyRecordsOnlyOneSale's control: two
// genuinely different requests (different keys) must both run.
func TestCreateWithDifferentKeysRunsTwice(t *testing.T) {
	store := &stubStore{createSale: &salestore.Sale{ID: uuid.New(), Total: 800}}
	h, _ := newTestHandler(store)
	idem := newIdempotencyGuard(t)
	guarded := idem.Guard("sales-create")(h.Create)

	ownerID, complexID, productID := uuid.New(), uuid.New(), uuid.New()
	body := `{"items":[{"product_id":"` + productID.String() + `","quantity":1}],"method":"cash"}`

	guarded(httptest.NewRecorder(), createRequest(t, ownerID, complexID, body, "key-a"))
	guarded(httptest.NewRecorder(), createRequest(t, ownerID, complexID, body, "key-b"))

	if store.createCalls != 2 {
		t.Fatalf("want two sales recorded for two different keys; got %d", store.createCalls)
	}
}

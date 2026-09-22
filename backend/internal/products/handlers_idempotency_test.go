package products

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
	productstore "github.com/stodulski/vibe-server/internal/products/store"
)

// newIdempotencyGuard is the same harness internal/cashbox's own copy uses: a
// live miniredis, because the SET NX claim, the TTLs and the replay path are
// most of what this middleware is.
func newIdempotencyGuard(t *testing.T) *middleware.Idempotency {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })

	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return middleware.NewIdempotency(rdb, "test", respond)
}

// restockRequest builds a POST .../restock request from ownerID (the same
// caller must send the same actor on a retry, or the idempotency guard reads
// it as a different request reusing the key — see requestFingerprint,
// internal/middleware/idempotency.go), with productID bound the way the
// router binds it.
func restockRequest(t *testing.T, ownerID, complexID, productID uuid.UUID, body, idempotencyKey string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/complexes/"+complexID.String()+"/products/"+productID.String()+"/restock",
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		r.Header.Set("Idempotency-Key", idempotencyKey)
	}
	r = httpx.ContextSetUser(r, &authstore.User{ID: ownerID, Role: "owner"})
	r = httpx.ContextSetComplex(r, &complexstore.Complex{ID: complexID})
	r.SetPathValue("productID", productID.String())
	return r
}

// TestRestockReplayedWithTheSameKeyRecordsOnlyOnce is the point of wiring
// products-restock into routeGuards' idempotentKey table: a double tap at
// the counter — the same Idempotency-Key sent twice — must record the
// restock once and replay the first answer for the retry, not restock twice.
func TestRestockReplayedWithTheSameKeyRecordsOnlyOnce(t *testing.T) {
	store := &stubStore{
		restockProduct:  &productstore.Product{ID: uuid.New(), StockOnHand: 24},
		restockMovement: &productstore.StockMovement{ID: uuid.New(), Kind: "restock", Quantity: 24},
	}
	h, _ := newTestHandler(store)
	idem := newIdempotencyGuard(t)
	guarded := idem.Guard("products-restock")(h.Restock)

	ownerID, complexID, productID := uuid.New(), uuid.New(), uuid.New()
	body := `{"quantity":24,"total_cost":12000,"method":"cash"}`
	const key = "counter-tap-0001"

	first := httptest.NewRecorder()
	guarded(first, restockRequest(t, ownerID, complexID, productID, body, key))

	second := httptest.NewRecorder()
	guarded(second, restockRequest(t, ownerID, complexID, productID, body, key))

	if store.restockCallCount != 1 {
		t.Fatalf("want exactly one restock recorded; got %d", store.restockCallCount)
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

// TestRestockWithDifferentKeysRunsTwice is
// TestRestockReplayedWithTheSameKeyRecordsOnlyOnce's control: two genuinely
// different requests (different keys) must both run.
func TestRestockWithDifferentKeysRunsTwice(t *testing.T) {
	store := &stubStore{
		restockProduct:  &productstore.Product{ID: uuid.New()},
		restockMovement: &productstore.StockMovement{ID: uuid.New()},
	}
	h, _ := newTestHandler(store)
	idem := newIdempotencyGuard(t)
	guarded := idem.Guard("products-restock")(h.Restock)

	ownerID, complexID, productID := uuid.New(), uuid.New(), uuid.New()
	body := `{"quantity":10,"total_cost":5000,"method":"cash"}`

	guarded(httptest.NewRecorder(), restockRequest(t, ownerID, complexID, productID, body, "key-a"))
	guarded(httptest.NewRecorder(), restockRequest(t, ownerID, complexID, productID, body, "key-b"))

	if store.restockCallCount != 2 {
		t.Fatalf("want two restocks recorded for two different keys; got %d", store.restockCallCount)
	}
}

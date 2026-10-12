//go:build integration

package main

import (
	"fmt"
	"net/http"
	"testing"
)

// The public page of a deleted complex answers 410 with only its name, city and
// city hub key, and that stays true until a live complex takes the slug: the
// route is cross-tenant, so the lookup has to see the deleted row through the
// bypass the route is granted, and the new complex's page has to answer 200.
func TestIntegration_PublicPageOfADeletedComplexAnswersGone(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	ownerA, csrfA := registerAndLogin(t, ts, "gone-owner-a@test.com", "TestPass123!")
	status, _, result := integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"Club Viejo","slug":"club-viejo","address":"Av. Viejo 1","city":"Caleta Olivia","province":"Santa Cruz","phone":"+5491100000011","cancellation_hours":24}`,
		ownerA, csrfA)
	if status != http.StatusCreated {
		t.Fatalf("create complex: expected 201, got %d", status)
	}
	complexID := result["complex"].(map[string]any)["id"].(string)

	status, _, _ = integrationGet(t, ts, "/api/v1/public/complexes/club-viejo", nil, "")
	if status != http.StatusOK {
		t.Fatalf("live page: expected 200, got %d", status)
	}

	status, _, _ = integrationDo(t, ts, http.MethodDelete, "/api/v1/complexes/"+complexID, "", ownerA, csrfA)
	if status != http.StatusOK {
		t.Fatalf("delete complex: expected 200, got %d", status)
	}

	status, _, body := integrationGet(t, ts, "/api/v1/public/complexes/club-viejo", nil, "")
	if status != http.StatusGone {
		t.Fatalf("deleted page: expected 410, got %d (%v)", status, body)
	}
	assertExactGoneBody(t, body, "Club Viejo", "Caleta Olivia", "caleta-olivia")

	status, _, _ = integrationGet(t, ts, "/api/v1/public/complexes/nunca-existio", nil, "")
	if status != http.StatusNotFound {
		t.Errorf("a slug that never existed: expected 404, got %d", status)
	}

	status, _, _ = integrationGet(t, ts, "/api/v1/public/complexes/club-viejo/availability?date=2026-12-01", nil, "")
	if status != http.StatusNotFound {
		t.Errorf("availability of a deleted complex keeps its 404: got %d", status)
	}

	ownerB, csrfB := registerAndLogin(t, ts, "gone-owner-b@test.com", "TestPass123!")
	status, _, _ = integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"Club Nuevo","slug":"club-viejo","address":"Av. Nuevo 2","city":"Rosario","province":"Santa Fe","phone":"+5491100000012","cancellation_hours":24}`,
		ownerB, csrfB)
	if status != http.StatusCreated {
		t.Fatalf("a new complex on the deleted slug: expected 201, got %d", status)
	}

	status, _, body = integrationGet(t, ts, "/api/v1/public/complexes/club-viejo", nil, "")
	if status != http.StatusOK {
		t.Fatalf("the slug reused by a live complex: expected 200, got %d (%v)", status, body)
	}
	complex, _ := body["complex"].(map[string]any)
	if complex["name"] != "Club Nuevo" {
		t.Errorf("the live page must be the new complex; got name %v", complex["name"])
	}
}

// assertExactGoneBody checks the 410 body carries exactly the gone object with
// exactly the three fields, and that every value is the deleted complex's own.
func assertExactGoneBody(t *testing.T, body map[string]any, name, city, citySlug string) {
	t.Helper()
	if len(body) != 1 {
		t.Fatalf("the 410 body must carry only \"gone\"; got keys %v", keysOf(body))
	}
	gone, ok := body["gone"].(map[string]any)
	if !ok {
		t.Fatalf("the 410 body must carry a \"gone\" object; got %v", body)
	}
	want := map[string]string{"name": name, "city": city, "city_slug": citySlug}
	if len(gone) != len(want) {
		t.Fatalf("the gone object must carry exactly name, city and city_slug; got keys %v", keysOf(gone))
	}
	for k, v := range want {
		if gone[k] != v {
			t.Errorf("gone.%s = %v; want %q", k, gone[k], v)
		}
	}
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, fmt.Sprint(k))
	}
	return keys
}

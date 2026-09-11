//go:build integration

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntegration_BlockedSlots(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)
	cleanupDB(t, pool)

	// Register and login.
	cookies, csrf := registerAndLogin(t, ts, "blocked-slots-test@test.com", "SecurePass123!")

	// Create a complex.
	complexBody := `{
		"name":"Blocked Slots Test Complex",
		"slug":"blocked-slots-test",
		"address":"Test Address",
		"city":"CABA",
		"province":"Buenos Aires",
		"phone":"+5491100000000",
		"deposit_percentage":30,
		"cancellation_hours":24
	}`
	status, _, complexResult := integrationPost(t, ts, "/api/v1/complexes", complexBody, cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create complex: want 201, got %d, body: %v", status, complexResult)
	}
	complex := complexResult["complex"].(map[string]any)
	complexID := complex["id"].(string)

	// Create a court.
	courtBody := `{"name":"Cancha Bloqueo","sport":"padel","court_type":"indoor"}`
	status, _, courtResult := integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts", complexID), courtBody, cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create court: want 201, got %d, body: %v", status, courtResult)
	}
	court := courtResult["court"].(map[string]any)
	courtID := court["id"].(string)

	// Dates are computed, not written down. This test carried a hardcoded
	// June 2026 and only ever ran once the harness was repointed — by which
	// time that month was in the past and the handler rejected every block
	// with "must not be in the past", correctly. A calendar month from now
	// keeps the two blocks and both list ranges moving together.
	firstOfNextMonth := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.UTC)
	const day = "2006-01-02"
	blockDate := firstOfNextMonth.AddDate(0, 0, 14).Format(day)
	blockDate2 := firstOfNextMonth.AddDate(0, 0, 19).Format(day)
	monthFrom, monthTo := firstOfNextMonth.Format(day), firstOfNextMonth.AddDate(0, 1, -1).Format(day)
	windowFrom := firstOfNextMonth.AddDate(0, 0, 17).Format(day)
	windowTo := firstOfNextMonth.AddDate(0, 0, 24).Format(day)

	// Block a slot.
	blockBody := fmt.Sprintf(`{"date":%q,"start_time":"18:00","end_time":"19:30","reason":"maintenance"}`, blockDate)
	status, _, blockResult := integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts/%s/block", complexID, courtID), blockBody, cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("block slot: want 201, got %d, body: %v", status, blockResult)
	}

	blockedSlot := blockResult["blocked_slot"].(map[string]any)
	slotID := blockedSlot["id"].(string)

	// Verify created_by is set.
	if blockedSlot["created_by"] == nil || blockedSlot["created_by"] == "" {
		t.Error("expected created_by to be set")
	}
	if blockedSlot["court_name"] != "Cancha Bloqueo" {
		t.Errorf("expected court_name 'Cancha Bloqueo'; got %v", blockedSlot["court_name"])
	}

	// Block a second slot on a different date.
	blockBody2 := fmt.Sprintf(`{"date":%q,"start_time":"10:00","end_time":"11:30","reason":"event"}`, blockDate2)
	status, _, _ = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts/%s/block", complexID, courtID), blockBody2, cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("block slot 2: want 201, got %d", status)
	}

	// List blocked slots for June.
	listPath := fmt.Sprintf("/api/v1/complexes/%s/blocked-slots?date_from=%s&date_to=%s", complexID, monthFrom, monthTo)
	status, _, listResult := integrationGet(t, ts, listPath, cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("list blocked slots: want 200, got %d, body: %v", status, listResult)
	}

	slots := listResult["blocked_slots"].([]any)
	if len(slots) != 2 {
		t.Errorf("want 2 blocked slots; got %d", len(slots))
	}

	// Verify first slot has court_name enrichment.
	firstSlot := slots[0].(map[string]any)
	if firstSlot["court_name"] != "Cancha Bloqueo" {
		t.Errorf("want court_name 'Cancha Bloqueo'; got %v", firstSlot["court_name"])
	}

	// List with different date range should return only matching slots.
	listPath2 := fmt.Sprintf("/api/v1/complexes/%s/blocked-slots?date_from=%s&date_to=%s", complexID, windowFrom, windowTo)
	status, _, listResult2 := integrationGet(t, ts, listPath2, cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("list blocked slots 2: want 200, got %d", status)
	}
	slots2 := listResult2["blocked_slots"].([]any)
	if len(slots2) != 1 {
		t.Errorf("want 1 blocked slot in date range; got %d", len(slots2))
	}

	// Delete the first blocked slot.
	deletePath := fmt.Sprintf("/api/v1/complexes/%s/blocked-slots/%s", complexID, slotID)
	status, _, deleteResult := integrationRequest(t, ts, http.MethodDelete, deletePath, "", cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("delete blocked slot: want 200, got %d, body: %v", status, deleteResult)
	}
	if deleteResult["message"] != "blocked slot deleted" {
		t.Errorf("want message 'blocked slot deleted'; got %v", deleteResult["message"])
	}

	// Verify only 1 slot remains.
	status, _, listResult3 := integrationGet(t, ts, listPath, cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("list after delete: want 200, got %d", status)
	}
	slots3 := listResult3["blocked_slots"].([]any)
	if len(slots3) != 1 {
		t.Errorf("want 1 blocked slot after delete; got %d", len(slots3))
	}

	// Try to delete with wrong complex ID.
	wrongComplexPath := fmt.Sprintf("/api/v1/complexes/%s/blocked-slots/%s", "00000000-0000-0000-0000-000000000000", slotID)
	status, _, _ = integrationRequest(t, ts, http.MethodDelete, wrongComplexPath, "", cookies, csrf)
	if status != http.StatusNotFound {
		t.Errorf("delete wrong complex: want 404, got %d", status)
	}
}

// integrationRequest sends a request with the given method.
//
// It was a fourth copy of the helper in testutils_integration_test.go, sharing
// that copy's dropped json.Unmarshal error; it now delegates so there is one
// place where a malformed response body fails the test instead of silently
// decoding to nil.
func integrationRequest(t *testing.T, ts *httptest.Server, method, path, body string, cookies []*http.Cookie, csrfToken string) (int, http.Header, map[string]any) {
	t.Helper()
	return integrationDo(t, ts, method, path, body, cookies, csrfToken)
}

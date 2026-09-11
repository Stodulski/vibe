//go:build integration

package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestIntegration_AuthRoundTrip(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Register
	status, _, _ := integrationPost(t, ts, "/api/v1/auth/register",
		`{"email":"integ@test.com","password":"TestPass123!","first_name":"Int","last_name":"Test","phone":"+5491100000001"}`,
		nil, "")
	if status != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", status)
	}

	// Login (should work because dev mode auto-verifies)
	status, headers, result := integrationPost(t, ts, "/api/v1/auth/login",
		`{"email":"integ@test.com","password":"TestPass123!"}`,
		nil, "")
	if status != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", status)
	}

	// Extract cookies and CSRF
	var cookies []*http.Cookie
	for _, line := range headers.Values("Set-Cookie") {
		if c, err := http.ParseSetCookie(line); err == nil {
			cookies = append(cookies, c)
		}
	}
	csrfToken, _ := result["csrf_token"].(string)
	if csrfToken == "" {
		t.Fatal("missing csrf_token in login response")
	}

	// Get current user
	status, _, meResult := integrationGet(t, ts, "/api/v1/auth/me", cookies, csrfToken)
	if status != http.StatusOK {
		t.Fatalf("get me: expected 200, got %d", status)
	}

	user, _ := meResult["user"].(map[string]any)
	if user["email"] != "integ@test.com" {
		t.Errorf("expected email integ@test.com, got %v", user["email"])
	}

	// Logout
	status, _, _ = integrationPost(t, ts, "/api/v1/auth/logout", "", cookies, csrfToken)
	if status != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", status)
	}

	// Verify access is denied after logout
	status, _, _ = integrationGet(t, ts, "/api/v1/auth/me", cookies, csrfToken)
	if status != http.StatusUnauthorized {
		t.Fatalf("after logout: expected 401, got %d", status)
	}
}

func TestIntegration_ComplexCRUD(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	cookies, csrf := registerAndLogin(t, ts, "complex-crud@test.com", "TestPass123!")

	// Create complex
	status, _, result := integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"Test Complex","slug":"test-complex","address":"Av. Test 123","city":"Buenos Aires","province":"Buenos Aires","phone":"+5491100000002","cancellation_hours":24}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create complex: expected 201, got %d", status)
	}

	complex, _ := result["complex"].(map[string]any)
	complexID, _ := complex["id"].(string)
	if complexID == "" {
		t.Fatal("missing complex ID")
	}

	// Get complex by ID
	status, _, result = integrationGet(t, ts, "/api/v1/complexes/"+complexID, cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("get complex: expected 200, got %d", status)
	}

	complex, _ = result["complex"].(map[string]any)
	if complex["name"] != "Test Complex" {
		t.Errorf("expected name 'Test Complex', got %v", complex["name"])
	}

	// Update complex
	status, _, _ = integrationPut(t, ts, "/api/v1/complexes/"+complexID,
		`{"name":"Updated Complex"}`,
		cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("update complex: expected 200, got %d", status)
	}

	// List complexes
	status, _, result = integrationGet(t, ts, "/api/v1/complexes", cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("list complexes: expected 200, got %d", status)
	}

	complexes, _ := result["complexes"].([]any)
	if len(complexes) != 1 {
		t.Fatalf("expected 1 complex, got %d", len(complexes))
	}
}

func TestIntegration_BookingLifecycle(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	cookies, csrf := registerAndLogin(t, ts, "booking-life@test.com", "TestPass123!")

	// Create complex
	status, _, result := integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"Booking Complex","slug":"booking-complex","address":"Av. Test 456","city":"Buenos Aires","province":"Buenos Aires","phone":"+5491100000003","cancellation_hours":24}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create complex: expected 201, got %d", status)
	}
	complex := result["complex"].(map[string]any)
	complexID := complex["id"].(string)

	// Create court
	status, _, result = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts", complexID),
		`{"name":"Cancha 1","sport":"padel","court_type":"indoor"}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create court: expected 201, got %d", status)
	}
	court := result["court"].(map[string]any)
	courtID := court["id"].(string)

	// Set schedules
	status, _, _ = integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/schedules", complexID),
		`{"schedules":[{"day":"monday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"tuesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"wednesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"thursday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"friday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"saturday","open_time":"09:00","close_time":"22:00","is_closed":false},{"day":"sunday","open_time":"09:00","close_time":"22:00","is_closed":false}]}`,
		cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("set schedules: expected 200, got %d", status)
	}

	// Set court prices
	status, _, _ = integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts/%s/prices", complexID, courtID),
		`{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"tuesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"wednesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"thursday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"friday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"saturday","time_from":"09:00","time_to":"22:00","price":20000},{"day_type":"sunday","time_from":"09:00","time_to":"22:00","price":20000}]}`,
		cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("set prices: expected 200, got %d", status)
	}

	// Create a booking for the next weekday.
	//
	// 11:00, not 10:00: the court above is sold in 90-minute slots and the
	// complex opens at 08:00, so its grid is 08:00, 09:30, 11:00, 12:30 —
	// 10:00 is not a position the availability endpoint ever offers. The
	// booking handlers now validate against that same grid (slots.Grid), so a
	// fixture booking 10:00 was asserting that the write path accepts what the
	// storefront does not.
	nextWeekday := getNextWeekday()
	bookingBody := fmt.Sprintf(`{
		"court_id":"%s",
		"date":"%s",
		"start_time":"11:00",
		"duration_minutes":90,
		"client_first_name":"Carlos",
		"client_last_name":"Test",
		"client_phone":"+5491100000004",
		"payment_method":"cash"
	}`, courtID, nextWeekday)

	status, _, result = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/bookings", complexID),
		bookingBody, cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create booking: expected 201, got %d", status)
	}

	booking := result["booking"].(map[string]any)
	bookingID := booking["id"].(string)
	if bookingID == "" {
		t.Fatal("missing booking ID")
	}

	// Get booking. The status alone proves only that the route answered; assert
	// the body carries the booking we just created, otherwise this step would
	// pass against any 200 the endpoint felt like returning.
	status, _, result = integrationGet(t, ts, fmt.Sprintf("/api/v1/complexes/%s/bookings/%s?date=%s", complexID, bookingID, nextWeekday),
		cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("get booking: expected 200, got %d", status)
	}
	fetched, ok := result["booking"].(map[string]any)
	if !ok {
		t.Fatalf("get booking: response has no booking object: %v", result)
	}
	if fetched["id"] != bookingID {
		t.Fatalf("get booking: expected booking %q, got %v", bookingID, fetched["id"])
	}

	// Confirm payment
	confirmBody := `{"method":"cash","amount":15000}`
	status, _, _ = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/bookings/%s/confirm-payment", complexID, bookingID),
		confirmBody, cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("confirm payment: expected 200, got %d", status)
	}

	// Cancel booking
	status, _, _ = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/bookings/%s/cancel", complexID, bookingID),
		`{"reason":"Testing cancellation"}`, cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("cancel booking: expected 200, got %d", status)
	}
}

func TestIntegration_PublicBookingFlow(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	// Setup: create owner, complex, court, schedule, prices
	cookies, csrf := registerAndLogin(t, ts, "public-flow@test.com", "TestPass123!")

	status, _, result := integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"Public Complex","slug":"public-complex","address":"Av. Public 789","city":"Buenos Aires","province":"Buenos Aires","phone":"+5491100000005","cancellation_hours":24}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create complex: expected 201, got %d", status)
	}
	complex := result["complex"].(map[string]any)
	complexID := complex["id"].(string)

	status, _, result = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts", complexID),
		`{"name":"Cancha Publica","sport":"padel","court_type":"outdoor"}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create court: expected 201, got %d", status)
	}
	court := result["court"].(map[string]any)
	courtID := court["id"].(string)

	status, _, _ = integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/schedules", complexID),
		`{"schedules":[{"day":"monday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"tuesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"wednesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"thursday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"friday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"saturday","open_time":"09:00","close_time":"22:00","is_closed":false},{"day":"sunday","open_time":"09:00","close_time":"22:00","is_closed":false}]}`,
		cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("set schedules: expected 200, got %d", status)
	}

	status, _, _ = integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts/%s/prices", complexID, courtID),
		`{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"tuesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"wednesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"thursday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"friday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"saturday","time_from":"09:00","time_to":"22:00","price":20000},{"day_type":"sunday","time_from":"09:00","time_to":"22:00","price":20000}]}`,
		cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("set prices: expected 200, got %d", status)
	}

	// Fake MercadoPago connection for public availability
	_, err := pool.Exec(context.Background(), "UPDATE complexes SET mp_user_id = 'fake-e2e', mp_access_token = 'fake-token' WHERE id = $1", complexID)
	if err != nil {
		t.Fatalf("fake MP connection: %v", err)
	}

	// Public: get complex by slug (no auth)
	status, _, result = integrationGet(t, ts, "/api/v1/public/complexes/public-complex", nil, "")
	if status != http.StatusOK {
		t.Fatalf("get public complex: expected 200, got %d", status)
	}

	publicComplex := result["complex"].(map[string]any)
	if publicComplex["name"] != "Public Complex" {
		t.Errorf("expected 'Public Complex', got %v", publicComplex["name"])
	}

	// Public: get availability (no auth)
	nextWeekday := getNextWeekday()
	status, _, result = integrationGet(t, ts, fmt.Sprintf("/api/v1/public/complexes/public-complex/availability?date=%s", nextWeekday), nil, "")
	if status != http.StatusOK {
		t.Fatalf("get availability: expected 200, got %d", status)
	}

	// Should have courts with slots (nested under "availability")
	availability, _ := result["availability"].(map[string]any)
	courts, _ := availability["courts"].([]any)
	if len(courts) == 0 {
		t.Fatalf("expected at least one court in availability response, got: %v", result)
	}

	// Note: Public booking creation requires real MercadoPago credentials
	// to create a payment preference. We skip the actual booking creation
	// in integration tests (tested via owner booking flow instead).
	// The availability endpoint above verifies the public API works correctly.
}

func TestIntegration_SlotCollision(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	cookies, csrf := registerAndLogin(t, ts, "collision@test.com", "TestPass123!")

	// Setup complex + court + schedule + prices
	status, _, result := integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"Collision Complex","slug":"collision-complex","address":"Av. Collis 1","city":"Buenos Aires","province":"Buenos Aires","phone":"+5491100000007","cancellation_hours":24}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create complex: expected 201, got %d", status)
	}
	complex := result["complex"].(map[string]any)
	complexID := complex["id"].(string)

	status, _, result = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts", complexID),
		`{"name":"Cancha Collis","sport":"padel","court_type":"indoor"}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create court: expected 201, got %d", status)
	}
	court := result["court"].(map[string]any)
	courtID := court["id"].(string)

	integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/schedules", complexID),
		`{"schedules":[{"day":"monday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"tuesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"wednesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"thursday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"friday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"saturday","open_time":"09:00","close_time":"22:00","is_closed":false},{"day":"sunday","open_time":"09:00","close_time":"22:00","is_closed":false}]}`,
		cookies, csrf)

	integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts/%s/prices", complexID, courtID),
		`{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"tuesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"wednesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"thursday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"friday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"saturday","time_from":"09:00","time_to":"22:00","price":20000},{"day_type":"sunday","time_from":"09:00","time_to":"22:00","price":20000}]}`,
		cookies, csrf)

	nextWeekday := getNextWeekday()

	// Create first booking at 11:00 — an actual position on this court's grid;
	// see the note in TestIntegration_BookingLifecycle. Both bookings use the
	// same start time, which is what makes the second one a collision.
	bookingBody := fmt.Sprintf(`{
		"court_id":"%s",
		"date":"%s",
		"start_time":"11:00",
		"duration_minutes":90,
		"client_first_name":"First",
		"client_last_name":"Booker",
		"client_phone":"+5491100000008",
		"payment_method":"cash"
	}`, courtID, nextWeekday)

	status, _, _ = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/bookings", complexID),
		bookingBody, cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("first booking: expected 201, got %d", status)
	}

	// Try to create second booking at the same time — should fail
	bookingBody2 := fmt.Sprintf(`{
		"court_id":"%s",
		"date":"%s",
		"start_time":"11:00",
		"duration_minutes":90,
		"client_first_name":"Second",
		"client_last_name":"Booker",
		"client_phone":"+5491100000009",
		"payment_method":"cash"
	}`, courtID, nextWeekday)

	status, _, _ = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/bookings", complexID),
		bookingBody2, cookies, csrf)
	if status != http.StatusConflict {
		t.Fatalf("slot collision: expected 409, got %d", status)
	}
}

func TestIntegration_ClientAutoCreation(t *testing.T) {
	pool := setupTestDB(t)
	cleanupDB(t, pool)

	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	cookies, csrf := registerAndLogin(t, ts, "client-auto@test.com", "TestPass123!")

	// Setup complex + court + schedule + prices
	status, _, result := integrationPost(t, ts, "/api/v1/complexes",
		`{"name":"Client Complex","slug":"client-complex","address":"Av. Client 1","city":"Buenos Aires","province":"Buenos Aires","phone":"+5491100000010","cancellation_hours":24}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create complex: expected 201, got %d", status)
	}
	complex := result["complex"].(map[string]any)
	complexID := complex["id"].(string)

	status, _, result = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts", complexID),
		`{"name":"Cancha Client","sport":"padel","court_type":"indoor"}`,
		cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create court: expected 201, got %d", status)
	}
	court := result["court"].(map[string]any)
	courtID := court["id"].(string)

	integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/schedules", complexID),
		`{"schedules":[{"day":"monday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"tuesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"wednesday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"thursday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"friday","open_time":"08:00","close_time":"23:00","is_closed":false},{"day":"saturday","open_time":"09:00","close_time":"22:00","is_closed":false},{"day":"sunday","open_time":"09:00","close_time":"22:00","is_closed":false}]}`,
		cookies, csrf)

	integrationPut(t, ts, fmt.Sprintf("/api/v1/complexes/%s/courts/%s/prices", complexID, courtID),
		`{"prices":[{"day_type":"monday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"tuesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"wednesday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"thursday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"friday","time_from":"08:00","time_to":"23:00","price":15000},{"day_type":"saturday","time_from":"09:00","time_to":"22:00","price":20000},{"day_type":"sunday","time_from":"09:00","time_to":"22:00","price":20000}]}`,
		cookies, csrf)

	// Create booking with client data
	nextWeekday := getNextWeekday()
	bookingBody := fmt.Sprintf(`{
		"court_id":"%s",
		"date":"%s",
		"start_time":"14:00",
		"duration_minutes":90,
		"client_first_name":"AutoCreated",
		"client_last_name":"Client",
		"client_phone":"+5491100000011",
		"client_email":"auto@test.com",
		"payment_method":"cash"
	}`, courtID, nextWeekday)

	status, _, _ = integrationPost(t, ts, fmt.Sprintf("/api/v1/complexes/%s/bookings", complexID),
		bookingBody, cookies, csrf)
	if status != http.StatusCreated {
		t.Fatalf("create booking: expected 201, got %d", status)
	}

	// Verify client was auto-created
	status, _, result = integrationGet(t, ts, fmt.Sprintf("/api/v1/complexes/%s/clients", complexID),
		cookies, csrf)
	if status != http.StatusOK {
		t.Fatalf("list clients: expected 200, got %d", status)
	}

	clients, _ := result["clients"].([]any)
	if len(clients) == 0 {
		t.Fatal("expected at least one client, got 0")
	}

	client := clients[0].(map[string]any)
	if client["first_name"] != "AutoCreated" {
		t.Errorf("expected client first_name 'AutoCreated', got %v", client["first_name"])
	}
}

func TestIntegration_HealthCheck(t *testing.T) {
	pool := setupTestDB(t)
	app := newIntegrationApp(t, pool)
	ts := newIntegrationServer(t, app)

	status, _, result := integrationGet(t, ts, "/api/v1/healthcheck", nil, "")
	if status != http.StatusOK {
		t.Fatalf("healthcheck: expected 200, got %d", status)
	}

	if result["status"] != "available" {
		t.Errorf("expected status 'available', got %v", result["status"])
	}
}

// getNextWeekday returns the next weekday date as YYYY-MM-DD.
func getNextWeekday() string {
	d := time.Now().AddDate(0, 0, 1)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d.Format("2006-01-02")
}

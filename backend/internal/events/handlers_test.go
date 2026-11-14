package events

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	eventstore "github.com/stodulski/vibe-server/internal/events/store"
	"github.com/stodulski/vibe-server/internal/httpx"
	paymentstore "github.com/stodulski/vibe-server/internal/payments/store"
	"github.com/stodulski/vibe-server/internal/pricing"
	"github.com/stodulski/vibe-server/internal/timezone"
)

func newHandler(f *fixture) *Handler {
	responder := httpx.NewResponder(f.svc.logger)
	return NewHandler(f.svc, responder, httpx.TrustedProxies{})
}

// ownerRequest is a request by the complex's authenticated owner.
func (f *fixture) ownerRequest(t *testing.T, method, target, body string, params map[string]string) *http.Request {
	t.Helper()
	r := f.publicRequest(t, method, target, body, params)
	r = httpx.ContextSetUser(r, &authstore.User{ID: f.owner, Role: "owner"})
	return httpx.ContextSetComplex(r, f.complex)
}

func (f *fixture) publicRequest(t *testing.T, method, target, body string, params map[string]string) *http.Request {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, target, nil)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range params {
		r.SetPathValue(k, v)
	}
	return r
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, w.Body.String())
	}
	return body
}

func tomorrowDate() string {
	return timezone.Day(time.Now()).AddDate(0, 0, 3).Format(time.DateOnly)
}

// S1: the owner publishes an event with the fields the owner gave, and the answer
// carries the event with its courts and spots.
func TestCreateAnswers201WithTheEvent(t *testing.T) {
	f := newFixture(t)
	h := newHandler(f)
	w := httptest.NewRecorder()
	h.Create(w, f.ownerRequest(t, http.MethodPost, "/", `{"name":"Canchas libres","date":"`+tomorrowDate()+
		`","start_time":"19:00","end_time":"21:00","capacity":20,"price":800000}`, nil))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	event := decodeBody(t, w)["event"].(map[string]any)
	if event["name"] != "Canchas libres" || event["end_time"] != "21:00" || event["duration_minutes"] != float64(120) ||
		event["capacity"] != float64(20) || event["spots_left"] != float64(20) || event["price"] != float64(800000) {
		t.Errorf("event = %v", event)
	}
}

func TestCreateAnswers422ForAnInvalidEventAnd409ForOccupiedHours(t *testing.T) {
	f := newFixture(t)
	h := newHandler(f)

	w := httptest.NewRecorder()
	h.Create(w, f.ownerRequest(t, http.MethodPost, "/", `{"name":"","date":"`+tomorrowDate()+
		`","start_time":"19:00","end_time":"21:00","capacity":0,"price":0}`, nil))
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "capacity") {
		t.Fatalf("want 422 naming the fields; got %d (%s)", w.Code, w.Body.String())
	}

	f.store.createErr = &eventstore.ConflictError{Reason: eventstore.ConflictBooking, CourtID: f.courts.courts[0].ID}
	w = httptest.NewRecorder()
	h.Create(w, f.ownerRequest(t, http.MethodPost, "/", `{"name":"Canchas libres","date":"`+tomorrowDate()+
		`","start_time":"19:00","end_time":"21:00","capacity":20,"price":800000}`, nil))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "a booking already holds those hours on Cancha 1") {
		t.Fatalf("want 409 saying what holds the hours; got %d (%s)", w.Code, w.Body.String())
	}
}

// S8: the owner sees the participant list with each payment.
func TestGetAnswersTheEventWithItsParticipantsAndPayments(t *testing.T) {
	f := newFixture(t)
	f.store.rows = []*eventstore.RegistrationRow{{
		Registration: eventstore.Registration{
			ID: uuid.New(), EventID: f.store.event.ID, FirstName: "Ana", LastName: "Diaz", Phone: "+5491100000000",
			Status: eventstore.RegistrationConfirmed, Price: 800_000, ServiceFee: 20_000, CreatedAt: time.Now(),
		},
		Payment: &eventstore.RegistrationPayment{Status: "deposit_paid", Amount: 800_000, ServiceFee: 20_000},
	}, {
		Registration: eventstore.Registration{
			ID: uuid.New(), EventID: f.store.event.ID, FirstName: "Beto", LastName: "Paz", Phone: "+5491100000001",
			Status: eventstore.RegistrationHeld, Price: 800_000, CreatedAt: time.Now(),
		},
	}}
	h := newHandler(f)
	w := httptest.NewRecorder()
	h.Get(w, f.ownerRequest(t, http.MethodGet, "/", "", map[string]string{"eventID": f.store.event.ID.String()}))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	participants := body["participants"].([]any)
	if len(participants) != 2 {
		t.Fatalf("participants = %v", participants)
	}
	paid := participants[0].(map[string]any)
	payment := paid["payment"].(map[string]any)
	if paid["phone"] != "+5491100000000" || paid["status"] != "confirmed" || payment["status"] != "deposit_paid" ||
		payment["amount"] != float64(800000) || payment["method"] != "mercadopago" {
		t.Errorf("participant = %v", paid)
	}
	if _, has := participants[1].(map[string]any)["payment"]; has {
		t.Error("a hold that never opened a checkout carries no payment")
	}
}

func TestOwnerCancelAnswersTheRefundOfEachPaidParticipant(t *testing.T) {
	f := newFixture(t)
	reg := &eventstore.Registration{
		ID: uuid.New(), EventID: f.store.event.ID, ComplexID: f.complex.ID, Status: eventstore.RegistrationConfirmed,
		Price: 800_000, ServiceFee: 20_000,
	}
	f.store.remember(reg, "t")
	f.store.paid = []uuid.UUID{reg.ID}
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}
	h := newHandler(f)

	w := httptest.NewRecorder()
	h.Cancel(w, f.ownerRequest(t, http.MethodPost, "/", `{"reason":"lluvia"}`, map[string]string{"eventID": f.store.event.ID.String()}))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200; got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	refunds := body["refunds"].([]any)
	refund := refunds[0].(map[string]any)["refund"].(map[string]any)
	if body["event"].(map[string]any)["status"] != "cancelled" || refund["status"] != "issued" || refund["amount"] != float64(820000) ||
		refunds[0].(map[string]any)["registration_id"] != reg.ID.String() {
		t.Errorf("body = %v", body)
	}

	f.store.cancelErr = eventstore.ErrEventNotActive
	w = httptest.NewRecorder()
	h.Cancel(w, f.ownerRequest(t, http.MethodPost, "/", "", map[string]string{"eventID": f.store.event.ID.String()}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a second cancel is 400; got %d (%s)", w.Code, w.Body.String())
	}
}

// S3/S7: the public page lists the upcoming events, and the sign-up answers the
// checkout and the credential, never the registration's primary key.
func TestPublicListAndSignUpAnswerWhatThePageNeeds(t *testing.T) {
	f := newFixture(t)
	f.store.event.Courts = nil
	h := newHandler(f)

	w := httptest.NewRecorder()
	h.PublicList(w, f.publicRequest(t, http.MethodGet, "/", "", map[string]string{"slug": "vibe"}))
	events := decodeBody(t, w)["events"].([]any)
	if w.Code != http.StatusOK || len(events) != 1 {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}
	listed := events[0].(map[string]any)
	if listed["spots_left"] != float64(20) || listed["total_price"] != float64(800000+pricing.ServiceFee(0)) ||
		listed["service_fee"] != float64(pricing.ServiceFee(0)) {
		t.Errorf("listed = %v", listed)
	}

	w = httptest.NewRecorder()
	h.PublicSignUp(w, f.publicRequest(t, http.MethodPost, "/",
		`{"first_name":"Ana","last_name":"Diaz","phone":"+54 9 11 1234-5678"}`,
		map[string]string{"slug": "vibe", "eventID": f.store.event.ID.String()}))
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["mp_init_point"] != "https://mp.test/pay" || body["token"] == "" || body["total_price"] == nil ||
		body["registration"].(map[string]any)["status"] != "held" {
		t.Errorf("body = %v", body)
	}
	if strings.Contains(w.Body.String(), f.store.signedUp[0].EventID.String()+`","token`) {
		t.Error("the registration's own id must never be in the response")
	}
	if f.store.signedUp[0].Phone != "+5491112345678" {
		t.Errorf("the phone must be normalized to E.164; got %q", f.store.signedUp[0].Phone)
	}
}

func TestPublicSignUpValidatesTheRequiredFieldsAndAnOptionalEmail(t *testing.T) {
	f := newFixture(t)
	h := newHandler(f)
	params := map[string]string{"slug": "vibe", "eventID": f.store.event.ID.String()}

	w := httptest.NewRecorder()
	h.PublicSignUp(w, f.publicRequest(t, http.MethodPost, "/", `{"first_name":"","last_name":"Diaz","phone":"abc"}`, params))
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "first_name") || !strings.Contains(w.Body.String(), "phone") {
		t.Fatalf("want 422 on the name and phone; got %d (%s)", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.PublicSignUp(w, f.publicRequest(t, http.MethodPost, "/", `{"first_name":"Ana","last_name":"Diaz","phone":"+5491112345678","email":"nope"}`, params))
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "email") {
		t.Fatalf("a given email must look like one; got %d (%s)", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.PublicSignUp(w, f.publicRequest(t, http.MethodPost, "/", `{"first_name":"Ana","last_name":"Diaz","phone":"+5491112345678"}`, params))
	if w.Code != http.StatusCreated {
		t.Fatalf("an absent email is fine; got %d (%s)", w.Code, w.Body.String())
	}
}

func TestPublicSignUpAnswers409WhenFullAnd400WithoutMercadoPago(t *testing.T) {
	f := newFixture(t)
	h := newHandler(f)
	params := map[string]string{"slug": "vibe", "eventID": f.store.event.ID.String()}
	body := `{"first_name":"Ana","last_name":"Diaz","phone":"+5491112345678"}`

	f.store.signUpErr = eventstore.ErrEventFull
	w := httptest.NewRecorder()
	h.PublicSignUp(w, f.publicRequest(t, http.MethodPost, "/", body, params))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "the event is full") {
		t.Fatalf("want 409 full; got %d (%s)", w.Code, w.Body.String())
	}

	f.store.signUpErr = nil
	f.checkout.err = errBoom
	w = httptest.NewRecorder()
	h.PublicSignUp(w, f.publicRequest(t, http.MethodPost, "/", body, params))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when MercadoPago refuses the checkout; got %d (%s)", w.Code, w.Body.String())
	}
}

// Each refusal of a sign-up answers its own problem kind, so the page can say
// which one it was without reading the English detail.
func TestPublicSignUpRefusalsEachAnswerTheirOwnKind(t *testing.T) {
	body := `{"first_name":"Ana","last_name":"Diaz","phone":"+5491112345678"}`
	for _, tc := range []struct {
		err    error
		status int
		kind   string
	}{
		{eventstore.ErrEventFull, http.StatusConflict, "event-full"},
		{eventstore.ErrEventClosed, http.StatusConflict, "event-closed"},
		{eventstore.ErrAlreadyRegistered, http.StatusConflict, "already-registered"},
		{eventstore.ErrHoldLimit, http.StatusTooManyRequests, "rate-limited"},
	} {
		f := newFixture(t)
		f.store.signUpErr = tc.err
		params := map[string]string{"slug": "vibe", "eventID": f.store.event.ID.String()}
		w := httptest.NewRecorder()
		newHandler(f).PublicSignUp(w, f.publicRequest(t, http.MethodPost, "/", body, params))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), "/problems/"+tc.kind+`"`) {
			t.Errorf("%v: want %d kind %s; got %d (%s)", tc.err, tc.status, tc.kind, w.Code, w.Body.String())
		}
	}
}

// S6: status and cancel authorize on the registration's token, and cancelling
// answers what became of the money.
func TestPublicStatusAndCancelFollowTheRegistrationToken(t *testing.T) {
	f := newFixture(t)
	token := signedUpAndPaid(t, f, 48*time.Hour, 72*time.Hour)
	f.payments.outcome = paymentstore.RefundOutcome{Result: paymentstore.RefundIssued, AmountCentavos: 820_000}
	h := newHandler(f)

	w := httptest.NewRecorder()
	h.PublicRegistrationStatus(w, f.publicRequest(t, http.MethodGet, "/?token="+token, "", nil))
	status := decodeBody(t, w)
	if w.Code != http.StatusOK || status["status"] != "confirmed" ||
		status["cancellation"].(map[string]any)["within_window"] != true {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.PublicRegistrationStatus(w, f.publicRequest(t, http.MethodGet, "/", "", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a missing token is 400; got %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.PublicRegistrationStatus(w, f.publicRequest(t, http.MethodGet, "/?token=nope", "", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("an unknown token is 404; got %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.PublicRegistrationCancel(w, f.publicRequest(t, http.MethodPost, "/", `{"token":"`+token+`"}`, nil))
	cancelled := decodeBody(t, w)
	if w.Code != http.StatusOK || cancelled["refunded"] != true || cancelled["registration"].(map[string]any)["status"] != "cancelled" ||
		cancelled["refund"].(map[string]any)["status"] != "issued" {
		t.Fatalf("cancel = %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.PublicRegistrationCancel(w, f.publicRequest(t, http.MethodPost, "/", `{"token":"`+token+`"}`, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("a second cancel is 400; got %d (%s)", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.PublicRegistrationCancel(w, f.publicRequest(t, http.MethodPost, "/", `{"token":""}`, nil))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a missing token is 422; got %d", w.Code)
	}
}

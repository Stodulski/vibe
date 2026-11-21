package bookings

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	bookingstore "github.com/stodulski/vibe-server/internal/bookings/store"
)

// staffDepositBody is a dashboard booking at 18:00 for 90 minutes (750_000
// centavos in staffFixture) taken with a seña; deposit is the raw JSON of
// the deposit_amount member, or "" to leave it out.
func staffDepositBody(courtID uuid.UUID, deposit string) string {
	date, _ := bookableDate()
	member := ""
	if deposit != "" {
		member = `,"deposit_amount":` + deposit
	}
	return fmt.Sprintf(`{"court_id":%q,"date":%q,"start_time":"18:00","duration_minutes":90,`+
		`"payment_option":"deposit","payment_method":"cash"%s,`+
		`"client_first_name":"Ana","client_last_name":"Perez","client_phone":"+541100000000"}`,
		courtID, date.Format("2006-01-02"), member)
}

func staffCreateBody(t *testing.T, f *fixture, complexID uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	f.handler.Create(w, ownerRequest(t, http.MethodPost, "/", complexID,
		map[string]string{"id": complexID.String()}, body))
	return w
}

// A seña that covers the whole price is the whole price: the booking reads
// fully paid, and the counter payment says so.
func TestADepositEqualToThePriceIsRecordedAsFullyPaid(t *testing.T) {
	f, complexID, courtID := staffFixture(t)

	w := staffCreateBody(t, f, complexID, staffDepositBody(courtID, "750000"))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201; got %d (%s)", w.Code, w.Body.String())
	}
	got := f.store.inserted[0]
	if got.CollectionStatus != bookingstore.CollectionStatusFullyPaid {
		t.Errorf("a deposit of the whole price must read fully paid; got %q", got.CollectionStatus)
	}
	paid := f.store.payments.inserted
	if len(paid) != 1 || paid[0].Amount != 750_000 || paid[0].Status != string(bookingstore.CollectionStatusFullyPaid) {
		t.Errorf("the counter payment must be the whole price, fully paid; got %+v", paid)
	}
}

// A seña with no amount is not a payment the owner described: the dashboard
// used to record the complex's percentage as cash taken at the counter.
func TestADepositWithNoAmountIsRefused(t *testing.T) {
	for name, deposit := range map[string]string{"absent": "", "zero": "0"} {
		t.Run(name, func(t *testing.T) {
			f, complexID, courtID := staffFixture(t)

			w := staffCreateBody(t, f, complexID, staffDepositBody(courtID, deposit))

			if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"deposit_amount"`) {
				t.Fatalf("want 422 on deposit_amount; got %d (%s)", w.Code, w.Body.String())
			}
			if len(f.store.inserted) != 0 {
				t.Error("nothing may be stored")
			}
		})
	}
}

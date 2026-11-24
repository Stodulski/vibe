package mp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A payment whose seller arrives only as the collector object still names the
// seller: the collector check reads CollectorID and nothing else.
func TestGetPaymentReadsTheCollectorObjectWhenCollectorIDIsMissing(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int64
	}{
		{name: "collector_id", body: `{"id":999,"collector_id":111}`, want: 111},
		{name: "only the collector object", body: `{"id":999,"collector":{"id":222}}`, want: 222},
		{name: "both, collector_id wins", body: `{"id":999,"collector_id":111,"collector":{"id":222}}`, want: 111},
		{name: "neither", body: `{"id":999}`, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()
			client := NewMPClient("test-token", "secret", "app", "client", nil)
			client.baseURL = ts.URL

			payment, err := client.GetPayment(t.Context(), "999", AsPlatform())
			if err != nil {
				t.Fatalf("GetPayment: %v", err)
			}
			if payment.CollectorID != tt.want {
				t.Errorf("CollectorID = %d, want %d", payment.CollectorID, tt.want)
			}
		})
	}
}

// A refused read says which refusal it was, so a caller can tell "this account
// cannot see the payment" from MercadoPago failing, and its text stays the one
// stored failure rows are classified by.
func TestGetPaymentFailureCarriesItsStatus(t *testing.T) {
	tests := []struct {
		status       int
		wantNotFound bool
		wantText     string
	}{
		{status: 404, wantNotFound: true, wantText: "mp: get payment failed with status 404: nope"},
		{status: 403, wantNotFound: true, wantText: "mp: get payment failed with status 403: nope"},
		{status: 401, wantNotFound: false, wantText: "mp: get payment failed with status 401: nope"},
		{status: 500, wantNotFound: false, wantText: "mp: get payment failed with status 500: nope"},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("nope"))
			}))
			defer ts.Close()
			client := NewMPClient("test-token", "secret", "app", "client", nil)
			client.baseURL = ts.URL

			_, err := client.GetPayment(t.Context(), "999", AsPlatform())
			if err == nil {
				t.Fatal("want an error")
			}
			if got := IsNotFoundOrForbidden(err); got != tt.wantNotFound {
				t.Errorf("IsNotFoundOrForbidden = %v, want %v", got, tt.wantNotFound)
			}
			if err.Error() != tt.wantText {
				t.Errorf("error text = %q, want %q", err.Error(), tt.wantText)
			}
		})
	}
}

// MercadoPago sends what the seller actually received under
// transaction_details.net_received_amount; the top-level field is not part of
// its payment object, so reading only that one logged 0 for every payment.
func TestGetPaymentReadsTheNetReceivedAmountFromTransactionDetails(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64
	}{
		{name: "transaction_details", body: `{"id":999,"transaction_details":{"net_received_amount":6510.5}}`, want: 6510.5},
		{name: "only top-level", body: `{"id":999,"net_received_amount":42}`, want: 42},
		{name: "both, transaction_details wins", body: `{"id":999,"net_received_amount":42,"transaction_details":{"net_received_amount":6510.5}}`, want: 6510.5},
		{name: "neither", body: `{"id":999}`, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()
			client := NewMPClient("test-token", "secret", "app", "client", nil)
			client.baseURL = ts.URL

			payment, err := client.GetPayment(t.Context(), "999", AsPlatform())
			if err != nil {
				t.Fatalf("GetPayment: %v", err)
			}
			if payment.NetReceivedAmount != tt.want {
				t.Errorf("NetReceivedAmount = %v, want %v", payment.NetReceivedAmount, tt.want)
			}
		})
	}
}

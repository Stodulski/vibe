package mp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchPaymentsByReference(t *testing.T) {
	t.Run("asks for the reference as the seller and returns every result", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/payments/search" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("external_reference"); got != "booking-123" {
				t.Errorf("want external_reference booking-123; got %q", got)
			}
			if r.Header.Get("Authorization") != "Bearer seller-token" {
				t.Errorf("want the seller's token; got %s", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"results": []Payment{
					{ID: 2, Status: "approved", ExternalReference: "booking-123"},
					{ID: 1, Status: "rejected", ExternalReference: "booking-123"},
				},
			}); err != nil {
				t.Errorf("Encode: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("platform-token", "secret", "app", "client", nil)
		client.baseURL = ts.URL
		seller, err := AsSeller("seller-token")
		if err != nil {
			t.Fatal(err)
		}

		got, err := client.SearchPaymentsByReference(t.Context(), "booking-123", seller)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0].ID != 2 || got[0].Status != "approved" || got[1].Status != "rejected" {
			t.Errorf("want both payments as MercadoPago listed them; got %+v", got)
		}
	})

	t.Run("a non-200 answer is an APIError, never an empty list", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"message":"invalid token"}`, http.StatusUnauthorized)
		}))
		defer ts.Close()

		client := NewMPClient("platform-token", "secret", "app", "client", nil)
		client.baseURL = ts.URL
		seller, err := AsSeller("seller-token")
		if err != nil {
			t.Fatal(err)
		}

		got, err := client.SearchPaymentsByReference(t.Context(), "booking-123", seller)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
			t.Fatalf("want an APIError carrying 401; got %v", err)
		}
		if got != nil {
			t.Errorf("a failed search must not answer with a list; got %+v", got)
		}
	})
}

package mp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewMPClient(t *testing.T) {
	client := NewMPClient("token", "secret", "app-id", "client-secret", nil)

	if client.accessToken != "token" {
		t.Errorf("want accessToken %q; got %q", "token", client.accessToken)
	}
	if client.webhookSecret != "secret" {
		t.Errorf("want webhookSecret %q; got %q", "secret", client.webhookSecret)
	}
	if client.appID != "app-id" {
		t.Errorf("want appID %q; got %q", "app-id", client.appID)
	}
	if client.clientSecret != "client-secret" {
		t.Errorf("want clientSecret %q; got %q", "client-secret", client.clientSecret)
	}
	if client.baseURL != "https://api.mercadopago.com" {
		t.Errorf("want baseURL %q; got %q", "https://api.mercadopago.com", client.baseURL)
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{StatusCode: 400, Body: "bad request"}
	want := "mp: API error 400: bad request"
	if err.Error() != want {
		t.Errorf("want %q; got %q", want, err.Error())
	}
}

func TestIsUnauthorized(t *testing.T) {
	t.Run("401 error", func(t *testing.T) {
		err := &APIError{StatusCode: 401, Body: "unauthorized"}
		if !IsUnauthorized(err) {
			t.Error("expected IsUnauthorized to return true for 401")
		}
	})

	t.Run("400 error", func(t *testing.T) {
		err := &APIError{StatusCode: 400, Body: "bad request"}
		if IsUnauthorized(err) {
			t.Error("expected IsUnauthorized to return false for 400")
		}
	})

	t.Run("non-API error", func(t *testing.T) {
		err := fmt.Errorf("some other error")
		if IsUnauthorized(err) {
			t.Error("expected IsUnauthorized to return false for non-APIError")
		}
	})
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "test-webhook-secret"
	client := NewMPClient("token", secret, "app-id", "client-secret", nil)

	t.Run("valid signature", func(t *testing.T) {
		dataID := "12345"
		requestID := "req-abc"
		ts := fmt.Sprintf("%d", time.Now().Unix())

		signedStr := fmt.Sprintf("id:%s;request-id:%s;ts:%s;", dataID, requestID, ts)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signedStr))
		v1 := hex.EncodeToString(mac.Sum(nil))

		xSignature := fmt.Sprintf("ts=%s,v1=%s", ts, v1)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Signature", xSignature)
		req.Header.Set("X-Request-Id", requestID)

		err := client.VerifyWebhookSignature(req, dataID)
		if err != nil {
			t.Errorf("expected no error; got %v", err)
		}
	})

	t.Run("missing X-Signature header", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		err := client.VerifyWebhookSignature(req, "12345")
		if err == nil {
			t.Error("expected error for missing X-Signature")
		}
	})

	t.Run("invalid signature format", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Signature", "invalid-format")
		err := client.VerifyWebhookSignature(req, "12345")
		if err == nil {
			t.Error("expected error for invalid X-Signature format")
		}
	})

	t.Run("wrong signature value", func(t *testing.T) {
		ts := fmt.Sprintf("%d", time.Now().Unix())
		xSignature := fmt.Sprintf("ts=%s,v1=wrong-hash", ts)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Signature", xSignature)
		req.Header.Set("X-Request-Id", "req-abc")

		err := client.VerifyWebhookSignature(req, "12345")
		if err == nil {
			t.Error("expected error for wrong signature")
		}
	})

	// A deployment that never set MP_WEBHOOK_SECRET has no trust boundary: the key
	// is the empty string, and anyone on the internet can compute HMAC-SHA256 over
	// it. The old code signed with that key and reported the forgery as verified,
	// so every caller's "the signature was checked" was true and meaningless at the
	// same time. The signature is what stands between a stranger and a confirmed
	// booking against captured money, so a missing secret has to refuse, not pass.
	t.Run("no webhook secret configured", func(t *testing.T) {
		unconfigured := NewMPClient("token", "", "app-id", "client-secret", nil)

		dataID := "12345"
		requestID := "req-abc"
		ts := fmt.Sprintf("%d", time.Now().Unix())

		// Exactly what a forger does: sign with the empty key.
		signedStr := fmt.Sprintf("id:%s;request-id:%s;ts:%s;", dataID, requestID, ts)
		mac := hmac.New(sha256.New, []byte(""))
		mac.Write([]byte(signedStr))
		v1 := hex.EncodeToString(mac.Sum(nil))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Signature", fmt.Sprintf("ts=%s,v1=%s", ts, v1))
		req.Header.Set("X-Request-Id", requestID)

		if err := unconfigured.VerifyWebhookSignature(req, dataID); err == nil {
			t.Error("a client with no webhook secret must refuse to verify anything; it accepted a signature computed with the empty key")
		}
	})

	t.Run("expired timestamp", func(t *testing.T) {
		dataID := "12345"
		requestID := "req-abc"
		ts := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

		signedStr := fmt.Sprintf("id:%s;request-id:%s;ts:%s;", dataID, requestID, ts)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signedStr))
		v1 := hex.EncodeToString(mac.Sum(nil))

		xSignature := fmt.Sprintf("ts=%s,v1=%s", ts, v1)

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Signature", xSignature)
		req.Header.Set("X-Request-Id", requestID)

		err := client.VerifyWebhookSignature(req, dataID)
		if err == nil {
			t.Error("expected error for expired timestamp")
		}
	})

	// Hardcoded vectors computed independently of the Go implementation via:
	//   printf 'id:<id>;request-id:<request-id>;ts:<ts>;' | \
	//     openssl dgst -sha256 -hmac 'test-webhook-secret'
	// This proves the manifest format against a source other than the code
	// under test, and (second vector) proves the data.id lowercasing rule
	// MercadoPago documents for alphanumeric ids.
	t.Run("pinned vector: numeric data.id", func(t *testing.T) {
		pinned := NewMPClient("token", secret, "app-id", "client-secret", nil)
		pinnedNow := time.Unix(1700000000, 0)
		pinned.now = func() time.Time { return pinnedNow }

		dataID := "123456789"
		requestID := "11111111-1111-1111-1111-111111111111"
		ts := "1700000000"
		const wantV1 = "87e0ae38aa39a07d8466b3aa5a6a43f3ed47e09c971bf77e428d99f3273f1c8c"

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Signature", fmt.Sprintf("ts=%s,v1=%s", ts, wantV1))
		req.Header.Set("X-Request-Id", requestID)

		if err := pinned.VerifyWebhookSignature(req, dataID); err != nil {
			t.Errorf("expected pinned numeric-id vector to verify; got %v", err)
		}
	})

	t.Run("pinned vector: uppercase alphanumeric data.id is lowercased", func(t *testing.T) {
		pinned := NewMPClient("token", secret, "app-id", "client-secret", nil)
		pinnedNow := time.Unix(1700000000, 0)
		pinned.now = func() time.Time { return pinnedNow }

		// The raw data.id as MP would send it, uppercase.
		dataID := "ORD01JQ4S"
		requestID := "11111111-1111-1111-1111-111111111111"
		ts := "1700000000"
		// Computed over the LOWERCASED id ("ord01jq4s"), proving the rule.
		const wantV1 = "d9bd32d55d8dfb9cb11bd6a5d96c0c5b45c8b5daba3f9b4801f3fa8aa7fe2759"

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Signature", fmt.Sprintf("ts=%s,v1=%s", ts, wantV1))
		req.Header.Set("X-Request-Id", requestID)

		if err := pinned.VerifyWebhookSignature(req, dataID); err != nil {
			t.Errorf("expected pinned uppercase-id vector (lowercased before HMAC) to verify; got %v", err)
		}
	})
}

func TestGetPayment(t *testing.T) {
	t.Run("successful payment fetch", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/payments/999" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(Payment{
				ID:                999,
				Status:            "approved",
				TransactionAmount: 5000,
				ExternalReference: "booking-123",
			}); err != nil {
				t.Errorf("Encode: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("test-token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		payment, err := client.GetPayment(t.Context(), "999", AsPlatform())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payment.ID != 999 {
			t.Errorf("want payment ID 999; got %d", payment.ID)
		}
		if payment.Status != "approved" {
			t.Errorf("want status approved; got %s", payment.Status)
		}
	})

	t.Run("uses seller token when provided", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer seller-token" {
				t.Errorf("expected seller-token; got %s", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(Payment{ID: 1}); err != nil {
				t.Errorf("Encode: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("marketplace-token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		_, err := client.GetPayment(t.Context(), "1", mustSeller(t, "seller-token"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-200 response returns error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte("not found")); err != nil {
				t.Errorf("w.Write: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		_, err := client.GetPayment(t.Context(), "999", AsPlatform())
		if err == nil {
			t.Error("expected error for 404 response")
		}
	})
}

func TestRefundPayment(t *testing.T) {
	t.Run("successful refund", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/payments/999/refunds" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("expected POST; got %s", r.Method)
			}

			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("Decode: %v", err)
			}
			if body["amount"] != 50.0 {
				t.Errorf("expected amount 50; got %v", body["amount"])
			}

			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(Refund{ID: 1, Amount: 50, Status: "approved"}); err != nil {
				t.Errorf("Encode: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		refund, err := client.RefundPayment(t.Context(), "999", 50.0, AsPlatform())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if refund.Status != "approved" {
			t.Errorf("want refund status approved; got %s", refund.Status)
		}
	})

	t.Run("failed refund returns error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte("cannot refund")); err != nil {
				t.Errorf("w.Write: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		_, err := client.RefundPayment(t.Context(), "999", 50.0, AsPlatform())
		if err == nil {
			t.Error("expected error for 400 response")
		}
	})
}

// TestCreatePreference_RefusesEveryCallerButASeller is the guard that replaces
// mp.go's platform-token fallback for CreatePreference specifically (see the
// four-site fallback table in design.md — this is the one site where
// MercadoPago accepts the marketplace's own credentials and settles the money
// into the marketplace's account, so unlike GetPayment/
// UpdatePreferenceExpired/RefundPayment, this method must refuse rather than
// fall back).
//
// Both refusals matter, and they are not the same one. The zero Caller is what
// an ignored AsSeller error leaves on the floor; AsPlatform is a call site
// asking for the wrong thing in so many words. Neither may reach MercadoPago.
func TestCreatePreference_RefusesEveryCallerButASeller(t *testing.T) {
	tests := map[string]struct {
		caller  Caller
		wantErr error
	}{
		"no caller named at all":      {caller: Caller{}, wantErr: ErrNoSellerToken},
		"the caller named AsPlatform": {caller: AsPlatform(), wantErr: ErrPlatformCannotSell},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			called := false
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			}))
			defer ts.Close()

			client := NewMPClient("marketplace-token", "secret", "app", "client", nil)
			client.baseURL = ts.URL

			_, err := client.CreatePreference(t.Context(), CreatePreferenceInput{
				BookingID: uuid.New(),
				Caller:    tt.caller,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("want %v; got %v", tt.wantErr, err)
			}
			if called {
				t.Error("CreatePreference must not reach MercadoPago — the marketplace's own credentials settle real money into the wrong account")
			}
		})
	}
}

// mustSeller is AsSeller for the tokens a test hard-codes and knows are
// non-empty. It fails the test rather than returning a Caller the client would
// refuse, so a test never silently exercises the refusal path it did not mean
// to.
func mustSeller(t *testing.T, token string) Caller {
	t.Helper()
	caller, err := AsSeller(token)
	if err != nil {
		t.Fatalf("AsSeller(%q): %v", token, err)
	}
	return caller
}

// TestAnEmptySellerTokenNeverBecomesThePlatform is the whole reason Caller
// exists.
//
// Before it, "" was a second and unwritten way of saying "the marketplace":
// UpdatePreferenceExpired, GetPayment and RefundPayment each read an empty
// seller token as permission to authenticate with the platform's own
// credentials, so a credential that failed to load turned a per-seller call
// into a platform call with nothing at the call site to show for it.
//
// Two claims, and the mutations that break each: make AsSeller accept the
// empty token and the first half fails; make bearer's unnamed-caller arm
// return c.accessToken and the second half fails, because the request reaches
// the server carrying the platform's token.
func TestAnEmptySellerTokenNeverBecomesThePlatform(t *testing.T) {
	unnamed, err := AsSeller("")
	if !errors.Is(err, ErrNoSellerToken) {
		t.Fatalf("AsSeller(\"\") must refuse; got caller %+v, err %v", unnamed, err)
	}

	// The caller a call site is left holding when it ignores that error. Every
	// method must refuse it rather than reach for the platform's credentials.
	var reached []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = append(reached, r.URL.Path+" "+r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewMPClient("platform-token", "secret", "app", "client", nil)
	client.baseURL = ts.URL

	calls := map[string]func() error{
		"CreatePreference": func() error {
			_, e := client.CreatePreference(t.Context(), CreatePreferenceInput{BookingID: uuid.New(), Caller: unnamed})
			return e
		},
		"UpdatePreferenceExpired": func() error {
			return client.UpdatePreferenceExpired(t.Context(), "pref-1", unnamed)
		},
		"GetPayment": func() error {
			_, e := client.GetPayment(t.Context(), "999", unnamed)
			return e
		},
		"RefundPayment": func() error {
			_, e := client.RefundPayment(t.Context(), "999", 50.0, unnamed)
			return e
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if e := call(); !errors.Is(e, ErrNoSellerToken) {
				t.Errorf("%s with an unnamed caller: want ErrNoSellerToken; got %v", name, e)
			}
		})
	}
	if len(reached) != 0 {
		t.Errorf("nothing may reach MercadoPago on a caller built from an empty seller token; got %v", reached)
	}
}

// TestEachMethodSendsTheTokenItsCallerNames crosses the wires on purpose: the
// platform's token and the seller's are different strings, and each method is
// asked for one while the server refuses the other. Swap the two arms of
// bearer and every subtest fails.
func TestEachMethodSendsTheTokenItsCallerNames(t *testing.T) {
	const (
		platformToken = "platform-token"
		sellerToken   = "seller-token"
	)

	tests := map[string]struct {
		call      func(*MPClient, Caller) error
		asSeller  bool
		wantToken string
	}{
		"UpdatePreferenceExpired as the seller": {
			call:      func(c *MPClient, caller Caller) error { return c.UpdatePreferenceExpired(t.Context(), "p", caller) },
			asSeller:  true,
			wantToken: sellerToken,
		},
		"UpdatePreferenceExpired as the platform": {
			call:      func(c *MPClient, caller Caller) error { return c.UpdatePreferenceExpired(t.Context(), "p", caller) },
			wantToken: platformToken,
		},
		"GetPayment as the seller": {
			call:      func(c *MPClient, caller Caller) error { _, e := c.GetPayment(t.Context(), "9", caller); return e },
			asSeller:  true,
			wantToken: sellerToken,
		},
		"GetPayment as the platform": {
			call:      func(c *MPClient, caller Caller) error { _, e := c.GetPayment(t.Context(), "9", caller); return e },
			wantToken: platformToken,
		},
		"RefundPayment as the seller": {
			call:      func(c *MPClient, caller Caller) error { _, e := c.RefundPayment(t.Context(), "9", 1, caller); return e },
			asSeller:  true,
			wantToken: sellerToken,
		},
		"RefundPayment as the platform": {
			call:      func(c *MPClient, caller Caller) error { _, e := c.RefundPayment(t.Context(), "9", 1, caller); return e },
			wantToken: platformToken,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var got string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(`{}`)); err != nil {
					t.Errorf("w.Write: %v", err)
				}
			}))
			defer ts.Close()

			client := NewMPClient(platformToken, "secret", "app", "client", nil)
			client.baseURL = ts.URL

			caller := AsPlatform()
			if tt.asSeller {
				caller = mustSeller(t, sellerToken)
			}
			if err := tt.call(client, caller); err != nil {
				t.Fatalf("call: %v", err)
			}
			if got != "Bearer "+tt.wantToken {
				t.Errorf("want %q; got %q", "Bearer "+tt.wantToken, got)
			}
		})
	}
}

// TestFallbackAsymmetryIsPreserved guards the three MPClient methods that
// MUST keep accepting AsPlatform, deliberately, unlike CreatePreference
// above:
//
//   - GetPayment: internal/payments' processPaymentWebhook calls it as the
//     platform on every webhook, because MercadoPago's marketplace API lets
//     the app owner read any payment under its own app_id and the seller who
//     collected the money is not known until the payment says so. Refusing
//     here breaks webhook payment confirmation — the step that marks a
//     booking paid.
//   - UpdatePreferenceExpired and RefundPayment: MercadoPago rejects both
//     against the wrong account, so trying as the platform is safe
//     (loud-but-continue is the design), and cron.go / cancel.go rely on the
//     call still going out rather than being refused.
//
// Caller made the choice explicit; it did not remove it. If a future edit
// "makes the four consistent" with CreatePreference's refusal, this test is
// the one that must fail — see design.md's apply-time warning.
func TestFallbackAsymmetryIsPreserved(t *testing.T) {
	t.Run("GetPayment as the platform still works", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer platform-token" {
				t.Errorf("want the platform token; got %s", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(Payment{ID: 1}); err != nil {
				t.Fatalf("Encode: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("platform-token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		if _, err := client.GetPayment(t.Context(), "1", AsPlatform()); err != nil {
			t.Fatalf("GetPayment(ctx, id, AsPlatform()) must still succeed against the platform token; got %v", err)
		}
	})

	t.Run("UpdatePreferenceExpired as the platform still works", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer platform-token" {
				t.Errorf("want the platform token; got %s", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := NewMPClient("platform-token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		if err := client.UpdatePreferenceExpired(t.Context(), "pref-1", AsPlatform()); err != nil {
			t.Fatalf("UpdatePreferenceExpired(ctx, id, AsPlatform()) must still succeed against the platform token; got %v", err)
		}
	})

	t.Run("RefundPayment as the platform still works", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer platform-token" {
				t.Errorf("want the platform token; got %s", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(Refund{ID: 1, Amount: 50, Status: "approved"}); err != nil {
				t.Fatalf("Encode: %v", err)
			}
		}))
		defer ts.Close()

		client := NewMPClient("platform-token", "secret", "app", "client", nil)
		client.baseURL = ts.URL

		if _, err := client.RefundPayment(t.Context(), "999", 50.0, AsPlatform()); err != nil {
			t.Fatalf("RefundPayment(ctx, id, amount, AsPlatform()) must still succeed against the platform token; got %v", err)
		}
	})
}

// TestCreatePreferenceBuildsBackURLsFromInput pins design.md Decision 3:
// internal/mp builds no client-facing URL at all — back_urls comes straight
// from input.BackURLs, which the caller (internal/booklink, through
// internal/bookings/public.go) already built.
func TestCreatePreferenceBuildsBackURLsFromInput(t *testing.T) {
	var captured map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(Preference{ID: "pref-1"}); err != nil {
			t.Fatalf("Encode: %v", err)
		}
	}))
	defer ts.Close()

	client := NewMPClient("marketplace-token", "secret", "app", "client", nil)
	client.baseURL = ts.URL

	input := CreatePreferenceInput{
		BookingID:   uuid.New(),
		ComplexName: "Acme Padel",
		CourtName:   "Court 1",
		Date:        "2025-06-15",
		StartTime:   "18:00",
		Amount:      10000,
		Caller:      mustSeller(t, "seller-token"),
		BackURLs: BackURLs{
			Success: "https://vibe.test/acme/book/success?booking_id=abc",
			Failure: "https://vibe.test/acme/book?error=payment_failed",
			Pending: "https://vibe.test/acme/book/success?booking_id=abc&status=pending",
		},
		BackendURL: "https://api.vibe.test",
	}

	if _, err := client.CreatePreference(t.Context(), input); err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}

	backURLs, ok := captured["back_urls"].(map[string]any)
	if !ok {
		t.Fatalf("captured request has no back_urls object: %v", captured)
	}
	if backURLs["success"] != input.BackURLs.Success {
		t.Errorf("back_urls.success = %v, want %v", backURLs["success"], input.BackURLs.Success)
	}
	if backURLs["failure"] != input.BackURLs.Failure {
		t.Errorf("back_urls.failure = %v, want %v", backURLs["failure"], input.BackURLs.Failure)
	}
	if backURLs["pending"] != input.BackURLs.Pending {
		t.Errorf("back_urls.pending = %v, want %v", backURLs["pending"], input.BackURLs.Pending)
	}
}

// TestCreatePreferenceInputHasNoClientURLFields is the compile-level proof
// task 6.7 asks for, made checkable at test-run time via reflection:
// CreatePreferenceInput must not carry FrontendURL or ComplexSlug —
// internal/mp builds no client-facing URL, so it has no use for either.
func TestCreatePreferenceInputHasNoClientURLFields(t *testing.T) {
	typ := reflect.TypeOf(CreatePreferenceInput{})
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if name == "FrontendURL" || name == "ComplexSlug" {
			t.Errorf("CreatePreferenceInput must not carry %s; internal/mp builds no client-facing URL", name)
		}
	}
}

// TestTitleDateReadsAsADateAfterMercadoPagoStripsPunctuation pins the checkout
// title date format: MercadoPago renders "04/09" as "0409", so the title uses a
// month abbreviation instead.
func TestTitleDateReadsAsADateAfterMercadoPagoStripsPunctuation(t *testing.T) {
	cases := map[string]string{
		"2026-09-04": "4 sep",
		"2026-12-25": "25 dic",
		"2026-01-01": "1 ene",
		"not-a-date": "not-a-date",
	}
	for in, want := range cases {
		if got := titleDate(in); got != want {
			t.Errorf("titleDate(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCreatePreferenceDescribesTheBookingWithoutASport pins what the payer
// reads: the item description names no sport, since a complex can run any of
// seven, and the card statement carries the neutral "VIBE RESERVA".
func TestCreatePreferenceDescribesTheBookingWithoutASport(t *testing.T) {
	var captured map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(Preference{ID: "pref-1"}); err != nil {
			t.Errorf("Encode: %v", err)
		}
	}))
	defer ts.Close()

	client := NewMPClient("marketplace-token", "secret", "app", "client", nil)
	client.baseURL = ts.URL

	input := CreatePreferenceInput{
		BookingID:   uuid.New(),
		ComplexName: "Acme",
		CourtName:   "Cancha 1",
		Date:        "2025-06-15",
		StartTime:   "18:00",
		Amount:      10000,
		Caller:      mustSeller(t, "seller-token"),
		BackendURL:  "https://api.vibe.test",
	}
	if _, err := client.CreatePreference(t.Context(), input); err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}

	items, ok := captured["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("captured request has no single item: %v", captured["items"])
	}
	item, _ := items[0].(map[string]any)
	if want := "Acme - Cancha 1 - 15 jun 18:00 hs"; item["title"] != want {
		t.Errorf("title = %q, want %q", item["title"], want)
	}
	if want := "Seña para reserva en Acme. Cancha 1, 15 jun a las 18:00 hs."; item["description"] != want {
		t.Errorf("description = %q, want %q", item["description"], want)
	}
	if captured["statement_descriptor"] != "VIBE RESERVA" {
		t.Errorf("statement_descriptor = %v, want VIBE RESERVA", captured["statement_descriptor"])
	}
}

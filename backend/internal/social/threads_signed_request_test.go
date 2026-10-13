package social

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

const testAppSecret = "app-secret-for-tests"

// signedRequestFor builds the signed_request Meta sends: a signature and the
// payload, each base64url without padding, joined by a dot. The signature is
// HMAC-SHA256 keyed by the app secret over the encoded payload text.
func signedRequestFor(secret, payloadJSON string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + payload
}

func TestParseSignedRequestAcceptsAValidSignature(t *testing.T) {
	t.Parallel()
	signed := signedRequestFor(testAppSecret, `{"algorithm":"HMAC-SHA256","issued_at":1760000000,"user_id":"42"}`)

	got, err := ParseSignedRequest(signed, testAppSecret)
	if err != nil {
		t.Fatalf("ParseSignedRequest() error = %v", err)
	}
	if got.UserID != "42" || got.Algorithm != "HMAC-SHA256" {
		t.Fatalf("payload = %+v; want user 42 under HMAC-SHA256", got)
	}
}

func TestParseSignedRequestRefusesEverythingThatIsNotAuthenticPayload(t *testing.T) {
	t.Parallel()
	goodPayload := `{"algorithm":"HMAC-SHA256","issued_at":1,"user_id":"42"}`
	// A valid signature over one payload, with the payload swapped for another:
	// the signature no longer covers what the request claims.
	sig, _, _ := strings.Cut(signedRequestFor(testAppSecret, goodPayload), ".")
	swapped := sig + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"algorithm":"HMAC-SHA256","user_id":"99"}`))

	tests := []struct {
		name   string
		signed string
	}{
		{name: "signed with another secret", signed: signedRequestFor("another-secret", goodPayload)},
		{name: "payload swapped after signing", signed: swapped},
		{name: "algorithm other than HMAC-SHA256", signed: signedRequestFor(testAppSecret, `{"algorithm":"HMAC-SHA1","user_id":"42"}`)},
		{name: "no separator", signed: "abc"},
		{name: "empty signature", signed: "." + base64.RawURLEncoding.EncodeToString([]byte(goodPayload))},
		{name: "signature is not base64", signed: "!!!." + base64.RawURLEncoding.EncodeToString([]byte(goodPayload))},
		{name: "payload is not JSON", signed: signedRequestFor(testAppSecret, "not-json")},
		{name: "empty", signed: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseSignedRequest(tc.signed, testAppSecret)
			if !errors.Is(err, ErrInvalidSignedRequest) {
				t.Fatalf("ParseSignedRequest() error = %v; want ErrInvalidSignedRequest", err)
			}
		})
	}
}

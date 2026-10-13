package social

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidSignedRequest is what a signed_request is refused with when it is
// malformed, is not signed with the app secret, or names another algorithm.
var ErrInvalidSignedRequest = errors.New("social: invalid signed_request")

// SignedRequest is the payload Meta signs into the callbacks it sends an app.
type SignedRequest struct {
	Algorithm string      `json:"algorithm"`
	IssuedAt  int64       `json:"issued_at"`
	UserID    json.Number `json:"user_id"`
}

// ParseSignedRequest verifies a signed_request and returns its payload. The
// request is "signature.payload", both base64url without padding. The
// signature is HMAC-SHA256, keyed by the app secret, over the encoded payload
// text exactly as sent, and it is compared in constant time.
func ParseSignedRequest(signed, appSecret string) (SignedRequest, error) {
	if appSecret == "" {
		return SignedRequest{}, fmt.Errorf("%w: the app secret is not set", ErrInvalidSignedRequest)
	}
	encodedSig, encodedPayload, ok := strings.Cut(signed, ".")
	if !ok || encodedSig == "" || encodedPayload == "" {
		return SignedRequest{}, fmt.Errorf("%w: want signature.payload", ErrInvalidSignedRequest)
	}
	sig, err := decodeBase64URL(encodedSig)
	if err != nil {
		return SignedRequest{}, fmt.Errorf("%w: signature is not base64url", ErrInvalidSignedRequest)
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(encodedPayload))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return SignedRequest{}, fmt.Errorf("%w: signature does not match", ErrInvalidSignedRequest)
	}
	raw, err := decodeBase64URL(encodedPayload)
	if err != nil {
		return SignedRequest{}, fmt.Errorf("%w: payload is not base64url", ErrInvalidSignedRequest)
	}
	var payload SignedRequest
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SignedRequest{}, fmt.Errorf("%w: payload is not JSON", ErrInvalidSignedRequest)
	}
	if !strings.EqualFold(payload.Algorithm, "HMAC-SHA256") {
		return SignedRequest{}, fmt.Errorf("%w: algorithm %q is not HMAC-SHA256", ErrInvalidSignedRequest, payload.Algorithm)
	}
	if payload.UserID == "" {
		return SignedRequest{}, fmt.Errorf("%w: payload names no user", ErrInvalidSignedRequest)
	}
	return payload, nil
}

func decodeBase64URL(s string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return nil, fmt.Errorf("social: decode base64url: %w", err)
	}
	return raw, nil
}

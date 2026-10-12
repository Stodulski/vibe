// Package lifecycle sends an owner the emails that move a signed-up account to
// taking bookings, and records the one-click opt-out those emails carry.
//
// Nothing here runs in the request path that creates an account. The nudges are
// one cron pass over the database, and every email they produce goes through
// the durable queue like every other notification.
package lifecycle

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/validator"
)

// ReasonUnsubscribe is the suppression reason an opt-out records.
const ReasonUnsubscribe = "unsubscribe"

// keyLabel domain-separates the unsubscribe key from every other use of the JWT
// secret. The same secret signs access tokens and CSRF tokens; a MAC over an
// address must never be mistakable for either.
const keyLabel = "vibe/email-unsubscribe/v1"

// ErrInvalidUnsubscribeToken is returned for any token this server did not mint,
// including one minted under a secret that has since been retired.
var ErrInvalidUnsubscribeToken = errors.New("lifecycle: invalid unsubscribe token")

// ErrMissingUnsubscribeToken is returned when a request carries no token in the
// query and none in a JSON body.
var ErrMissingUnsubscribeToken = errors.New("lifecycle: a token is required, in the query string or a JSON body")

// Signer mints and verifies unsubscribe tokens. A token is the lowercased address
// (base64url) and a MAC over it, joined by a dot. There is no expiry: an opt-out
// link in an old email must keep working, so the token carries no clock.
//
// The key derives from the JWT signing secret, so no new configuration is needed.
// That couples the two: rotating JWT_SECRET while dropping JWT_SECRET_PREVIOUS
// invalidates every unsubscribe link ever sent. A rotation must keep the previous
// secret until the links it signed are no longer a concern.
type Signer struct {
	// keys holds the active key first, then the retired one when a rotation is in
	// flight. Only the first signs; every key verifies.
	keys [][]byte
}

// NewSigner builds a signer over the active JWT secret and, during a rotation,
// the previous one. An empty previous secret is no key at all.
func NewSigner(secret, previous string) *Signer {
	s := &Signer{keys: [][]byte{deriveKey(secret)}}
	if previous != "" && previous != secret {
		s.keys = append(s.keys, deriveKey(previous))
	}
	return s
}

func deriveKey(secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(keyLabel))
	return mac.Sum(nil)
}

// Sign returns the token for an address. The address is normalized first, so the
// same person's casing never yields two different tokens.
func (s *Signer) Sign(email string) string {
	email = normalize(email)
	return base64.RawURLEncoding.EncodeToString([]byte(email)) + "." + s.macFor(email)
}

// Verify returns the address a token was minted for, or ErrInvalidUnsubscribeToken.
func (s *Signer) Verify(token string) (string, error) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok || sig == "" || strings.Contains(sig, ".") {
		return "", ErrInvalidUnsubscribeToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", ErrInvalidUnsubscribeToken
	}
	email := string(raw)
	if email != normalize(email) || !validator.Matches(email, validator.EmailRX) {
		return "", ErrInvalidUnsubscribeToken
	}
	for _, key := range s.keys {
		if hmac.Equal([]byte(macWith(key, email)), []byte(sig)) {
			return email, nil
		}
	}
	return "", ErrInvalidUnsubscribeToken
}

// macFor is the MAC the active key produces for an address.
func (s *Signer) macFor(email string) string {
	return macWith(s.keys[0], email)
}

func macWith(key []byte, email string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("unsubscribe:" + email))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// SuppressionStore records an address that asked not to receive lifecycle email.
// It is idempotent: suppressing an address twice is the same as once.
type SuppressionStore interface {
	Suppress(ctx context.Context, email, reason string) error
}

// UnsubscribeHandler answers the one-click opt-out. Mail clients POST to the link
// in the List-Unsubscribe header with the token in the query and the body
// List-Unsubscribe=One-Click (RFC 8058). The /baja page in the footer POSTs the
// token as JSON. Both end in the same suppression row.
type UnsubscribeHandler struct {
	respond *httpx.Responder
	signer  *Signer
	store   SuppressionStore
}

// NewUnsubscribeHandler returns the opt-out handler.
func NewUnsubscribeHandler(respond *httpx.Responder, signer *Signer, store SuppressionStore) *UnsubscribeHandler {
	return &UnsubscribeHandler{respond: respond, signer: signer, store: store}
}

// Unsubscribe handles POST /api/v1/public/email/unsubscribe. queryToken is the
// token from the query string, nil when absent; a body token is read only when the
// query carries none. It answers 204 whether or not the address was already
// suppressed, and 400 for a token this server did not mint.
func (h *UnsubscribeHandler) Unsubscribe(w http.ResponseWriter, r *http.Request, queryToken *string) {
	token := ""
	if queryToken != nil {
		token = strings.TrimSpace(*queryToken)
	}
	if token == "" {
		// A form body (the mail client's One-Click) cannot carry the token, so
		// without one in the query it is a missing token, not an unsupported type.
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			h.respond.BadRequest(w, r, ErrMissingUnsubscribeToken)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if err := httpx.ReadJSON(w, r, &body); err != nil {
			h.respond.BadRequest(w, r, err)
			return
		}
		token = strings.TrimSpace(body.Token)
	}

	email, err := h.signer.Verify(token)
	if err != nil {
		h.respond.BadRequest(w, r, err)
		return
	}
	if err := h.store.Suppress(r.Context(), email, ReasonUnsubscribe); err != nil {
		h.respond.ServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

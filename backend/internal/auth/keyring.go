package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// This file is how the JWT signing secret is rotated without logging everybody
// out.
//
// With one secret and no key id in the header, replacing JWT_SECRET is a hard
// cut: every access token, every profile token and every CSRF token minted
// under the old secret stops verifying the instant the new one is deployed, so
// a rotation — scheduled or, worse, after a leak — is an outage for every live
// session. The only way out was to leave the secret alone, which is the
// opposite of what a leak calls for.
//
// So a token names the key that signed it, in the `kid` header, and the
// verifier resolves the secret by that name out of a keyring of at most two:
// the active one, which both signs and verifies, and an optional previous one
// that only verifies. A rotation is then two deploys — move the current secret
// to JWT_SECRET_PREVIOUS and put the new one in JWT_SECRET, then drop
// JWT_SECRET_PREVIOUS once the longest-lived token minted under it has expired
// (30 days, the refresh window) — and no session breaks in between.
//
// A token with no kid, or with a kid the ring does not hold, is refused. There
// is no "try the active key anyway" fallback: that fallback is exactly what
// makes the header optional, and an optional header is one an attacker can
// simply omit. The deployment this ships to has no production users, so there
// is no population of kid-less tokens to carry over.

// jwtKey is one secret and the name tokens signed with it carry.
type jwtKey struct {
	id     string
	secret []byte
}

// jwtKeyring is the set of keys a token may be verified against. Exactly one
// of them signs.
type jwtKeyring struct {
	active jwtKey
	byID   map[string]jwtKey
}

// newJWTKeyring builds the ring from the configured secrets.
//
// The previous key is dropped when it is empty, when it is the same secret as
// the active one, or when the two would answer to the same id — in all three
// cases it is not a second key, and keeping it would only make the ring lie
// about how many secrets are live.
func newJWTKeyring(cfg TokenServiceConfig) jwtKeyring {
	active := jwtKey{id: keyID(cfg.JWTKeyID, cfg.JWTSecret), secret: []byte(cfg.JWTSecret)}

	ring := jwtKeyring{active: active, byID: map[string]jwtKey{active.id: active}}

	if cfg.JWTSecretPrevious == "" || cfg.JWTSecretPrevious == cfg.JWTSecret {
		return ring
	}
	previous := jwtKey{
		id:     keyID(cfg.JWTKeyIDPrevious, cfg.JWTSecretPrevious),
		secret: []byte(cfg.JWTSecretPrevious),
	}
	if previous.id == active.id {
		return ring
	}
	ring.byID[previous.id] = previous
	return ring
}

// keyID returns the name a key answers to: the operator's own, when they gave
// one, and otherwise a short digest of the secret.
//
// The derived form is what makes this usable without any new configuration:
// deploy, and the id is already stable and already different from the next
// secret's. It is a truncated SHA-256 and it is published in every token, so
// it must not be reversible into the secret — eight hex characters of a digest
// of a 32-byte secret is not, and it is wide enough that two keys colliding
// needs deliberate effort rather than luck.
//
// An explicit JWT_KEY_ID exists for the deployment that wants to name its keys
// (k1, k2, …) rather than read digests; if it is used for the active key it
// has to be used for the previous one too, or the retired key changes name
// halfway through the rotation and the tokens naming it stop verifying.
func keyID(explicit, secret string) string {
	if explicit != "" {
		return explicit
	}
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:4])
}

// lookup returns the secret registered under kid.
func (r jwtKeyring) lookup(kid string) ([]byte, bool) {
	key, ok := r.byID[kid]
	if !ok {
		return nil, false
	}
	return key.secret, true
}

// all returns every secret in the ring, active first. It is for the CSRF
// token, which is an HMAC rather than a JWT and so carries no header to name
// its key: it has to be checked against each.
func (r jwtKeyring) all() [][]byte {
	secrets := make([][]byte, 0, len(r.byID))
	secrets = append(secrets, r.active.secret)
	for id, key := range r.byID {
		if id != r.active.id {
			secrets = append(secrets, key.secret)
		}
	}
	return secrets
}

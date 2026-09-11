package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const (
	// envelopeVersion is the only envelope shape this package writes or
	// reads. It exists as a leading field, not a package constant baked
	// into the parsing logic, so a future format change can add a "v2"
	// case to Open without breaking "v1" values already at rest.
	envelopeVersion = "v1"
	// nonceSize is the standard AES-GCM nonce length: 96 bits.
	nonceSize = 12
)

// Sentinel errors returned by Seal and Open.
var (
	// ErrNilKeyring is returned by Seal and Open when called on a nil
	// *Keyring. It is never a plaintext passthrough: a caller that forgot
	// to configure a keyring gets a loud error on the very first credential
	// it tries to touch, not silent storage of an unencrypted value that
	// happens to look like it worked.
	ErrNilKeyring = errors.New("crypto: keyring is nil, refusing to seal or open")
	// ErrMalformedEnvelope is returned by Open when the input is not
	// shaped like a version-tagged envelope ("v1.<kid>.<payload>"), is not
	// valid base64, or decodes shorter than one nonce.
	ErrMalformedEnvelope = errors.New("crypto: malformed credential envelope")
	// ErrUnknownKeyID is returned by Open when the envelope names a key id
	// that is not present in this Keyring — for example, a value sealed
	// under a key that has since been retired and dropped.
	ErrUnknownKeyID = errors.New("crypto: envelope names a key id this keyring does not hold")
)

// Seal encrypts plaintext under the keyring's active (first) key, binds it
// to aad, and returns the ASCII envelope "v1.<kid>.<base64(nonce||sealed)>".
//
// aad is opaque to this package: callers bind whatever context makes a
// swapped ciphertext detectable (row id, column name, ...). Open must be
// called with the exact same aad bytes used here, or authentication fails.
//
// Seal on a nil *Keyring returns ErrNilKeyring — see its doc comment.
func (k *Keyring) Seal(aad []byte, plaintext string) (string, error) {
	if k == nil {
		return "", ErrNilKeyring
	}

	active := k.entries[0]
	gcm, err := newGCM(active.key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("crypto: generating nonce: %w", err)
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), aad)

	payload := make([]byte, 0, len(nonce)+len(sealed))
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)

	return envelopeVersion + "." + active.kid + "." + base64.RawURLEncoding.EncodeToString(payload), nil
}

// Open decrypts an envelope produced by Seal and authenticates it against
// aad — the same bytes passed to Seal. A tampered, truncated, wrong-key or
// wrong-AAD envelope fails rather than producing a different, unintended
// plaintext: AES-GCM is an AEAD, so corruption and mismatched AAD are both
// detected, not silently decrypted into garbage.
//
// Open on a nil *Keyring returns ErrNilKeyring — see its doc comment.
func (k *Keyring) Open(aad []byte, envelope string) (string, error) {
	if k == nil {
		return "", ErrNilKeyring
	}

	parts := strings.SplitN(envelope, ".", 3)
	if len(parts) != 3 || parts[0] != envelopeVersion {
		return "", ErrMalformedEnvelope
	}
	kid, encoded := parts[1], parts[2]

	key, ok := k.keys[kid]
	if !ok {
		return "", ErrUnknownKeyID
	}

	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrMalformedEnvelope, err)
	}
	if len(payload) < nonceSize {
		return "", ErrMalformedEnvelope
	}
	nonce, sealed := payload[:nonceSize], payload[nonceSize:]

	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, sealed, aad)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypting envelope: %w", err)
	}

	return string(plaintext), nil
}

// newGCM builds an AES-256-GCM AEAD from a 32-byte raw key. ParseKeyring
// already enforces the key length, so the only realistic failure here is a
// key that was somehow tampered with in memory between parsing and use.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: building AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: building AES-GCM: %w", err)
	}
	return gcm, nil
}

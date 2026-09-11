// Package crypto provides a small, MP-agnostic authenticated-encryption
// envelope for values that must not be readable outside the application:
// AES-256-GCM sealing/opening behind an ordered, key-rotatable Keyring. It
// knows nothing about MercadoPago, complexes, or any other domain concept —
// callers supply the plaintext and the additional authenticated data (AAD)
// that binds a sealed value to whatever it must not be copied away from.
package crypto

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// keyIDPattern matches a valid key id: 1-16 characters of letters, digits,
// underscore or hyphen. It deliberately excludes ".", so
// strings.SplitN(envelope, ".", 3) in Open is guaranteed to split an
// envelope into exactly version, kid and payload with no ambiguity — a kid
// could never absorb part of the payload or vice versa.
var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,16}$`)

// keySize is the raw key length required for AES-256.
const keySize = 32

// keyEntry is one key id / raw key pair parsed from a keyring spec.
type keyEntry struct {
	kid string
	key []byte
}

// Keyring holds the ordered set of encryption keys parsed from a keyring
// spec. The first entry is the active key: every Seal call writes under it.
// Every entry can open a value sealed under its own key id, which is what
// makes key rotation possible with no maintenance window — prepend the new
// key, deploy, and previously sealed values keep opening under their
// original key id until they are re-sealed and the old entry is finally
// dropped.
//
// The zero value is not usable; construct one with ParseKeyring. A nil
// *Keyring is a valid (typed) value that Seal and Open both refuse — see
// their doc comments — so a caller that fails to configure one gets a loud
// error on first use rather than a silent plaintext passthrough.
type Keyring struct {
	entries []keyEntry
	keys    map[string][]byte
}

// ParseKeyring parses a keyring spec of the shape
// "kid:base64key[,kid:base64key...]". The first entry is the active
// (writing) key; every entry can open. Every key id must match
// ^[A-Za-z0-9_-]{1,16}$, be unique within the spec, and decode (standard
// base64) to exactly 32 raw bytes (AES-256). The spec must name at least
// one entry.
func ParseKeyring(spec string) (*Keyring, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, errors.New("crypto: keyring spec is empty")
	}

	rawEntries := strings.Split(spec, ",")
	entries := make([]keyEntry, 0, len(rawEntries))
	keys := make(map[string][]byte, len(rawEntries))

	for _, raw := range rawEntries {
		entry, err := parseKeyEntry(raw, keys)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			continue // blank entry between commas, tolerated
		}
		entries = append(entries, *entry)
		keys[entry.kid] = entry.key
	}

	if len(entries) == 0 {
		return nil, errors.New("crypto: keyring spec has no entries")
	}

	return &Keyring{entries: entries, keys: keys}, nil
}

// ActiveKeyID returns the key id Seal writes under — the keyring's first
// entry. It exists so a caller (cmd/mpcredkey's rekey subcommand) can tell
// whether a given envelope is already sealed under the active key without
// attempting to open and re-seal it. A nil *Keyring returns "", matching
// Seal/Open's refusal for a nil receiver rather than panicking.
func (k *Keyring) ActiveKeyID() string {
	if k == nil || len(k.entries) == 0 {
		return ""
	}
	return k.entries[0].kid
}

// parseKeyEntry parses one "kid:base64key" entry, returning nil for a blank
// entry (tolerated so a trailing comma or accidental double comma in the
// spec is not an error) and an error for anything malformed.
func parseKeyEntry(raw string, seen map[string][]byte) (*keyEntry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	kid, b64key, found := strings.Cut(raw, ":")
	if !found {
		return nil, fmt.Errorf("crypto: keyring entry %q is not shaped kid:base64key", raw)
	}

	if !keyIDPattern.MatchString(kid) {
		return nil, fmt.Errorf("crypto: keyring entry %q has an invalid key id (must match %s)", raw, keyIDPattern.String())
	}
	if _, dup := seen[kid]; dup {
		return nil, fmt.Errorf("crypto: keyring has duplicate key id %q", kid)
	}

	key, err := base64.StdEncoding.DecodeString(b64key)
	if err != nil {
		return nil, fmt.Errorf("crypto: keyring entry %q: decoding key: %w", kid, err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("crypto: keyring entry %q must decode to %d raw bytes, got %d", kid, keySize, len(key))
	}

	return &keyEntry{kid: kid, key: key}, nil
}

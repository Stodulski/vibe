package crypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// b64Key32 returns a valid standard-base64 encoding of a distinct 32-byte
// key, so tests can build keyring specs without hardcoding opaque literals.
func b64Key32(fill byte) string {
	return base64.StdEncoding.EncodeToString(fillBytes(32, fill))
}

func fillBytes(n int, fill byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = fill
	}
	return b
}

func TestKeyring_ActiveKeyID(t *testing.T) {
	kr, err := ParseKeyring("new:" + b64Key32(0x20) + ",old:" + b64Key32(0x21))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := kr.ActiveKeyID(); got != "new" {
		t.Errorf("ActiveKeyID() = %q, want %q (the first entry)", got, "new")
	}

	var nilKeyring *Keyring
	if got := nilKeyring.ActiveKeyID(); got != "" {
		t.Errorf("ActiveKeyID() on a nil keyring = %q, want \"\"", got)
	}
}

func TestParseKeyring_ValidSingleEntry(t *testing.T) {
	spec := "k1:" + b64Key32(0x01)

	kr, err := ParseKeyring(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(kr.entries) != 1 || kr.entries[0].kid != "k1" {
		t.Fatalf("want one entry with kid k1; got %+v", kr.entries)
	}
	if _, ok := kr.keys["k1"]; !ok {
		t.Fatal("want k1 present in the open map")
	}
}

func TestParseKeyring_FirstEntryWritesEveryEntryOpens(t *testing.T) {
	spec := "new:" + b64Key32(0x02) + ",old:" + b64Key32(0x03)

	kr, err := ParseKeyring(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sealed, err := kr.Seal([]byte("aad"), "secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if !strings.HasPrefix(sealed, "v1.new.") {
		t.Errorf("want the active (first) key %q to write; got envelope %q", "new", sealed)
	}

	// Simulate an old envelope sealed under the second (retired) key by
	// sealing directly with a single-entry keyring for that kid, then
	// confirm the combined keyring can still open it.
	oldOnly, err := ParseKeyring("old:" + b64Key32(0x03))
	if err != nil {
		t.Fatalf("unexpected error building single-key keyring: %v", err)
	}
	oldSealed, err := oldOnly.Seal([]byte("aad"), "legacy-secret")
	if err != nil {
		t.Fatalf("Seal under old key: %v", err)
	}

	got, err := kr.Open([]byte("aad"), oldSealed)
	if err != nil {
		t.Fatalf("want the combined keyring to open a value sealed under a non-active entry; got error: %v", err)
	}
	if got != "legacy-secret" {
		t.Errorf("got %q, want %q", got, "legacy-secret")
	}
}

func TestParseKeyring_RejectsMalformedSpecs(t *testing.T) {
	validKey := b64Key32(0x04)

	tests := []struct {
		name string
		spec string
	}{
		{"empty spec", ""},
		{"whitespace only", "   "},
		{"missing colon", "k1" + validKey},
		{"invalid base64", "k1:not-valid-base64!!!"},
		{"wrong key length", "k1:" + base64.StdEncoding.EncodeToString(fillBytes(16, 0x05))},
		{"duplicate kid", "k1:" + validKey + ",k1:" + b64Key32(0x06)},
		{"kid contains a dot", "k.1:" + validKey},
		{"kid too long", strings.Repeat("k", 17) + ":" + validKey},
		{"kid empty", ":" + validKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseKeyring(tt.spec); err == nil {
				t.Errorf("ParseKeyring(%q) succeeded, want an error", tt.spec)
			}
		})
	}
}

func TestParseKeyring_AcceptsValidKidCharset(t *testing.T) {
	spec := "Az_9-x:" + b64Key32(0x07)

	if _, err := ParseKeyring(spec); err != nil {
		t.Errorf("unexpected error for a kid using the full allowed charset: %v", err)
	}
}

// TestSealOpen_NilKeyring is the test the design calls out explicitly: a nil
// *Keyring must error on every Seal and every Open call, never a plaintext
// passthrough. "Silently not encrypted" is the exact failure class this
// change exists to remove.
func TestSealOpen_NilKeyring(t *testing.T) {
	var kr *Keyring

	if _, err := kr.Seal([]byte("aad"), "secret"); !errors.Is(err, ErrNilKeyring) {
		t.Errorf("Seal on a nil keyring: want ErrNilKeyring, got %v", err)
	}
	if _, err := kr.Open([]byte("aad"), "v1.k1.whatever"); !errors.Is(err, ErrNilKeyring) {
		t.Errorf("Open on a nil keyring: want ErrNilKeyring, got %v", err)
	}
}

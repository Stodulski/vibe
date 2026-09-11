package crypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testKeyring(t *testing.T, kid string, fill byte) *Keyring {
	t.Helper()
	kr, err := ParseKeyring(kid + ":" + b64Key32(fill))
	if err != nil {
		t.Fatalf("building test keyring: %v", err)
	}
	return kr
}

func TestSealOpen_RoundTrip(t *testing.T) {
	kr := testKeyring(t, "k1", 0x10)
	aad := []byte("mpcred\x1fv1\x1fcomplex-1\x1fmp_access_token")

	sealed, err := kr.Seal(aad, "seller-access-token")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if !strings.HasPrefix(sealed, "v1.k1.") {
		t.Fatalf("envelope %q does not match the v1.<kid>.<payload> shape", sealed)
	}

	got, err := kr.Open(aad, sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got != "seller-access-token" {
		t.Errorf("got %q, want %q", got, "seller-access-token")
	}
}

func TestSealOpen_ProducesFreshNoncePerCall(t *testing.T) {
	kr := testKeyring(t, "k1", 0x11)
	aad := []byte("aad")

	first, err := kr.Seal(aad, "same-plaintext")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	second, err := kr.Seal(aad, "same-plaintext")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if first == second {
		t.Error("two seals of the same plaintext produced identical ciphertext — nonce is not fresh per call")
	}
}

func TestOpen_WrongKeyID(t *testing.T) {
	sealingKeyring := testKeyring(t, "k1", 0x12)
	openingKeyring := testKeyring(t, "k2", 0x13) // k1 is absent from this keyring
	aad := []byte("aad")

	sealed, err := sealingKeyring.Seal(aad, "secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if _, err := openingKeyring.Open(aad, sealed); !errors.Is(err, ErrUnknownKeyID) {
		t.Errorf("want ErrUnknownKeyID; got %v", err)
	}
}

func TestOpen_WrongAAD(t *testing.T) {
	kr := testKeyring(t, "k1", 0x14)

	sealed, err := kr.Seal([]byte("complex-1"), "secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Same envelope, same key, but a different complex ID bound into the
	// AAD — this is exactly the attack the AAD exists to catch: copying a
	// sealed value from one row (or one column) to another must fail to
	// open rather than silently succeed against the wrong row.
	if _, err := kr.Open([]byte("complex-2"), sealed); err == nil {
		t.Error("want an error when the AAD used to open differs from the AAD used to seal; got nil")
	}
}

func TestOpen_FlippedCiphertextByte(t *testing.T) {
	kr := testKeyring(t, "k1", 0x15)
	aad := []byte("aad")

	sealed, err := kr.Seal(aad, "secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	tampered := flipLastPayloadByte(t, sealed)
	if _, err := kr.Open(aad, tampered); err == nil {
		t.Error("want an error when a ciphertext/tag byte is flipped; got nil (silent corruption)")
	}
}

// flipLastPayloadByte corrupts an envelope by flipping a bit in its DECODED
// bytes and re-encoding, so the tamper is guaranteed to be real.
//
// It used to poke the last base64 character instead, on the theory that any
// different character changes at least one decoded bit. That is false for a
// RawURLEncoding payload: the final character carries only the leftover bits
// of the last byte, and the rest are padding the decoder discards. Roughly
// half the time the "tampered" envelope decoded to the original bytes, GCM
// opened it correctly, and this test failed — the failure was the code being
// right. Flipping a decoded byte cannot be a no-op.
func flipLastPayloadByte(t *testing.T, envelope string) string {
	t.Helper()

	parts := strings.SplitN(envelope, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("envelope %q is not shaped v1.<kid>.<payload>", envelope)
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decoding payload of %q: %v", envelope, err)
	}
	if len(raw) == 0 {
		t.Fatalf("payload of %q decoded to nothing", envelope)
	}
	raw[len(raw)-1] ^= 0x01

	return parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(raw)
}

func TestOpen_TruncatedEnvelope(t *testing.T) {
	kr := testKeyring(t, "k1", 0x16)
	aad := []byte("aad")

	sealed, err := kr.Seal(aad, "secret")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Cut the payload segment down to something shorter than one nonce.
	parts := strings.SplitN(sealed, ".", 3)
	truncated := parts[0] + "." + parts[1] + "." + parts[2][:4]

	if _, err := kr.Open(aad, truncated); !errors.Is(err, ErrMalformedEnvelope) {
		t.Errorf("want ErrMalformedEnvelope for a truncated envelope; got %v", err)
	}
}

func TestOpen_PlaintextInputIsNotAnEnvelope(t *testing.T) {
	kr := testKeyring(t, "k1", 0x17)
	aad := []byte("aad")

	// Exactly the hazard this format exists to catch: a raw stored value
	// that was never sealed at all must not be silently accepted as if it
	// were an envelope.
	for _, in := range []string{
		"APP_USR-1234567890abcdef-081512-abcdef",
		"",
		"v1",
		"v1.k1",
		"v2.k1.whatever",
	} {
		if _, err := kr.Open(aad, in); !errors.Is(err, ErrMalformedEnvelope) {
			t.Errorf("Open(%q): want ErrMalformedEnvelope; got %v", in, err)
		}
	}
}

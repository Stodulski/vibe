package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/social"
)

func testKeys(t *testing.T) *crypto.Keyring {
	t.Helper()
	keys, err := crypto.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatalf("keyring: %v", err)
	}
	return keys
}

func TestThreadsTokenIsSealedAndOpensOnlyWithItsColumn(t *testing.T) {
	t.Parallel()
	keys := testKeys(t)
	sealed, err := keys.Seal(threadsTokenAAD(), "long-token-value")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if strings.Contains(sealed, "long-token-value") {
		t.Fatal("the sealed value must not hold the token in plain text")
	}
	plain, err := keys.Open(threadsTokenAAD(), sealed)
	if err != nil || plain != "long-token-value" {
		t.Fatalf("open = %q, %v; want the token back", plain, err)
	}

	otherColumn := []byte("social.threads\x1fv1\x1frefresh_token")
	if _, err := keys.Open(otherColumn, sealed); err == nil {
		t.Fatal("a token copied to another column must not open")
	}
	if _, err := keys.Open([]byte("mpcred\x1fv1\x1fmp_access_token"), sealed); err == nil {
		t.Fatal("a token copied to another credential kind must not open")
	}
}

func TestSavingWithoutAKeyringNeverWritesThePlainToken(t *testing.T) {
	t.Parallel()
	// The store has no database here: a nil keyring must fail before any query,
	// so a missing key can never store the token unsealed.
	s := &Threads{}
	err := s.SaveThreads(context.Background(), social.ThreadsConnection{
		UserID: "42", Username: "vibe.padel", AccessToken: "long-token-value",
		ConnectedAt: time.Now(), RefreshedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	})
	if !errors.Is(err, crypto.ErrNilKeyring) {
		t.Fatalf("err = %v; want ErrNilKeyring", err)
	}
}

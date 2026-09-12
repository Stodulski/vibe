package redis_test

import (
	"strings"
	"testing"

	platformredis "github.com/stodulski/vibe-server/internal/platform/redis"
)

// TestKeyPrefixNamesTheApplicationAndTheEnvironment is RED-01's rule in one
// place: every key this service writes has to say which application and which
// deployment wrote it, or two environments sharing one Redis serve each
// other's sessions and cached accounts.
func TestKeyPrefixNamesTheApplicationAndTheEnvironment(t *testing.T) {
	tests := []struct {
		env  string
		want string
	}{
		{"production", "vibe:production:"},
		{"staging", "vibe:staging:"},
		{"development", "vibe:development:"},
		// Not "development": an empty environment means nobody said, and
		// guessing at a namespace is exactly the collision this prevents.
		{"", "vibe:unset:"},
	}

	for _, tt := range tests {
		if got := platformredis.KeyPrefix(tt.env); got != tt.want {
			t.Errorf("KeyPrefix(%q) = %q, want %q", tt.env, got, tt.want)
		}
	}
}

// TestTwoEnvironmentsNeverShareAPrefix is the property the table above only
// illustrates.
func TestTwoEnvironmentsNeverShareAPrefix(t *testing.T) {
	staging := platformredis.KeyPrefix("staging")
	production := platformredis.KeyPrefix("production")

	if staging == production {
		t.Fatal("staging and production share a key namespace")
	}
	// Neither may be a prefix of the other, or a SCAN or a bulk delete aimed
	// at one would reach the other.
	if strings.HasPrefix(staging, production) || strings.HasPrefix(production, staging) {
		t.Errorf("%q and %q overlap; a scan over one reaches the other", staging, production)
	}
}

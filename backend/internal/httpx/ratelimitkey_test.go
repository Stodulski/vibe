package httpx

import "testing"

func TestRateLimitKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"an IPv4 address is its own key", "203.0.113.7", "203.0.113.7"},
		{"an IPv4-mapped IPv6 address keys as the IPv4 address", "::ffff:203.0.113.7", "203.0.113.7"},
		{"an IPv6 address keys as its /64", "2001:db8:1:2:aaaa:bbbb:cccc:dddd", "2001:db8:1:2::/64"},
		{"two addresses in one /64 share a key", "2001:db8:1:2::1", "2001:db8:1:2::/64"},
		{"an IPv6 zone is dropped before masking", "fe80::1%eth0", "fe80::/64"},
		{"an unparseable value is kept as it came", "unknown", "unknown"},
		{"empty stays empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RateLimitKey(tc.in); got != tc.want {
				t.Fatalf("RateLimitKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRateLimitKeySeparatesNeighbouringNetworks(t *testing.T) {
	a := RateLimitKey("2001:db8:1:2::1")
	b := RateLimitKey("2001:db8:1:3::1")
	if a == b {
		t.Fatalf("addresses in different /64s share the key %q", a)
	}
}

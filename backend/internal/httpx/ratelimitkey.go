package httpx

import "net/netip"

// ipv6RateLimitPrefix is the network an IPv6 client is counted as. One
// subscriber is routinely handed a whole /64 and can pick a fresh address in
// it for every request, so keying a ceiling on the full address gives that one
// host a full bucket per request. A /64 is the smallest block an ISP assigns
// to a single site, so it is the narrowest key that cannot be rotated.
const ipv6RateLimitPrefix = 64

// RateLimitKey turns a client address, as ClientIPFrom returns it, into the
// key the rate limiter counts against.
//
// An IPv4 address (including an IPv4-mapped IPv6 one) is its own key. An IPv6
// address is masked to its /64, written in prefix form ("2001:db8:1:2::/64")
// so it can never collide with a single address. A value that does not parse
// is returned unchanged: the limiter still has to count it as something, and
// ClientIPFrom already canonicalises every address it can read.
//
// Only the limiter uses this. The audit trail and Turnstile keep the full
// address, which is what identifies the request.
func RateLimitKey(clientIP string) string {
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return clientIP
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String()
	}
	prefix, err := addr.Prefix(ipv6RateLimitPrefix)
	if err != nil {
		return clientIP
	}
	return prefix.String()
}

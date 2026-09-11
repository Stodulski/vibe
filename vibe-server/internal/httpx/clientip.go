package httpx

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedProxies is the set of peers allowed to speak for somebody else.
//
// # Why a CIDR set rather than a hop count
//
// Both models answer the same question — how much of X-Forwarded-For was
// written by our own infrastructure — and both are sound. The set is chosen
// here for one reason: it degrades safely when the deployment changes shape.
//
// A hop count says "discard the last N entries". Get N wrong, or let the
// platform add an edge (a CDN in front of the load balancer) or drop one, and
// the header index that lands is an entry the client wrote. The failure is a
// forged address that looks entirely legitimate, and nothing in the request
// distinguishes it from the real thing.
//
// A set says "discard entries appended by a peer I recognise". Get it wrong
// and the walk stops early: the address that lands is the proxy's own, which
// is wrong but is not attacker-chosen. One misconfiguration mints buckets for
// an attacker; the other collapses everyone behind one bucket, which is
// visible within minutes and hurts nobody's account. Between an error that
// fails open and an error that fails loud, this takes the loud one.
//
// The set also matches how the deployment is actually described. Railway,
// Cloudflare and a plain nginx in a compose file all have knowable address
// ranges; none of them promises a stable hop count.
//
// The zero value trusts nobody, which is the safe default: with no proxy in
// front, the forwarded headers are attacker-supplied and RemoteAddr is the
// only address that means anything.
type TrustedProxies struct {
	prefixes []netip.Prefix
}

// privatePrefixes is what "trust the platform in front of me" resolves to: the
// address ranges a container platform routes its own edge traffic over. It is
// deliberately not "everything" — see ParseTrustedProxies, which refuses a
// default route.
var privatePrefixes = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),    // loopback
	netip.MustParsePrefix("10.0.0.0/8"),     // RFC 1918
	netip.MustParsePrefix("172.16.0.0/12"),  // RFC 1918
	netip.MustParsePrefix("192.168.0.0/16"), // RFC 1918
	netip.MustParsePrefix("100.64.0.0/10"),  // RFC 6598, what most PaaS meshes use
	netip.MustParsePrefix("169.254.0.0/16"), // link-local
	netip.MustParsePrefix("::1/128"),        // loopback
	netip.MustParsePrefix("fc00::/7"),       // unique local
	netip.MustParsePrefix("fe80::/10"),      // link-local
}

// DefaultTrustedProxies is the set TRUSTED_PROXIES=true resolves to: loopback,
// the RFC 1918 ranges, the carrier-grade NAT range and the IPv6 equivalents.
//
// A proxy that reaches this process from a public address is not covered and
// must be named explicitly — that is the point. A public peer allowed to
// rewrite the client address by default is the whole vulnerability.
func DefaultTrustedProxies() TrustedProxies {
	return TrustedProxies{prefixes: privatePrefixes}
}

// ErrTrustsEveryPeer is returned by ParseTrustedProxies for a set that would
// trust every possible peer.
var ErrTrustsEveryPeer = errors.New("a trusted-proxy set covering every address trusts every caller, " +
	"which is the forgery this setting exists to prevent")

// ParseTrustedProxies reads the operator's trusted-proxy setting.
//
// Accepted forms:
//
//	"", "false", "off", "0", "none"  trust nobody (the default)
//	"true", "on", "1", "private"     the DefaultTrustedProxies set
//	"10.0.0.0/8, 192.0.2.7"          exactly these prefixes and addresses
//
// A bare address is read as a single-host prefix. A prefix covering the whole
// address space is refused rather than accepted, because "trust everyone" is
// indistinguishable from having no check at all, and an operator who types it
// into an environment variable will not find out until somebody forges a
// header. Failing at boot is the only moment that mistake is cheap.
func ParseTrustedProxies(spec string) (TrustedProxies, error) {
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case "", "false", "off", "0", "none":
		return TrustedProxies{}, nil
	case "true", "on", "1", "private":
		return DefaultTrustedProxies(), nil
	}

	var prefixes []netip.Prefix
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		prefix, err := parsePrefixOrAddr(field)
		if err != nil {
			return TrustedProxies{}, err
		}
		if prefix.Bits() == 0 {
			return TrustedProxies{}, fmt.Errorf("%q: %w", field, ErrTrustsEveryPeer)
		}
		prefixes = append(prefixes, prefix)
	}

	return TrustedProxies{prefixes: prefixes}, nil
}

// parsePrefixOrAddr reads either a CIDR prefix or a single address.
func parsePrefixOrAddr(field string) (netip.Prefix, error) {
	if strings.Contains(field, "/") {
		prefix, err := netip.ParsePrefix(field)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("trusted proxy %q is not a CIDR prefix: %w", field, err)
		}
		return prefix.Masked(), nil
	}

	addr, err := netip.ParseAddr(field)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("trusted proxy %q is not an address or CIDR prefix: %w", field, err)
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// Any reports whether any peer at all is trusted. A set that trusts nobody
// makes every forwarded header irrelevant.
func (tp TrustedProxies) Any() bool { return len(tp.prefixes) > 0 }

// String renders the set for a startup log line.
func (tp TrustedProxies) String() string {
	if len(tp.prefixes) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(tp.prefixes))
	for _, p := range tp.prefixes {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, ",")
}

// contains reports whether addr is one of the trusted peers.
func (tp TrustedProxies) contains(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range tp.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// TrustsPeer reports whether the request's own peer address is a trusted
// proxy. A request carrying X-Forwarded-For from an untrusted peer is either
// an attempted forgery or a misconfigured trusted-proxy set, and the caller
// may want to say so in a log line — from this side the two are identical.
func (tp TrustedProxies) TrustsPeer(r *http.Request) bool {
	peer, ok := peerAddr(r)
	return ok && tp.contains(peer)
}

// maxForwardedHops bounds the right-to-left walk. Nobody runs fifty proxies;
// a header with more entries than this is padding, and walking all of it is
// work an unauthenticated caller gets to choose the size of.
const maxForwardedHops = 50

// ClientIPFrom returns the address the request came from, in canonical form.
//
// The forwarded headers are read only when the peer is a trusted proxy, and
// then only right-to-left: each entry is believed exactly as far as the host
// that appended it is trusted. The rightmost entry was appended by the peer;
// entry i was appended by the host named in entry i+1. The walk stops at the
// first entry whose appender is trusted but which is not itself trusted — that
// is the client, and everything to its left is whatever the client chose to
// send.
//
// Everything that comes back parses as an IP address. The previous version
// returned the header string as it arrived, and an unparseable one reached the
// audit trail as a NULL ip_address: a garbage header erased attribution
// without erroring anywhere.
func ClientIPFrom(r *http.Request, trusted TrustedProxies) string {
	peer, peerOK := peerAddr(r)

	if peerOK && trusted.contains(peer) {
		if addr, ok := forwardedClient(r, trusted); ok {
			return addr.String()
		}
		if addr, ok := parseHeaderAddr(r.Header.Get("X-Real-IP")); ok {
			return addr.String()
		}
	}

	if peerOK {
		return peer.String()
	}
	// RemoteAddr is not always host:port — a unix socket, for instance. There
	// is nothing better to key on, so it is returned as-is.
	return r.RemoteAddr
}

// ClientIP returns the address the request came from, trusting the forwarded
// headers only when trustProxies is set.
//
// It is the boolean-shaped entry point the audit-log call sites still use, and
// it resolves to DefaultTrustedProxies. A deployment whose proxy sits on a
// public address must therefore configure the set AND move those call sites to
// ClientIPFrom, or the rate limiter and the audit trail will disagree about
// who made the request. Prefer ClientIPFrom in new code.
func ClientIP(r *http.Request, trustProxies bool) string {
	if trustProxies {
		return ClientIPFrom(r, DefaultTrustedProxies())
	}
	return ClientIPFrom(r, TrustedProxies{})
}

// forwardedClient walks X-Forwarded-For from the right and returns the first
// entry whose appending peer is trusted but which is not itself a trusted
// proxy.
func forwardedClient(r *http.Request, trusted TrustedProxies) (netip.Addr, bool) {
	entries := forwardedEntries(r)

	for i := len(entries) - 1; i >= 0 && len(entries)-i <= maxForwardedHops; i-- {
		addr, ok := parseHeaderAddr(entries[i])
		if !ok {
			// An entry that is not an address means the chain is not what it
			// claims to be from here leftwards. Stop and fall back to the peer
			// rather than guessing.
			return netip.Addr{}, false
		}
		if !trusted.contains(addr) {
			return addr, true
		}
	}

	// Every entry was a trusted proxy: the client is not in the header.
	return netip.Addr{}, false
}

// forwardedEntries flattens X-Forwarded-For into its entries, in order. The
// header may legitimately appear more than once; the entries then concatenate.
func forwardedEntries(r *http.Request) []string {
	values := r.Header.Values("X-Forwarded-For")
	if len(values) == 0 {
		return nil
	}

	var entries []string
	for _, value := range values {
		for _, entry := range strings.Split(value, ",") {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

// parseHeaderAddr parses one forwarded entry, tolerating the port some proxies
// append.
func parseHeaderAddr(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Addr{}, false
	}

	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap(), true
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr.Unmap(), true
		}
	}
	return netip.Addr{}, false
}

// peerAddr is the address the TCP connection was accepted from.
func peerAddr(r *http.Request) (netip.Addr, bool) {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr.Unmap(), true
		}
	}
	if addr, err := netip.ParseAddr(r.RemoteAddr); err == nil {
		return addr.Unmap(), true
	}
	return netip.Addr{}, false
}

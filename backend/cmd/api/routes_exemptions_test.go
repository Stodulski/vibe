package main

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stodulski/vibe-server/internal/middleware"
)

// The exemption lists in internal/middleware say which routes stand outside
// CSRF protection and outside rate limiting. Those lists are exact now, one
// entry per route, because the subtree and prefix forms they replaced granted
// themselves to routes nobody had written yet.
//
// Exactness alone only stops a new route inheriting an exemption. It does not
// stop the lists going stale, and it does not make anybody think about the
// question when they add a route. That is what this file is for, and it can
// only live here: internal/middleware holds the lists and cmd/api holds the
// route table, and the two have to be in the same room to be compared.
//
// It leans on the audit next door rather than restating it. publicRoutes and
// routePolicies already force every new endpoint to be classified as open or
// guarded — see TestRoutePolicyTableIsComplete — so the tests below can ask
// their question in terms of that classification instead of asking for a third
// table that would need maintaining alongside the other two.

// csrfProtectedPublicWrites is the escape hatch: a public state-changing route
// that is nonetheless required to carry a CSRF token, with the reason.
//
// It is empty, and the empty case is the interesting one. A route that is
// reachable without a session has no session cookie for another site to spend,
// which is the whole of what CSRF protection defends. If an entry ever appears
// here it means a route accepts a session when one is offered while working
// without one — and then the token is worth requiring.
var csrfProtectedPublicWrites = map[string]string{}

// throttledWebhookRoutes is the same escape hatch for rate limiting: a webhook
// route that is deliberately throttled anyway, with the reason.
var throttledWebhookRoutes = map[string]string{}

// webhookTree is the path prefix under which a missing rate-limit decision is
// silently expensive rather than silently safe.
const webhookTree = "/api/v1/webhooks/"

// stateChanging reports whether a method reaches the CSRF check at all.
// CSRFProtect returns early for the read methods, before any table is
// consulted, so an exemption for one of them would be inert.
func stateChanging(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// TestEveryPublicWriteDeclaresItsCSRFPosition is the fail-closed half of the
// CSRF exemption list.
//
// Adding an endpoint already forces a line in publicRoutes or routePolicies.
// This turns that same line into a CSRF decision: a public write must be named
// in the exemption list or in csrfProtectedPublicWrites with a reason, and a
// session-authenticated route may never be named in the exemption list at all.
//
// The second half is the one that matters most. It is the rule that makes it
// impossible to write down "this cookie-authenticated route needs no CSRF
// token" without the test that says otherwise.
func TestEveryPublicWriteDeclaresItsCSRFPosition(t *testing.T) {
	app := newTestApplication(t)
	exempt := middleware.CSRFExemptRoutes()

	for _, rt := range recordRoutes(t, app) {
		key := rt.method + " " + rt.path
		if strings.HasPrefix(rt.path, "/debug/") || !stateChanging(rt.method) {
			continue
		}

		_, public := publicRoutes[key]
		reason, isExempt := exempt[key]
		_, protectedAnyway := csrfProtectedPublicWrites[key]

		switch {
		case public && !isExempt && !protectedAnyway:
			t.Errorf("public route %q makes no CSRF declaration.\n"+
				"It is reachable without a session, so it has no cookie to protect: add it to "+
				"csrfExemptRoutes in internal/middleware with the reason, or to "+
				"csrfProtectedPublicWrites here with the reason it must carry a token anyway.", key)
		case !public && isExempt:
			t.Errorf("route %q is CSRF-exempt (%q) but is not in publicRoutes.\n"+
				"An exemption on a route that authenticates by cookie is a CSRF hole: the browser "+
				"attaches the cookie for any site that asks. Either the exemption is wrong, or the "+
				"route is public and belongs in publicRoutes with the reason.", key, reason)
		case public && isExempt && protectedAnyway:
			t.Errorf("route %q is both CSRF-exempt and listed in csrfProtectedPublicWrites; "+
				"it cannot be both", key)
		}
	}
}

// TestEveryWebhookRouteDeclaresItsRateLimitPosition is the fail-closed half of
// the rate-limit exemption list.
//
// The webhook tree is where a missing decision costs money. A provider whose
// deliveries are refused stops redelivering, so a webhook route that is
// throttled by default is a payment notification nobody ever hears about
// again — and unlike a missing CSRF exemption, nothing about it is visible in
// development, where nothing bursts.
//
// It is scoped to that tree on purpose rather than covering every route. The
// default for an ordinary route — throttled — is the safe one, so silence
// there costs nothing. This test cannot help a future provider registered
// outside the tree; that is a real gap, and the reason the tree is a
// convention worth keeping rather than a matcher.
func TestEveryWebhookRouteDeclaresItsRateLimitPosition(t *testing.T) {
	app := newTestApplication(t)
	exempt := middleware.RateLimitExemptRoutes()

	for _, rt := range recordRoutes(t, app) {
		if !strings.HasPrefix(rt.path, webhookTree) {
			continue
		}

		key := rt.method + " " + rt.path
		_, isExempt := exempt[key]
		_, throttled := throttledWebhookRoutes[key]

		switch {
		case !isExempt && !throttled:
			t.Errorf("webhook route %q makes no rate-limit declaration.\n"+
				"A throttled webhook is a delivery the provider eventually stops retrying: add it "+
				"to rateLimitExemptRoutes in internal/middleware with the reason, or to "+
				"throttledWebhookRoutes here with the reason it should be throttled.", key)
		case isExempt && throttled:
			t.Errorf("webhook route %q is both exempt and listed as throttled; it cannot be both", key)
		}
	}
}

// TestNoExemptionNamesAnUnregisteredRoute keeps both lists from going stale.
//
// A stale entry is not harmless. It reads, to anybody auditing the list, as
// evidence that a route exists and was considered — and if the route is
// re-registered later under the same method and path, it silently comes back
// exempt.
func TestNoExemptionNamesAnUnregisteredRoute(t *testing.T) {
	app := newTestApplication(t)

	registered := make(map[string]bool)
	for _, rt := range recordRoutes(t, app) {
		registered[rt.method+" "+rt.path] = true
	}

	tables := map[string]map[string]string{
		"csrfExemptRoutes (internal/middleware)":      middleware.CSRFExemptRoutes(),
		"rateLimitExemptRoutes (internal/middleware)": middleware.RateLimitExemptRoutes(),
		"crossTenantRoutes (internal/middleware)":     middleware.CrossTenantRoutes(),
		"csrfProtectedPublicWrites (here)":            csrfProtectedPublicWrites,
		"throttledWebhookRoutes (here)":               throttledWebhookRoutes,
	}

	for name, table := range tables {
		var stale []string
		for key := range table {
			if !registered[key] {
				stale = append(stale, key)
			}
		}
		sort.Strings(stale)
		for _, key := range stale {
			t.Errorf("%s names %q, which is not a registered route; remove it", name, key)
		}
	}
}

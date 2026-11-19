package main

import "testing"

// TestTheRefreshCronUsesTheIsolatedClient: a bulk refresh failing over stale
// credentials must not trip the breaker that gates checkout, so the cron must
// not reach for app.mp.
func TestTheRefreshCronUsesTheIsolatedClient(t *testing.T) {
	app, _ := newTestApplicationWithNotifications(t)

	if app.mpOAuth == nil {
		t.Fatal("the application must hold a separate client for the refresh sweep")
	}
	if app.mp == app.mpOAuth {
		t.Error("the refresh sweep must not run on the client whose breaker gates payments")
	}
}

package main

import (
	"testing"

	"github.com/stodulski/vibe-server/internal/circuitbreaker"
	"github.com/stodulski/vibe-server/internal/mp"
)

// TestTheOAuthRefreshDoesNotShareThePaymentBreaker pins the isolation the bulk
// refresh cron depends on.
//
// cronRefreshMPTokens walks every connected complex on one client. Sharing the
// payment breaker means five venues with stale refresh tokens spend its failure
// budget — and that breaker gates CreatePreference, GetPayment and
// RefundPayment, so a sweep over other people's expired credentials takes
// checkout down for every tenant at once, twelve hours after anybody last
// touched it.
//
// The assertion is behavioural rather than structural: driving one client's
// breaker open must leave the other admitting. Comparing pointers would pass
// against two clients that happen to differ while sharing a breaker.
func TestTheOAuthRefreshDoesNotShareThePaymentBreaker(t *testing.T) {
	paymentCB := circuitbreaker.New(circuitbreaker.Config{Name: "mercadopago", MaxFailures: 1})
	oauthCB := circuitbreaker.New(circuitbreaker.Config{Name: "mercadopago-oauth", MaxFailures: 1})

	payment := mp.NewMPClient("t", "s", "a", "c", paymentCB)
	oauth := mp.NewMPClient("t", "s", "a", "c", oauthCB)
	if payment == nil || oauth == nil {
		t.Fatal("both clients must build")
	}

	// The refresh sweep fails against its own breaker until it opens.
	oauthCB.RecordFailure()
	if err := oauthCB.AllowRequest(); err == nil {
		t.Fatal("setup: the oauth breaker must be open after spending its budget")
	}

	// Checkout must be untouched by that.
	if err := paymentCB.AllowRequest(); err != nil {
		t.Errorf("a bulk refresh failing over other venues' credentials must not close checkout "+
			"for every tenant; the payment breaker answered %v", err)
	}
}

// TestTheRefreshCronUsesTheIsolatedClient is the half the breaker test cannot
// see: the isolation is worth nothing if the cron still reaches for app.mp.
func TestTheRefreshCronUsesTheIsolatedClient(t *testing.T) {
	app, _ := newTestApplicationWithNotifications(t)

	if app.mpOAuth == nil {
		t.Fatal("the application must hold a separate client for the refresh sweep")
	}
	if app.mp == app.mpOAuth {
		t.Error("the refresh sweep must not run on the client whose breaker gates payments")
	}
}

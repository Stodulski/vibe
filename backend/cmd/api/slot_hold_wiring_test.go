package main

import (
	"bytes"
	"net/http"
	"testing"
)

// The hold release is called by navigator.sendBeacon as the page goes away: no
// session, no CSRF header, a text/plain body or none. Through the whole chain
// it must answer an empty 204, for a token nobody holds as for one somebody
// does.
func TestReleasingASlotHoldThroughTheChainIsAnEmpty204(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	for _, body := range []string{"", "beacon"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
			ts.URL+"/api/v1/book/holds/no-such-token/release", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
		}
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("body %q: want 204; got %d", body, resp.StatusCode)
		}
	}
}

// Taking a hold is public and CSRF-exempt like POST /book: a malformed request
// reaches the handler's own validation (422), not a CSRF or auth refusal.
func TestTakingASlotHoldReachesTheHandlerWithoutASession(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	status, body := postJSON(t, ts, "/api/v1/book/holds", `{"complex_id":"x"}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 from the handler; got %d (%s)", status, body)
	}
}

package instagram

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The publisher tells Meta's refusals apart by their Graph code: 9007 and 24
// are retried, 190 asks the owner to reconnect. The code must survive on both
// kinds of answer, and the token must not reach the error.
func TestPublishingErrorsCarryTheGraphCode(t *testing.T) {
	cases := []struct {
		status int
		body   string
		kind   error
		code   int
	}{
		{http.StatusBadRequest, `{"error":{"message":"Media ID is not available","code":9007,"error_subcode":2207027}}`, ErrRejected, CodeMediaNotReady},
		{http.StatusBadRequest, `{"error":{"message":"Invalid OAuth access token","code":190}}`, ErrRejected, CodeInvalidToken},
		{http.StatusInternalServerError, `{"error":{"message":"An unexpected error has occurred","code":2}}`, ErrUnavailable, 2},
		{http.StatusBadGateway, `<html>bad gateway</html>`, ErrUnavailable, 0},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		_, err := New(Config{GraphBase: srv.URL}).PublishContainer(t.Context(), "1784", secretToken, "c1")
		srv.Close()
		if !errors.Is(err, tc.kind) {
			t.Errorf("%d %s: err = %v, want %v", tc.status, tc.body, err, tc.kind)
		}
		if got := GraphCode(err); got != tc.code {
			t.Errorf("%d %s: GraphCode = %d, want %d", tc.status, tc.body, got, tc.code)
		}
		if err != nil && strings.Contains(err.Error(), secretToken) {
			t.Errorf("the error carries the token: %v", err)
		}
	}
}

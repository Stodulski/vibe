package meta

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
)

const grantedAll = `{"data":[` +
	`{"permission":"pages_show_list","status":"granted"},` +
	`{"permission":"pages_read_engagement","status":"granted"},` +
	`{"permission":"pages_manage_posts","status":"granted"},` +
	`{"permission":"instagram_basic","status":"granted"},` +
	`{"permission":"instagram_content_publish","status":"granted"},` +
	`{"permission":"business_management","status":"granted"}]}`

const onePageWithInstagram = `{"data":[{"id":"page-1","name":"Vibe","instagram_business_account":{"id":"ig-1","username":"vibe.padel"}}]}`

// connectionAnswer answers the two reads CheckConnection makes.
func connectionAnswer(accounts, permissions string) func(exchange) (int, string) {
	return func(e exchange) (int, string) {
		switch e.path {
		case "/v25.0/me/accounts":
			return http.StatusOK, accounts
		case "/v25.0/me/permissions":
			return http.StatusOK, permissions
		}
		return http.StatusNotFound, `{"error":{"message":"no route"}}`
	}
}

func TestConnectionCheckReportsThePageInstagramAndAllPermissions(t *testing.T) {
	t.Parallel()
	fake, client := newFake(t, connectionAnswer(onePageWithInstagram, grantedAll))

	r := client.CheckConnection(context.Background())
	if !r.OK() {
		t.Fatalf("CheckConnection() = %+v, want OK", r)
	}
	if r.PageName != "Vibe" || r.PageID != "page-1" || r.IGUsername != "vibe.padel" || r.IGUserID != "ig-1" {
		t.Fatalf("report = %+v, want the page and the Instagram username and id", r)
	}
	if got := fake.calls()[0].params.Get("fields"); got != "id,name,instagram_business_account{id,username}" {
		t.Fatalf("accounts fields = %q, want the Instagram username requested", got)
	}
}

func TestConnectionCheckNamesEveryMissingPermission(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, connectionAnswer(onePageWithInstagram,
		`{"data":[{"permission":"pages_manage_posts","status":"declined"},{"permission":"instagram_basic","status":"granted"}]}`))

	r := client.CheckConnection(context.Background())
	if r.OK() {
		t.Fatal("a declined permission must not report OK")
	}
	if !slices.Contains(r.Missing, "pages_manage_posts") || !slices.Contains(r.Missing, "instagram_content_publish") {
		t.Fatalf("missing = %v, want the declined and the never-granted permissions", r.Missing)
	}
	if slices.Contains(r.Missing, "instagram_basic") {
		t.Fatal("a granted permission must not be listed as missing")
	}
}

func TestConnectionCheckKeepsGoingWhenAPICallFails(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if e.path == "/v25.0/me/accounts" {
			return http.StatusBadRequest, `{"error":{"message":"Error validating access token","code":190}}`
		}
		return connectionAnswer("", grantedAll)(e)
	})

	r := client.CheckConnection(context.Background())
	if len(r.Problems) == 0 || !strings.Contains(strings.Join(r.Problems, " "), "190") {
		t.Fatalf("problems = %v, want the Graph error named", r.Problems)
	}
	if strings.Contains(strings.Join(r.Problems, " "), systemToken) {
		t.Fatal("a problem must never carry the token")
	}
	if len(r.Missing) != 0 {
		t.Fatalf("missing = %v, want the permissions still read after the page call failed", r.Missing)
	}
}

func TestConnectionCheckRefusesAmbiguousPages(t *testing.T) {
	t.Parallel()
	twoPages := `{"data":[` +
		`{"id":"a","name":"A","instagram_business_account":{"id":"ig-a","username":"a"}},` +
		`{"id":"b","name":"B","instagram_business_account":{"id":"ig-b","username":"b"}}]}`
	_, client := newFake(t, connectionAnswer(twoPages, grantedAll))

	r := client.CheckConnection(context.Background())
	if r.OK() || !strings.Contains(strings.Join(r.Problems, " "), "META_SOCIAL_PAGE_ID") {
		t.Fatalf("report = %+v, want a problem asking for META_SOCIAL_PAGE_ID", r)
	}
}

func TestConnectionCheckFlagsAPageWithoutInstagram(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, connectionAnswer(`{"data":[{"id":"page-1","name":"Vibe"}]}`, grantedAll))

	r := client.CheckConnection(context.Background())
	if r.PageID != "page-1" || r.IGUserID != "" || r.OK() {
		t.Fatalf("report = %+v, want the page named and a problem for the missing Instagram account", r)
	}
}

func TestConnectionCheckReportsAPermissionReadFailure(t *testing.T) {
	t.Parallel()
	_, client := newFake(t, func(e exchange) (int, string) {
		if e.path == "/v25.0/me/accounts" {
			return http.StatusOK, onePageWithInstagram
		}
		return http.StatusInternalServerError, `{"error":{"message":"down"}}`
	})

	r := client.CheckConnection(context.Background())
	if r.OK() || len(r.Problems) == 0 {
		t.Fatalf("report = %+v, want a problem for the failed permission read", r)
	}
}

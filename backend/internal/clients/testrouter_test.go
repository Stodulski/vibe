package clients

import (
	"net/http"
)

// withParam attaches a router path parameter to r, the way net/http's ServeMux
// does when it dispatches a matched route.
func withParam(r *http.Request, key, value string) *http.Request {
	r.SetPathValue(key, value)
	return r
}

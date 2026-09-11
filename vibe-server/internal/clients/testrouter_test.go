package clients

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// withParam attaches a router path parameter to r, the way httprouter does
// when it dispatches a matched route.
func withParam(r *http.Request, key, value string) *http.Request {
	params := httprouter.Params{{Key: key, Value: value}}
	return r.WithContext(context.WithValue(r.Context(), httprouter.ParamsKey, params))
}

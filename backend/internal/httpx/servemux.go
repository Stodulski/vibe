package httpx

import (
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
)

// ServeMux is the application's router: net/http's own ServeMux, behind the
// Router interface the domain modules register against.
//
// Go 1.22 gave ServeMux method-and-wildcard patterns ("GET /api/v1/x/{id}"),
// which is the whole reason a third-party router was here. What it does not
// give is the two answers this API owes a caller who missed: a JSON 404 and a
// JSON 405 with an Allow header. ServeMux answers both in plain text, and it
// cannot tell the two apart for a caller — an unrouted method on a known path
// is simply "404 page not found".
//
// So registration is recorded as it happens, and Build closes the table with
// two fallbacks derived from it: one pattern per known path with no method,
// which ServeMux consults only after every method-qualified pattern for that
// path has failed to match, and one "/" pattern, which is less specific than
// anything else and therefore catches only the paths nobody registered.
// Precedence does the work; nothing is matched twice.
type ServeMux struct {
	mux *http.ServeMux
	// methods is every method registered for a path, keyed by path. It is
	// what Build turns into the 405 fallbacks and their Allow headers.
	methods map[string][]string

	notFound         http.Handler
	methodNotAllowed http.Handler
}

// NewServeMux returns a router that answers an unknown path with notFound and
// a known path reached by an unregistered method with methodNotAllowed.
//
// Both are passed in rather than built here because the JSON shape of an error
// body belongs to the Responder, and this type is about routing.
func NewServeMux(notFound, methodNotAllowed http.Handler) *ServeMux {
	return &ServeMux{
		mux:              http.NewServeMux(),
		methods:          make(map[string][]string),
		notFound:         notFound,
		methodNotAllowed: methodNotAllowed,
	}
}

// HandlerFunc registers handler for method and path.
func (m *ServeMux) HandlerFunc(method, path string, handler http.HandlerFunc) {
	m.Handler(method, path, handler)
}

// Handler registers a plain http.Handler for method and path.
func (m *ServeMux) Handler(method, path string, handler http.Handler) {
	m.mux.Handle(method+" "+path, handler)
	m.methods[path] = append(m.methods[path], method)
}

// Build closes the route table and returns the handler that serves it. It must
// be called once, after every route is registered; registering afterwards
// leaves the new path without its 405 fallback.
func (m *ServeMux) Build() http.Handler {
	for path, methods := range m.methods {
		allow := allowHeader(methods)
		m.mux.Handle(path, m.methodNotAllowedOr200(allow))
	}

	// Least specific pattern there is, so it matches only what nothing above
	// claimed: a path this API does not serve at all — unless cleaning it up
	// lands on one that is, in which case the caller is redirected there.
	m.mux.Handle("/", m.redirectOrNotFound())

	return m.mux
}

// redirectOrNotFound answers a request nothing above matched by retrying
// path.Clean(r.URL.Path) — a trailing slash, or a "//"/"./"/"../" segment
// that survived to a genuine miss. If the cleaned path is one this API
// serves, the caller is redirected there (301 GET/HEAD, 308 otherwise, so
// method and body survive the hop) — httprouter's RedirectTrailingSlash and
// RedirectFixedPath. Anything still unresolved gets the JSON 404, as before.
func (m *ServeMux) redirectOrNotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := path.Clean(r.URL.Path)
		probe := &http.Request{Method: r.Method, URL: &url.URL{Path: cleaned}}
		if _, pattern := m.mux.Handler(probe); cleaned != r.URL.Path && pattern != "" && pattern != "/" {
			target := cleaned
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			status := http.StatusPermanentRedirect
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				status = http.StatusMovedPermanently
			}
			http.Redirect(w, r, target, status) //nolint:gosec // target is path.Clean(r.URL.Path)+query, never caller-supplied
			return
		}
		m.notFound.ServeHTTP(w, r)
	})
}

// methodNotAllowedOr200 answers a request that reached a known path by a method
// it does not serve.
//
// OPTIONS is the exception, and it is deliberate rather than incidental: a bare
// OPTIONS (not a CORS preflight — those are answered by the CORS middleware
// several layers out, and never reach the router) is a caller asking what this
// path accepts, and RFC 9110 §9.3.7 says to tell them. The previous router
// answered it the same way, with 200 and an empty body, and a client that
// discovers an API this way would otherwise start seeing 405s.
func (m *ServeMux) methodNotAllowedOr200(allow string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		m.methodNotAllowed.ServeHTTP(w, r)
	})
}

// allowHeader renders the Allow header for a path from the methods registered
// on it.
//
// HEAD rides along with GET because ServeMux serves a HEAD request from a GET
// pattern, and OPTIONS because this router answers it for every known path.
// The list is sorted so the same route always advertises the same string.
func allowHeader(methods []string) string {
	set := make(map[string]struct{}, len(methods)+2)
	for _, method := range methods {
		set[method] = struct{}{}
		if method == http.MethodGet {
			set[http.MethodHead] = struct{}{}
		}
	}
	set[http.MethodOptions] = struct{}{}

	allowed := make([]string, 0, len(set))
	for method := range set {
		allowed = append(allowed, method)
	}
	slices.Sort(allowed)

	return strings.Join(allowed, ", ")
}

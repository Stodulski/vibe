package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// SpecValidator checks incoming requests against internal/openapi/openapi.yaml
// before a handler sees them.
//
// The document was already the source of truth for what the API accepts, but
// nothing enforced it on the way in: kin-openapi appeared only in the response
// conformance tests (API-03), so a request the document forbids — a missing
// required field, a string where an integer is declared, an enum value that is
// not in the list — reached the handler and was refused, or not, by whatever
// the handler happened to check. Where the two disagreed the document was
// wrong and nobody found out.
//
// It runs in development and staging, never in production. The point is to
// fail a mismatch in front of the person who can fix it, not to put a second
// JSON parse in the path of every production request; production's version of
// this check is the conformance suite in cmd/api, which runs the same
// validation in CI at no runtime cost.
type SpecValidator struct {
	router  routers.Router
	respond *httpx.Responder
	logger  *slog.Logger
	// exempt are the routes whose bodies this must not touch. See
	// exemptRoutes.
	exempt map[string]struct{}
}

// exemptRoutes are the request bodies validation is not allowed to read or
// refuse.
//
// The webhooks are somebody else's payload. MercadoPago and Meta send what
// they send, their signature is computed over the exact bytes that arrived,
// and the document's description of those bodies is our best understanding of
// a third party's format rather than a contract they agreed to. Refusing one
// for not matching would drop a real payment notification over a field Meta
// added on a Tuesday, and reading the body to check it risks the signature
// check seeing something other than what was signed.
var exemptRoutes = []string{
	"POST /api/v1/webhooks/mercadopago",
	"POST /api/v1/webhooks/whatsapp",
	"GET /api/v1/webhooks/whatsapp",
}

// NewSpecValidator builds a validator over the parsed document.
//
// The document's servers are dropped from the router's copy: the routing table
// here matches on path alone, because this process answers on whatever host
// and port it was given and the two servers the document declares are
// documentation for a reader, not a deployment list.
func NewSpecValidator(doc *openapi3.T, respond *httpx.Responder, logger *slog.Logger) (*SpecValidator, error) {
	if doc == nil {
		return nil, errors.New("middleware: the OpenAPI document is required to validate requests against it")
	}

	// A shallow copy: Paths and everything under it stay shared and read-only,
	// only the servers list differs from the document the rest of the process
	// holds.
	pathOnly := *doc
	pathOnly.Servers = nil

	router, err := legacy.NewRouter(&pathOnly)
	if err != nil {
		return nil, fmt.Errorf("middleware: building a router from the OpenAPI document: %w", err)
	}

	exempt := make(map[string]struct{}, len(exemptRoutes))
	for _, route := range exemptRoutes {
		exempt[route] = struct{}{}
	}

	return &SpecValidator{router: router, respond: respond, logger: logger, exempt: exempt}, nil
}

// ValidateRequests refuses a request the document does not allow, with 400 and
// the validation message.
//
// A request the document has no route for passes through untouched. That is
// not a hole: cmd/api/openapi_sync_test.go already fails the build when a
// registered route is undocumented, so the only paths that reach this branch
// are the ones deliberately outside the document — /debug/pprof and
// /debug/vars, which exist in development only.
func (v *SpecValidator) ValidateRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, pathParams, err := v.router.FindRoute(r)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		if _, skip := v.exempt[r.Method+" "+route.Path]; skip {
			next.ServeHTTP(w, r)
			return
		}

		body, err := v.rewindableBody(w, r)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				v.respond.Refuse(w, r, httpx.TooLarge(
					fmt.Sprintf("body must not be larger than %d bytes", tooLarge.Limit)))
				return
			}
			v.respond.ServerError(w, r, fmt.Errorf("middleware: reading the request body to validate it: %w", err))
			return
		}

		input := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				// Authentication is this chain's own job, several layers out.
				// Without this the validator refuses every request carrying a
				// security requirement, because it has no idea how to check a
				// cookie it was never told about.
				AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error { return nil },
			},
		}

		if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
			v.logger.Warn("openapi: request does not match the document",
				"method", r.Method, "path", r.URL.Path, "error", err,
				"request_id", httpx.ContextGetRequestID(r))
			v.respond.Refuse(w, r, httpx.BadRequest(validationMessage(err)))
			return
		}

		// ValidateRequest consumed the body to check it; the handler still
		// needs it.
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// rewindableBody reads the body out and puts a fresh reader back, so that both
// the validator and the handler behind it see the whole thing. The read is
// capped at httpx.MaxJSONBody, the same limit ReadJSON enforces.
func (v *SpecValidator) rewindableBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, httpx.MaxJSONBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close() //nolint:errcheck // the bytes are already in hand; nothing is left to fail on
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// validationMessage is what the caller is told.
//
// kin-openapi's error carries the schema, the offending value and a stack of
// wrapped causes, and the whole thing describes our own document to somebody
// who cannot change it. The first line is the part that names what is wrong
// with their request; the rest goes to the log line above instead.
func validationMessage(err error) string {
	msg := err.Error()
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	return msg
}

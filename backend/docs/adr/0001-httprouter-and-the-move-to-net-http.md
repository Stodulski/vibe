# 0001. `net/http` `ServeMux` + oapi-codegen, replacing httprouter

## Status

Accepted and completed. The API routed through
[httprouter](https://github.com/julienschmidt/httprouter) until PR 2b (backend audit,
2026-09); httprouter has left `go.mod`.

## Context

The API used to be a single Go binary (`cmd/api`) routing plain `net/http` handlers through
httprouter for path parameters and method dispatch. The HTTP surface was also hand-written
against a committed OpenAPI 3.1 document (`internal/openapi/openapi.yaml`), kept honest only
by a test (`cmd/api/openapi_sync_test.go`) that failed when a route and its OpenAPI entry
drifted apart — a route could still be registered without a document entry, or documented
without ever being registered, for as long as nobody ran that test.

httprouter predates Go's standard-library router gaining method- and pattern-based routing
(`http.ServeMux` since Go 1.22): at the time this service was started, the standard library
had no equivalent, and httprouter was — and still is — a small, dependency-free, widely used
choice for exactly this shape of API. But every handler already depended only on
`http.ResponseWriter` / `*http.Request` plus httprouter's own parameter extraction, so nothing
about the design was httprouter-specific in a way that would be expensive to change once the
standard library caught up.

## Decision

Route through the standard library's `net/http` `ServeMux` (`internal/httpx/servemux.go`
wraps it for the JSON 404/405 the plain library answers in text), and stop hand-maintaining
the router registration and the request/response DTOs. `internal/openapi/openapi.yaml` is now
the single source of truth end to end: [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)
(pinned by the `tool` directive in `go.mod`, run through `make generate/api`) generates
`internal/openapi/gen` — the request/response models, `ServerInterface`, and
`HandlerWithOptions`/`StdHTTPServerOptions` in the plain (non-strict) shape, so handlers keep
`http.ResponseWriter`/`*http.Request` and the existing middleware, guards and `Responder`
survive unchanged. `cmd/api/apiserver.go` implements `ServerInterface` — one adapter method
per operation, composing the domain packages' own handlers — and registers it through
`HandlerWithOptions`, so the mux's patterns come from the document.

This collapses the old manual sync test from the only guarantee into a second one: a route
in the document with no implementation now fails to *compile* (`ServerInterface` gains a
method nothing satisfies), and a handler with no matching operation cannot be registered at
all. `cmd/api/openapi_sync_test.go` (`TestOpenAPISyncWithRouter`) still runs, as a second,
independent proof that the live route table and the document agree — belt and braces, not
a fallback for a guarantee the compiler doesn't fully cover on its own (a renamed path with a
renamed operation ID still compiles on both sides).

Handlers decode into and encode through the generated types (`gen.*`), mapping from
store/service types explicitly — never serializing a store or sqlc struct directly. Errors
answer as [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details
(`application/problem+json`) through one `Problem` component in the document, reused by
every 4xx/5xx response; see the README's "Errors" section.

## Consequences

- httprouter is gone from `go.mod`. `internal/httpx/servemux.go` is the whole router; its own
  tests (`servemux_test.go`) cover the 404/405/redirect behaviour httprouter used to own.
- `internal/openapi/gen` is generated and committed. It is never hand-edited; CI's lint job
  re-runs `make generate/api` and fails the build on a diff, so the generated package and the
  document cannot drift apart the way the hand-written route table and the document once
  could.
- A document change lands before the code that depends on it: edit
  `internal/openapi/openapi.yaml`, run `make generate/api`, then update the handler(s). Where
  a generated model's stricter decoding or encoding would change the wire, the document is
  fixed first rather than working around it silently — this migration dropped `format: email`
  (oapi-codegen's `openapi_types.Email` fails to marshal a stored value that isn't a valid
  address, and fails to unmarshal one at the wrong status code), added `format: double` to
  every coordinate (bare `type: number` narrows to `float32`), and documented `Complex`'s
  long-sent `version` field. A genuine remaining mismatch is a local type with a comment
  explaining why, not a workaround hidden in the handler.

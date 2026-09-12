# 0001. `net/http` + httprouter today, `net/http` `ServeMux` + oapi-codegen later

## Status

Accepted (current state). The move described under "Planned" is not scheduled or started —
nothing in this repository depends on it yet.

## Context

The API is a single Go binary (`cmd/api`) routing plain `net/http` handlers through
[httprouter](https://github.com/julienschmidt/httprouter) for path parameters and method
dispatch. The HTTP surface is also hand-written against a committed OpenAPI 3.1 document
(`internal/openapi/openapi.yaml`), kept honest only by a test
(`cmd/api/openapi_sync_test.go`) that fails when a route and its OpenAPI entry drift apart.

httprouter predates Go's standard-library router gaining method- and pattern-based routing
(`http.ServeMux` since Go 1.22): at the time this service was started, the standard library
had no equivalent, and httprouter was — and still is — a small, dependency-free, widely used
choice for exactly this shape of API.

## Decision

Keep httprouter for now. Every handler already depends only on `http.ResponseWriter` /
`*http.Request` plus httprouter's own parameter extraction, so nothing about the current
design is httprouter-specific in a way that would be expensive to change later.

## Planned direction

Move routing to the standard library's `net/http` `ServeMux`, and stop hand-maintaining
`internal/openapi/openapi.yaml` in favor of generating both the router registration and the
OpenAPI document (or generating handlers stubs from the document) with
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen). This removes the httprouter
dependency and collapses the manual sync test into a build-time guarantee: a route that isn't
in the document (or vice versa) stops compiling instead of failing a test.

This has not started. No code in this PR or any open branch depends on it — it is recorded
here so the direction is written down once, instead of living only in conversation.

## Consequences

- No change today: this ADR documents intent, not a migration in progress.
- When undertaken, the migration is mostly mechanical (route table + generated types), but
  touches every handler file under `cmd/api/`, so it should land as its own change, not mixed
  into unrelated work.

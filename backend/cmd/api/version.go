package main

// version is the build's reported version: surfaced by the health endpoints,
// published as an expvar, carried on every log line and used as the Sentry
// release when SENTRY_RELEASE is unset.
//
// It is a var, not a const, so the build can stamp the commit into it with
// -ldflags "-X main.version=<commit>". "dev" is what a `go build` with no
// flags — a local run, a test binary — reports, and saying so is more honest
// than a hardcoded version number that has not changed since it was typed.
var version = "dev"

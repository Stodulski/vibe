// Package datatest is the shared fixture every store package's integration
// tests hang their rows on: a pool against the E2E database, a credential
// keyring, and one complex's worth of real rows — owner, complex, court and
// client — with the helpers that create and re-read what a test cares about.
//
// It lives outside the test files themselves because the fixture is now shared
// across a dozen packages, and a test file can only be shared with the package
// it is in.
//
// Everything in it is behind the `integration` build tag, which is why this
// file — carrying only the package clause and this comment — is not: a package
// whose every file is excluded by a build constraint is a build error for
// `go build ./...`, and the fixture must not be in an ordinary build.
package datatest

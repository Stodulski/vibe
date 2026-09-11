// Command openapicheck loads and schema-validates the embedded OpenAPI
// document, exiting non-zero on failure.
//
// It exists for `make lint/openapi`: a check that runs in CI or as a
// pre-commit hook without standing up the mock-store test harness that
// internal/openapi's own tests use. Route-vs-document parity (every
// registered route documented, and vice versa) is not repeated here — that
// needs the running application's route table, and already lives in
// cmd/api/openapi_sync_test.go.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/stodulski/vibe-server/internal/httpx"
	"github.com/stodulski/vibe-server/internal/openapi"
)

func main() {
	respond := httpx.NewResponder(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if _, err := openapi.NewHandler(respond); err != nil {
		fmt.Fprintln(os.Stderr, "openapicheck:", err)
		os.Exit(1)
	}

	fmt.Println("openapicheck: internal/openapi/openapi.yaml loads and validates")
}

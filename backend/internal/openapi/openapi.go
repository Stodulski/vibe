// Package openapi serves this API's own OpenAPI 3.1 document: as JSON, as raw
// YAML, and as an interactive HTML reference built from it.
//
// internal/openapi/openapi.yaml is the single source of truth for the API
// surface. cmd/api/openapi_sync_test.go proves every registered route is
// documented there and every documented route is registered, so the document
// cannot drift silently the way a hand-maintained one does.
//
// The document is parsed and schema-validated once, at construction, rather
// than per request: NewHandler returns an error when it does not load or
// validate, so a broken document fails the boot with a clear message instead
// of serving nothing (or garbage) on the first request to reach it.
package openapi

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/stodulski/vibe-server/internal/httpx"
)

// specYAML is the committed source of truth. Embedding it here, rather than
// under a top-level openapi/ directory, keeps the embed directive and the
// file it names in the same package: go:embed cannot reach outside its own
// package directory.
//
//go:embed openapi.yaml
var specYAML []byte

// docsCacheControl applies to all three routes: the document changes only on
// deploy, so a browser or crawler may hold it for an hour.
const docsCacheControl = "public, max-age=3600"

// Handler serves the parsed OpenAPI document and the interactive reference
// built from it.
type Handler struct {
	respond  *httpx.Responder
	doc      *openapi3.T
	jsonBody []byte
}

// NewHandler hands out the embedded document, parsed and validated once per
// process (see load). The returned error is meant to abort application
// startup: see cmd/api/app.go.
func NewHandler(respond *httpx.Responder) (*Handler, error) {
	doc, rendered, err := load()
	if err != nil {
		return nil, err
	}
	return &Handler{respond: respond, doc: doc, jsonBody: rendered}, nil
}

// loaded is the embedded document, parsed, validated and rendered once per
// process. The bytes are fixed at build time, so every application built in
// the same process gets the same result: production builds one, but a test
// binary builds hundreds, and parsing plus schema-validating the document
// was more than a third of the CPU the cmd/api suite spent. The document is
// shared and must be treated as read-only by everyone who receives it.
var loaded struct {
	once sync.Once
	doc  *openapi3.T
	json []byte
	err  error
}

func load() (*openapi3.T, []byte, error) {
	loaded.once.Do(func() {
		doc, err := openapi3.NewLoader().LoadFromData(specYAML)
		if err != nil {
			loaded.err = fmt.Errorf("openapi: parsing the embedded document: %w", err)
			return
		}
		if err := doc.Validate(context.Background()); err != nil {
			loaded.err = fmt.Errorf("openapi: the embedded document failed validation: %w", err)
			return
		}
		rendered, err := doc.MarshalJSON()
		if err != nil {
			loaded.err = fmt.Errorf("openapi: rendering the document as JSON: %w", err)
			return
		}
		loaded.doc, loaded.json = doc, rendered
	})
	return loaded.doc, loaded.json, loaded.err
}

// Document returns the parsed document, for anything that needs to inspect
// it directly (the sync and conformance tests). It is shared by every
// handler in the process: read it, never modify it.
func (h *Handler) Document() *openapi3.T {
	return h.doc
}

// Routes registers the three documentation endpoints. All three are public,
// sit under the general rate-limit tier, and touch no tenant-scoped table.
func (h *Handler) Routes(router httpx.Router, _ httpx.Guards) {
	router.HandlerFunc(http.MethodGet, "/api/v1/openapi.json", h.JSON)
	router.HandlerFunc(http.MethodGet, "/api/v1/openapi.yaml", h.YAML)
	router.HandlerFunc(http.MethodGet, "/api/v1/docs", h.Docs)
}

// JSON handles GET /api/v1/openapi.json, the document rendered from the
// parsed, validated in-memory representation.
func (h *Handler) JSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", docsCacheControl)
	w.WriteHeader(http.StatusOK)
	// The response is committed; a write failure can no longer be reported.
	_, _ = w.Write(h.jsonBody)
}

// YAML handles GET /api/v1/openapi.yaml, the exact bytes committed to the
// repository.
func (h *Handler) YAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", docsCacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(specYAML)
}

// Docs handles GET /api/v1/docs: an HTML page that loads Scalar's API
// reference pointed at /api/v1/openapi.json. It carries no session and is
// marked non-indexable, since it is a developer tool rather than a page
// meant to rank.
func (h *Handler) Docs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", docsCacheControl)
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docsHTML)
}

// docsHTML is static: it never depends on request state, only on the fixed
// /api/v1/openapi.json URL it points Scalar at.
var docsHTML = []byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Vibe API Reference</title>
  <meta name="robots" content="noindex">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
  <script id="api-reference" data-url="/api/v1/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>
`)

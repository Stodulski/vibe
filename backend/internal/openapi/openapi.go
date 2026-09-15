// Package openapi serves this API's own OpenAPI 3.1 document: as JSON, as raw
// YAML, as an interactive HTML reference built from it, and as an RFC 9727
// API catalog (a linkset) derived from it.
//
// internal/openapi/openapi.yaml is the single source of truth for the API
// surface. cmd/api/openapi_sync_test.go proves every registered route is
// documented there and every documented route is registered, so the document
// cannot drift silently the way a hand-maintained one does.
//
// The document is parsed and schema-validated once, at construction, rather
// than per request: NewHandler returns an error when it does not load or
// validate, so a broken document fails the boot with a clear message instead
// of serving nothing (or garbage) on the first request to reach it. The
// catalog is built in that same step and fails the boot the same way if it
// would advertise a path the document does not serve — see buildCatalog.
package openapi

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	respond     *httpx.Responder
	doc         *openapi3.T
	jsonBody    []byte
	catalogBody []byte
}

// NewHandler hands out the embedded document, parsed and validated once per
// process (see load). The returned error is meant to abort application
// startup: see cmd/api/app.go.
func NewHandler(respond *httpx.Responder) (*Handler, error) {
	doc, rendered, catalog, err := load()
	if err != nil {
		return nil, err
	}
	return &Handler{respond: respond, doc: doc, jsonBody: rendered, catalogBody: catalog}, nil
}

// loaded is the embedded document, parsed, validated and rendered once per
// process. The bytes are fixed at build time, so every application built in
// the same process gets the same result: production builds one, but a test
// binary builds hundreds, and parsing plus schema-validating the document
// was more than a third of the CPU the cmd/api suite spent. The document is
// shared and must be treated as read-only by everyone who receives it.
var loaded struct {
	once    sync.Once
	doc     *openapi3.T
	json    []byte
	catalog []byte
	err     error
}

func load() (*openapi3.T, []byte, []byte, error) {
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
		catalog, err := buildCatalog(doc)
		if err != nil {
			loaded.err = err
			return
		}
		loaded.doc, loaded.json, loaded.catalog = doc, rendered, catalog
	})
	return loaded.doc, loaded.json, loaded.catalog, loaded.err
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

// Catalog handles GET /.well-known/api-catalog: an RFC 9727 API catalog (a
// linkset) that points an agent at this API's machine-readable description,
// interactive reference and health check, without requiring it to already
// know this API's URLs. The body is built once, by buildCatalog, from the
// same embedded document JSON and YAML are served from.
//
// Access-Control-Allow-Origin is set here rather than through the
// application's CORS policy (cmd/api/routes.go's corsOptions, which allows
// only the configured frontend origin): a discovery document is meant to be
// fetched from anywhere, the same way the landing site's copy of it is a
// static asset with no CORS restriction at all. rs/cors, further out in the
// middleware chain, only sets this header for an origin it recognizes and
// otherwise leaves it alone — see routes.go — so setting it unconditionally
// here is safe.
func (h *Handler) Catalog(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", linksetContentType)
	w.Header().Set("Cache-Control", docsCacheControl)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.catalogBody)
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

// linksetContentType is RFC 9727's media type for an API catalog: a linkset
// document, not a bare "application/json" object.
const linksetContentType = "application/linkset+json"

// catalogSiteURL is the marketing/landing origin the catalog credits as its
// author and points at for llms.txt. It is not part of this API and has no
// entry in the document's servers list, so it is the one URL here that
// cannot be derived from openapi.yaml — same as the landing's own copy of
// this catalog (landing/scripts/build-api-catalog.mjs's SITE constant).
const catalogSiteURL = "https://vibe.com.ar"

// catalogLink is one entry this API's catalog advertises: a path that MUST
// be a GET operation in the embedded document. buildCatalog fails startup
// naming any path here that is not, instead of shipping a catalog that
// points an agent at a 404 — the same discipline
// landing/scripts/build-api-catalog.mjs applies when it derives the
// landing's copy of this same document.
type catalogLink struct {
	rel   string
	path  string
	mtype string
	title string
}

// catalogAdvertised must stay in step with the landing's ADVERTISED list in
// build-api-catalog.mjs: both are derived from this document, so drift
// between them would mean the two published catalogs disagree about what
// this API offers.
var catalogAdvertised = []catalogLink{
	{rel: "service-desc", path: "/api/v1/openapi.json", mtype: "application/openapi+json", title: "OpenAPI 3.1 description (JSON)"},
	{rel: "service-desc", path: "/api/v1/openapi.yaml", mtype: "application/openapi+yaml", title: "OpenAPI 3.1 description (YAML)"},
	{rel: "service-doc", path: "/api/v1/docs", mtype: "text/html", title: "Interactive reference"},
	{rel: "status", path: "/api/v1/healthcheck", mtype: "application/json", title: "Health check"},
}

// catalogDocument is the RFC 9727 top-level shape: a "linkset" array holding
// exactly one entry, this API's own.
type catalogDocument struct {
	Linkset []catalogEntry `json:"linkset"`
}

// catalogEntry is one linkset entry. Field order matches the landing's
// committed copy (landing/public/.well-known/api-catalog) so the two
// documents are easy to diff by eye.
type catalogEntry struct {
	Anchor      string       `json:"anchor"`
	ServiceDesc []catalogRef `json:"service-desc,omitempty"`
	ServiceDoc  []catalogRef `json:"service-doc,omitempty"`
	Status      []catalogRef `json:"status,omitempty"`
	Author      []catalogRef `json:"author,omitempty"`
	DescribedBy []catalogRef `json:"describedby,omitempty"`
}

// catalogRef is one target link.
type catalogRef struct {
	Href  string `json:"href"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

// buildCatalog derives the RFC 9727 catalog body from doc: the base URL
// comes from its first https servers entry, and every advertised link is
// checked against its paths before anything is rendered. Returning early on
// either failure is deliberate — see load, which treats this the same as a
// document that does not parse or does not validate: a broken catalog aborts
// the boot instead of shipping quietly.
func buildCatalog(doc *openapi3.T) ([]byte, error) {
	base, err := catalogBaseURL(doc)
	if err != nil {
		return nil, err
	}
	if err := catalogCheckAdvertisedPaths(doc); err != nil {
		return nil, err
	}

	name := "Vibe API"
	if doc.Info != nil && doc.Info.Title != "" {
		name = doc.Info.Title
	}

	entry := catalogEntry{Anchor: base + "/api/v1"}
	for _, link := range catalogAdvertised {
		ref := catalogRef{Href: base + link.path, Type: link.mtype, Title: name + " - " + link.title}
		switch link.rel {
		case "service-desc":
			entry.ServiceDesc = append(entry.ServiceDesc, ref)
		case "service-doc":
			entry.ServiceDoc = append(entry.ServiceDoc, ref)
		case "status":
			entry.Status = append(entry.Status, ref)
		default:
			// Unreachable outside a typo in catalogAdvertised above: every
			// entry there uses one of the three rels handled above.
			return nil, fmt.Errorf("openapi: catalog link for %q has unknown rel %q", link.path, link.rel)
		}
	}
	entry.Author = []catalogRef{{Href: catalogSiteURL, Title: "Vibe"}}
	entry.DescribedBy = []catalogRef{{Href: catalogSiteURL + "/llms.txt", Type: "text/plain", Title: "Vibe - plain-text overview for agents"}}

	body, err := json.Marshal(catalogDocument{Linkset: []catalogEntry{entry}})
	if err != nil {
		return nil, fmt.Errorf("openapi: rendering the API catalog as JSON: %w", err)
	}
	return body, nil
}

// catalogBaseURL is the first https entry in the document's servers list.
// The document also lists a localhost entry for local development, and a
// catalog derived from that would point every agent at a base URL nobody
// outside this machine can reach.
func catalogBaseURL(doc *openapi3.T) (string, error) {
	for _, s := range doc.Servers {
		if s != nil && strings.HasPrefix(s.URL, "https://") {
			return s.URL, nil
		}
	}
	return "", fmt.Errorf("openapi: no https server in the document's servers list — the API catalog has no base URL to advertise")
}

// catalogCheckAdvertisedPaths proves every path in catalogAdvertised is a GET
// operation in doc, so the catalog can never advertise a route this API does
// not serve.
func catalogCheckAdvertisedPaths(doc *openapi3.T) error {
	var missing []string
	for _, link := range catalogAdvertised {
		item := doc.Paths.Find(link.path)
		if item == nil || item.Get == nil {
			missing = append(missing, link.path)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"openapi: %s advertised by the API catalog but not a GET operation in the document — "+
				"renamed or removed? update catalogAdvertised in internal/openapi/openapi.go, or the "+
				"catalog ships pointing at a 404",
			strings.Join(missing, ", "))
	}
	return nil
}

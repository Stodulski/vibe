package publicsite

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/httpx"
)

// prerenderCacheHeader lets a CDN serve the page for five minutes and keep
// serving a stale copy for another minute while it refreshes.
const prerenderCacheHeader = "public, max-age=300, stale-while-revalidate=60"

// prerenderRetryAfterSeconds tells a crawler, in RFC 9110 §10.2.3's
// whole-seconds form, when to retry a venue this request could not read.
const prerenderRetryAfterSeconds = "60"

// prerenderResultHeader records what this endpoint decided (ok, degraded,
// venue-not-found). Crawlers now reach this endpoint through a vercel.json
// rewrite rather than an edge middleware that inspected the answer, so the
// status code itself is what a crawler acts on; the header stays for logs and
// for anyone debugging a crawl with curl.
const prerenderResultHeader = "X-Prerender-Result"

// Prerender handles GET /api/v1/public/prerender/{slug}.
//
// It fetches the frontend's index.html and substitutes the complex's own title,
// description, image and structured data, so a crawler that runs no JavaScript
// sees a fully described page rather than an empty app shell.
func (h *Handler) Prerender(w http.ResponseWriter, r *http.Request) {
	slug := httpx.ReadStringParam(r, "slug")
	if slug == "" {
		h.respond.NotFound(w, r)
		return
	}

	rendered, err := h.svc.Prerender(r.Context(), slug)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			w.Header().Set(prerenderResultHeader, "venue-not-found")
			h.respond.NotFound(w, r)
		case errors.Is(err, ErrComplexUnavailable):
			// A crawler retries a transient 5xx; a 200 here would get the old
			// generic shell indexed in this venue's own place instead.
			h.respond.LogError(r, err)
			w.Header().Set("Retry-After", prerenderRetryAfterSeconds)
			h.respond.Refuse(w, r, httpx.Unavailable(nil))
		default:
			// Unreachable today: Service.Prerender only returns the two errors
			// above. It is a 503 rather than a 500 so that a future error path
			// cannot hand a crawler the one status it reads as "this URL is
			// broken" — there is no longer a middleware in front to translate it.
			h.respond.LogError(r, err)
			w.Header().Set("Retry-After", prerenderRetryAfterSeconds)
			h.respond.Refuse(w, r, httpx.Unavailable(nil))
		}
		return
	}

	// A degraded page is still a page, and for this endpoint's readers that is
	// the difference that matters: a crawler reads a 500 as "this URL is
	// broken" and drops the entry, where a shell is merely a page it will see
	// again on its next pass. The failure is logged through the Responder, so
	// the line carries the request id the client was handed.
	result := "ok"
	if rendered.Degraded != nil {
		result = "degraded"
		h.respond.LogError(r, fmt.Errorf("prerender %q degraded to a plainer page: %w", slug, rendered.Degraded))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", prerenderCacheHeader)
	w.Header().Set(prerenderResultHeader, result)
	w.WriteHeader(http.StatusOK)
	// The response is committed; a write failure can no longer be reported.
	//
	//nolint:gosec // G705: the page is the frontend's own index.html with this
	// complex's meta tags substituted in, and every owner-supplied value in
	// them is HTML-escaped where it is interpolated (Service.render below).
	_, _ = w.Write([]byte(rendered.Page))
}

// render substitutes the frontend's default meta tags with this complex's, and
// injects the canonical URL and JSON-LD before </head>.
//
// Every interpolated value is HTML-escaped: complex names, descriptions and
// logo URLs are owner-supplied, and they are being written into markup.
func (s *Service) render(tmpl string, complex *complexstore.Complex, schedules []*complexstore.Schedule, slug string) string {
	baseURL := strings.TrimRight(s.frontendURL, "/")
	canonicalURL := baseURL + "/" + slug

	escapedName := html.EscapeString(complex.Name)
	title := pageTitle(escapedName)
	description := pageDescription(escapedName)

	imageURL := defaultImage
	if complex.LogoURL != nil && *complex.LogoURL != "" {
		imageURL = *complex.LogoURL
	}

	page := tmpl
	for _, sub := range []struct{ placeholder, replacement string }{
		{placeholderTitleTag, fmt.Sprintf(`<title>%s | Vibe</title>`, title)},
		{placeholderDescription, fmt.Sprintf(`content="%s"`, description)},
		{placeholderOGTitle, fmt.Sprintf(`content="%s"`, title)},
		{placeholderOGDesc, fmt.Sprintf(`content="%s"`, description)},
		{placeholderImage, fmt.Sprintf(`content="%s"`, html.EscapeString(imageURL))},
	} {
		page = strings.ReplaceAll(page, sub.placeholder, sub.replacement)
	}

	escapedCanonical := html.EscapeString(canonicalURL)
	extraHead := fmt.Sprintf(`<link rel="canonical" href="%s" />`+"\n", escapedCanonical) +
		fmt.Sprintf(`    <meta property="og:url" content="%s" />`+"\n", escapedCanonical) +
		fmt.Sprintf("    <script type=\"application/ld+json\">%s</script>\n    ", structuredData(complex, schedules, canonicalURL))

	return strings.Replace(page, "</head>", extraHead+"</head>", 1)
}

// fallbackTemplate is the last resort: the frontend's index.html has never been
// fetched successfully in this process and there is no stale copy to reuse.
//
// It carries the same placeholders the real one does, so a complex's own tags
// still substitute into it, and it redirects a human to the real app. It is
// deliberately tiny — it exists so that a crawler is answered with a page
// rather than a 500, not so that anyone reads it.
const fallbackTemplate = `<!doctype html><html lang="es"><head><meta charset="utf-8" />` +
	`<title>Vibe</title>` +
	`<meta name="description" content="Vibe - Gestión de complejos deportivos, reservas y canchas" />` +
	`<meta property="og:type" content="website" />` +
	`<meta property="og:title" content="Vibe - Reserva tu cancha" />` +
	`<meta property="og:description" content="Reserva canchas de pádel, tenis y fútbol de forma rápida y segura." />` +
	`<meta property="og:image" content="https://app.vibe.com.ar/logo.png" />` +
	`</head><body></body></html>`

// schemaDays maps the stored day names to the capitalised forms schema.org
// requires.
var schemaDays = map[string]string{
	"monday": "Monday", "tuesday": "Tuesday", "wednesday": "Wednesday",
	"thursday": "Thursday", "friday": "Friday", "saturday": "Saturday", "sunday": "Sunday",
}

// structuredData builds the schema.org SportsActivityLocation document that
// lets search engines show the complex's address and opening hours directly in
// results.
func structuredData(complex *complexstore.Complex, schedules []*complexstore.Schedule, url string) string {
	schema := map[string]any{
		"@context":  "https://schema.org",
		"@type":     "SportsActivityLocation",
		"name":      complex.Name,
		"url":       url,
		"telephone": complex.Phone,
		"address": map[string]any{
			"@type":           "PostalAddress",
			"streetAddress":   complex.Address,
			"addressLocality": complex.City,
			"addressRegion":   complex.Province,
			"addressCountry":  complex.CountryCode,
		},
	}

	// Optional fields are omitted rather than emitted empty, because an empty
	// value in structured data is treated as an error by validators.
	if complex.LogoURL != nil {
		schema["image"] = *complex.LogoURL
	}
	if complex.Email != nil {
		schema["email"] = *complex.Email
	}
	if complex.Latitude != nil && complex.Longitude != nil {
		schema["geo"] = map[string]any{
			"@type":     "GeoCoordinates",
			"latitude":  *complex.Latitude,
			"longitude": *complex.Longitude,
		}
	}

	var hours []map[string]any
	for _, s := range schedules {
		if s.IsClosed {
			continue
		}
		hours = append(hours, map[string]any{
			"@type":     "OpeningHoursSpecification",
			"dayOfWeek": schemaDays[s.Day],
			"opens":     s.OpenTime,
			"closes":    s.CloseTime,
		})
	}
	if len(hours) > 0 {
		schema["openingHoursSpecification"] = hours
	}

	// The schema is built from values that always marshal, so an error here
	// would be a programming fault rather than a runtime condition.
	b, _ := json.Marshal(schema) //nolint:errcheck // see above: an error here would be a programming fault, not a runtime condition
	return string(b)
}

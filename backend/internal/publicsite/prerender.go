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

// Prerender handles GET /api/v1/public/prerender/:slug.
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

	page, err := h.svc.Prerender(r.Context(), slug)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			h.respond.NotFound(w, r)
		} else {
			h.respond.ServerError(w, r, err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", prerenderCacheHeader)
	w.WriteHeader(http.StatusOK)
	// The response is committed; a write failure can no longer be reported.
	//
	//nolint:gosec // G705: the page is the frontend's own index.html with this
	// complex's meta tags substituted in, and every owner-supplied value in
	// them is HTML-escaped where it is interpolated (Service.render below).
	_, _ = w.Write([]byte(page))
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
	b, _ := json.Marshal(schema)
	return string(b)
}

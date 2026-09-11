package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
)

// This file is the last thing that runs before an event leaves the building.
//
// Sentry is a third party. Everything the process panics with, every provider
// error string it captures and every header on the request that was in flight
// is shipped there verbatim unless something stops it — and this deployment
// handles named clients, their phone numbers, their session cookies and a
// payment provider whose own error type carries the raw HTTP response body it
// got back. `mp: API error 400: {...}` is one string containing whatever
// MercadoPago chose to say, which on an authorization failure includes the
// request it was unhappy about.
//
// So: an allowlist for headers, a redaction pass over every free-text field,
// and a length cap on the fields a remote service gets to choose the size of.

// redaction is the marker left where something was removed. It is deliberately
// conspicuous: a reader has to be able to tell "there was a secret here" from
// "this field was empty".
const redaction = "[redacted]"

// maxCapturedValue bounds a single free-text field. A provider that answers an
// error with a kilobyte of JSON should not get to decide how much of our
// quota, and of a reviewer's attention, that error consumes.
const maxCapturedValue = 1024

// allowedHeaders are the request headers worth keeping on an event. Everything
// else is dropped rather than redacted, because the interesting ones are few
// and an allowlist cannot be outgrown by a header somebody adds later.
//
// Cookie and Authorization are the obvious omissions — they carry the session
// itself. X-CSRF-Token is omitted for the same reason: it is derived from the
// access token.
var allowedHeaders = map[string]bool{
	"Accept":           true,
	"Accept-Encoding":  true,
	"Accept-Language":  true,
	"Content-Length":   true,
	"Content-Type":     true,
	"Origin":           true,
	"Referer":          true,
	"User-Agent":       true,
	"X-Request-Id":     true,
	"X-Forwarded-For":  true,
	"X-Forwarded-Host": true,
}

// scrubbers are applied in order to every free-text field. Order matters: the
// provider-body rule runs first because it swallows the rest of the string,
// and a JWT rule that ran first would leave the surrounding body behind.
var scrubbers = []struct {
	name    string
	pattern *regexp.Regexp
	replace string
}{
	// internal/mp's APIError formats as "mp: API error <code>: <body>", where
	// the body is whatever MercadoPago returned. It is the single largest
	// unreviewed thing this process can send anywhere.
	{
		name:    "provider response body",
		pattern: regexp.MustCompile(`(?s)(mp: API error \d+: ).*`),
		replace: "${1}[redacted provider response]",
	},
	// A JWT is the session. Ours are the access and refresh tokens; the shape
	// is generic enough to catch anyone else's too.
	{
		name:    "jwt",
		pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}\.[A-Za-z0-9_-]{4,}`),
		replace: redaction,
	},
	// MercadoPago access and refresh tokens, per-seller. One of these is a
	// tenant's whole payment account.
	{
		name:    "mercadopago credential",
		pattern: regexp.MustCompile(`\b(APP_USR|TEST|TG)-[A-Za-z0-9_-]{6,}`),
		replace: redaction,
	},
	{
		name:    "bearer credential",
		pattern: regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`),
		replace: "${1} " + redaction,
	},
	// key=value and "key":"value" for anything that names itself a secret. The
	// bare "token" alternative comes last: Go alternation is leftmost-first, and
	// the \b before it cannot match inside "access_token" (an underscore is a
	// word character), so the longer names still win where both could apply.
	{
		name: "named secret",
		pattern: regexp.MustCompile(
			`(?i)("?\b(?:access_token|refresh_token|client_secret|api_key|apikey|webhook_secret|` +
				`jwt_secret|password|passwd|secret|authorization|csrf[_-]?token|token)\b"?\s*[:=]\s*"?)[^\s",;&}]+`),
		replace: "${1}" + redaction,
	},
	// A client's email address is the account identifier here, and the thing a
	// GDPR-style deletion request is about.
	{
		name:    "email address",
		pattern: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
		replace: redaction,
	},
	// E.164-ish phone numbers. Clients are identified by phone on the public
	// booking flow, so these appear in booking errors.
	{
		name:    "phone number",
		pattern: regexp.MustCompile(`\+\d[\d\s().-]{7,}\d`),
		replace: redaction,
	},
	// A bare run of 13 to 19 digits is a card number often enough that no
	// legitimate identifier here is worth the risk: this schema uses UUIDs.
	{
		name:    "card-length digit run",
		pattern: regexp.MustCompile(`\b\d{13,19}\b`),
		replace: redaction,
	},
}

// scrubText applies every rule and bounds the result.
func scrubText(s string) string {
	if s == "" {
		return s
	}
	for _, rule := range scrubbers {
		s = rule.pattern.ReplaceAllString(s, rule.replace)
	}
	if len(s) > maxCapturedValue {
		s = s[:maxCapturedValue] + "…[truncated " + strconv.Itoa(len(s)-maxCapturedValue) + " bytes]"
	}
	return s
}

// scrubEvent is the BeforeSend hook. It returns the event to send, and never
// nil: dropping events silently would trade one invisible failure for another.
func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}

	event.Message = scrubText(event.Message)
	event.Transaction = scrubText(event.Transaction)

	for i := range event.Exception {
		event.Exception[i].Value = scrubText(event.Exception[i].Value)
		// Type is a Go type name for a real error and the panic value's type
		// for a panic; neither carries payload, but a panic(string) puts the
		// string in Value, which the line above covers.
	}

	// The user is kept as an opaque id. The email and username identify a
	// person to a third party for no diagnostic gain — the id already joins to
	// our own records, which is where a support request starts anyway.
	event.User.Email = ""
	event.User.Username = ""
	event.User.Name = ""
	event.User.Data = nil

	for key, value := range event.Tags {
		event.Tags[key] = scrubText(value)
	}

	scrubRequest(event.Request)

	for _, crumb := range event.Breadcrumbs {
		scrubCrumb(crumb)
	}

	return event
}

// scrubRequest strips the credential-bearing parts of the captured request.
func scrubRequest(request *sentry.Request) {
	if request == nil {
		return
	}

	// The body of a request is a login payload as often as it is anything
	// else, and nothing in it is needed to locate a bug that already has a
	// stack trace and a request id.
	request.Data = ""
	request.Cookies = ""
	request.QueryString = scrubText(request.QueryString)
	request.URL = scrubText(request.URL)
	request.Env = nil

	if request.Headers == nil {
		return
	}
	for name, value := range request.Headers {
		if !allowedHeaders[canonicalHeader(name)] {
			delete(request.Headers, name)
			continue
		}
		request.Headers[name] = scrubText(value)
	}
}

// canonicalHeader normalises a header name to the Http-Header-Case the
// allowlist is written in, so "x-request-id" and "X-Request-ID" match the same
// entry.
func canonicalHeader(name string) string {
	parts := strings.Split(strings.ToLower(name), "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "-")
}

// scrubBreadcrumb is the BeforeBreadcrumb hook: breadcrumbs are attached to
// whatever event comes next, so they leave the building too.
func scrubBreadcrumb(crumb *sentry.Breadcrumb, _ *sentry.BreadcrumbHint) *sentry.Breadcrumb {
	scrubCrumb(crumb)
	return crumb
}

func scrubCrumb(crumb *sentry.Breadcrumb) {
	if crumb == nil {
		return
	}
	crumb.Message = scrubText(crumb.Message)
	for key, value := range crumb.Data {
		if s, ok := value.(string); ok {
			crumb.Data[key] = scrubText(s)
		}
	}
}

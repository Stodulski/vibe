package httpx

import (
	"net/http"
	"sort"
)

// problemBaseURI is the stable namespace every Problem's type URI is built
// under. It never resolves to a served document; RFC 9457 only requires the
// URI to be a stable identifier the client can switch on.
const problemBaseURI = "https://vibe.com.ar/problems/"

// Kind is the stable, machine-readable category a refused request's Problem
// belongs to. It is what `type` is built from (problemBaseURI + Kind), and it
// is what the frontend is expected to switch on rather than status or detail.
//
// A kind is not always the same as a status: RouteNotFound and NotFound both
// answer 404, but a caller who mistyped the path and a caller who asked for a
// booking that was deleted are different problems, and the mux miss carries
// no domain detail worth explaining the same way a domain 404 does.
type Kind string

// The kinds every Refusal constructor and Responder helper answers as. Their
// string values are the exact path segment appended to problemBaseURI.
const (
	// KindBadRequest is the generic 400: a request the caller can fix, whose
	// cause is not itself malformed JSON. Responder.BadRequest and the
	// httpx.BadRequest refusal both answer as this kind; KindInvalidJSON is
	// reserved for the one cause with its own dedicated refusal, ReadJSON's
	// body-decode failures.
	KindBadRequest   Kind = "bad-request"
	KindInvalidJSON  Kind = "invalid-json"
	KindValidation   Kind = "validation"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindNotFound     Kind = "not-found"
	KindConflict     Kind = "conflict"
	// KindDuplicateBooking and KindSlotUnavailable are 409s distinct from the
	// generic KindConflict: bookingstore.ErrDuplicateBooking and
	// ErrSlotUnavailable/ErrSlotLocked each get a kind the frontend can switch
	// on without parsing Detail.
	KindDuplicateBooking Kind = "duplicate-booking"
	KindSlotUnavailable  Kind = "slot-unavailable"
	KindGone             Kind = "gone"
	KindTooLarge         Kind = "too-large"
	KindRateLimited      Kind = "rate-limited"
	KindUnavailable      Kind = "unavailable"
	KindInternal         Kind = "internal"
	KindMethodNotAllowed Kind = "method-not-allowed"
	KindRouteNotFound    Kind = "route-not-found"
)

// URI returns the stable type URI for k.
func (k Kind) URI() string { return problemBaseURI + string(k) }

// FieldError is one field's validation failure, reported under a Problem's
// errors array.
type FieldError struct {
	// Field is dotted or bracketed for nested fields, e.g.
	// "schedules[3].open_time".
	Field string `json:"field"`
	// Message is free text, or one of the stable machine codes in codes.go,
	// intended for frontend-localized copy.
	Message string `json:"message"`
}

// Problem is this API's RFC 9457 (https://www.rfc-editor.org/rfc/rfc9457)
// error body. Every 4xx/5xx response is one of these, served as
// application/problem+json.
type Problem struct {
	// Type is a stable URI identifying this problem's kind, e.g.
	// "https://vibe.com.ar/problems/validation". This is the field the
	// frontend switches on.
	Type string `json:"type"`
	// Title is a short summary of this problem's type. It does not vary
	// between occurrences of the same kind.
	Title string `json:"title"`
	// Status repeats the HTTP status code, per RFC 9457 §3.1.
	Status int `json:"status"`
	// Detail is human-readable and specific to this occurrence, safe to
	// display. It is today's error message.
	Detail string `json:"detail,omitempty"`
	// Instance is the request path that produced this problem.
	Instance string `json:"instance,omitempty"`
	// RequestID correlates this response with the server's logs.
	RequestID string `json:"request_id,omitempty"`
	// Errors is present on a validation problem: one entry per invalid
	// field.
	Errors []FieldError `json:"errors,omitempty"`
	// Error mirrors Detail/Errors under the pre-RFC-9457 key an
	// already-deployed frontend still reads: the detail string for most
	// problems, or a field->message object for a validation problem.
	// legacy: remove once the frontend's ApiError is deployed everywhere
	Error any `json:"error,omitempty"`
}

// titles gives every kind a short, stable summary. It intentionally mirrors
// http.StatusText for most kinds; the kinds that share a status with another
// (RouteNotFound alongside NotFound, InvalidJSON/DuplicateBooking/
// SlotUnavailable alongside BadRequest/Conflict) get a title of their own so
// a reader can tell them apart without decoding the type URI.
var titles = map[Kind]string{
	KindBadRequest:       "Bad Request",
	KindInvalidJSON:      "Malformed JSON",
	KindValidation:       "Validation Failed",
	KindUnauthorized:     "Unauthorized",
	KindForbidden:        "Forbidden",
	KindNotFound:         "Not Found",
	KindConflict:         "Conflict",
	KindDuplicateBooking: "Duplicate Booking",
	KindSlotUnavailable:  "Slot Unavailable",
	KindGone:             "Gone",
	KindTooLarge:         "Payload Too Large",
	KindRateLimited:      "Too Many Requests",
	KindUnavailable:      "Service Unavailable",
	KindInternal:         "Internal Server Error",
	KindMethodNotAllowed: "Method Not Allowed",
	KindRouteNotFound:    "Route Not Found",
}

// writeProblem builds and writes a Problem for status/kind/detail, with the
// given field errors when this is a validation problem. It is the one place
// that assembles a Problem body, so every error response — however it was
// reached — carries the same instance/request_id/title machinery.
func (rs *Responder) writeProblem(w http.ResponseWriter, r *http.Request, status int, kind Kind, detail string, fieldErrors []FieldError) {
	problem := Problem{
		Type:      kind.URI(),
		Title:     titles[kind],
		Status:    status,
		Detail:    detail,
		Instance:  r.URL.Path,
		RequestID: ContextGetRequestID(r),
		Errors:    fieldErrors,
		// legacy: remove once the frontend's ApiError is deployed everywhere
		Error: legacyError(detail, fieldErrors),
	}

	w.Header().Set("Cache-Control", "no-store")
	if err := WriteProblemJSON(w, status, problem); err != nil {
		rs.LogError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// legacyError renders a Problem's pre-RFC-9457 "error" key: the per-field
// object a validation problem answered with, or the detail string every
// other problem answered with, before this API moved to RFC 9457 bodies.
// legacy: remove once the frontend's ApiError is deployed everywhere
func legacyError(detail string, fieldErrors []FieldError) any {
	if len(fieldErrors) == 0 {
		return detail
	}
	out := make(map[string]string, len(fieldErrors))
	for _, fe := range fieldErrors {
		out[fe.Field] = fe.Message
	}
	return out
}

// sortedFieldErrors turns a validator's field->message map into a
// deterministically ordered slice, so the same set of failures always
// serializes in the same order. encoding/json already sorts a map's keys
// when it marshals one directly; this is that same guarantee carried over
// now that validation errors travel as an array instead of an object.
func sortedFieldErrors(errors map[string]string) []FieldError {
	if len(errors) == 0 {
		return nil
	}
	fields := make([]string, 0, len(errors))
	for field := range errors {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	out := make([]FieldError, len(fields))
	for i, field := range fields {
		out[i] = FieldError{Field: field, Message: errors[field]}
	}
	return out
}

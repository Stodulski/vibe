// Package httpx holds the HTTP transport kernel shared by every domain module:
// JSON encoding and decoding, request parameter reading, request-scoped context
// values, and the typed error responses. It knows about HTTP and about the
// domain types it carries in context, and nothing else — no business rules, no
// stores, no external services.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

// MaxJSONBody caps decoded request bodies at 1 MiB. A guard reading the body
// ahead of ReadJSON caps its own read at this same limit.
const MaxJSONBody = 1_048_576

// BodyTooLargeError marks a ReadJSON failure caused by an oversized request
// body. The Responder checks for it so this one case answers 413 Request
// Entity Too Large instead of the generic 400 every other decoding failure
// gets, while keeping the same JSON error envelope and message.
type BodyTooLargeError struct {
	msg string
}

func (e *BodyTooLargeError) Error() string { return e.msg }

// InvalidJSONError marks a ReadJSON failure caused by the body not being
// decodable JSON — bad syntax, the wrong type for a field, an unknown key,
// more than one value, or an empty body. The Responder checks for it so this
// is the one BadRequest cause that keeps the dedicated invalid-json Problem
// kind; every other 400 answers the generic bad-request kind.
type InvalidJSONError struct {
	err error
}

func (e *InvalidJSONError) Error() string { return e.err.Error() }
func (e *InvalidJSONError) Unwrap() error { return e.err }

// UnsupportedMediaTypeError marks a ReadJSON failure caused by the request's
// Content-Type not declaring application/json — a missing header, or a
// different media type such as text/plain or
// application/x-www-form-urlencoded.
//
// It exists because a JSON route exempted from CSRF
// (internal/middleware/chain.go's csrfExemptRoutes — an auth route that mints
// or spends a session cookie before there is a token to derive) has no other
// defence against a cross-site request: a plain HTML <form
// method=post enctype="text/plain"> submission carries SameSite=Lax cookies
// on a top-level navigation, and the browser stores whatever Set-Cookie the
// response answers with. That form cannot set an arbitrary Content-Type, so
// requiring the exact media type closes the gap without adding a token check
// to routes that mint the very cookie a token would be derived from. The
// Responder recognises this error the same way it recognises
// InvalidJSONError, so every existing ReadJSON call site needs no change.
type UnsupportedMediaTypeError struct {
	msg string
}

func (e *UnsupportedMediaTypeError) Error() string { return e.msg }

// checkJSONContentType requires the request to declare "application/json" as
// its media type; a "; charset=utf-8" (or any other) parameter is accepted,
// since it does not change what bytes the body holds. A missing or
// mismatched Content-Type is refused before the body is ever read.
func checkJSONContentType(r *http.Request) error {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return &UnsupportedMediaTypeError{"Content-Type header must be application/json"}
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return &UnsupportedMediaTypeError{
			fmt.Sprintf("Content-Type %q is not supported; must be application/json", contentType),
		}
	}

	return nil
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// Envelope is the wrapper every JSON response body is written in, so that
// responses are always a keyed object rather than a bare value or array.
type Envelope map[string]any

// WriteJSON encodes data as JSON and writes it with the given status and
// headers. Encoding happens into a pooled buffer first, so an encoding failure
// returns an error before any byte reaches the client.
func WriteJSON(w http.ResponseWriter, status int, data Envelope, headers http.Header) error {
	buf, _ := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	enc := json.NewEncoder(buf)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("httpx: encode json body: %w", err)
	}

	maps.Copy(w.Header(), headers)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// WriteProblemJSON encodes problem as application/problem+json (RFC 9457) and
// writes it with the given status. It exists apart from WriteJSON because a
// Problem is a struct with a fixed shape, not a caller-assembled Envelope,
// and because its content type is never "application/json".
func WriteProblemJSON(w http.ResponseWriter, status int, problem Problem) error {
	buf, _ := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	enc := json.NewEncoder(buf)
	if err := enc.Encode(problem); err != nil {
		return fmt.Errorf("httpx: encode problem json: %w", err)
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// ReadJSON decodes a single JSON value from the request body into dst. The
// request must declare Content-Type: application/json (a charset parameter
// is fine; anything else, or no header at all, is refused as
// UnsupportedMediaTypeError before the body is touched). Unknown fields are
// rejected, the body is capped at MaxJSONBody, and every decoding failure is
// translated into a message safe to return to the client.
//
// Once the body decodes, every string in dst is trimmed of surrounding
// whitespace (see trimStrings), so a handler validating "required" sees "   "
// as the empty string it means, and no stored text carries a stray space.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := checkJSONContentType(r); err != nil {
		return err
	}
	if err := decodeJSON(w, r, dst); err != nil {
		return err
	}
	trimStrings(reflect.ValueOf(dst))
	return nil
}

// trimStrings strips leading and trailing whitespace from every string a
// decoded request carries: string and *string fields, slices and arrays,
// map values, and whatever nested structs or pointers hold them. A field
// tagged `trim:"-"` (a password, which may legitimately start or end with a
// space) and unexported fields are left as they were. Non-string values are
// never changed, and nil pointers are skipped.
func trimStrings(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			trimStrings(v.Elem())
		}

	case reflect.Struct:
		t := v.Type()
		for i := range v.NumField() {
			field := t.Field(i)
			if (!field.IsExported() && !field.Anonymous) || field.Tag.Get("trim") == "-" {
				continue
			}
			trimStrings(v.Field(i))
		}

	case reflect.Slice, reflect.Array:
		// A byte slice is binary (base64 or raw), never owner-typed text.
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return
		}
		for i := range v.Len() {
			trimStrings(v.Index(i))
		}

	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			// Map values are not addressable, so trim a settable copy and store it back.
			value := reflect.New(v.Type().Elem()).Elem()
			value.Set(iter.Value())
			trimStrings(value)
			v.SetMapIndex(iter.Key(), value)
		}

	case reflect.String:
		if v.CanSet() {
			v.SetString(strings.TrimSpace(v.String()))
		}
	}
}

// ReadJSONAnyContentType decodes exactly like ReadJSON, but skips the
// Content-Type check entirely.
//
// It exists for the one route that cannot be held to it:
// leads.CaptureAbandonedRegistration is reached by navigator.sendBeacon
// during page unload, which cannot do a CORS preflight and so can only send
// a cross-origin body under one of the CORS-safelisted content types —
// never application/json — and sometimes sends no Content-Type at all. That
// route mints no session cookie and carries no CSRF token for the same
// reason (see csrfExemptRoutes in internal/middleware/chain.go: "reached
// before an account exists, so no cookie is in play"), so it has no ambient
// credential for the Content-Type check to protect, and skipping it here
// reopens nothing ReadJSON's default closes on the routes that do mint one.
func ReadJSONAnyContentType(w http.ResponseWriter, r *http.Request, dst any) error {
	// Deliberately no trimStrings: third-party payloads must reach the handler exactly as sent.
	return decodeJSON(w, r, dst)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return &InvalidJSONError{fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)}

		case errors.Is(err, io.ErrUnexpectedEOF):
			return &InvalidJSONError{errors.New("body contains badly-formed JSON")}

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return &InvalidJSONError{fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)}
			}
			return &InvalidJSONError{fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)}

		case errors.Is(err, io.EOF):
			return &InvalidJSONError{errors.New("body must not be empty")}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return &InvalidJSONError{fmt.Errorf("body contains unknown key %s", fieldName)}

		case errors.As(err, &maxBytesError):
			return &BodyTooLargeError{msg: fmt.Sprintf("body must not be larger than %d bytes", maxBytesError.Limit)}

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return &InvalidJSONError{err}
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return &InvalidJSONError{errors.New("body must only contain a single JSON value")}
	}

	return nil
}

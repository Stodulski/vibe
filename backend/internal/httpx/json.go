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
	"net/http"
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
		return err
	}

	maps.Copy(w.Header(), headers)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// ReadJSON decodes a single JSON value from the request body into dst. Unknown
// fields are rejected, the body is capped at MaxJSONBody, and every decoding
// failure is translated into a message safe to return to the client.
func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
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
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		case errors.As(err, &maxBytesError):
			return &BodyTooLargeError{msg: fmt.Sprintf("body must not be larger than %d bytes", maxBytesError.Limit)}

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

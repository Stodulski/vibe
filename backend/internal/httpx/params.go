package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ReadUUIDParam parses the named router path parameter as a UUID. The error
// names the parameter so the caller can return it to the client unchanged.
//
// The value comes from net/http's own pattern matching (r.PathValue), so a
// parameter the matched route never declared reads as "" and fails here,
// which is the same answer the previous router gave for an unbound name.
func ReadUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s parameter", name)
	}

	return id, nil
}

// ReadStringParam returns the named router path parameter, or "" when the route
// does not define it.
func ReadStringParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

// ReadString returns the query-string value for key, or defaultValue when the
// key is absent or empty.
func ReadString(qs url.Values, key, defaultValue string) string {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	return s
}

// ReadInt returns the query-string value for key parsed as an int. A missing,
// empty, or unparseable value yields defaultValue rather than an error, because
// every caller treats a malformed page or limit as "use the default".
func ReadInt(qs url.Values, key string, defaultValue int) int {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return i
}

// ReadIntStrict returns the query-string value for key parsed as an int, and
// an error when the key is present but does not parse as one. A missing or
// empty key is still not an error — it yields defaultValue, exactly like
// ReadInt — because "the caller sent nothing" and "the caller sent garbage"
// are different situations and only the second should fail the request.
//
// H-05: ReadInt discards strconv.Atoi's error and falls back to
// defaultValue, which is the right call at ten of its twelve call sites
// (limit, weeks — a malformed page size defaulting is harmless). It is the
// wrong call for the monthly report's month/year: `month=13` was correctly
// refused with a 400 by the range check further down, while `month=abc`
// silently became "the current month" and answered 200 with real money
// figures for a period the caller never asked for. Same bad input in two
// spellings, two different outcomes, and the one that succeeded was the
// dangerous one. Use this variant wherever a silent default would hide a
// mistake instead of merely being a convenience.
func ReadIntStrict(qs url.Values, key string, defaultValue int) (int, error) {
	s := qs.Get(key)
	if s == "" {
		return defaultValue, nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s must be a whole number", key)
	}
	return i, nil
}

// ExpectedVersion reads the optimistic-concurrency precondition a client sends
// with a write: the row version it read before it filled in the form.
//
// Two spellings, because the two callers are different. `If-Match: "3"` is the
// HTTP one (RFC 9110 §13.1.1) and is what a client library already knows how to
// send; a `version` field in the body is what a form posting JSON reaches for
// first. They mean the same thing, the header wins where both are present, and
// neither is required — nil means the client sent no precondition and the write
// is the last-write-wins it was before (API-08).
//
// The entity-tag quoting is optional here: `If-Match: "3"` and `If-Match: 3`
// are both read as 3, because a version is a number this API minted and there
// is nothing to gain from refusing the unquoted form over punctuation. `*`
// means "any current version", which is the same thing as sending nothing.
func ExpectedVersion(r *http.Request, bodyVersion *int) (*int, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" || raw == "*" {
		return bodyVersion, nil
	}

	raw = strings.TrimPrefix(raw, "W/")
	raw = strings.Trim(raw, `"`)

	version, err := strconv.Atoi(raw)
	if err != nil || version < 1 {
		return nil, errors.New(`If-Match must be a row version, e.g. "3"`)
	}
	return &version, nil
}

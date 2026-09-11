package httpx

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

// ReadUUIDParam parses the named router path parameter as a UUID. The error
// names the parameter so the caller can return it to the client unchanged.
func ReadUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id, err := uuid.Parse(params.ByName(name))
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s parameter", name)
	}

	return id, nil
}

// ReadStringParam returns the named router path parameter, or "" when the route
// does not define it.
func ReadStringParam(r *http.Request, name string) string {
	params := httprouter.ParamsFromContext(r.Context())
	return params.ByName(name)
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

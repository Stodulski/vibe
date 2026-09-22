package data

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPg wraps a uuid.UUID as a valid pgtype.UUID.
func UUIDToPg(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// UUIDPtrToPg wraps an optional uuid.UUID as a pgtype.UUID; nil becomes SQL NULL.
func UUIDPtrToPg(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

// PgToUUID unwraps a pgtype.UUID; a NULL becomes uuid.Nil.
func PgToUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return uuid.UUID(id.Bytes)
}

// PgToUUIDPtr unwraps a pgtype.UUID into an optional uuid.UUID; a NULL becomes nil.
func PgToUUIDPtr(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	u := uuid.UUID(id.Bytes)
	return &u
}

// UUIDSliceToPg wraps a slice of uuid.UUID for a query taking a uuid[] parameter.
func UUIDSliceToPg(ids []uuid.UUID) []pgtype.UUID {
	result := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		result[i] = UUIDToPg(id)
	}
	return result
}

// TimeToPg wraps a time.Time as a pgtype.Timestamptz; a zero time becomes SQL NULL.
func TimeToPg(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// PgToTime unwraps a pgtype.Timestamptz; a NULL becomes the zero time.
func PgToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// TimePtrToPg and PgToTimePtr are the nullable-pointer pair for a timestamp
// column, matching the shape TextToPg/PgToTextPtr already has for a nullable
// text column: nil means the SQL value is NULL rather than a zero time, so a
// caller that never touched the field cannot be confused with one that
// explicitly cleared it.
func TimePtrToPg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// PgToTimePtr unwraps a pgtype.Timestamptz into an optional time.Time; a NULL
// becomes nil. See TimePtrToPg above.
func PgToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

// DateToPg wraps a time.Time as a pgtype.Date; a zero time becomes SQL NULL.
func DateToPg(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: !t.IsZero()}
}

// PgToDate unwraps a pgtype.Date; a NULL becomes the zero time.
func PgToDate(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

// TextToPg wraps an optional string as a pgtype.Text; nil becomes SQL NULL.
func TextToPg(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// PgToTextPtr unwraps a pgtype.Text into an optional string; a NULL becomes nil.
func PgToTextPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// likeEscapeReplacer escapes the three characters LIKE/ILIKE treat as
// operators — '%', '_' and the escape character itself, '\' — in that order,
// so a search term is matched literally rather than as a wildcard pattern.
// The backslash pair must come first: strings.Replacer applies every pair in
// one left-to-right scan of the input rather than re-scanning its own output,
// so ordering the pairs any other way would double-escape a literal
// backslash a caller typed.
var likeEscapeReplacer = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLikeTerm escapes s for safe use inside a LIKE/ILIKE pattern.
//
// H-04: a search term built into a pattern with plain string concatenation —
// "%" + search + "%", or the SQL-side equivalent '%' || $1 || '%' — lets a
// caller's own '%' and '_' act as wildcards instead of the literal characters
// they typed, so searching for a literal "%" matched every row instead of
// none. This is not the injection every other input finding here is about:
// every one of these queries is already parameterized, so the term never
// reaches the SQL parser — only LIKE's own pattern language sees it, and that
// is what this escapes it out of.
//
// Every call site must also add ESCAPE '\' to its predicate. Without it this
// escaping is inert: Postgres only treats '\' as LIKE's default escape
// character in some configurations, so the clause has to be explicit rather
// than assumed.
func EscapeLikeTerm(s string) string {
	return likeEscapeReplacer.Replace(s)
}

// Int4ToPg wraps an int as a valid pgtype.Int4.
func Int4ToPg(n int) pgtype.Int4 {
	//nolint:gosec // G115: all call sites pass currency amounts (refund cents, bounded by the original payment
	// amount) or other domain values already validated to fit comfortably within int32 range.
	return pgtype.Int4{Int32: int32(n), Valid: true}
}

// Int4PtrToPg wraps an optional int as a pgtype.Int4; a nil becomes SQL NULL.
// It is what an optional precondition looks like in a query — see
// `sqlc.narg('expected_version')` in db/queries/complexes.sql.
func Int4PtrToPg(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return Int4ToPg(*n)
}

// PgToInt4Ptr unwraps a pgtype.Int4 into an optional int; a NULL becomes nil.
// The pointer-returning counterpart to Int4PtrToPg, for a column whose NULL
// and zero are different facts — cash_sessions' close-state columns
// (counted_cash, expected_cash, difference) are NULL while a session is open
// and set once it closes, so PgToInt's zero-for-invalid would read a still-open
// session as "counted 0" instead of "not counted yet".
func PgToInt4Ptr(n pgtype.Int4) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int32)
	return &v
}

// PgToInt unwraps a pgtype.Int4; a NULL becomes zero.
func PgToInt(n pgtype.Int4) int {
	if !n.Valid {
		return 0
	}
	return int(n.Int32)
}

// Int8ToPg wraps an int64 as a valid pgtype.Int8. Unlike Int4ToPg this takes
// no narrowing cast: a BIGINT column's Go value already is an int64, so there
// is nothing to validate a call site's range against.
func Int8ToPg(n int64) pgtype.Int8 {
	return pgtype.Int8{Int64: n, Valid: true}
}

// Int8PtrToPg wraps an optional int64 as a pgtype.Int8; a nil becomes SQL
// NULL — the int64 counterpart of Int4PtrToPg, for a nullable BIGINT column.
func Int8PtrToPg(n *int64) pgtype.Int8 {
	if n == nil {
		return pgtype.Int8{}
	}
	return Int8ToPg(*n)
}

// PgToInt8Ptr unwraps a pgtype.Int8 into an optional int64; a NULL becomes
// nil. The pointer-returning counterpart to Int8PtrToPg, for a column whose
// NULL and zero are different facts — cash_sessions' close-state columns
// (counted_cash, expected_cash, difference) are NULL while a session is open
// and set once it closes, the same reasoning PgToInt4Ptr's own comment gives,
// just BIGINT rather than INTEGER (see db/migrations/003_cashbox.sql for why
// this trio needed the wider column).
func PgToInt8Ptr(n pgtype.Int8) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// PgToTimeStr renders a pgtype.Time as the "HH:MM" wall-clock string the
// domain types carry; a NULL becomes the empty string.
func PgToTimeStr(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	totalSec := t.Microseconds / 1_000_000
	return fmt.Sprintf("%02d:%02d", totalSec/3600, (totalSec%3600)/60)
}

// Float8ToPg wraps an optional float64 as a pgtype.Float8; nil becomes SQL NULL.
func Float8ToPg(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *f, Valid: true}
}

// PgToFloat8Ptr unwraps a pgtype.Float8 into an optional float64; a NULL becomes nil.
func PgToFloat8Ptr(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

// TimeStrToPg parses an "HH:MM" wall-clock string into a pgtype.Time; an
// unparseable string becomes SQL NULL.
func TimeStrToPg(s string) pgtype.Time {
	var h, m int
	if n, _ := fmt.Sscanf(s, "%d:%d", &h, &m); n != 2 { //nolint:errcheck // n is the check: fewer than two fields is the NULL below
		return pgtype.Time{}
	}
	return pgtype.Time{
		Microseconds: int64(h*3600+m*60) * 1_000_000,
		Valid:        true,
	}
}

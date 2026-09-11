package data

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func uuidToPg(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func uuidPtrToPg(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func pgToUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return uuid.UUID(id.Bytes)
}

func pgToUUIDPtr(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	u := uuid.UUID(id.Bytes)
	return &u
}

func uuidSliceToPg(ids []uuid.UUID) []pgtype.UUID {
	result := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		result[i] = uuidToPg(id)
	}
	return result
}

func timeToPg(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

func pgToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// timePtrToPg and pgToTimePtr are the nullable-pointer pair for a timestamp
// column, matching the shape textToPg/pgToTextPtr already has for a nullable
// text column: nil means the SQL value is NULL rather than a zero time, so a
// caller that never touched the field cannot be confused with one that
// explicitly cleared it.
func timePtrToPg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func pgToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tm := t.Time
	return &tm
}

func dateToPg(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: !t.IsZero()}
}

func pgToDate(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

func textToPg(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func pgToTextPtr(t pgtype.Text) *string {
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

// escapeLikeTerm escapes s for safe use inside a LIKE/ILIKE pattern.
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
func escapeLikeTerm(s string) string {
	return likeEscapeReplacer.Replace(s)
}

func int4ToPg(n int) pgtype.Int4 {
	//nolint:gosec // G115: all call sites pass currency amounts (refund cents, bounded by the original payment
	// amount) or other domain values already validated to fit comfortably within int32 range.
	return pgtype.Int4{Int32: int32(n), Valid: true}
}

func pgToInt(n pgtype.Int4) int {
	if !n.Valid {
		return 0
	}
	return int(n.Int32)
}

func pgToTimeStr(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	totalSec := t.Microseconds / 1_000_000
	return fmt.Sprintf("%02d:%02d", totalSec/3600, (totalSec%3600)/60)
}

func float8ToPg(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *f, Valid: true}
}

func pgToFloat8Ptr(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

func timeStrToPg(s string) pgtype.Time {
	var h, m int
	if n, _ := fmt.Sscanf(s, "%d:%d", &h, &m); n != 2 {
		return pgtype.Time{}
	}
	return pgtype.Time{
		Microseconds: int64(h*3600+m*60) * 1_000_000,
		Valid:        true,
	}
}

// Package timezone holds the single wall-clock the product operates in.
//
// Every complex on the platform is in Argentina, and almost every date
// decision depends on it: whether a booking is today, which month a payment
// falls in, when a cancellation window closes. Those answers are all wrong if
// they are computed in UTC, which is what a server does by default.
//
// It is a package rather than a constant in one handler file because five
// unrelated areas need it, and the alternative is each of them re-deriving the
// same location — which is how two of them end up disagreeing.
package timezone

import "time"

// name is the IANA zone. Argentina has had no DST since 2009, but the zone is
// still the right thing to load: it carries the historical offsets that a
// fixed -03:00 would get wrong for older records.
const name = "America/Argentina/Buenos_Aires"

// Argentina is the product's wall-clock.
//
// It falls back to UTC if the zone is missing from the system's tzdata rather
// than panicking at init: a container without tzdata should still start and
// serve, with dates that are off by three hours, instead of refusing to boot.
var Argentina = func() *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}()

// Now returns the current time in the product's wall-clock.
func Now() time.Time {
	return time.Now().In(Argentina)
}

// Today returns the current date in the product's wall-clock, with the time of
// day zeroed — the form date-scoped queries compare against.
func Today() time.Time {
	now := Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, Argentina)
}

// ParseDay reads a "YYYY-MM-DD" calendar date as midnight in the product's
// wall-clock.
//
// time.Parse yields the same calendar date at midnight UTC, which is a
// different instant — three hours earlier, and therefore still the previous day
// here. That gap is invisible while a date is only ever compared against
// another date, and becomes a three-hour error the moment the date is combined
// with a time of day and compared against something the database stored as an
// instant. Both kinds of comparison now exist in the same handlers, so the
// parse is the place to settle it.
//
// It is the parsing counterpart to Today: a parsed day and today's day are the
// same kind of value, anchored the same way.
func ParseDay(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, Argentina)
}

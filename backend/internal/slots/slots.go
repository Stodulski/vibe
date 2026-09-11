// Package slots holds the arithmetic for the "HH:MM" wall-clock strings that
// courts, schedules and bookings are expressed in. It is deliberately free of
// dates, time zones and domain rules: a slot time is a position within a day,
// and everything that needs a real instant composes it with a date elsewhere.
package slots

import (
	"fmt"
	"time"
)

// minutesPerDay is the wrap-around point for slot arithmetic.
const minutesPerDay = 1440

// DefaultDuration is the slot length a court gets when none is chosen.
const DefaultDuration = 90

// PermittedDurations returns the slot lengths a court may be sold in.
//
// The grid is laid end to end from opening in steps of this length, so the set
// is the grid's interval and not a cosmetic preference: it was written out at
// three sites in the courts handler — the create default, the create check and
// the update check — and a fourth would have been written the next time one was
// added. It is a function rather than a package variable so that no caller can
// alter the set for everyone else.
func PermittedDurations() []int { return []int{60, 90, 120} }

// Overlap reports whether the half-open range [startA, endA) shares any minute
// with [startB, endB).
//
// The comparison is lexicographic, which is exact for "HH:MM" and only for
// "HH:MM": both ranges must be zero-padded and neither may wrap past midnight.
// Grid guarantees the second condition for everything it emits or accepts, and
// ValidFormat guarantees the first at every handler.
//
// It lives here because three call sites had written it out — the availability
// grid, the blocked-slot write, and the booking write paths — and a booking
// that is refused by one and accepted by another is the defect this whole
// package exists to remove.
func Overlap(startA, endA, startB, endB string) bool {
	return startA < endB && endA > startB
}

// OverlapAt reports whether two dated spans share any moment.
//
// The same test as Overlap, on instants rather than times of day, and half-open
// in the same direction: a span ending exactly when another begins does not
// overlap it. That convention is now written in four places — here, Overlap
// above, the '[)' bounds bookings.span carries, and court_prices' exclusion
// constraint — and they agree on purpose.
//
// Prefer this one wherever the spans come from stored bookings. Overlap's
// precondition is that neither range wraps past midnight, which the grid can
// promise about what it offers but nothing can promise about what is already
// on the books once a booking is allowed to run into the next day.
func OverlapAt(startA, endA, startB, endB time.Time) bool {
	return startA.Before(endB) && endA.After(startB)
}

// ValidFormat reports whether s is a well-formed 24-hour "HH:MM" time.
//
// It reads the four digits itself rather than delegating to fmt.Sscanf, and
// that is the whole point of the function. Sscanf's %d verb skips leading
// spaces, accepts a sign, and stops at the first byte it cannot use without
// complaining about the rest — so "+9:30", " 9:30", "09: 5", "09:+5", "09:5 "
// and "12:3x" all passed the old length-and-colon gate, scanned as two
// integers, and were answered "yes, that is HH:MM".
//
// Answering yes was worse than answering no, because every caller then treated
// the string as validated and handed it to ToMinutes, which reinterprets it by
// the same lenient rule: a booking submitted as "12:3x" was accepted, stored as
// 12:03 (timeStrToPg scans it identically), given an end_time derived from
// 12:03, and echoed back to the client as "12:3x". The client was told a time
// nobody sold and the court was booked for a different one.
//
// A time this product can act on is exactly five bytes, four of them digits,
// with a colon between them. Nothing else is HH:MM, and nothing else is
// reinterpreted into it.
func ValidFormat(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	for _, i := range [...]int{0, 1, 3, 4} {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	return h <= 23 && m <= 59
}

// ToMinutes converts "HH:MM" to minutes since midnight.
//
// A malformed input yields 0 rather than an error: every call site passes a
// string already gated by ValidFormat at the handler, or a schedule value the
// database stored through that same validated path.
func ToMinutes(s string) int {
	var h, m int
	_, _ = fmt.Sscanf(s, "%d:%d", &h, &m)
	return h*60 + m
}

// FromMinutes converts minutes since midnight to "HH:MM", wrapping at midnight
// so that a slot running past 23:59 reports the next day's clock time.
func FromMinutes(minutes int) string {
	minutes %= minutesPerDay
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// Add returns timeStr shifted by minutes, wrapping at midnight.
func Add(timeStr string, minutes int) string {
	return FromMinutes(ToMinutes(timeStr) + minutes)
}

// DayName maps a Go weekday to the lowercase day string that complex opening
// schedules are keyed by.
//
// It exists because time.Weekday.String() returns "Monday", while the schedule
// rows store "monday"; lowercasing at each call site invites one of them to
// forget.
func DayName(wd time.Weekday) string {
	switch wd {
	case time.Monday:
		return "monday"
	case time.Tuesday:
		return "tuesday"
	case time.Wednesday:
		return "wednesday"
	case time.Thursday:
		return "thursday"
	case time.Friday:
		return "friday"
	case time.Saturday:
		return "saturday"
	case time.Sunday:
		return "sunday"
	default:
		return ""
	}
}

// ToHours converts "HH:MM" to decimal hours, so that a span between two slot
// times can be arithmetic rather than a duration parse.
//
// A malformed input yields 0, matching ToMinutes: callers pass values already
// gated by ValidFormat.
func ToHours(s string) float64 {
	var h, m int
	if n, _ := fmt.Sscanf(s, "%d:%d", &h, &m); n != 2 {
		return 0
	}
	return float64(h) + float64(m)/60.0
}

// At places a time of day ("HH:MM") on a calendar date, keeping date's own
// location.
//
// It exists so a caller can walk forward through a booking without wrapping.
// FromMinutes is deliberately modular — it answers "what does the clock read?"
// and 1440 reads as 00:00 — which is the right answer for a clock and the wrong
// one for a span: adding thirty minutes to 23:45 has to land on the next day,
// not back at the start of this one. Adding to the value this returns rolls the
// date, so the weekday a block falls on follows it.
//
// The result is a wall-clock position, not an absolute instant. Callers use it
// to ask which weekday and which minute a block lands on, so the location is
// carried through from date rather than imposed here, and no offset arithmetic
// is involved that a daylight-saving shift could disturb.
func At(date time.Time, hhmm string) time.Time {
	midnight := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	return midnight.Add(time.Duration(ToMinutes(hhmm)) * time.Minute)
}

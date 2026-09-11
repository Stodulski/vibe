package timezone

import "time"

// This file holds the two things every renderer of a booking's hours needs:
// the calendar day a stored date stands for on the product's wall-clock, and
// the one sentence the product prints for a stretch of booked hours.
//
// They live beside Argentina rather than in internal/slots because slots is
// deliberately free of dates and time zones — a slot time there is a position
// within a day — and both of these answer questions a position within a day
// cannot: which day, and whether the end is on a different one.
//
// HoursLabel carries a Spanish word, which is the same trade-off
// internal/reporting/copy.go documents: Spanish is the only locale the product
// ships, an owner and a client read this string in an email, a WhatsApp
// message and a spreadsheet, and gathering it in one place means moving it
// behind a backend i18n layer is one edit.

// nextDayMarker is what a booking whose hours land on the following day is
// marked with.
//
// It is the vocabulary vibe-client already uses for exactly this fact — the
// complex schedule row's "Día sig." — rather than a second wording invented
// here. A customer who reads "23:00 a 01:00" in a WhatsApp message and
// "23:00 – 01:00 Día sig." on the confirmation screen is being told two
// different things by the same product.
const nextDayMarker = "día sig."

// Day anchors a calendar date at midnight on the product's wall-clock.
//
// It reads the year, month and day exactly as they stand in t and rebuilds
// them here. That is what a `date` column means: pgx decodes one as midnight
// UTC and time.Parse yields midnight UTC too, so the calendar fields are
// right in both cases and only the anchoring is wrong. Anchoring matters as
// soon as such a date is combined with a time of day and compared against
// something the database stored as an instant — the two land three hours
// apart, and the comparison quietly answers about the wrong hours.
//
// ParseDay is the same value built from a string; this is the same value built
// from a time that already carries the calendar fields.
func Day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Argentina)
}

// HoursLabel renders a booking's span as the hours a person reads.
//
// Both instants are put on the product's wall-clock first, so the string says
// what the clock at the venue reads, whatever zone the process runs in or pgx
// decoded the row into. When the end falls on a later calendar day than the
// start, the marker is appended — which is the whole reason this function
// exists rather than two calls to Format.
//
// It replaced `booking.StartTime + " - " + booking.EndTime` at six call sites.
// That spelling read bookings.end_time, a bare clock reading produced by
// modular arithmetic (internal/slots.Add), so a 23:00 booking of two hours was
// announced to its customer as "23:00 - 01:00" — an end two hours before its
// own start, on a day nobody named. The column is gone; this is
// what the six sites say instead.
//
// The separator is the word "a", not a dash: WhatsApp's template validator
// rejects two variables sitting next to each other with nothing fixed between
// them, and a dash inside this one value reads as exactly that kind of
// ambiguous joint once it sits beside the other fixed words in the approved
// body. "09:00 a 10:30" is unambiguous either way.
//
// A zero instant on either side yields the empty string rather than the year
// one: a caller holding a booking with no span has nothing to render, and
// "01:00:00 a 01:00:00" would look like an answer.
func HoursLabel(startsAt, endsAt time.Time) string {
	if startsAt.IsZero() || endsAt.IsZero() {
		return ""
	}
	start := startsAt.In(Argentina)
	end := endsAt.In(Argentina)

	label := start.Format("15:04") + " a " + end.Format("15:04")
	if EndsOnALaterDay(startsAt, endsAt) {
		label += " (" + nextDayMarker + ")"
	}
	return label
}

// EndsOnALaterDay reports whether the end of a span falls on a calendar day
// after the day it started on, both read on the product's wall-clock.
//
// The comparison is on the calendar fields rather than on elapsed time: a
// booking is on the next day because the date rolled, not because it ran for
// more than some number of hours. A span ending exactly at midnight is on the
// next day by this test and is meant to be — a booking that ends at 00:00
// finishes as the following day begins, and telling the customer "23:00 -
// 00:00" with nothing else is the ambiguity this whole change removes.
func EndsOnALaterDay(startsAt, endsAt time.Time) bool {
	if startsAt.IsZero() || endsAt.IsZero() {
		return false
	}
	sy, sm, sd := startsAt.In(Argentina).Date()
	ey, em, ed := endsAt.In(Argentina).Date()
	return ey != sy || em != sm || ed != sd
}

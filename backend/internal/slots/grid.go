package slots

import "errors"

// A court is sold in fixed-length slots laid end to end from the moment the
// complex opens. That grid is the whole definition of "a bookable position on
// this court today", and it used to exist twice: the availability handler
// generated it to render the storefront, and the booking handlers re-derived a
// weaker version of it — schedule containment only — to accept a write. The two
// disagreed on both ends. Availability offered slots past midnight that the
// write path refuses, and the write path accepted start times that are not on
// the grid at all, which takes two grid positions out of circulation and
// defeats a slot lock keyed on an exact start time.
//
// Grid is that derivation, once. The storefront renders Slots(); the write path
// calls Validate on the value a caller asked for. Neither can offer or accept
// what the other would not.

// Errors returned by Grid.Validate. They are distinct because the three
// refusals mean different things to the person who hit them: "we are shut",
// "we cannot sell hours that cross midnight", and "pick a real starting time".
var (

	// ErrOutsideSchedule means the range falls outside the complex's opening
	// hours for that day.
	ErrOutsideSchedule = errors.New("slots: time is outside the complex schedule")

	// ErrOffGrid means the start time is inside the opening hours but is not
	// one of the court's slot boundaries.
	ErrOffGrid = errors.New("slots: time does not fall on the court's slot grid")
)

// Slot is one bookable position: a start and the end that follows from the
// court's own slot length.
type Slot struct {
	Start string
	End   string
	// StartMin is Start's position inside the window that produced it, in
	// minutes from that window's own day's midnight. It exceeds 1440 for a
	// slot past the rollover, which is exactly what Start cannot say: a venue
	// trading to 01:30 offers a slot whose Start reads "00:30", and only this
	// number distinguishes it from half past midnight the previous morning.
	// It is what the slot is priced from.
	StartMin int
}

// gridStep is the interval start times are offered on.
//
// A court used to be sold in one fixed length, and the grid's step was that
// length: a 90-minute court only ever started a slot every 90 minutes. Now any
// court may be booked for 60, 90 or 120 minutes, chosen per booking, and a step
// tied to any one of those durations makes valid combinations unreachable — a
// 90-minute step could never follow a 60-minute booking with one starting where
// it ends. 30 is the GCD of 60, 90 and 120: the finest interval that still lands
// every permitted duration's start and end on a grid position.
const gridStep = 30

// Grid is a court's bookable start times on one day.
//
// It is built from the complex's opening hours for that weekday, which is all
// the shape of the day depends on now that a booking's length is chosen per
// request rather than fixed on the court. What is priced, booked, blocked or
// already past is applied on top by the caller — those change through the day,
// the grid does not.
type Grid struct {
	// open is the opening time in minutes since midnight.
	open int
	// closes is the closing time in minutes since midnight, carried past 1440
	// when the venue closes after midnight, so that the window is always a
	// forward range from open.
	closes int
}

// NewGrid returns the grid a court has on a day the complex is open between
// openTime and closeTime ("HH:MM").
//
// A closeTime at or before openTime is read as closing after midnight, which is
// how a venue open 18:00-02:00 stores its hours.
func NewGrid(openTime, closeTime string) Grid {
	open := ToMinutes(openTime)
	closes := ToMinutes(closeTime)
	if closes <= open {
		closes += minutesPerDay
	}
	return Grid{open: open, closes: closes}
}

// lastEnd is the latest minute a slot on this grid may end at: the closing
// time, however far past midnight that falls.
//
// It used to cap one minute short of midnight, and the comment here explained
// why in terms of storage: a booking was one row on one date whose end_time had
// to exceed its start_time, so 24:00 was not a time this product could hold. A
// venue trading until 02:00 therefore had its last four hours quietly withheld
// from sale. That is no longer true — a booking carries a tstzrange
// (the generated span) that crosses midnight without noticing, and the check
// that forbade it is gone — so the cap has nothing left to
// protect and the hours it withheld are for sale.
func (g Grid) lastEnd() int {
	return g.closes
}

// Slots returns every start time this grid sells a booking of durationMinutes
// at, in order.
//
// The result is never nil, so a caller can render it directly; a day with no
// room for a single booking of that length returns an empty list rather than
// nothing. Start times are offered every gridStep minutes regardless of
// durationMinutes — that is what lets a 60-minute booking be followed
// immediately by a 90-minute one starting where it ends, which a step tied to
// either duration could not offer.
func (g Grid) Slots(durationMinutes int) []Slot {
	out := []Slot{}
	if durationMinutes <= 0 {
		return out
	}

	limit := g.lastEnd()
	for t := g.open; t+durationMinutes <= limit; t += gridStep {
		out = append(out, Slot{Start: FromMinutes(t), End: FromMinutes(t + durationMinutes), StartMin: t})
	}
	return out
}

// OwnsSameDay reports whether this window covers a time of day falling on the
// window's own date, and at what minute from that date's midnight.
//
// OwnsNextDay asks the other half of the same question. They are separate on
// purpose: "00:30" and "10:00" are both just clock readings, and whether a
// window contains one depends entirely on which date the reading belongs to.
// A single function trying both readings claimed Friday 10:00 for a Thursday
// window that runs 08:00 to 01:30 — minute 600 is inside [480, 1530), but it is
// minute 600 of Friday, not of Thursday.
func (g Grid) OwnsSameDay(hhmm string) (int, bool) {
	return g.owns(ToMinutes(hhmm))
}

// OwnsNextDay reports whether this window is still open at a time of day
// falling on the date after the window's own — the hours a venue trading past
// midnight sells after the calendar has rolled over.
func (g Grid) OwnsNextDay(hhmm string) (int, bool) {
	return g.owns(ToMinutes(hhmm) + minutesPerDay)
}

func (g Grid) owns(minute int) (int, bool) {
	if minute >= g.open && minute < g.closes {
		return minute, true
	}
	return 0, false
}

// OnGridStep reports whether start ("HH:MM") falls on the package's
// 30-minute grid step, independent of any court or complex's opening hours.
//
// It exists for the owner/staff write path, which may book any time of day
// manually whether or not the complex is open then, but still cannot accept
// an arbitrary start such as "10:07": nothing else in the system — not the
// storefront's grid, not a slot lock keyed on an exact start time — can offer
// or reconcile a position off this step.
func OnGridStep(start string) bool {
	return ToMinutes(start)%gridStep == 0
}

// Validate reports whether a booking of durationMinutes starting at start
// ("HH:MM") is a position this grid actually sells.
//
// It answers the question the write path has to ask and the storefront has
// already answered by construction: every start Slots(durationMinutes) emits
// validates with that same duration, and nothing else does. The order of the
// checks is the order the refusals matter in — a caller who asked for hours
// past midnight is told that, not that their start time is off by fifteen
// minutes.
func (g Grid) Validate(start string, durationMinutes int) error {
	if durationMinutes <= 0 {
		return ErrOffGrid
	}

	startMin := ToMinutes(start)

	if startMin < g.open || startMin+durationMinutes > g.closes {
		return ErrOutsideSchedule
	}
	if (startMin-g.open)%gridStep != 0 {
		return ErrOffGrid
	}
	return nil
}

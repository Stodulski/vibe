/**
 * Span arithmetic for bookings, on absolute instants.
 *
 * `time.ts` next door works in minutes since midnight, which is the right unit
 * for a grid of a single day and the wrong one for a booking. A booking may run
 * past midnight, and once it can, "01:00" is not a smaller number than "23:00"
 * — it is the following day. Every comparison built on the smaller number being
 * earlier reports that such a booking occupies nothing, which is how its hours
 * go back on sale while somebody is playing on them.
 *
 * These functions take the `starts_at` / `ends_at` the API now sends and answer
 * the two questions the UI actually has: does this collide with that, and where
 * on today's grid does it sit. They mirror `slots.OverlapAt` and `slots.At` on
 * the server, deliberately — a booking the storefront draws as free and the
 * write path refuses is the defect both sides exist to prevent.
 */

/**
 * The venue's IANA zone. Every complex on the platform is in Argentina, and the
 * server says the same thing in `internal/timezone` for the same reason: a date
 * decision computed on the runtime's own clock is wrong the moment the runtime
 * is not there, which for a browser is most of the time and for CI is always.
 */
export const VENUE_TIME_ZONE = 'America/Argentina/Buenos_Aires';

/**
 * The venue's UTC offset, written out.
 *
 * Used to build an instant from a calendar day at the venue: `venueInstant`
 * for a booking the customer has picked but not yet paid for, which lives in
 * sessionStorage until the server answers with its own `starts_at`/`ends_at`
 * and replaces it; and `startOfDay` below, for every place on the client that
 * turns a calendar day into "midnight" and needs that to be the venue's
 * midnight, not the runtime's. Argentina has had no daylight saving since
 * 2009, so this is exact for every booking the storefront can sell; and for
 * the sessionStorage case, if it ever stops being, the value it produces is
 * overwritten by the API's within a second of the confirmation page loading.
 *
 * Nothing that renders a booking the server has answered about should use it:
 * those carry real instants.
 */
const VENUE_UTC_OFFSET = '-03:00';

/**
 * The instant `minutes` past midnight on `date` at the venue, as RFC3339.
 *
 * `minutes` may exceed 1440, in which case the instant simply lands on the
 * following day — which is the point. It is the client-side twin of the
 * server's `booking_starts_at(date, time)`.
 */
export function venueInstant(date: string, minutes: number): string | null {
  const midnight = new Date(`${date.slice(0, 10)}T00:00:00${VENUE_UTC_OFFSET}`);
  if (Number.isNaN(midnight.getTime())) return null;
  return new Date(midnight.getTime() + minutes * 60_000).toISOString();
}

/** Minutes in a day — the grid's full height, and the wrap these avoid. */
export const MINUTES_PER_DAY = 24 * 60;

/**
 * The venue's midnight of a `YYYY-MM-DD` calendar day, as an instant.
 *
 * Built the same way `venueInstant` builds its own midnight — the date string
 * plus `VENUE_UTC_OFFSET` — deliberately not `new Date(y, m-1, d, ...)`, which
 * reads the parts in the *runtime's* zone. `minutesInto` and `spanOnDay` both
 * derive from this, so a runtime-zone midnight would move every booking block
 * on the grid, every free-slot check and every occupancy check by the gap
 * between the runtime's offset and Argentina's — invisible on a contributor's
 * machine set to Argentina time, and wrong for every booking on any other one,
 * CI included. The server computes the same midnight in the venue zone
 * (`internal/timezone`), and this keeps the client agreeing with it. Returns
 * `null` for a string that is not a calendar date, so a caller decides what to
 * do rather than silently getting an Invalid Date that poisons every
 * arithmetic downstream.
 */
export function startOfDay(date: string): Date | null {
  const midnight = new Date(`${date.slice(0, 10)}T00:00:00${VENUE_UTC_OFFSET}`);
  if (Number.isNaN(midnight.getTime())) return null;
  return midnight;
}

/**
 * How many minutes past `date`'s local midnight `instant` falls.
 *
 * Deliberately unclamped: a booking that began yesterday returns a negative
 * number and one ending tomorrow returns more than 1440. That is the honest
 * answer, and it is what lets the grid clip such a booking at the edge it
 * crosses instead of folding it back into the middle of the day.
 */
export function minutesInto(date: string, instant: Date): number | null {
  const midnight = startOfDay(date);
  if (!midnight || Number.isNaN(instant.getTime())) return null;
  return (instant.getTime() - midnight.getTime()) / 60_000;
}

/**
 * Whether two spans share any moment.
 *
 * Half-open: a span ending exactly when another begins does not overlap it.
 * The same convention as the server's `slots.OverlapAt`, the `[)` bounds on
 * `bookings.span`, and the exclusion constraint that enforces them — four
 * places that agree on purpose.
 */
export function overlaps(aStart: Date, aEnd: Date, bStart: Date, bEnd: Date): boolean {
  return aStart.getTime() < bEnd.getTime() && aEnd.getTime() > bStart.getTime();
}

/**
 * A booking's span, ready to compare.
 *
 * Takes the two ISO strings the API sends rather than a whole booking, so the
 * blocked slots — which have no `starts_at` yet — can be lifted into the same
 * shape and compared against bookings without either side knowing about the
 * other's row type.
 */
export interface Span {
  start: Date;
  end: Date;
}

/** Reads a span from a pair of ISO instants; `null` if either is unparseable. */
export function toSpan(startsAt: string, endsAt: string): Span | null {
  const start = new Date(startsAt);
  const end = new Date(endsAt);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return null;
  return { start, end };
}

/**
 * Builds a span from a calendar day and two times of day.
 *
 * For the rows that still carry `HH:MM` — blocked slots, and the grid's own
 * candidate slots — so they can be compared against bookings without anyone
 * reintroducing a minutes-since-midnight comparison to bridge the two.
 * `endMinutes` may exceed 1440; the resulting end simply lands on the next day.
 */
export function spanOnDay(date: string, startMinutes: number, endMinutes: number): Span | null {
  const midnight = startOfDay(date);
  if (!midnight) return null;
  return {
    start: new Date(midnight.getTime() + startMinutes * 60_000),
    end: new Date(midnight.getTime() + endMinutes * 60_000),
  };
}

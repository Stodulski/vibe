import { describe, it, expect } from 'vitest';
import { courtObstacles, windowIsOccupied } from './courtOccupancy';
import { spanOnDay } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';
import { makeBooking } from '@/test/factories';
import type { BlockedSlot } from '@/shared/types/api.types';

const COURT = 'ct1';
const DAY = '2026-03-18';
const HALF_HOUR = 30;

function blocked(start: string, end: string, date = DAY): BlockedSlot {
  return {
    id: `bs-${start}`,
    complex_id: 'c1',
    court_id: COURT,
    date,
    start_time: start,
    end_time: end,
    reason: null,
    created_at: '2026-03-01T00:00:00Z',
  } as BlockedSlot;
}

/**
 * The `starts_at`/`ends_at` pair for a booking running `durationMinutes` from
 * `startHhMm` on `date`, built with `spanOnDay` — the exact function
 * `windowIsOccupied` uses to build the comparison window it is tested
 * against.
 *
 * `spanOnDay` reads `date` at the *venue's* midnight (see its doc comment on
 * `startOfDay`), never the test process's own zone — the same reason the
 * server computes the same thing in `internal/timezone`. `makeBooking`'s own
 * default instead derives `starts_at`/`ends_at` from the *runner's* zone
 * (`localIso`), which only agrees with `spanOnDay`'s venue-zone window when
 * the runner happens to be set to Argentina time. Building the fixture
 * through the same `spanOnDay` call keeps it and the window it is compared
 * against describing the same moment regardless of which zone the test
 * process runs in, so the assertions below are about the overlap logic, not
 * about which zone ran them. Every booking fixture compared against
 * `taken()` below needs it, not only the ones crossing midnight.
 */
function venueSpan(date: string, startHhMm: string, durationMinutes: number) {
  const startMinutes = timeToMinutes(startHhMm);
  const span = spanOnDay(date, startMinutes, startMinutes + durationMinutes);
  if (!span) throw new Error(`invalid fixture span: ${date} ${startHhMm}+${String(durationMinutes)}`);
  return { starts_at: span.start.toISOString(), ends_at: span.end.toISOString() };
}

/** Is the half hour starting at `hhmm` taken on this day? */
function taken(obstacles: ReturnType<typeof courtObstacles>, hhmm: string, date = DAY) {
  const [h, m] = hhmm.split(':').map(Number);
  return windowIsOccupied(obstacles, date, (h ?? 0) * 60 + (m ?? 0), HALF_HOUR);
}

describe("a court's obstacles", () => {
  it('ignores another court', () => {
    const elsewhere = makeBooking({
      court_id: 'ct2',
      date: DAY,
      start_time: '10:00',
      duration_minutes: 60,
    });

    expect(courtObstacles([elsewhere], [], COURT, DAY)).toHaveLength(0);
  });

  it('ignores a cancelled booking, because the court is free again', () => {
    const cancelled = makeBooking({
      court_id: COURT,
      date: DAY,
      start_time: '10:00',
      duration_minutes: 60,
      status: 'cancelled',
    });

    expect(courtObstacles([cancelled], [], COURT, DAY)).toHaveLength(0);
  });
});

describe('whether a window is occupied', () => {
  it('is taken while a booking runs, and free once it ends', () => {
    const o = courtObstacles(
      [
        makeBooking({
          court_id: COURT,
          date: DAY,
          start_time: '10:00',
          duration_minutes: 60,
          ...venueSpan(DAY, '10:00', 60),
        }),
      ],
      [],
      COURT,
      DAY,
    );

    expect(taken(o, '10:00')).toBe(true);
    expect(taken(o, '10:30')).toBe(true);
    expect(taken(o, '11:00')).toBe(false);
  });

  // Half-open at both ends: a booking that ends at 10:00 does not hold 10:00,
  // and one that starts at 11:00 does not hold 10:30.
  it('does not hold the half hour it ends on', () => {
    const o = courtObstacles(
      [
        makeBooking({
          court_id: COURT,
          date: DAY,
          start_time: '09:00',
          duration_minutes: 60,
          ...venueSpan(DAY, '09:00', 60),
        }),
      ],
      [],
      COURT,
      DAY,
    );

    expect(taken(o, '10:00')).toBe(false);
  });

  // Overlap, not a sample of the window's first instant. A booking starting at
  // 10:15 holds the 10:00 half hour even though 10:00 itself is still free.
  it('is taken by a booking that starts inside the window', () => {
    const o = courtObstacles(
      [
        makeBooking({
          court_id: COURT,
          date: DAY,
          start_time: '10:15',
          duration_minutes: 60,
          ...venueSpan(DAY, '10:15', 60),
        }),
      ],
      [],
      COURT,
      DAY,
    );

    expect(taken(o, '10:00')).toBe(true);
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('whether a window is occupied — spans crossing midnight', () => {
  // THE ONE THIS MODULE EXISTS FOR. Comparing minutes of the day, a booking
  // from 23:00 to 01:00 asks `slotMin >= 1380 && slotMin < 60`, which no minute
  // satisfies — the booking vanishes and its hours read as free.
  it('is taken by a booking that runs past midnight', () => {
    const overnight = makeBooking({
      court_id: COURT,
      date: DAY,
      start_time: '23:00',
      duration_minutes: 120,
      ...venueSpan(DAY, '23:00', 120), // 23:00 to 01:00 the next day
    });
    const o = courtObstacles([overnight], [], COURT, DAY);

    expect(taken(o, '23:00')).toBe(true);
    expect(taken(o, '23:30')).toBe(true);
  });

  // The other half of the same booking, seen from the day it ends on.
  it("is still taken the next morning by last night's booking", () => {
    const overnight = makeBooking({
      court_id: COURT,
      date: DAY,
      start_time: '23:00',
      duration_minutes: 120,
      ...venueSpan(DAY, '23:00', 120),
    });
    const nextDay = '2026-03-19';
    const o = courtObstacles([overnight], [], COURT, nextDay);

    expect(taken(o, '00:00', nextDay)).toBe(true);
    expect(taken(o, '00:30', nextDay)).toBe(true);
    expect(taken(o, '01:00', nextDay)).toBe(false);
  });

  it('is taken by a blocked slot', () => {
    const o = courtObstacles([], [blocked('14:00', '15:00')], COURT, DAY);

    expect(taken(o, '14:00')).toBe(true);
    expect(taken(o, '15:00')).toBe(false);
  });
});

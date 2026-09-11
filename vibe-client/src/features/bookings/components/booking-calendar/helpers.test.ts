import { describe, it, expect } from 'vitest';
import {
  courtObstacles,
  fittingDurations as fittingIn,
  isSlotOccupied as occupiedIn,
  generateSlots,
  upcomingSlots,
} from './helpers';
import { venueInstant } from '@/shared/lib/instants';
import { timeToMinutes } from '@/shared/lib/time';

// Both questions are asked of a court's prepared obstacles, which the column
// computes once. The tests state the day the way a caller thinks about it —
// these bookings, these blocks, this court — and prepare it on the way in.
function fittingDurations(bookings: Booking[], blocked: BlockedSlot[], courtId: string, slot: string, date: string) {
  return fittingIn(courtObstacles(bookings, blocked, courtId, date), slot, date);
}

function isSlotOccupied(bookings: Booking[], blocked: BlockedSlot[], courtId: string, slot: string, date: string) {
  return occupiedIn(courtObstacles(bookings, blocked, courtId, date), slot, date);
}
import type { Booking, BlockedSlot } from '@/shared/types/api.types';

const COURT = 'court-1';

/** The day every fixture below sits on, and the day the helpers are asked about. */
const DAY = '2026-08-28';

/**
 * The instant a wall-clock reading names, in the venue's own zone — never
 * the test runner's. `courtObstacles`/`fittingDurations`/`isSlotOccupied`
 * all compare against spans built by `spanOnDay`, which reads the venue's
 * midnight; a fixture built from the runner's own zone would name a
 * different real moment on any machine or CI runner not itself set to
 * Argentina time, and the overlap assertions below would be wrong.
 */
function at(date: string, hhmm: string): string {
  const instant = venueInstant(date, timeToMinutes(hhmm));
  if (!instant) throw new Error(`invalid fixture date/time: ${date} ${hhmm}`);
  return instant;
}

/**
 * A booking on DAY. `startDay`/`endDay` name the calendar day each end falls
 * on, so a fixture can describe one that leaves the day being displayed —
 * which is the only kind these helpers used to get wrong.
 */
function booking(start: string, end: string, over: Partial<Booking> = {}, startDay = DAY, endDay = DAY): Booking {
  return {
    id: `b-${startDay}-${start}`,
    court_id: COURT,
    starts_at: at(startDay, start),
    ends_at: at(endDay, end),
    start_time: start,
    status: 'confirmed',
    ...over,
  } as Booking;
}

function blocked(start: string, end: string): BlockedSlot {
  return { id: `s-${start}`, court_id: COURT, start_time: start, end_time: end } as BlockedSlot;
}

describe('fittingDurations', () => {
  it('offers every duration when the rest of the day is free', () => {
    expect(fittingDurations([], [], COURT, '08:00', DAY)).toEqual([60, 90, 120]);
  });

  it('drops the durations that would run into the next booking', () => {
    // 08:00 + 120 would reach 10:00 and overlap; 60 and 90 still fit.
    expect(fittingDurations([booking('09:30', '11:00')], [], COURT, '08:00', DAY)).toEqual([60, 90]);
  });

  it('allows a duration that ends exactly when the next booking starts', () => {
    // Touching is not overlapping: 08:00–09:00 against a 09:00 start is fine.
    expect(fittingDurations([booking('09:00', '10:30')], [], COURT, '08:00', DAY)).toEqual([60]);
  });

  it('ignores cancelled bookings, which free their slot', () => {
    const cancelled = booking('09:00', '10:30', { status: 'cancelled' });
    expect(fittingDurations([cancelled], [], COURT, '08:00', DAY)).toEqual([60, 90, 120]);
  });

  it('ignores other courts', () => {
    const elsewhere = booking('09:00', '10:30', { court_id: 'court-2' });
    expect(fittingDurations([elsewhere], [], COURT, '08:00', DAY)).toEqual([60, 90, 120]);
  });

  it('is limited by a blocked slot the same way as by a booking', () => {
    expect(fittingDurations([], [blocked('09:00', '10:00')], COURT, '08:00', DAY)).toEqual([60]);
  });

  // These three asserted the opposite until the day a booking could end on the
  // following one. The server refused anything reaching midnight, so offering
  // it opened a form that could not be submitted; it now sells those hours
  // (a booking may cross midnight), and withholding them here would hide real availability
  // from the only screen that shows the whole day.
  it('offers the full night to a start late in the evening', () => {
    expect(fittingDurations([], [], COURT, '22:30', DAY)).toEqual([60, 90, 120]);
  });

  it('offers a duration that ends exactly at midnight', () => {
    expect(fittingDurations([], [], COURT, '23:00', DAY)).toEqual([60, 90, 120]);
  });

  it('offers a duration that runs into the next day', () => {
    expect(fittingDurations([], [], COURT, '23:30', DAY)).toEqual([60, 90, 120]);
  });

  it('returns nothing when not even the shortest duration fits', () => {
    expect(fittingDurations([booking('08:30', '10:00')], [], COURT, '08:00', DAY)).toEqual([]);
  });

  it('takes the nearest obstacle when several lie ahead', () => {
    const far = booking('12:00', '13:00');
    const near = booking('09:30', '10:00');
    expect(fittingDurations([far, near], [], COURT, '08:00', DAY)).toEqual([60, 90]);
  });

  // A booking already on the court still shortens what fits, whichever day it
  // started on. This is the bound that replaced the day's end.
  it('is still limited by an obstacle later the same night', () => {
    expect(
      fittingDurations([booking('00:30', '01:30', {}, '2026-08-29', '2026-08-29')], [], COURT, '23:30', DAY),
    ).toEqual([60]);
  });
});

describe('upcomingSlots', () => {
  const slots = ['08:00', '08:30', '09:00', '09:30'];

  it('keeps every slot on a date that is not today', () => {
    expect(upcomingSlots(slots, null)).toEqual(slots);
  });

  it('drops the slots already gone by today', () => {
    expect(upcomingSlots(slots, 9 * 60)).toEqual(['09:00', '09:30']);
  });

  it('keeps the slot starting exactly now', () => {
    expect(upcomingSlots(slots, 8 * 60 + 30)).toEqual(['08:30', '09:00', '09:30']);
  });

  it('drops everything once the day is over', () => {
    expect(upcomingSlots(slots, 23 * 60)).toEqual([]);
  });
});

describe('isSlotOccupied', () => {
  it('catches a booking that starts inside the slot', () => {
    // 09:00–09:30 is not free: sampling only the 09:00 instant would miss this.
    expect(isSlotOccupied([booking('09:15', '11:00')], [], COURT, '09:00', DAY)).toBe(true);
  });

  it('leaves a slot free when the booking starts where it ends', () => {
    expect(isSlotOccupied([booking('09:30', '11:00')], [], COURT, '09:00', DAY)).toBe(false);
  });

  it('leaves a slot free when the booking ends exactly as it starts', () => {
    expect(isSlotOccupied([booking('08:00', '09:00')], [], COURT, '09:00', DAY)).toBe(false);
  });

  // The reason this reads instants instead of clock strings. Compared as times
  // of day, "00:00" is below the booking's "23:00" start and the overlap test
  // never fires — so the hours somebody is playing on read as free, and the
  // grid offers them again.
  describe('a booking that ran past midnight', () => {
    const lastNight = booking('23:00', '01:00', {}, '2026-08-27', DAY);

    it('still holds the court at 00:00', () => {
      expect(isSlotOccupied([lastNight], [], COURT, '00:00', DAY)).toBe(true);
    });

    it('still holds it at 00:30', () => {
      expect(isSlotOccupied([lastNight], [], COURT, '00:30', DAY)).toBe(true);
    });

    it('releases it at 01:00, where it ends', () => {
      expect(isSlotOccupied([lastNight], [], COURT, '01:00', DAY)).toBe(false);
    });

    it('does not reach the evening it is named after', () => {
      expect(isSlotOccupied([lastNight], [], COURT, '23:00', DAY)).toBe(false);
    });
  });

  // The mirror image: a booking starting tonight and ending tomorrow occupies
  // tonight, and its tail is tomorrow's problem, not today's.
  it('holds the court for a booking that runs into tomorrow', () => {
    const tonight = booking('23:00', '01:00', {}, DAY, '2026-08-29');
    expect(isSlotOccupied([tonight], [], COURT, '23:00', DAY)).toBe(true);
    expect(isSlotOccupied([tonight], [], COURT, '23:30', DAY)).toBe(true);
    expect(isSlotOccupied([tonight], [], COURT, '00:00', DAY)).toBe(false);
  });
});

describe('generateSlots', () => {
  it('steps every half hour, so a 90-minute booking can be followed', () => {
    expect(generateSlots('08:00', '10:00')).toEqual(['08:00', '08:30', '09:00', '09:30']);
  });

  it('drops a slot that would run past closing', () => {
    expect(generateSlots('08:00', '09:20')).toEqual(['08:00', '08:30']);
  });

  it('spans the whole day when generating owner-dashboard slots', () => {
    const slots = generateSlots('00:00', '24:00');
    expect(slots[0]).toBe('00:00');
    expect(slots.at(-1)).toBe('23:30');
    expect(slots).toHaveLength(48);
  });
});

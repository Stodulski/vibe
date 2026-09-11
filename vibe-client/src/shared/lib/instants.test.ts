import { startOfDay, minutesInto, overlaps, toSpan, spanOnDay, MINUTES_PER_DAY } from './instants';

describe('startOfDay', () => {
  // The venue's midnight, not the runtime's. Asserted as an instant rather
  // than through local getters — a getter reads back through the *runtime's*
  // zone, which would pass on a machine set to Argentina time and hide the
  // exact bug this function was rewritten for everywhere else.
  it('is the venue midnight for the calendar day, regardless of the runtime zone', () => {
    expect(startOfDay('2026-08-28')).toEqual(new Date('2026-08-28T03:00:00Z'));
  });

  // The API sends a calendar date as `YYYY-MM-DDT00:00:00Z`; only the date part
  // is a calendar day, and the rest must not shift it.
  it('accepts the API shape without shifting the day', () => {
    expect(startOfDay('2026-08-28T00:00:00Z')).toEqual(new Date('2026-08-28T03:00:00Z'));
  });

  it('refuses a string that is not a calendar date', () => {
    expect(startOfDay('not a date')).toBeNull();
    expect(startOfDay('')).toBeNull();
  });
});

describe('minutesInto', () => {
  it('measures from the venue midnight of the day it is asked about, regardless of the runtime zone', () => {
    expect(minutesInto('2026-08-28', new Date('2026-08-28T13:00:00Z'))).toBe(600);
  });

  // The two that make a cross-midnight booking drawable. Folding these back
  // into 0..1440 is what put a booking in the wrong place on the grid.
  it('goes negative for an instant on the previous day', () => {
    expect(minutesInto('2026-08-28', new Date('2026-08-27T23:00:00-03:00'))).toBe(-60);
  });

  it('exceeds a full day for an instant on the next one', () => {
    expect(minutesInto('2026-08-28', new Date('2026-08-29T01:00:00-03:00'))).toBe(MINUTES_PER_DAY + 60);
  });

  it('refuses an invalid instant rather than returning NaN', () => {
    expect(minutesInto('2026-08-28', new Date('nonsense'))).toBeNull();
  });
});

describe('overlaps', () => {
  const span = (from: string, to: string) => ({
    start: new Date(from),
    end: new Date(to),
  });

  it('detects a plain overlap', () => {
    const a = span('2026-08-28T10:00:00', '2026-08-28T11:00:00');
    const b = span('2026-08-28T10:30:00', '2026-08-28T11:30:00');
    expect(overlaps(a.start, a.end, b.start, b.end)).toBe(true);
  });

  it('treats touching as not overlapping', () => {
    const a = span('2026-08-28T10:00:00', '2026-08-28T11:00:00');
    const b = span('2026-08-28T11:00:00', '2026-08-28T12:00:00');
    expect(overlaps(a.start, a.end, b.start, b.end)).toBe(false);
  });

  // The case the string comparison got backwards: no clock reading is both
  // after 23:00 and before 01:00, so the booking matched nothing at all.
  it('sees a slot inside a booking that crossed midnight', () => {
    const booking = span('2026-08-27T23:00:00', '2026-08-28T01:00:00');
    const slot = span('2026-08-28T00:00:00', '2026-08-28T01:00:00');
    expect(overlaps(slot.start, slot.end, booking.start, booking.end)).toBe(true);
  });

  it('leaves the slot after such a booking free', () => {
    const booking = span('2026-08-27T23:00:00', '2026-08-28T01:00:00');
    const slot = span('2026-08-28T01:00:00', '2026-08-28T02:00:00');
    expect(overlaps(slot.start, slot.end, booking.start, booking.end)).toBe(false);
  });
});

describe('toSpan', () => {
  it('reads the pair the API sends', () => {
    const s = toSpan('2026-08-27T23:00:00Z', '2026-08-28T01:00:00Z');
    expect(s?.end.getTime()).toBeGreaterThan(s?.start.getTime() ?? 0);
  });

  it('refuses an unparseable pair', () => {
    expect(toSpan('nonsense', '2026-08-28T01:00:00Z')).toBeNull();
    expect(toSpan('2026-08-27T23:00:00Z', '')).toBeNull();
  });
});

describe('spanOnDay', () => {
  it('places two times of day on the venue midnight, regardless of the runtime zone', () => {
    const s = spanOnDay('2026-08-28', 18 * 60, 19 * 60 + 30);
    expect(s?.start).toEqual(new Date('2026-08-28T21:00:00Z'));
    expect(s?.end).toEqual(new Date('2026-08-28T22:30:00Z'));
  });

  it('rolls into the next day rather than wrapping', () => {
    const s = spanOnDay('2026-08-28', 23 * 60, MINUTES_PER_DAY + 60);
    expect(s?.end).toEqual(new Date('2026-08-29T04:00:00Z'));
  });
});

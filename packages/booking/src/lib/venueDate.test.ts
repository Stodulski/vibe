import { describe, expect, it } from 'vitest';
import { lateNightGridDay, todayInVenueTz, venueDateString } from './venueDate';

describe('venueDateString', () => {
  it('is the previous day in Argentina at 02:30 UTC (Argentina is UTC-3)', () => {
    expect(venueDateString(new Date('2026-10-11T02:30:00Z'))).toBe('2026-10-10');
  });

  it('turns to the next day at 03:00 UTC', () => {
    expect(venueDateString(new Date('2026-10-11T03:00:00Z'))).toBe('2026-10-11');
  });

  it('stays on the same day late in the Argentine evening', () => {
    expect(venueDateString(new Date('2026-10-10T23:59:59Z'))).toBe('2026-10-10');
  });

  it('accepts another zone', () => {
    expect(venueDateString(new Date('2026-10-10T12:00:00Z'), 'Pacific/Auckland')).toBe('2026-10-11');
  });
});

describe('todayInVenueTz', () => {
  it('is local midnight of the venue date, whatever the process zone', () => {
    const today = todayInVenueTz(new Date('2026-10-11T02:30:00Z'));
    expect([today.getFullYear(), today.getMonth(), today.getDate()]).toEqual([2026, 9, 10]);
    expect(today.getHours()).toBe(0);
  });
});

describe('lateNightGridDay', () => {
  const schedule = (day: string, open: string, close: string, isClosed = false) => ({
    day,
    open_time: open,
    close_time: close,
    is_closed: isClosed,
  });
  // 2026-10-15 is a Thursday in Argentina; the night before is Wednesday's grid.
  const at = (hhmm: string) => new Date(`2026-10-15T${hhmm}:00-03:00`);

  it("is yesterday while yesterday's window still runs past midnight", () => {
    const day = lateNightGridDay(at('00:10'), [schedule('wednesday', '20:00', '02:00')]);
    expect(day && [day.getFullYear(), day.getMonth(), day.getDate()]).toEqual([2026, 9, 14]);
  });

  it("is null once yesterday's window has closed", () => {
    expect(lateNightGridDay(at('02:00'), [schedule('wednesday', '20:00', '02:00')])).toBeNull();
  });

  it('is null for a window that ends by midnight', () => {
    expect(lateNightGridDay(at('00:10'), [schedule('wednesday', '08:00', '23:00')])).toBeNull();
    expect(lateNightGridDay(at('00:10'), [schedule('wednesday', '08:00', '00:00')])).toBeNull();
  });

  it('is null when yesterday was closed or has no schedule', () => {
    expect(lateNightGridDay(at('00:10'), [schedule('wednesday', '20:00', '02:00', true)])).toBeNull();
    expect(lateNightGridDay(at('00:10'), [schedule('thursday', '20:00', '02:00')])).toBeNull();
  });
});

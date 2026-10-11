import { describe, expect, it } from 'vitest';
import { format } from 'date-fns/format';
import { todayInVenueTz, venueDateString, VENUE_TIME_ZONE } from './venueDate';

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

  it('reads the venue zone by default and accepts another zone', () => {
    expect(VENUE_TIME_ZONE).toBe('America/Argentina/Buenos_Aires');
    expect(venueDateString(new Date('2026-10-10T12:00:00Z'), 'Pacific/Auckland')).toBe('2026-10-11');
  });
});

describe('todayInVenueTz', () => {
  it('is local midnight of the venue date, whatever the process zone', () => {
    const today = todayInVenueTz(new Date('2026-10-11T02:30:00Z'));
    expect([today.getFullYear(), today.getMonth(), today.getDate()]).toEqual([2026, 9, 10]);
    expect(today.getHours()).toBe(0);
  });

  it('formats to the venue date', () => {
    expect(format(todayInVenueTz(new Date('2026-10-11T02:30:00Z')), 'yyyy-MM-dd')).toBe('2026-10-10');
  });
});

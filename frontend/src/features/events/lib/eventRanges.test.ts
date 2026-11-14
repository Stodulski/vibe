import { describe, it, expect } from 'vitest';
import { pastRange, upcomingRange } from './eventRanges';

describe('event list ranges', () => {
  it('upcoming runs from today for a year, inside the 366 days the API accepts', () => {
    expect(upcomingRange('2026-11-14')).toEqual({ from: '2026-11-14', to: '2027-11-14' });
  });

  it('past ends yesterday and reaches back a year', () => {
    expect(pastRange('2026-11-14')).toEqual({ from: '2025-11-14', to: '2026-11-13' });
  });

  it('both stay within 366 days across a leap year', () => {
    for (const { from, to } of [upcomingRange('2027-11-14'), pastRange('2028-03-01')]) {
      const days = (Date.parse(to) - Date.parse(from)) / 86_400_000;
      expect(days).toBeLessThanOrEqual(366);
    }
  });
});

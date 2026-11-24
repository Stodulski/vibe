// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { makeCourt, makePrice } from '@/test/factories';
import { uncoveredOpeningHours } from './priceCoverage';

const monday = (openTime: string, closeTime: string) => ({
  day: 'monday' as const,
  open_time: openTime,
  close_time: closeTime,
  is_closed: false,
});
const court = (overrides: Parameters<typeof makeCourt>[0] = {}) => ({
  ...makeCourt(overrides),
  prices: [makePrice({ day_type: 'monday', time_from: '08:00', time_to: '23:00' })],
});

describe('uncoveredOpeningHours', () => {
  it('reports the hours an earlier opening adds before the first band', () => {
    expect(uncoveredOpeningHours([court()], [monday('07:00', '23:00')])).toEqual([
      { courtId: 'ct1', courtName: 'Cancha 1', day: 'monday', from: '07:00', to: '08:00' },
    ]);
  });

  // A band that starts before the window still covers the window's hours.
  it('reports nothing when the window shrinks inside the bands', () => {
    expect(uncoveredOpeningHours([court()], [monday('09:00', '22:00')])).toEqual([]);
  });

  // An inactive court is not offered anyway: warning about it is noise.
  it('skips inactive courts', () => {
    expect(uncoveredOpeningHours([court({ is_active: false })], [monday('07:00', '01:00')])).toEqual([]);
  });
});

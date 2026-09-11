import { rateAt, findTotalPrice } from './pricing';
import { makeCourt, makePrice } from '@/test/factories';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

/** A week of identical hours, so a test not about the schedule need not state one. */
function openEveryDay(open: string, close: string): Schedule[] {
  const days = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'] as const;
  return days.map((day, i) => ({
    id: `s${String(i)}`,
    complex_id: 'c1',
    day,
    open_time: open,
    close_time: close,
    is_closed: false,
  }));
}

function courtWith(prices: CourtWithPrices['prices']): CourtWithPrices {
  return { ...makeCourt({ id: 'ct1' }), prices };
}

// 2026-08-28 is a Friday.
const FRIDAY = '2026-08-28';

describe('rateAt', () => {
  const prices = [
    makePrice({ day_type: 'friday', time_from: '08:00', time_to: '18:00', price: 10000 }),
    makePrice({ day_type: 'friday', time_from: '18:00', time_to: '02:00', price: 20000 }),
  ];

  it('returns the band covering the minute', () => {
    expect(rateAt(prices, 'friday', 9 * 60)).toBe(10000);
    expect(rateAt(prices, 'friday', 19 * 60)).toBe(20000);
  });

  // The band running to 02:00 is stored as minutes 1080 to 1560, so the hours
  // past the rollover are inside it. Compared as clock strings its end would
  // sit before its start and it would cover nothing at all.
  it('covers the hours past midnight of a band that wraps', () => {
    expect(rateAt(prices, 'friday', 24 * 60 + 30)).toBe(20000);
    expect(rateAt(prices, 'friday', 24 * 60 + 90)).toBe(20000);
  });

  it('is half-open at the end', () => {
    expect(rateAt(prices, 'friday', 24 * 60 + 120)).toBeNull();
  });

  it('has no fallback to another weekday', () => {
    expect(rateAt(prices, 'saturday', 9 * 60)).toBeNull();
  });
});

describe('findTotalPrice', () => {
  const schedules = openEveryDay('08:00', '02:00');
  const court = courtWith([
    makePrice({ day_type: 'friday', time_from: '08:00', time_to: '18:00', price: 10000 }),
    makePrice({ day_type: 'friday', time_from: '18:00', time_to: '02:00', price: 20000 }),
    makePrice({ day_type: 'saturday', time_from: '08:00', time_to: '02:00', price: 90000 }),
  ]);

  it('sums the hourly rates across blocks and halves once', () => {
    // 09:00 for 60 minutes: two blocks at 10000/hour.
    expect(findTotalPrice([court], 'ct1', FRIDAY, '09:00', 60, schedules)).toBe(10000);
  });

  it('splits across a band boundary inside the day', () => {
    // 17:00 for 120: 17:00 and 17:30 off-peak, 18:00 and 18:30 peak.
    expect(findTotalPrice([court], 'ct1', FRIDAY, '17:00', 120, schedules)).toBe((10000 + 10000 + 20000 + 20000) / 2);
  });

  // The rule the product chose: the whole session is priced by the window it
  // was sold out of. A Friday night running to 01:00 is Friday's, all of it —
  // Saturday's card is not consulted even though the calendar has rolled.
  it('prices a session that runs past midnight at the window day', () => {
    expect(findTotalPrice([court], 'ct1', FRIDAY, '23:00', 120, schedules)).toBe((20000 * 4) / 2);
  });

  // The tie-break, and the only case that tells the two rules apart. Saturday
  // 00:30 is inside Friday's window, which has not closed — so it is Friday's
  // hour at Friday's rate, not Saturday's. Pricing it by the calendar day would
  // charge 90000 here, and the server would charge 20000.
  it('prices the small hours at the night that sold them, not the calendar day', () => {
    const saturday = '2026-08-29';
    expect(findTotalPrice([court], 'ct1', saturday, '00:30', 60, schedules)).toBe(20000);
  });

  // Once that night has closed, the same clock reading is the new day's.
  it('prices past the previous night close at the day that has opened', () => {
    const saturday = '2026-08-29';
    expect(findTotalPrice([court], 'ct1', saturday, '09:00', 60, schedules)).toBe(90000);
  });

  it('refuses when the band stops short of the window', () => {
    const short = courtWith([makePrice({ day_type: 'friday', time_from: '08:00', time_to: '23:59', price: 10000 })]);
    expect(findTotalPrice([short], 'ct1', FRIDAY, '23:00', 120, schedules)).toBeNull();
  });

  it('returns null before a court and a time are chosen', () => {
    expect(findTotalPrice([court], '', FRIDAY, '09:00', 60, schedules)).toBeNull();
    expect(findTotalPrice([court], 'ct1', FRIDAY, '', 60, schedules)).toBeNull();
  });
});

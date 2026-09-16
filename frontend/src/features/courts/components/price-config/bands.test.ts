// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { makePrice } from '@/test/factories';
import {
  bandField,
  buildDayBands,
  dayPriceField,
  dayPriceValues,
  endsNextDay,
  nextDifferentiatedBand,
  openingBandFor,
  priceFormValues,
} from './bands';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

function schedule(overrides: Partial<Schedule>): Schedule {
  return {
    id: 's1',
    complex_id: 'c1',
    day: 'monday',
    open_time: '08:00',
    close_time: '23:00',
    is_closed: false,
    ...overrides,
  };
}

function court(prices: CourtWithPrices['prices'] = []): CourtWithPrices {
  return {
    id: 'ct1',
    complex_id: 'c1',
    name: 'Cancha 1',
    sport: 'padel',
    court_type: 'outdoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices,
  };
}

describe('openingBandFor', () => {
  it("uses the day's own opening window", () => {
    expect(openingBandFor([schedule({ day: 'monday', open_time: '09:00', close_time: '22:00' })], 'monday')).toEqual({
      time_from: '09:00',
      time_to: '22:00',
    });
  });

  // A closed day still gets a window: staff book days the venue is shut, and
  // the rate they are charged comes from that day's card.
  it('covers the calendar day for a closed, missing or zero-length day', () => {
    const wholeDay = { time_from: '00:00', time_to: '23:59' };
    expect(openingBandFor([schedule({ day: 'sunday', is_closed: true })], 'sunday')).toEqual(wholeDay);
    expect(openingBandFor([], 'sunday')).toEqual(wholeDay);
    expect(openingBandFor([schedule({ day: 'sunday', open_time: '08:00', close_time: '08:00' })], 'sunday')).toEqual(
      wholeDay,
    );
  });

  // Sent as-is: the server reads an end at or before the start as "ends the
  // next day", so a window built straight from the schedule is already the
  // shape it expects.
  it('keeps a window that runs past midnight intact', () => {
    expect(
      openingBandFor([schedule({ day: 'thursday', open_time: '08:00', close_time: '01:30' })], 'thursday'),
    ).toEqual({ time_from: '08:00', time_to: '01:30' });
  });
});

describe('endsNextDay', () => {
  it('is true for a band whose end reads at or before its start', () => {
    expect(endsNextDay({ time_from: '22:00', time_to: '01:30' })).toBe(true);
    expect(endsNextDay({ time_from: '10:00', time_to: '10:00' })).toBe(true);
  });

  it('is false for an ordinary band', () => {
    expect(endsNextDay({ time_from: '08:00', time_to: '23:00' })).toBe(false);
  });
});

describe('nextDifferentiatedBand', () => {
  const schedules = [schedule({ day: 'friday', open_time: '08:00', close_time: '23:00' })];

  it("claims the window's last hour when the day has no rows yet", () => {
    expect(nextDifferentiatedBand([], schedules, 'friday')).toEqual({
      time_from: '22:00',
      time_to: '23:00',
      price: Number.NaN,
    });
  });

  it('grows backward from whichever existing row starts earliest', () => {
    expect(nextDifferentiatedBand([{ time_from: '22:00', time_to: '23:00', price: 200 }], schedules, 'friday')).toEqual(
      { time_from: '21:00', time_to: '22:00', price: Number.NaN },
    );

    expect(
      nextDifferentiatedBand(
        [
          { time_from: '21:00', time_to: '22:00', price: 200 },
          { time_from: '22:00', time_to: '23:00', price: 250 },
        ],
        schedules,
        'friday',
      ),
    ).toEqual({ time_from: '20:00', time_to: '21:00', price: Number.NaN });
  });

  it('does not suggest hours before the window opens', () => {
    // The earliest row starts only 30 minutes after opening — an hour before
    // it would read "07:30", before the venue is even open — so the new
    // row's start clamps to the window's own open time instead.
    expect(nextDifferentiatedBand([{ time_from: '08:30', time_to: '09:00', price: 200 }], schedules, 'friday')).toEqual(
      { time_from: '08:00', time_to: '08:30', price: Number.NaN },
    );
  });
});

describe('buildDayBands', () => {
  const schedules = [schedule({ day: 'friday', open_time: '08:00', close_time: '23:00' })];

  it('sends the plain single band, byte-identical to the pre-feature shape, when there are no rows', () => {
    expect(buildDayBands({ price: 150, bands: [] }, schedules, 'friday')).toEqual([
      { time_from: '08:00', time_to: '23:00', price: 150, source: { kind: 'day' } },
    ]);
  });

  it('fills the gap before and after a single row at the full-day rate', () => {
    const built = buildDayBands(
      { price: 100, bands: [{ time_from: '20:00', time_to: '23:00', price: 200 }] },
      schedules,
      'friday',
    );
    expect(built).toEqual([
      { time_from: '08:00', time_to: '20:00', price: 100, source: { kind: 'day' } },
      { time_from: '20:00', time_to: '23:00', price: 200, source: { kind: 'band', index: 0 } },
    ]);
  });

  it('fills only the gap between two rows that already reach both edges', () => {
    const built = buildDayBands(
      {
        price: 999,
        bands: [
          { time_from: '08:00', time_to: '12:00', price: 100 },
          { time_from: '20:00', time_to: '23:00', price: 200 },
        ],
      },
      schedules,
      'friday',
    );
    expect(built).toEqual([
      { time_from: '08:00', time_to: '12:00', price: 100, source: { kind: 'band', index: 0 } },
      { time_from: '12:00', time_to: '20:00', price: 999, source: { kind: 'day' } },
      { time_from: '20:00', time_to: '23:00', price: 200, source: { kind: 'band', index: 1 } },
    ]);
  });

  it('deleting every row leaves exactly the single full-day band again', () => {
    expect(buildDayBands({ price: 150, bands: [] }, schedules, 'friday')).toEqual(
      buildDayBands({ price: 150, bands: [] }, schedules, 'friday'),
    );
  });

  // A row entered as "00:30–01:30" belongs in the wrapped, early-morning
  // portion of an 08:00→01:30 window, not before it — anchored the same way
  // the schema's own `bandSpan` extends a band's own end past midnight.
  it('anchors a row inside a window that itself runs past midnight', () => {
    const overnight = [schedule({ day: 'thursday', open_time: '08:00', close_time: '01:30' })];
    const built = buildDayBands(
      { price: 100, bands: [{ time_from: '00:30', time_to: '01:30', price: 200 }] },
      overnight,
      'thursday',
    );
    expect(built).toEqual([
      { time_from: '08:00', time_to: '00:30', price: 100, source: { kind: 'day' } },
      { time_from: '00:30', time_to: '01:30', price: 200, source: { kind: 'band', index: 0 } },
    ]);
  });
});

describe('dayPriceValues / priceFormValues', () => {
  it('gives a court with no stored prices a blank full-day price and no rows', () => {
    const values = dayPriceValues(court(), 'monday');
    expect(values).toEqual({ price: Number.NaN, bands: [] });
  });

  it('reads a single stored band as the plain full-day price', () => {
    const values = dayPriceValues(court([makePrice({ day_type: 'friday', price: 15000 })]), 'friday');
    expect(values).toEqual({ price: 150, bands: [] });
  });

  // No tag on the wire says which stored band was "the base rate" — the
  // longest-covering one becomes it, and the shorter one becomes a row, kept
  // at its own stored hours.
  it('reconstructs the longest stored band as the full-day price and the rest as rows', () => {
    const values = dayPriceValues(
      court([
        makePrice({ day_type: 'friday', time_from: '08:00', time_to: '20:00', price: 15000 }),
        makePrice({ day_type: 'friday', time_from: '20:00', time_to: '23:00', price: 25000 }),
      ]),
      'friday',
    );
    expect(values).toEqual({ price: 150, bands: [{ time_from: '20:00', time_to: '23:00', price: 250 }] });
  });

  it('gives every weekday an entry, keeping each day to itself', () => {
    const values = priceFormValues(
      court([
        makePrice({ day_type: 'monday', time_from: '08:00', time_to: '23:00', price: 10000 }),
        makePrice({ day_type: 'saturday', time_from: '08:00', time_to: '23:00', price: 15000 }),
      ]),
    );

    expect(Object.keys(values)).toHaveLength(7);
    expect(values.monday).toEqual({ price: 100, bands: [] });
    expect(values.saturday).toEqual({ price: 150, bands: [] });
    expect(values.tuesday).toEqual({ price: Number.NaN, bands: [] });
  });
});

describe('bandField / dayPriceField', () => {
  it("builds the react-hook-form path for a row's field", () => {
    expect(bandField('thursday', 2, 'time_from')).toBe('thursday.bands.2.time_from');
  });

  it("builds the react-hook-form path for a day's full-day price", () => {
    expect(dayPriceField('thursday')).toBe('thursday.price');
  });
});

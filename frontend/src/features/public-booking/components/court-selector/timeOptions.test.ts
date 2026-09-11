import { buildTimeOptions, groupTimeOptions } from './timeOptions';
import { needsCourtChoice } from './courtChoice';
import type { AvailabilitySlot, CourtAvailability } from '@/shared/types/api.types';

/**
 * `startMin` defaults to the clock reading, which is what it equals for a
 * venue that closes before midnight. Overnight cases pass it explicitly —
 * that divergence is the whole point of the field.
 */
function slot(start: string, price: number, available = true, startMin?: number): AvailabilitySlot {
  const [h = '0', m = '0'] = start.split(':');
  return {
    start_time: start,
    end_time: `${start}-end`,
    start_min: startMin ?? Number(h) * 60 + Number(m),
    duration_minutes: 90,
    price,
    available,
  };
}

function court(id: string, type: string, slots: AvailabilitySlot[]): CourtAvailability {
  return {
    court_id: id,
    court_name: `Cancha ${id}`,
    sport: 'padel',
    court_type: type,
    duration_minutes: 90,
    slots,
  };
}

describe('buildTimeOptions', () => {
  it('collapses the same hour across courts into one option', () => {
    const options = buildTimeOptions([
      court('1', 'indoor', [slot('20:00', 5000)]),
      court('2', 'indoor', [slot('20:00', 5000)]),
      court('3', 'indoor', [slot('20:00', 5000)]),
    ]);

    // Three courts, one hour — one button, not three.
    expect(options).toHaveLength(1);
    expect(options[0]?.courts).toHaveLength(3);
  });

  it('drops hours nobody can book', () => {
    const options = buildTimeOptions([court('1', 'indoor', [slot('08:00', 5000), slot('09:30', 5000, false)])]);

    expect(options.map((o) => o.startTime)).toEqual(['08:00']);
  });

  it('keeps an hour that any one court still has free', () => {
    const options = buildTimeOptions([
      court('1', 'indoor', [slot('20:00', 5000, false)]),
      court('2', 'indoor', [slot('20:00', 5000)]),
    ]);

    expect(options).toHaveLength(1);
    expect(options[0]?.courts).toHaveLength(1);
  });

  it('orders hours by time regardless of the order courts arrived in', () => {
    const options = buildTimeOptions([
      court('1', 'indoor', [slot('22:00', 5000), slot('08:00', 5000)]),
      court('2', 'indoor', [slot('14:00', 5000)]),
    ]);

    expect(options.map((o) => o.startTime)).toEqual(['08:00', '14:00', '22:00']);
  });

  it('reports the cheapest price and that prices differ', () => {
    // `court_prices` hangs off the court, so a venue may charge more for the
    // covered one at the same hour.
    const options = buildTimeOptions([
      court('1', 'indoor', [slot('20:00', 8000)]),
      court('2', 'outdoor', [slot('20:00', 6000)]),
    ]);

    expect(options[0]?.minPrice).toBe(6000);
    expect(options[0]?.priceVaries).toBe(true);
  });

  it('reports no variation when the courts agree', () => {
    const options = buildTimeOptions([
      court('1', 'indoor', [slot('20:00', 6000)]),
      court('2', 'outdoor', [slot('20:00', 6000)]),
    ]);

    expect(options[0]?.priceVaries).toBe(false);
  });
});

describe('buildTimeOptions — venues trading past midnight', () => {
  it('keeps the hours in window order', () => {
    // A club open 20:00-01:00 publishes its closing hours as "00:00" and
    // "00:30" — `slots.FromMinutes` takes the minute modulo a day to render a
    // clock face. Sorted as strings those land ABOVE 20:00, so the page would
    // open on the end of the night. `start_min` does not wrap.
    const options = buildTimeOptions([
      court('1', 'indoor', [
        slot('00:30', 5000, true, 1470),
        slot('20:00', 5000, true, 1200),
        slot('00:00', 5000, true, 1440),
      ]),
    ]);

    expect(options.map((o) => o.startTime)).toEqual(['20:00', '00:00', '00:30']);
  });

  it('files past-midnight hours under the night they close, not the morning', () => {
    const grouped = groupTimeOptions(
      buildTimeOptions([court('1', 'indoor', [slot('20:00', 1, true, 1200), slot('00:30', 1, true, 1470)])]),
    );

    expect(grouped.get('evening')?.map((o) => o.startTime)).toEqual(['20:00', '00:30']);
    expect(grouped.get('morning')).toBeUndefined();
  });
});

describe('groupTimeOptions', () => {
  it('buckets hours into parts of the day', () => {
    const grouped = groupTimeOptions(
      buildTimeOptions([court('1', 'indoor', [slot('08:00', 1), slot('14:00', 1), slot('20:00', 1)])]),
    );

    expect(grouped.get('morning')?.map((o) => o.startTime)).toEqual(['08:00']);
    expect(grouped.get('afternoon')?.map((o) => o.startTime)).toEqual(['14:00']);
    expect(grouped.get('evening')?.map((o) => o.startTime)).toEqual(['20:00']);
  });
});

describe('needsCourtChoice', () => {
  function entriesFor(courts: CourtAvailability[]) {
    const [option] = buildTimeOptions(courts);
    if (!option) throw new Error('fixture must produce one time option');
    return option.courts;
  }

  it('asks nothing when a single court is free', () => {
    expect(needsCourtChoice(entriesFor([court('1', 'indoor', [slot('20:00', 5000)])]))).toBe(false);
  });

  it('asks nothing when the free courts match on type and price', () => {
    const entries = entriesFor([
      court('1', 'indoor', [slot('20:00', 5000)]),
      court('2', 'indoor', [slot('20:00', 5000)]),
    ]);

    expect(needsCourtChoice(entries)).toBe(false);
  });

  it('asks when the type differs', () => {
    const entries = entriesFor([
      court('1', 'indoor', [slot('20:00', 5000)]),
      court('2', 'outdoor', [slot('20:00', 5000)]),
    ]);

    expect(needsCourtChoice(entries)).toBe(true);
  });

  it('asks when only the price differs', () => {
    const entries = entriesFor([
      court('1', 'outdoor', [slot('20:00', 8000)]),
      court('2', 'outdoor', [slot('20:00', 6000)]),
    ]);

    expect(needsCourtChoice(entries)).toBe(true);
  });
});

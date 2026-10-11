import { describe, expect, it } from 'vitest';
import type { CourtPrice, CourtWithPrices, Sport, Weekday } from '../api/types';
import { priceRange, sportSummaries } from './courtPrices';

function band(price: number, day: Weekday = 'monday'): CourtPrice {
  return {
    id: `band-${String(price)}-${day}`,
    court_id: 'court',
    price,
    day_type: day,
    time_from: '08:00',
    time_to: '12:00',
    from_min: 480,
    to_min: 720,
  };
}

function court(id: string, sport: Sport, prices: CourtPrice[], isActive = true): CourtWithPrices {
  return {
    id,
    complex_id: 'complex',
    name: `Cancha ${id}`,
    sport,
    court_type: 'indoor',
    is_active: isActive,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices,
  };
}

describe('sportSummaries', () => {
  it('counts the active courts of each sport, in the order each sport first appears', () => {
    const courts = [
      court('1', 'soccer', [band(2400000)]),
      court('2', 'padel', [band(1800000)]),
      court('3', 'soccer', [band(2100000)]),
    ];

    expect(sportSummaries(courts).map((s) => [s.sport, s.courts])).toEqual([
      ['soccer', 2],
      ['padel', 1],
    ]);
  });

  it('takes the cheapest priced hour across the courts of one sport', () => {
    const courts = [
      court('1', 'soccer', [band(2400000), band(3000000, 'saturday')]),
      court('2', 'soccer', [band(2100000)]),
    ];

    expect(sportSummaries(courts)).toEqual([{ sport: 'soccer', courts: 2, minPrice: 2100000 }]);
  });

  it('leaves out inactive courts entirely', () => {
    const courts = [court('1', 'padel', [band(1800000)]), court('2', 'padel', [band(900000)], false)];

    expect(sportSummaries(courts)).toEqual([{ sport: 'padel', courts: 1, minPrice: 1800000 }]);
  });

  it('has no minimum for a sport whose bands are all unpriced, so no "desde $0" is shown', () => {
    const courts = [court('1', 'tennis', []), court('2', 'tennis', [band(0)])];

    expect(sportSummaries(courts)).toEqual([{ sport: 'tennis', courts: 2, minPrice: null }]);
  });
});

describe('priceRange', () => {
  it('spans the cheapest and the dearest priced hour of the active courts', () => {
    const courts = [
      court('1', 'padel', [band(1800000)]),
      court('2', 'soccer', [band(3000000, 'saturday'), band(0)]),
      court('3', 'soccer', [band(900000)], false),
    ];

    expect(priceRange(courts)).toEqual({ min: 1800000, max: 3000000 });
  });

  it('is null when no active band is priced', () => {
    expect(priceRange([court('1', 'padel', [band(0)])])).toBeNull();
    expect(priceRange([])).toBeNull();
  });
});

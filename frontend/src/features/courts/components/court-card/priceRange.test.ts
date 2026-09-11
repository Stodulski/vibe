import { describe, it, expect } from 'vitest';
import { getPriceRange, formatPriceLabel } from './priceRange';
import { makePrice as makeCourtPrice } from '@/test/factories';
import type { CourtPrice } from '@/shared/types/api.types';

// Delegates so the band's minute span is filled the way a real one arrives;
// this file's own default is a 1000 price rather than the factory's.
function makePrice(overrides: Partial<CourtPrice> = {}): CourtPrice {
  return makeCourtPrice({ price: 1000, ...overrides });
}

describe('getPriceRange', () => {
  it('returns null ranges and hasPrices false when there are no prices', () => {
    expect(getPriceRange([])).toEqual({
      weekdayMin: null,
      weekdayMax: null,
      weekendMin: null,
      weekendMax: null,
      hasPrices: false,
    });
  });

  it('computes separate weekday and weekend min/max', () => {
    const prices = [
      makePrice({ day_type: 'monday', price: 1000 }),
      makePrice({ day_type: 'friday', price: 1500 }),
      makePrice({ day_type: 'saturday', price: 2000 }),
      makePrice({ day_type: 'sunday', price: 2500 }),
    ];
    expect(getPriceRange(prices)).toEqual({
      weekdayMin: 1000,
      weekdayMax: 1500,
      weekendMin: 2000,
      weekendMax: 2500,
      hasPrices: true,
    });
  });
});

describe('formatPriceLabel', () => {
  const format = (v: number) => `$${String(v)}`;

  it('returns an em dash when there is no price', () => {
    expect(formatPriceLabel(null, null, format)).toBe('—');
  });

  it('returns a single formatted value when min equals max', () => {
    expect(formatPriceLabel(1000, 1000, format)).toBe('$1000');
  });

  it('returns a range when min and max differ', () => {
    expect(formatPriceLabel(1000, 1500, format)).toBe('$1000 - $1500');
  });
});

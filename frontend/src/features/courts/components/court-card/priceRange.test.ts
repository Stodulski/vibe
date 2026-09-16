import { describe, it, expect } from 'vitest';
import { getPriceRange, getOverallPriceRange, formatPriceLabel } from './priceRange';
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

  // A day priced per time band contributes several rows, and the card's range
  // has to open up to cover all of them — showing "$1000" for a Friday that
  // also charges $3000 after seven understates what the court costs. This is a
  // min/max over rows rather than over days precisely so the count per day
  // does not matter, but nothing pinned that until bands could be edited.
  it('spans every band of a day, not just the first', () => {
    const prices = [
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '14:00', price: 1000 }),
      makePrice({ day_type: 'friday', time_from: '14:00', time_to: '19:00', price: 2000 }),
      makePrice({ day_type: 'friday', time_from: '19:00', time_to: '23:00', price: 3000 }),
      makePrice({ day_type: 'saturday', time_from: '08:00', time_to: '19:00', price: 2500 }),
      makePrice({ day_type: 'saturday', time_from: '19:00', time_to: '23:00', price: 4000 }),
    ];
    expect(getPriceRange(prices)).toEqual({
      weekdayMin: 1000,
      weekdayMax: 3000,
      weekendMin: 2500,
      weekendMax: 4000,
      hasPrices: true,
    });
  });
});

describe('getOverallPriceRange', () => {
  it('reports no prices for an empty list', () => {
    expect(getOverallPriceRange([])).toEqual({ min: null, max: null, hasPrices: false });
  });

  it('collapses every band of every day into one range', () => {
    const prices = [
      makePrice({ day_type: 'friday', time_from: '08:00', time_to: '19:00', price: 1000 }),
      makePrice({ day_type: 'friday', time_from: '19:00', time_to: '23:00', price: 3000 }),
      makePrice({ day_type: 'sunday', time_from: '08:00', time_to: '23:00', price: 2000 }),
    ];
    expect(getOverallPriceRange(prices)).toEqual({ min: 1000, max: 3000, hasPrices: true });
  });

  it('reports a single figure when every band charges the same', () => {
    const prices = [
      makePrice({ day_type: 'monday', time_from: '08:00', time_to: '14:00', price: 1500 }),
      makePrice({ day_type: 'monday', time_from: '14:00', time_to: '23:00', price: 1500 }),
    ];
    expect(getOverallPriceRange(prices)).toEqual({ min: 1500, max: 1500, hasPrices: true });
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

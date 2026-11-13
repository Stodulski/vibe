import { describe, it, expect } from 'vitest';
import { courtDetailsLine } from './courtDetails';
import { isSurfaceAllowed, surfacesForSport } from './surfaces';

describe('courtDetailsLine', () => {
  it('joins surface and lighting', () => {
    expect(courtDetailsLine({ surface: 'synthetic_grass', has_lighting: true })).toBe(
      'Césped sintético · Con iluminación',
    );
  });
  it('shows only what is known', () => {
    expect(courtDetailsLine({ surface: 'clay', has_lighting: false })).toBe('Polvo de ladrillo');
    expect(courtDetailsLine({ has_lighting: true })).toBe('Con iluminación');
    expect(courtDetailsLine({ surface: null, has_lighting: false })).toBe('');
  });
});

describe('surfaces by sport', () => {
  it('follows the product table', () => {
    expect(surfacesForSport('padel')).toEqual(['synthetic_grass', 'concrete']);
    expect(isSurfaceAllowed('hockey', 'clay')).toBe(false);
    expect(isSurfaceAllowed('volleyball', 'sand')).toBe(true);
  });
});

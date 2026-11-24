import { describe, expect, it } from 'vitest';
import { courtFactsLine } from './courtFacts';

describe('courtFactsLine', () => {
  it('always says the roof type', () => {
    expect(courtFactsLine({ court_type: 'semi_covered', has_lighting: false })).toBe('Semi cubierta');
  });

  it('adds the surface when set and the lighting when true, in that order', () => {
    expect(courtFactsLine({ court_type: 'indoor', surface: 'clay', has_lighting: true })).toBe(
      'Techada · Polvo de ladrillo · Con iluminación',
    );
  });

  it('shows a surface this build has no label for as sent', () => {
    expect(courtFactsLine({ court_type: 'outdoor', surface: 'ice', has_lighting: false })).toBe('Descubierta · ice');
  });

  it('says what it has when the roof type is not given, and nothing when nothing is known', () => {
    expect(courtFactsLine({ surface: 'synthetic_grass', has_lighting: true })).toBe('Césped sintético · Con iluminación');
    expect(courtFactsLine({ surface: null, has_lighting: false })).toBe('');
  });
});

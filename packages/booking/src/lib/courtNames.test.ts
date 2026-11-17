import { describe, expect, it } from 'vitest';
import { courtNamesLine } from './courtNames';

describe('courtNamesLine', () => {
  it('names one or two courts, joined by a comma', () => {
    expect(courtNamesLine(['Cancha 1'])).toBe('Cancha 1');
    expect(courtNamesLine(['Cancha 1', 'Cancha 2'])).toBe('Cancha 1, Cancha 2');
  });

  it('counts more than two courts instead of naming each one', () => {
    expect(courtNamesLine(['A', 'B', 'C', 'D', 'E', 'F'])).toBe('6 canchas');
  });
});

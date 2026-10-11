import { describe, it, expect } from 'vitest';
import { blankToUndefined } from './blankToUndefined';

describe('blankToUndefined', () => {
  it('maps undefined and the empty string to undefined', () => {
    expect(blankToUndefined(undefined)).toBeUndefined();
    expect(blankToUndefined('')).toBeUndefined();
  });

  it('maps a whitespace-only value to undefined', () => {
    expect(blankToUndefined('   ')).toBeUndefined();
  });

  it('returns a padded value trimmed', () => {
    expect(blankToUndefined('  nota de caja  ')).toBe('nota de caja');
  });
});

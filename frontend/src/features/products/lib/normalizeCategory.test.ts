import { describe, it, expect } from 'vitest';
import { normalizeCategory } from './normalizeCategory';

describe('normalizeCategory', () => {
  it('matches an existing category case-insensitively and keeps its spelling', () => {
    expect(normalizeCategory('bebidas', ['Bebidas', 'Snacks'])).toBe('Bebidas');
  });

  it('trims surrounding whitespace before matching', () => {
    expect(normalizeCategory('  Bebidas  ', ['Bebidas'])).toBe('Bebidas');
  });

  it('returns an empty string for a blank value', () => {
    expect(normalizeCategory('   ', ['Bebidas'])).toBe('');
  });

  it('keeps a new category unchanged (trimmed) when it matches nothing', () => {
    expect(normalizeCategory('  Fiambrería  ', ['Bebidas', 'Snacks'])).toBe('Fiambrería');
  });
});

import { describe, expect, it } from 'vitest';
import { keepTogether } from './keepTogether';

describe('keepTogether', () => {
  it('joins the words of a name with non-breaking spaces so it never wraps', () => {
    expect(keepTogether('Cancha 1')).toBe('Cancha 1');
    expect(keepTogether('Cancha central techada')).toBe('Cancha central techada');
  });

  it('leaves a single word alone', () => {
    expect(keepTogether('Panorámica')).toBe('Panorámica');
  });
});

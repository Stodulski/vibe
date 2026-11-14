import { describe, expect, it } from 'vitest';
import { keepTogether } from './keepTogether';

describe('keepTogether', () => {
  it('replaces regular spaces with non-breaking spaces', () => {
    expect(keepTogether('Cancha 1')).toBe('Cancha 1');
    expect(keepTogether('Cancha Central Norte')).toBe('Cancha Central Norte');
  });

  it('leaves text without spaces untouched', () => {
    expect(keepTogether('Cancha')).toBe('Cancha');
    expect(keepTogether('')).toBe('');
  });
});

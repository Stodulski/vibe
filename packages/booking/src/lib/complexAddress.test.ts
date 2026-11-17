import { describe, expect, it } from 'vitest';
import { shortAddress } from './complexAddress';

describe('shortAddress', () => {
  it('joins street and city, without the province', () => {
    expect(shortAddress({ address: 'Av. Constitución 5840', city: 'Mar del Plata' })).toBe(
      'Av. Constitución 5840, Mar del Plata',
    );
  });

  it('shows a repeated part once', () => {
    expect(shortAddress({ address: 'CABA', city: 'CABA' })).toBe('CABA');
  });

  it('skips a blank part', () => {
    expect(shortAddress({ address: 'Piquiyin', city: '  ' })).toBe('Piquiyin');
  });
});

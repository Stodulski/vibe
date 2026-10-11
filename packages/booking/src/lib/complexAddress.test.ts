import { describe, expect, it } from 'vitest';
import { fullAddress } from './complexAddress';

describe('fullAddress', () => {
  it('joins street, city and province', () => {
    expect(fullAddress({ address: 'Piquiyin', city: 'Caleta Olivia', province: 'Santa Cruz' })).toBe(
      'Piquiyin, Caleta Olivia, Santa Cruz',
    );
  });

  it('shows a repeated part once, such as a city that is also its province', () => {
    expect(fullAddress({ address: 'Av. Libertador 1234', city: 'CABA', province: 'CABA' })).toBe(
      'Av. Libertador 1234, CABA',
    );
  });

  it('skips a blank part', () => {
    expect(fullAddress({ address: 'Piquiyin', city: 'Caleta Olivia', province: '  ' })).toBe('Piquiyin, Caleta Olivia');
  });
});

import { describe, it, expect } from 'vitest';
import { pesosToCentavos, centavosToPesos } from './money';

describe('pesosToCentavos', () => {
  it('converts a whole peso amount', () => {
    expect(pesosToCentavos(5000)).toBe(500000);
  });

  it('rounds a peso amount with cents', () => {
    expect(pesosToCentavos(10.5)).toBe(1050);
    expect(pesosToCentavos(10.005)).toBe(1001); // floating point: 1000.5 rounds to 1001
  });

  it('converts zero', () => {
    expect(pesosToCentavos(0)).toBe(0);
  });
});

describe('centavosToPesos', () => {
  it('is the inverse of pesosToCentavos for round values', () => {
    expect(centavosToPesos(pesosToCentavos(1234.56))).toBeCloseTo(1234.56);
  });

  it('converts centavos back to pesos', () => {
    expect(centavosToPesos(150000)).toBe(1500);
  });
});

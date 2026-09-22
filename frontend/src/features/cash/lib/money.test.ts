import { describe, it, expect } from 'vitest';
import {
  pesosToCentavos,
  centavosToPesos,
  MAX_SESSION_CASH_CENTAVOS,
  MAX_SESSION_CASH_PESOS,
  MAX_MOVEMENT_AMOUNT_CENTAVOS,
  MAX_MOVEMENT_AMOUNT_PESOS,
} from './money';

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
});

describe('caps', () => {
  it('matches the API session cash cap (BIGINT column, db/migrations/003_cashbox.sql)', () => {
    expect(MAX_SESSION_CASH_CENTAVOS).toBe(99_999_999_999);
    expect(pesosToCentavos(MAX_SESSION_CASH_PESOS)).toBe(MAX_SESSION_CASH_CENTAVOS);
  });

  it('matches the API movement amount cap (INTEGER column)', () => {
    expect(MAX_MOVEMENT_AMOUNT_CENTAVOS).toBe(2_000_000_000);
    expect(pesosToCentavos(MAX_MOVEMENT_AMOUNT_PESOS)).toBe(MAX_MOVEMENT_AMOUNT_CENTAVOS);
  });
});

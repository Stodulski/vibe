import { describe, it, expect } from 'vitest';
import { pesosToCentavos } from '@/shared/lib/money';
import {
  MAX_SESSION_CASH_CENTAVOS,
  MAX_SESSION_CASH_PESOS,
  MAX_MOVEMENT_AMOUNT_CENTAVOS,
  MAX_MOVEMENT_AMOUNT_PESOS,
} from './money';

// `pesosToCentavos`/`centavosToPesos` themselves are tested in
// `shared/lib/money.test.ts` now that they live there (pos-products-screen
// T5a); this file keeps only the cash-specific caps.
describe('caps', () => {
  it('matches the API session cash cap (BIGINT column, db/migrations/003_cashbox.sql), rounded down to a whole peso', () => {
    expect(MAX_SESSION_CASH_CENTAVOS).toBe(99_999_999_999);
    // 99,999,999,999 centavos is not an even number of pesos
    // (999,999,999.99) — the pesos-side cap floors to the nearest whole peso
    // (the cashbox works in whole pesos only), so converting it back is 99
    // centavos short of the raw BIGINT cap. That gap is an arbitrary safety
    // ceiling losing 99 centavos, not a real limitation.
    expect(MAX_SESSION_CASH_PESOS).toBe(999_999_999);
    expect(pesosToCentavos(MAX_SESSION_CASH_PESOS)).toBe(MAX_SESSION_CASH_CENTAVOS - 99);
  });

  it('matches the API movement amount cap (INTEGER column)', () => {
    expect(MAX_MOVEMENT_AMOUNT_CENTAVOS).toBe(2_000_000_000);
    expect(pesosToCentavos(MAX_MOVEMENT_AMOUNT_PESOS)).toBe(MAX_MOVEMENT_AMOUNT_CENTAVOS);
  });
});

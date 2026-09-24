import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeCashSession, makeCashMovement } from '@/test/factories';
import {
  cashSessionSchema,
  cashMovementSchema,
  cashSessionsListResponseSchema,
  cashSessionCurrentResponseSchema,
  cashSessionDetailResponseSchema,
  cashSessionEnvelopeSchema,
  cashMovementEnvelopeSchema,
  cashSessionSummarySchema,
} from './cash.schema';

const summary = {
  opening_cash: 500000,
  expected_cash: 650000,
  cash_manual_refunds: 20000,
  movement_totals: [{ method: 'cash', kind: 'expense', category: 'supplies', total: 150000, count: 1 }],
  booking_payments: [{ method: 'mercadopago', count: 2, amount: 800000, service_fee: 32000, refunded: 0 }],
  manual_refunds: [{ method: 'cash', count: 1, amount: 20000 }],
};

describe('cashSessionSchema', () => {
  it('validates an open session (nullable close fields absent)', () => {
    expect(cashSessionSchema.safeParse(makeCashSession()).success).toBe(true);
  });

  it('validates a closed session', () => {
    const closed = makeCashSession({
      closed_at: '2026-01-01T20:00:00Z',
      closed_by: 'u1',
      counted_cash: 640000,
      expected_cash: 650000,
      difference: -10000,
      closing_note: 'faltante chico',
    });
    expect(cashSessionSchema.safeParse(closed).success).toBe(true);
  });
});

describe('cashMovementSchema', () => {
  it('validates a plain movement', () => {
    expect(cashMovementSchema.safeParse(makeCashMovement()).success).toBe(true);
  });

  it('validates a void movement (voids_movement_id set)', () => {
    const voidMovement = makeCashMovement({ id: 'cm2', kind: 'income', voids_movement_id: 'cm1' });
    expect(cashMovementSchema.safeParse(voidMovement).success).toBe(true);
  });

  it('rejects an unknown category', () => {
    const invalid = { ...makeCashMovement(), category: 'not_a_category' };
    expect(cashMovementSchema.safeParse(invalid).success).toBe(false);
  });

  // System categories written by the sales and restock flows (never offered
  // on the manual movement form, see `features/cash/schemas/cash.schema.ts`),
  // but present on GET responses — a session with one POS sale used to fail
  // client-side parsing entirely because these two values were missing from
  // this enum.
  it('validates a sale income movement', () => {
    const sale = makeCashMovement({ kind: 'income', category: 'sale' });
    expect(cashMovementSchema.safeParse(sale).success).toBe(true);
  });

  it('validates a restock expense movement', () => {
    const restock = makeCashMovement({ kind: 'expense', category: 'restock' });
    expect(cashMovementSchema.safeParse(restock).success).toBe(true);
  });
});

describe('cashSessionSummarySchema', () => {
  it('validates a summary with mercadopago among booking_payments', () => {
    expect(cashSessionSummarySchema.safeParse(summary).success).toBe(true);
  });
});

describe('response envelope schemas', () => {
  it('cashSessionsListResponseSchema', () => {
    const result = cashSessionsListResponseSchema.safeParse({
      cash_sessions: [makeCashSession()],
      metadata: { has_more: false },
    });
    expect(result.success).toBe(true);
  });

  it('cashSessionCurrentResponseSchema', () => {
    const result = cashSessionCurrentResponseSchema.safeParse({ cash_session: makeCashSession(), summary });
    expect(result.success).toBe(true);
  });

  it('cashSessionDetailResponseSchema', () => {
    const result = cashSessionDetailResponseSchema.safeParse({
      cash_session: makeCashSession(),
      summary,
      movements: [makeCashMovement()],
    });
    expect(result.success).toBe(true);
  });

  it('cashSessionEnvelopeSchema (open/close)', () => {
    expect(cashSessionEnvelopeSchema.safeParse({ cash_session: makeCashSession() }).success).toBe(true);
  });

  it('cashMovementEnvelopeSchema (create/void)', () => {
    expect(cashMovementEnvelopeSchema.safeParse({ cash_movement: makeCashMovement() }).success).toBe(true);
  });
});

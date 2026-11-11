import { describe, it, expect } from 'vitest';
import { makeCashMovement } from '@/test/factories';
import { cashMovementTotals, ledgerRows } from './movementLedger';

describe('ledgerRows', () => {
  it('folds a void into the movement it reverses, so the pair reads as one cancelled row', () => {
    const original = makeCashMovement({ id: 'm1', kind: 'income', category: 'classes' });
    const reversal = makeCashMovement({ id: 'm2', kind: 'expense', category: 'classes', voids_movement_id: 'm1' });

    const rows = ledgerRows([reversal, original]);

    expect(rows).toHaveLength(1);
    expect(rows[0]?.movement.id).toBe('m1');
    expect(rows[0]?.voidedBy?.id).toBe('m2');
  });

  it('keeps a void whose original is not in the ledger as its own row', () => {
    const orphan = makeCashMovement({ id: 'm2', voids_movement_id: 'gone' });

    const rows = ledgerRows([orphan]);

    expect(rows).toEqual([{ movement: orphan, voidedBy: undefined }]);
  });
});

describe('cashMovementTotals', () => {
  it('sums cash income and expense, leaving out cancelled pairs and other methods', () => {
    const movements = [
      makeCashMovement({ id: 'a', kind: 'income', method: 'cash', amount: 300000 }),
      makeCashMovement({ id: 'b', kind: 'expense', method: 'cash', amount: 50000 }),
      makeCashMovement({ id: 'c', kind: 'income', method: 'transfer', amount: 900000 }),
      makeCashMovement({ id: 'd', kind: 'income', method: 'cash', amount: 123200 }),
      makeCashMovement({ id: 'e', kind: 'expense', method: 'cash', amount: 123200, voids_movement_id: 'd' }),
    ];

    expect(cashMovementTotals(movements)).toEqual({ income: 300000, expense: 50000 });
  });
});

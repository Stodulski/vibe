import type { CashMovement } from '@/shared/types/api.types';

/** One line of the till ledger: a movement, and the void that cancelled it, if any. */
export interface LedgerRow {
  movement: CashMovement;
  voidedBy: CashMovement | undefined;
}

/**
 * Folds every void into the movement it reverses, so a cancelled movement
 * reads as one crossed-out line instead of an income and an expense that
 * look like two real events. A void whose original is not in the list keeps
 * its own row. Order follows `movements`.
 */
export function ledgerRows(movements: CashMovement[]): LedgerRow[] {
  const ids = new Set(movements.map((m) => m.id));
  const voidOf = new Map<string, CashMovement>();
  for (const m of movements) {
    if (m.voids_movement_id) voidOf.set(m.voids_movement_id, m);
  }
  return movements
    .filter((m) => !(m.voids_movement_id && ids.has(m.voids_movement_id)))
    .map((movement) => ({ movement, voidedBy: voidOf.get(movement.id) }));
}

/**
 * Cash income and expense that actually moved the till: cancelled pairs net
 * to zero and are left out, and other methods never touch the drawer. These
 * are the movement terms of the backend's expected-cash formula.
 */
export function cashMovementTotals(movements: CashMovement[]): { income: number; expense: number } {
  const totals = { income: 0, expense: 0 };
  for (const { movement, voidedBy } of ledgerRows(movements)) {
    if (voidedBy || movement.voids_movement_id || movement.method !== 'cash') continue;
    totals[movement.kind] += movement.amount;
  }
  return totals;
}

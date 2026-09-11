import type { Payment } from '@/shared/types/api.types';

/**
 * Outstanding manual (cash/transfer) amount still owed to the client on a
 * `partial_refund` booking — the MercadoPago portion already came back
 * automatically, so this sums only the non-MP payment rows that haven't
 * been marked refunded yet. Centavos.
 */
export function manualRefundOwed(payments: Payment[]): number {
  return payments
    .filter((p) => !p.mp_payment_id && p.status !== 'refunded')
    .reduce((sum, p) => sum + p.amount + p.service_fee - p.refund_amount, 0);
}

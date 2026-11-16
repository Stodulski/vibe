import type { Payment } from '@/shared/types/api.types';

/**
 * Whether a Mercado Pago refund on this booking has been asked for and has not
 * landed yet. The booking row itself does not say so (its refund_status stays
 * as it was while the claim is in flight); the payment row does.
 */
export function mercadoPagoRefundInFlight(payments: Payment[]): boolean {
  return payments.some((p) => Boolean(p.mp_payment_id) && p.status === 'refund_pending');
}

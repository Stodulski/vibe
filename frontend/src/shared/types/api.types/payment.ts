import type { Open, Spec } from './spec';

// ─── Payment ───

/**
 * `method` and `status` are read through {@link Open}: the document's lists
 * are what a current server sends, but a payments row written before one of
 * them changed still has to render. `status` is the payments table's own,
 * wider vocabulary — distinct from a booking's `collection_status` and
 * `refund_status`, which were split off it at the booking level.
 */
export type Payment = Omit<Spec<'Payment'>, 'method' | 'status'> & {
  method: Open<Spec<'Payment'>['method']>;
  status: Open<Spec<'Payment'>['status']>;
};

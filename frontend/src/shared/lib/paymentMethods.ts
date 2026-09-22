/**
 * Every payment method staff may record directly at the counter — every
 * `payment_method` value except `mercadopago`, which the online checkout owns
 * exclusively (it carries `mp_payment_id` and drives an automatic MercadoPago
 * refund; a counter QR payment has no id for that).
 *
 * The single source both bookings' method selects, the bookings Zod schemas
 * and the cash feature's movement form read from, so a new counter method is
 * one edit here instead of one per component. Lives in `shared/` (not
 * `features/bookings/`) because both `features/bookings` and `features/cash`
 * need it, and features never import from one another.
 */
export const COUNTER_PAYMENT_METHODS = ['cash', 'transfer', 'debit_card', 'credit_card', 'qr_wallet'] as const;

export type CounterPaymentMethod = (typeof COUNTER_PAYMENT_METHODS)[number];

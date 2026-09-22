/**
 * Every payment method staff may record directly at the counter — every
 * `payment_method` value except `mercadopago`, which the online checkout owns
 * exclusively (it carries `mp_payment_id` and drives an automatic MercadoPago
 * refund; a counter QR payment has no id for that).
 *
 * The single source both method selects and both Zod schemas read from, so a
 * new counter method is one edit here instead of one per component.
 */
export const COUNTER_PAYMENT_METHODS = ['cash', 'transfer', 'debit_card', 'credit_card', 'qr_wallet'] as const;

export type CounterPaymentMethod = (typeof COUNTER_PAYMENT_METHODS)[number];

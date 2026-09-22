/**
 * Re-exported from `shared/lib/paymentMethods` — moved there (pos-cashbox T3)
 * because `features/cash` needs the same list and features never import from
 * one another. Kept here too so every existing import in this feature (and
 * its tests) keeps working unchanged.
 */
export { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';

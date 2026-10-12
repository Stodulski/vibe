import type { Body, Ok, Spec } from './spec';

// ─── Sales (pos-cashbox T5b) ───

export type Sale = Spec<'Sale'>;
export type SaleItem = Spec<'SaleItem'>;
export type SaleStockWarning = Spec<'SaleStockWarning'>;
/** A booking balance collected at the counter (`sale.booking_payments` or a checkout's `booking_payments`). */
export type SaleBookingPayment = Spec<'SaleBookingPayment'>;
/** One booking of today that still owes money (`GET /cash/bookings-due`). */
export type BalanceDue = Spec<'BalanceDue'>;

export type SalesListResponse = Ok<'salesList'>;
export type SaleCreateResponse = Ok<'salesCreate'>;
/** `GET /complexes/{id}/cash/bookings-due` — today's bookings that still owe money. */
export type BalancesDueResponse = Ok<'salesBookingsDue'>;
// `salesGet` and `salesVoid` both answer `{ sale: Sale }` — one name (read
// from the operation both share it with), same reasoning as
// `ProductEnvelopeResponse`/`CashSessionOpenResponse`.
export type SaleEnvelopeResponse = Ok<'salesGet'>;

export type CreateSaleRequest = Body<'salesCreate'>;
export type VoidSaleRequest = Body<'salesVoid'>;

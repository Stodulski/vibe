import type { Body, Ok, Spec } from './spec';

// ─── Sales (pos-cashbox T5b) ───

export type Sale = Spec<'Sale'>;
export type SaleItem = Spec<'SaleItem'>;
export type SaleStockWarning = Spec<'SaleStockWarning'>;

export type SalesListResponse = Ok<'salesList'>;
export type SaleCreateResponse = Ok<'salesCreate'>;
// `salesGet` and `salesVoid` both answer `{ sale: Sale }` — one name (read
// from the operation both share it with), same reasoning as
// `ProductEnvelopeResponse`/`CashSessionOpenResponse`.
export type SaleEnvelopeResponse = Ok<'salesGet'>;

export type CreateSaleRequest = Body<'salesCreate'>;
export type VoidSaleRequest = Body<'salesVoid'>;

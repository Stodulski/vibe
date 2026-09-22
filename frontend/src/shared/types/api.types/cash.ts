import type { Body, Ok, Spec } from './spec';

// ─── Cash (pos-cashbox T3) ───

export type CashSession = Spec<'CashSession'>;
export type CashMovement = Spec<'CashMovement'>;
export type CashMovementTotal = Spec<'CashMovementTotal'>;
export type CashBookingPaymentTotal = Spec<'CashBookingPaymentTotal'>;
export type CashSessionSummary = Spec<'CashSessionSummary'>;

export type CashSessionsListResponse = Ok<'cashSessionsList'>;
export type CashSessionCurrentResponse = Ok<'cashSessionsCurrent'>;
export type CashSessionDetailResponse = Ok<'cashSessionsGet'>;
// `cashSessionsOpen` and `cashSessionsClose` answer the same
// `{ cash_session: CashSession }` shape, as do `cashMovementsCreate` and
// `cashMovementsVoid` with `{ cash_movement: CashMovement }` — one name each
// (read from the operation both share it with) rather than two structurally
// identical types with a name apiece.
export type CashSessionOpenResponse = Ok<'cashSessionsOpen'>;
export type CashMovementCreateResponse = Ok<'cashMovementsCreate'>;

export type OpenCashSessionRequest = Body<'cashSessionsOpen'>;
export type CloseCashSessionRequest = Body<'cashSessionsClose'>;
export type CreateCashMovementRequest = Body<'cashMovementsCreate'>;
export type VoidCashMovementRequest = Body<'cashMovementsVoid'>;

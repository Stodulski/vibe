import api, { withSignal } from '@/shared/lib/ky';
import type { CashSessionCurrentResponse } from '@/shared/types/api.types';
import { cashSessionCurrentResponseSchema } from '@/shared/schemas';
import { parseWith } from '@/shared/lib/apiParse';

/**
 * The complex's currently open cash session, if any — the one endpoint
 * `useCashSession` (`shared/hooks/useCashSession.ts`) needs.
 *
 * Lives in `shared/` (not `features/cash/api/cash.api.ts`, which still owns
 * every other cashbox endpoint and delegates its own `current` method to
 * this) because `features/products`' restock dialog needs to know whether
 * the till is open too, and features never import from one another — same
 * reasoning as `shared/lib/paymentMethods.ts`.
 *
 * Rejects with a ky `HTTPError` (status 404) when no session is open —
 * callers treat that as "closed".
 */
export function getCurrentCashSession(complexId: string, signal?: AbortSignal): Promise<CashSessionCurrentResponse> {
  return api
    .get(`complexes/${complexId}/cash-session`, withSignal(signal))
    .json()
    .then(parseWith(cashSessionCurrentResponseSchema, 'cashSessionApi.getCurrentCashSession'));
}

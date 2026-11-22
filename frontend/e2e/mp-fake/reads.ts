/**
 * The read side of the fake Mercado Pago API: a payment by id, payments
 * searched by reference, and a preference read back by its collector.
 */
import type { IncomingMessage, ServerResponse } from 'node:http';
import { authorize, forbid, isCollector, preferenceView } from './access.ts';
import { sendError, sendJson } from './http.ts';
import { PUBLIC_URL, state } from './store.ts';

export function getPayment(req: IncomingMessage, res: ServerResponse, id: number): void {
  const party = authorize(req, res);
  if (!party) return;
  const payment = state.payments.get(id);
  if (!payment) {
    sendError(res, 404, `payment ${String(id)} not found`);
    return;
  }
  // The platform reads every payment made under its application; a seller only its own.
  if (party.kind !== 'platform' && !isCollector(party, payment.collector_id)) {
    forbid(res, `payment ${String(id)} belongs to another collector`);
    return;
  }
  sendJson(res, 200, payment);
}

/**
 * GET /checkout/preferences/{id}: the collector reads its own preference back
 * (mp.GetPreference), with the expiry a close wrote into it (`expires`,
 * `expiration_date_to`). Another party gets 403, as at Mercado Pago.
 */
export function getPreference(req: IncomingMessage, res: ServerResponse, id: string): void {
  const party = authorize(req, res);
  if (!party) return;
  const pref = state.preferences.get(id);
  if (!pref) {
    sendError(res, 404, `preference ${id} not found`);
    return;
  }
  if (!isCollector(party, pref.collector_id)) {
    forbid(res, `preference ${id} belongs to another collector`);
    return;
  }
  sendJson(res, 200, preferenceView(pref));
}

/** The fake's cap on one page of /v1/payments/search; no spec comes near it. */
const SEARCH_MAX_LIMIT = 1000;

/** A non-negative integer query parameter, or fallback when it is absent or not one. */
function searchInt(params: URLSearchParams, name: string, fallback: number): number {
  const raw = params.get(name);
  const n = raw === null || raw === '' ? NaN : Number(raw);
  return Number.isInteger(n) && n >= 0 ? n : fallback;
}

/**
 * GET /v1/payments/search (mp.SearchPaymentsByReference): the payments the
 * caller may see, filtered by external_reference, sorted by `sort`
 * (date_created or id) in `criteria` order, paged by `offset` and `limit`. A
 * seller sees what it collected; the platform sees every payment of its
 * application. The page is Mercado Pago's shape: `paging` and `results`.
 */
export function searchPayments(req: IncomingMessage, res: ServerResponse): void {
  const party = authorize(req, res);
  if (!party) return;
  const params = new URL(req.url ?? '/', PUBLIC_URL).searchParams;
  const reference = params.get('external_reference');
  const sort = params.get('sort') === 'id' ? 'id' : 'date_created';
  const desc = params.get('criteria') === 'desc';
  const limit = Math.min(searchInt(params, 'limit', 30) || 30, SEARCH_MAX_LIMIT);
  const offset = searchInt(params, 'offset', 0);

  const found = [...state.payments.values()]
    .filter((p) => party.kind === 'platform' || isCollector(party, p.collector_id))
    .filter((p) => reference === null || p.external_reference === reference)
    .sort((a, b) => {
      const order =
        sort === 'id' ? a.id - b.id : Date.parse(a.date_created) - Date.parse(b.date_created) || a.id - b.id;
      return desc ? -order : order;
    });
  sendJson(res, 200, {
    paging: { total: found.length, limit, offset },
    results: found.slice(offset, offset + limit),
  });
}

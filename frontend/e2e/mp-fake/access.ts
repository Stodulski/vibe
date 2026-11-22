/**
 * Who may call the fake Mercado Pago API, and how a refusal reads.
 *
 * Mercado Pago's own refusals: 401 when the token is missing, unknown or
 * expired, 403 when a valid token belongs to a party that may not act on the
 * resource (the platform creating a seller's preference, another seller
 * refunding a payment it did not collect).
 */
import type { IncomingMessage, ServerResponse } from 'node:http';
import { authenticate } from './auth.ts';
import type { Party } from './auth.ts';
import { sendJson } from './http.ts';
import type { Json, Preference } from './store.ts';

/** The caller's party, or the 401 already answered. */
export function authorize(req: IncomingMessage, res: ServerResponse): Party | null {
  const auth = authenticate(req);
  if (!auth.ok) {
    sendJson(res, 401, { message: auth.reason, error: 'unauthorized', status: 401, cause: [] });
    return null;
  }
  return auth.party;
}

export function forbid(res: ServerResponse, message: string): void {
  sendJson(res, 403, { message, error: 'forbidden', status: 403, cause: [] });
}

export function isCollector(party: Party, collectorId: number): boolean {
  return party.kind === 'seller' && party.collectorId === collectorId;
}

/** A preference as Mercado Pago answers it: the request body, merged with later PUTs, plus its identity. */
export function preferenceView(pref: Preference): Json {
  return {
    ...pref.body,
    id: pref.id,
    init_point: pref.init_point,
    sandbox_init_point: pref.sandbox_init_point,
    collector_id: pref.collector_id,
    date_created: pref.date_created,
  };
}

/**
 * A fake Mercado Pago for the E2E suite.
 *
 * It answers exactly the calls `backend/internal/mp/mp.go` makes (preferences
 * created, read back and closed; payments read and searched by reference;
 * refunds; OAuth), serves a checkout page at each preference's
 * `init_point` where the browser picks the outcome, delivers the signed webhook
 * the backend verifies, and redirects to the preference's `back_urls`. A small
 * control API under `/__fake/` lets specs read and steer its state.
 *
 * Dependency-free on purpose: run it with Node's type stripping,
 * `node frontend/e2e/mp-fake/server.ts`. Keep the types erasable-only (no
 * enums, namespaces or parameter properties).
 *
 * Environment:
 *   MP_FAKE_PORT            port to listen on (default 8090)
 *   MP_FAKE_PUBLIC_URL      base URL the browser reaches it on (default http://localhost:<port>)
 *   MP_FAKE_WEBHOOK_SECRET  secret the webhook is signed with; must equal the backend's MP_WEBHOOK_SECRET
 *   MP_FAKE_COLLECTOR_ID    numeric id of the seeded seller; must equal the seeded complex's mp_user_id
 *   MP_FAKE_PLATFORM_TOKEN  the marketplace's own token; must equal the backend's MP_ACCESS_TOKEN
 *   MP_FAKE_APP_ID, MP_FAKE_CLIENT_SECRET  the OAuth client; must equal the backend's MP_APP_ID / MP_CLIENT_SECRET
 *
 * Every API call is authenticated like Mercado Pago's (see ./auth.ts):
 * preferences and refunds take the collecting seller's token, a payment is read
 * by the platform or its collector, a payment search shows a seller only what
 * it collected (the platform everything), anything else is 401/403. A preference past
 * its expiration_date_to cannot be paid, and every refund is followed by a
 * signed payment.updated webhook.
 *
 * State lives in memory and dies with the process.
 */
import { createServer } from 'node:http';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { randomUUID } from 'node:crypto';
import {
  COLLECTOR_ID,
  PORT,
  PUBLIC_URL,
  applyRefund,
  createPayment,
  isObject,
  isPreferenceExpired,
  now,
  preferenceAmount,
  refundView,
  sendWebhook,
  state,
} from './store.ts';
import { describeBearer, grant } from './auth.ts';
import { authorize, forbid, isCollector, preferenceView } from './access.ts';
import { CONTROL_ROUTES } from './control.ts';
import { getPayment, getPreference, searchPayments } from './reads.ts';
import { escapeHtml, readBody, readJson, sendError, sendJson } from './http.ts';
import type { Route } from './http.ts';
import type { Payment, Preference } from './store.ts';

// --- Mercado Pago API ------------------------------------------------------
// Who may call what is ./access.ts; the reads (payments, payment search,
// preference read-back) are ./reads.ts.

async function createPreference(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const party = authorize(req, res);
  if (!party) return;
  if (party.kind !== 'seller') {
    // The real API would take it and pay the marketplace; the backend must
    // never ask (mp.ErrPlatformCannotSell), so the fake refuses it loudly.
    forbid(res, 'a checkout preference must be created with the seller access token');
    return;
  }
  const key = req.headers['x-idempotency-key'];
  const idempotencyKey = typeof key === 'string' && key !== '' ? key : null;
  if (idempotencyKey) {
    const existingID = state.preferenceByIdempotencyKey.get(idempotencyKey);
    const existing = existingID ? state.preferences.get(existingID) : undefined;
    if (existing?.collector_id === party.collectorId) {
      sendJson(res, 201, preferenceView(existing));
      return;
    }
  }
  const body = await readJson(req);
  const id = `${String(party.collectorId)}-${randomUUID()}`;
  const pref: Preference = {
    id,
    collector_id: party.collectorId,
    init_point: `${PUBLIC_URL}/checkout/${id}`,
    sandbox_init_point: `${PUBLIC_URL}/checkout/${id}`,
    date_created: now(),
    body,
  };
  state.preferences.set(id, pref);
  if (idempotencyKey) state.preferenceByIdempotencyKey.set(idempotencyKey, id);
  sendJson(res, 201, preferenceView(pref));
}

async function updatePreference(req: IncomingMessage, res: ServerResponse, id: string): Promise<void> {
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
  pref.body = { ...pref.body, ...(await readJson(req)) };
  sendJson(res, 200, preferenceView(pref));
}

async function refundPayment(req: IncomingMessage, res: ServerResponse, id: number): Promise<void> {
  const party = authorize(req, res);
  if (!party) return;
  const payment = state.payments.get(id);
  if (!payment) {
    sendError(res, 404, `payment ${String(id)} not found`);
    return;
  }
  // The money comes out of whoever authorized the refund: only the collector may.
  if (!isCollector(party, payment.collector_id)) {
    forbid(res, `payment ${String(id)} can only be refunded by its collector`);
    return;
  }
  const key = req.headers['x-idempotency-key'];
  const idempotencyKey = typeof key === 'string' && key !== '' ? key : null;
  const replay = idempotencyKey ? state.refunds.find((r) => r.idempotency_key === idempotencyKey) : undefined;
  if (replay) {
    sendJson(res, 201, refundView(replay));
    return;
  }
  const result = applyRefund(payment, (await readJson(req)).amount, idempotencyKey, 'api');
  if (!result.ok) {
    sendError(res, result.status, result.message);
    return;
  }
  sendJson(res, 201, refundView(result.refund));
  notifyLater(payment);
}

/**
 * Mercado Pago tells the seller about a refund with a payment.updated
 * notification a moment after answering the refund call; the backend has
 * already recorded it by then, and must not record it twice.
 */
function notifyLater(payment: Payment): void {
  setTimeout(() => {
    void sendWebhook(payment, 'payment.updated');
  }, 300);
}

async function oauthToken(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const result = grant(await readJson(req));
  if (!result.ok) {
    sendJson(res, result.status, { error: result.error, message: result.message, status: result.status, cause: [] });
    return;
  }
  sendJson(res, 200, {
    access_token: result.credentials.access_token,
    refresh_token: result.credentials.refresh_token,
    token_type: 'Bearer',
    scope: 'offline_access read write',
    user_id: result.credentials.collector_id,
    expires_in: 15_552_000,
    live_mode: false,
  });
}

// --- Checkout page ---------------------------------------------------------

const OUTCOMES = {
  approved: { label: 'Aprobar pago', backURL: 'success' },
  rejected: { label: 'Rechazar pago', backURL: 'failure' },
  pending: { label: 'Dejar pendiente', backURL: 'pending' },
} as const;
type Outcome = keyof typeof OUTCOMES;

function isOutcome(v: string): v is Outcome {
  return Object.hasOwn(OUTCOMES, v);
}

function checkoutPage(res: ServerResponse, id: string): void {
  const pref = state.preferences.get(id);
  if (!pref) {
    res.writeHead(404, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end('<!doctype html><title>Preferencia inexistente</title><h1>Preferencia inexistente</h1>');
    return;
  }
  if (isPreferenceExpired(pref)) {
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    res.end(`<!doctype html>
<html lang="es">
  <head><meta charset="utf-8"><title>Mercado Pago (fake)</title></head>
  <body>
    <h1>Link de pago vencido</h1>
    <p data-testid="mp-fake-expired">Este link de pago ya no está disponible.</p>
  </body>
</html>`);
    return;
  }
  const items = Array.isArray(pref.body.items) ? (pref.body.items as unknown[]) : [];
  const first = items.find(isObject);
  const title = first && typeof first.title === 'string' ? first.title : 'Pago';
  const buttons = (Object.keys(OUTCOMES) as Outcome[])
    .map((o) => `<button type="submit" name="outcome" value="${o}">${OUTCOMES[o].label}</button>`)
    .join('\n      ');
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end(`<!doctype html>
<html lang="es">
  <head><meta charset="utf-8"><title>Mercado Pago (fake)</title></head>
  <body>
    <h1>Mercado Pago (fake)</h1>
    <p data-testid="mp-fake-title">${escapeHtml(title)}</p>
    <p data-testid="mp-fake-amount">Total: $ ${String(preferenceAmount(pref))}</p>
    <form method="post" action="/checkout/${encodeURIComponent(id)}">
      ${buttons}
    </form>
  </body>
</html>`);
}

async function checkoutSubmit(req: IncomingMessage, res: ServerResponse, id: string): Promise<void> {
  const pref = state.preferences.get(id);
  if (!pref) {
    sendError(res, 404, `preference ${id} not found`);
    return;
  }
  if (isPreferenceExpired(pref)) {
    sendError(res, 403, `preference ${id} expired`);
    return;
  }
  const outcome = new URLSearchParams(await readBody(req)).get('outcome') ?? '';
  if (!isOutcome(outcome)) {
    sendError(res, 400, `unknown outcome "${outcome}"`);
    return;
  }
  const payment = createPayment(pref, outcome);
  await sendWebhook(payment, 'payment.created');

  const backURLs = isObject(pref.body.back_urls) ? pref.body.back_urls : {};
  const target = backURLs[OUTCOMES[outcome].backURL];
  if (typeof target !== 'string' || target === '') {
    sendJson(res, 200, payment);
    return;
  }
  // Mercado Pago appends the payment to the back URL; the app ignores these.
  const location = new URL(target);
  location.searchParams.set('payment_id', String(payment.id));
  location.searchParams.set('collection_status', payment.status);
  location.searchParams.set('external_reference', payment.external_reference);
  location.searchParams.set('preference_id', pref.id);
  res.writeHead(303, { Location: location.toString() });
  res.end();
}

// --- Router ----------------------------------------------------------------

const ROUTES: Route[] = [
  ['POST', /^\/checkout\/preferences$/, createPreference],
  ['PUT', /^\/checkout\/preferences\/([^/]+)$/, updatePreference],
  [
    'GET',
    /^\/checkout\/preferences\/([^/]+)$/,
    (req, res, id) => {
      getPreference(req, res, id);
    },
  ],
  [
    'GET',
    /^\/v1\/payments\/search$/,
    (req, res) => {
      searchPayments(req, res);
    },
  ],
  [
    'GET',
    /^\/v1\/payments\/(\d+)$/,
    (req, res, id) => {
      getPayment(req, res, Number(id));
    },
  ],
  ['POST', /^\/v1\/payments\/(\d+)\/refunds$/, (req, res, id) => refundPayment(req, res, Number(id))],
  ['POST', /^\/oauth\/token$/, oauthToken],
  [
    'GET',
    /^\/checkout\/([^/]+)$/,
    (_req, res, id) => {
      checkoutPage(res, id);
    },
  ],
  ['POST', /^\/checkout\/([^/]+)$/, checkoutSubmit],
  ...CONTROL_ROUTES,
];

/** The Mercado Pago API surface the backend calls, as opposed to the checkout page and the control API. */
const API_PATH = /^\/(checkout\/preferences|v1\/payments|oauth\/token)(\/|$)/;

async function route(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const method = req.method ?? 'GET';
  const path = new URL(req.url ?? '/', PUBLIC_URL).pathname;
  for (const [routeMethod, pattern, handler] of ROUTES) {
    const match = routeMethod === method ? pattern.exec(path) : null;
    if (match) {
      if (API_PATH.test(path)) {
        const bearer = describeBearer(req);
        res.on('finish', () => {
          state.requests.push({ method, path, status: res.statusCode, bearer, date_created: now() });
        });
      }
      await handler(req, res, decodeURIComponent(match[1] ?? ''));
      return;
    }
  }
  sendError(res, 404, `no fake route for ${method} ${path}`);
}

const server = createServer((req, res) => {
  route(req, res).catch((err: unknown) => {
    const message = err instanceof Error ? err.message : String(err);
    console.error(`[mp-fake] ${req.method ?? ''} ${req.url ?? ''}: ${message}`);
    if (!res.headersSent) sendError(res, 400, message);
    else res.end();
  });
});

server.listen(PORT, '127.0.0.1', () => {
  console.log(`[mp-fake] listening on ${PUBLIC_URL} (collector ${String(COLLECTOR_ID)})`);
});

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.on(signal, () => {
    server.close(() => process.exit(0));
    server.closeAllConnections();
  });
}

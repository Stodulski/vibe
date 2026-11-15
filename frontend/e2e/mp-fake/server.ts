/**
 * A fake Mercado Pago for the E2E suite.
 *
 * It answers exactly the calls `backend/internal/mp/mp.go` makes (preferences,
 * payments, refunds, OAuth), serves a checkout page at each preference's
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
 *   MP_FAKE_COLLECTOR_ID    numeric seller id put on every payment; must equal the seeded complex's mp_user_id
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
  STATUS_DETAIL,
  createPayment,
  isObject,
  now,
  preferenceAmount,
  resetState,
  round2,
  sendWebhook,
  settle,
  state,
} from './store.ts';
import { escapeHtml, readBody, readJson, sendError, sendJson } from './http.ts';
import type { Json, PaymentStatus, Preference, Refund } from './store.ts';

// --- Mercado Pago API ------------------------------------------------------

async function createPreference(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const key = req.headers['x-idempotency-key'];
  const idempotencyKey = typeof key === 'string' && key !== '' ? key : null;
  if (idempotencyKey) {
    const existingID = state.preferenceByIdempotencyKey.get(idempotencyKey);
    const existing = existingID ? state.preferences.get(existingID) : undefined;
    if (existing) {
      sendJson(res, 201, preferenceView(existing));
      return;
    }
  }
  const body = await readJson(req);
  const id = `${String(COLLECTOR_ID)}-${randomUUID()}`;
  const pref: Preference = {
    id,
    init_point: `${PUBLIC_URL}/checkout/${id}`,
    sandbox_init_point: `${PUBLIC_URL}/checkout/${id}`,
    date_created: now(),
    body,
  };
  state.preferences.set(id, pref);
  if (idempotencyKey) state.preferenceByIdempotencyKey.set(idempotencyKey, id);
  sendJson(res, 201, preferenceView(pref));
}

function preferenceView(pref: Preference): Json {
  return {
    ...pref.body,
    id: pref.id,
    init_point: pref.init_point,
    sandbox_init_point: pref.sandbox_init_point,
    collector_id: COLLECTOR_ID,
    date_created: pref.date_created,
  };
}

async function updatePreference(req: IncomingMessage, res: ServerResponse, id: string): Promise<void> {
  const pref = state.preferences.get(id);
  if (!pref) {
    sendError(res, 404, `preference ${id} not found`);
    return;
  }
  pref.body = { ...pref.body, ...(await readJson(req)) };
  sendJson(res, 200, preferenceView(pref));
}

function getPayment(res: ServerResponse, id: number): void {
  const payment = state.payments.get(id);
  if (!payment) {
    sendError(res, 404, `payment ${String(id)} not found`);
    return;
  }
  sendJson(res, 200, payment);
}

async function refundPayment(req: IncomingMessage, res: ServerResponse, id: number): Promise<void> {
  const payment = state.payments.get(id);
  if (!payment) {
    sendError(res, 404, `payment ${String(id)} not found`);
    return;
  }
  const key = req.headers['x-idempotency-key'];
  const idempotencyKey = typeof key === 'string' && key !== '' ? key : null;
  const replay = idempotencyKey ? state.refunds.find((r) => r.idempotency_key === idempotencyKey) : undefined;
  if (replay) {
    sendJson(res, 201, refundView(replay));
    return;
  }
  if (payment.status !== 'approved') {
    sendError(res, 400, `payment ${String(id)} is ${payment.status} and cannot be refunded`);
    return;
  }
  const body = await readJson(req);
  const remaining = round2(payment.transaction_amount - payment.transaction_amount_refunded);
  const amount = body.amount === undefined ? remaining : round2(Number(body.amount));
  if (!(amount > 0) || amount > remaining) {
    sendError(res, 400, `invalid refund amount ${String(body.amount)} (remaining ${String(remaining)})`);
    return;
  }
  const refund: Refund = {
    id: state.nextRefundId++,
    payment_id: id,
    amount,
    status: 'approved',
    date_created: now(),
    idempotency_key: idempotencyKey,
  };
  state.refunds.push(refund);
  payment.transaction_amount_refunded = round2(payment.transaction_amount_refunded + amount);
  if (payment.transaction_amount_refunded >= payment.transaction_amount) {
    payment.status = 'refunded';
    payment.status_detail = STATUS_DETAIL.refunded;
  }
  payment.date_last_updated = now();
  sendJson(res, 201, refundView(refund));
}

function refundView(refund: Refund): Json {
  return { id: refund.id, payment_id: refund.payment_id, amount: refund.amount, status: refund.status };
}

async function oauthToken(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const body = await readJson(req);
  if (body.grant_type !== 'authorization_code' && body.grant_type !== 'refresh_token') {
    sendError(res, 400, 'invalid_grant');
    return;
  }
  sendJson(res, 200, {
    access_token: `APP_USR-fake-${randomUUID()}`,
    refresh_token: `TG-fake-${randomUUID()}`,
    token_type: 'Bearer',
    scope: 'offline_access read write',
    user_id: COLLECTOR_ID,
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
  const outcome = new URLSearchParams(await readBody(req)).get('outcome') ?? '';
  if (!isOutcome(outcome)) {
    sendError(res, 400, `unknown outcome "${outcome}"`);
    return;
  }
  const payment = createPayment(pref, outcome);
  await sendWebhook(payment);

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

// --- Control API -----------------------------------------------------------

async function setPaymentStatus(req: IncomingMessage, res: ServerResponse, id: number): Promise<void> {
  const payment = state.payments.get(id);
  if (!payment) {
    sendError(res, 404, `payment ${String(id)} not found`);
    return;
  }
  const status = (await readJson(req)).status;
  if (typeof status !== 'string' || !Object.hasOwn(STATUS_DETAIL, status)) {
    sendError(res, 400, `unknown status "${String(status)}"`);
    return;
  }
  const pref = state.preferences.get(payment.preference_id);
  payment.status = status as PaymentStatus;
  if (pref) settle(payment, pref);
  const webhook = await sendWebhook(payment);
  sendJson(res, 200, { payment, webhook });
}

// --- Router ----------------------------------------------------------------

type Handler = (req: IncomingMessage, res: ServerResponse, param: string) => void | Promise<void>;

const ROUTES: [method: string, path: RegExp, handler: Handler][] = [
  ['POST', /^\/checkout\/preferences$/, createPreference],
  ['PUT', /^\/checkout\/preferences\/([^/]+)$/, updatePreference],
  [
    'GET',
    /^\/v1\/payments\/(\d+)$/,
    (_req, res, id) => {
      getPayment(res, Number(id));
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
  [
    'GET',
    /^\/__fake\/preferences$/,
    (_req, res) => {
      sendJson(res, 200, [...state.preferences.values()]);
    },
  ],
  [
    'GET',
    /^\/__fake\/payments$/,
    (_req, res) => {
      sendJson(res, 200, [...state.payments.values()]);
    },
  ],
  [
    'GET',
    /^\/__fake\/refunds$/,
    (_req, res) => {
      sendJson(res, 200, state.refunds.map(refundView));
    },
  ],
  [
    'GET',
    /^\/__fake\/webhooks$/,
    (_req, res) => {
      sendJson(res, 200, state.webhooks);
    },
  ],
  [
    'POST',
    /^\/__fake\/reset$/,
    (_req, res) => {
      resetState();
      sendJson(res, 200, { ok: true });
    },
  ],
  ['POST', /^\/__fake\/payments\/(\d+)\/status$/, (req, res, id) => setPaymentStatus(req, res, Number(id))],
];

async function route(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const method = req.method ?? 'GET';
  const path = new URL(req.url ?? '/', PUBLIC_URL).pathname;
  for (const [routeMethod, pattern, handler] of ROUTES) {
    const match = routeMethod === method ? pattern.exec(path) : null;
    if (match) {
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

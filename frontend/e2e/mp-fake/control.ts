/**
 * The fake Mercado Pago's control API (see ./server.ts), under `/__fake/`: what
 * specs read and steer. None of it exists at the real Mercado Pago; it stands
 * in for what happens there outside the app (a payment approved later, a
 * refund made from the seller's panel) and for fixture setup.
 */
import type { IncomingMessage, ServerResponse } from 'node:http';
import {
  STATUS_DETAIL,
  applyRefund,
  refundRecord,
  resetState,
  seedPayment,
  sendWebhook,
  settle,
  state,
} from './store.ts';
import { expireSeller, registerSeller } from './auth.ts';
import { readJson, sendError, sendJson } from './http.ts';
import type { Route } from './http.ts';
import type { PaymentStatus } from './store.ts';

/**
 * Statuses a payment never leaves. Under binary_mode a rejected payment stays
 * rejected, and so does one cancelled (expired unpaid): another card is a new
 * payment, with a new id, never the old one turning approved.
 */
const FINAL_STATUSES: ReadonlySet<PaymentStatus> = new Set(['rejected', 'cancelled']);

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
  if (FINAL_STATUSES.has(payment.status) && status !== payment.status) {
    sendError(res, 409, `payment ${String(id)} is ${payment.status}, which is final: pay again for a new payment`);
    return;
  }
  const pref = state.preferences.get(payment.preference_id);
  payment.status = status as PaymentStatus;
  if (pref) settle(payment, pref);
  if (payment.status === 'refunded') payment.transaction_amount_refunded = payment.transaction_amount;
  const webhook = await sendWebhook(payment, 'payment.updated');
  sendJson(res, 200, { payment, webhook });
}

/** A refund made from Mercado Pago's panel: nothing asks the backend, which hears of it by webhook alone. */
async function panelRefund(req: IncomingMessage, res: ServerResponse, id: number): Promise<void> {
  const payment = state.payments.get(id);
  if (!payment) {
    sendError(res, 404, `payment ${String(id)} not found`);
    return;
  }
  const result = applyRefund(payment, (await readJson(req)).amount, null, 'panel');
  if (!result.ok) {
    sendError(res, result.status, result.message);
    return;
  }
  const webhook = await sendWebhook(payment, 'payment.updated');
  sendJson(res, 201, { refund: refundRecord(result.refund), payment, webhook });
}

async function seedPaymentRoute(req: IncomingMessage, res: ServerResponse): Promise<void> {
  sendJson(res, 201, seedPayment(await readJson(req)));
}

function expireSellerRoute(res: ServerResponse, collectorId: number): void {
  sendJson(res, 200, { collector_id: collectorId, expired: expireSeller(collectorId) });
}

export const CONTROL_ROUTES: Route[] = [
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
      sendJson(res, 200, state.refunds.map(refundRecord));
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
  ['POST', /^\/__fake\/payments\/(\d+)\/refunds$/, (req, res, id) => panelRefund(req, res, Number(id))],
  ['POST', /^\/__fake\/payments$/, seedPaymentRoute],
  [
    'GET',
    /^\/__fake\/requests$/,
    (_req, res) => {
      sendJson(res, 200, state.requests);
    },
  ],
  [
    'POST',
    /^\/__fake\/sellers$/,
    (_req, res) => {
      sendJson(res, 201, registerSeller());
    },
  ],
  [
    'POST',
    /^\/__fake\/sellers\/(\d+)\/expire$/,
    (_req, res, id) => {
      expireSellerRoute(res, Number(id));
    },
  ],
];

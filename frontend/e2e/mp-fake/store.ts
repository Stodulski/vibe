/**
 * State, configuration and webhook delivery of the fake Mercado Pago (see ./server.ts).
 */
import { createHmac, randomUUID } from 'node:crypto';

export const PORT = Number(process.env.MP_FAKE_PORT ?? '8090');
export const PUBLIC_URL = (process.env.MP_FAKE_PUBLIC_URL ?? `http://localhost:${String(PORT)}`).replace(/\/+$/, '');
export const WEBHOOK_SECRET = process.env.MP_FAKE_WEBHOOK_SECRET ?? '';
export const COLLECTOR_ID_RAW = process.env.MP_FAKE_COLLECTOR_ID ?? '20260001';
export const BODY_LIMIT = 1 << 20;

if (!/^\d+$/.test(COLLECTOR_ID_RAW)) {
  console.error(`[mp-fake] MP_FAKE_COLLECTOR_ID must be numeric, got "${COLLECTOR_ID_RAW}"`);
  process.exit(1);
}
if (WEBHOOK_SECRET === '') {
  console.warn('[mp-fake] MP_FAKE_WEBHOOK_SECRET is empty: the backend refuses every webhook signed with it');
}
export const COLLECTOR_ID = Number(COLLECTOR_ID_RAW);

export type PaymentStatus =
  'approved' | 'rejected' | 'pending' | 'in_process' | 'cancelled' | 'refunded' | 'charged_back';
export type Json = Record<string, unknown>;

export interface Preference {
  id: string;
  /** The seller whose token created it; every payment made at it is credited to them. */
  collector_id: number;
  init_point: string;
  sandbox_init_point: string;
  date_created: string;
  // The request body as the backend sent it, merged with later PUTs.
  body: Json;
}

export interface FeeDetail {
  type: string;
  amount: number;
  fee_payer: string;
}

export interface Payment {
  id: number;
  preference_id: string;
  status: PaymentStatus;
  status_detail: string;
  transaction_amount: number;
  transaction_amount_refunded: number;
  net_received_amount: number;
  fee_details: FeeDetail[];
  metadata: Json;
  external_reference: string;
  collector_id: number;
  currency_id: string;
  date_created: string;
  date_approved: string | null;
  date_last_updated: string;
  /** Only on a seeded payment (no preference to read it from). */
  notification_url?: string;
}

export interface Refund {
  id: number;
  payment_id: number;
  amount: number;
  status: 'approved';
  date_created: string;
  idempotency_key: string | null;
  /** `api`: the backend asked for it; `panel`: made from Mercado Pago's own panel (a control call). */
  origin: 'api' | 'panel';
}

export interface WebhookDelivery {
  payment_id: number;
  action: WebhookAction;
  url: string;
  status: number | null;
  error: string | null;
  date_created: string;
}

export type WebhookAction = 'payment.created' | 'payment.updated';

/** One call the backend made to the Mercado Pago API, for specs to assert on (e.g. a 401 then a refresh). */
export interface RequestLogEntry {
  method: string;
  path: string;
  status: number;
  /** platform, seller:<collector>, expired:<collector>, unknown or none. */
  bearer: string;
  date_created: string;
}

export const state = {
  preferences: new Map<string, Preference>(),
  preferenceByIdempotencyKey: new Map<string, string>(),
  payments: new Map<number, Payment>(),
  refunds: [] as Refund[],
  webhooks: [] as WebhookDelivery[],
  requests: [] as RequestLogEntry[],
  nextPaymentId: 1_000_000_001,
  nextRefundId: 2_000_000_001,
};

export function resetState(): void {
  state.preferences.clear();
  state.preferenceByIdempotencyKey.clear();
  state.payments.clear();
  state.refunds = [];
  state.webhooks = [];
  state.requests = [];
}

export const STATUS_DETAIL: Record<PaymentStatus, string> = {
  approved: 'accredited',
  rejected: 'cc_rejected_other_reason',
  pending: 'pending_contingency',
  in_process: 'pending_review_manual',
  cancelled: 'expired',
  refunded: 'refunded',
  charged_back: 'settled',
};

export const round2 = (n: number): number => Math.round(n * 100) / 100;
export const now = (): string => new Date().toISOString();

export function isObject(v: unknown): v is Json {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

export function preferenceAmount(pref: Preference): number {
  const items = Array.isArray(pref.body.items) ? (pref.body.items as unknown[]) : [];
  return round2(
    items.reduce<number>((sum, item) => {
      if (!isObject(item)) return sum;
      const price = Number(item.unit_price ?? 0);
      const qty = Number(item.quantity ?? 1);
      return sum + price * qty;
    }, 0),
  );
}

export function feesFor(pref: Preference, amount: number, status: PaymentStatus): FeeDetail[] {
  if (status !== 'approved') return [];
  const fees: FeeDetail[] = [{ type: 'mercadopago_fee', amount: round2(amount * 0.0629), fee_payer: 'collector' }];
  const marketplaceFee = Number(pref.body.marketplace_fee ?? 0);
  if (marketplaceFee > 0)
    fees.push({ type: 'application_fee', amount: round2(marketplaceFee), fee_payer: 'collector' });
  return fees;
}

export function settle(payment: Payment, pref: Preference): void {
  payment.status_detail = STATUS_DETAIL[payment.status];
  payment.fee_details = feesFor(pref, payment.transaction_amount, payment.status);
  payment.net_received_amount =
    payment.status === 'approved'
      ? round2(payment.transaction_amount - payment.fee_details.reduce((s, f) => s + f.amount, 0))
      : 0;
  if (payment.status === 'approved' && payment.date_approved === null) payment.date_approved = now();
  payment.date_last_updated = now();
}

export function createPayment(pref: Preference, status: PaymentStatus): Payment {
  const created = now();
  const payment: Payment = {
    id: state.nextPaymentId++,
    preference_id: pref.id,
    status,
    status_detail: '',
    transaction_amount: preferenceAmount(pref),
    transaction_amount_refunded: 0,
    net_received_amount: 0,
    fee_details: [],
    metadata: isObject(pref.body.metadata) ? { ...pref.body.metadata } : {},
    external_reference: typeof pref.body.external_reference === 'string' ? pref.body.external_reference : '',
    collector_id: pref.collector_id,
    currency_id: 'ARS',
    date_created: created,
    date_approved: null,
    date_last_updated: created,
  };
  settle(payment, pref);
  state.payments.set(payment.id, payment);
  return payment;
}

/**
 * Delivers the payment notification the way Mercado Pago does, signed the way
 * `mp.VerifyWebhookSignature` checks it: `X-Signature: ts=<unix>,v1=<hex>`
 * over `id:<lowercase id>;request-id:<X-Request-Id>;ts:<ts>;`.
 */
export async function sendWebhook(payment: Payment, action: WebhookAction): Promise<WebhookDelivery> {
  const notificationURL = notificationURLOf(payment);
  const delivery: WebhookDelivery = {
    payment_id: payment.id,
    action,
    url: '',
    status: null,
    error: null,
    date_created: now(),
  };
  state.webhooks.push(delivery);
  if (notificationURL === '') {
    delivery.error = 'payment has no notification_url';
    return delivery;
  }

  const dataID = String(payment.id);
  const url = new URL(notificationURL);
  url.searchParams.set('data.id', dataID);
  url.searchParams.set('type', 'payment');
  delivery.url = url.toString();

  const requestID = randomUUID();
  // Milliseconds, as Mercado Pago documents and sends it (ts=1742505638683).
  const ts = String(Date.now());
  const signed = `id:${dataID.toLowerCase()};request-id:${requestID};ts:${ts};`;
  const v1 = createHmac('sha256', WEBHOOK_SECRET).update(signed).digest('hex');

  try {
    const res = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Signature': `ts=${ts},v1=${v1}`,
        'X-Request-Id': requestID,
      },
      body: JSON.stringify({ type: 'payment', action, data: { id: dataID } }),
      signal: AbortSignal.timeout(10_000),
    });
    delivery.status = res.status;
    await res.arrayBuffer();
  } catch (err) {
    delivery.error = err instanceof Error ? err.message : String(err);
    console.error(`[mp-fake] webhook for payment ${dataID} to ${delivery.url} failed: ${delivery.error}`);
  }
  return delivery;
}

/**
 * Where a payment's notifications go: its preference's notification_url, or
 * the one a seeded payment (made without a checkout) was given.
 */
function notificationURLOf(payment: Payment): string {
  if (payment.notification_url) return payment.notification_url;
  const pref = state.preferences.get(payment.preference_id);
  return pref && typeof pref.body.notification_url === 'string' ? pref.body.notification_url : '';
}

/**
 * Whether a preference can no longer be paid: Mercado Pago honours `expires`
 * with `expiration_date_from`/`expiration_date_to`, and the backend closes a
 * link by moving `expiration_date_to` into the past (PUT /checkout/preferences/{id}).
 */
export function isPreferenceExpired(pref: Preference, at = Date.now()): boolean {
  if (pref.body.expires !== true) return false;
  const to = typeof pref.body.expiration_date_to === 'string' ? Date.parse(pref.body.expiration_date_to) : NaN;
  if (!Number.isNaN(to) && at > to) return true;
  const from = typeof pref.body.expiration_date_from === 'string' ? Date.parse(pref.body.expiration_date_from) : NaN;
  // Second precision on the backend's side: allow the second the link was made in.
  return !Number.isNaN(from) && at < from - 1000;
}

export type RefundResult = { ok: true; refund: Refund } | { ok: false; status: number; message: string };

/**
 * Refunds `amount` pesos of a payment (all that remains when undefined), the
 * way Mercado Pago does: a full refund leaves the payment `refunded`, a partial
 * one keeps it `approved` with `status_detail: partially_refunded`, and either
 * adds to `transaction_amount_refunded`.
 */
export function applyRefund(
  payment: Payment,
  amount: unknown,
  idempotencyKey: string | null,
  origin: Refund['origin'],
): RefundResult {
  if (payment.status !== 'approved') {
    return {
      ok: false,
      status: 400,
      message: `payment ${String(payment.id)} is ${payment.status} and cannot be refunded`,
    };
  }
  const remaining = round2(payment.transaction_amount - payment.transaction_amount_refunded);
  const value = amount === undefined || amount === null ? remaining : round2(Number(amount));
  if (!(value > 0) || value > remaining) {
    return {
      ok: false,
      status: 400,
      message: `invalid refund amount ${String(amount)} (remaining ${String(remaining)})`,
    };
  }
  const refund: Refund = {
    id: state.nextRefundId++,
    payment_id: payment.id,
    amount: value,
    status: 'approved',
    date_created: now(),
    idempotency_key: idempotencyKey,
    origin,
  };
  state.refunds.push(refund);
  payment.transaction_amount_refunded = round2(payment.transaction_amount_refunded + value);
  if (payment.transaction_amount_refunded >= payment.transaction_amount) {
    payment.status = 'refunded';
    payment.status_detail = STATUS_DETAIL.refunded;
  } else {
    payment.status_detail = 'partially_refunded';
  }
  payment.date_last_updated = now();
  return { ok: true, refund };
}

/**
 * A payment that never went through a checkout: an approved Mercado Pago
 * payment for rows the fixtures write straight into the database, so the
 * backend can look it up and refund it like any other.
 */
export function seedPayment(body: Json): Payment {
  const amount = round2(Number(body.transaction_amount));
  if (!(amount > 0)) throw new Error(`transaction_amount must be positive, got ${String(body.transaction_amount)}`);
  const collector = body.collector_id === undefined ? COLLECTOR_ID : Number(body.collector_id);
  if (!Number.isInteger(collector))
    throw new Error(`collector_id must be an integer, got ${String(body.collector_id)}`);
  const created = now();
  const payment: Payment = {
    id: state.nextPaymentId++,
    preference_id: '',
    status: 'approved',
    status_detail: STATUS_DETAIL.approved,
    transaction_amount: amount,
    transaction_amount_refunded: 0,
    net_received_amount: round2(amount * (1 - 0.0629)),
    fee_details: [{ type: 'mercadopago_fee', amount: round2(amount * 0.0629), fee_payer: 'collector' }],
    metadata: isObject(body.metadata) ? { ...body.metadata } : {},
    external_reference: typeof body.external_reference === 'string' ? body.external_reference : '',
    collector_id: collector,
    currency_id: 'ARS',
    date_created: created,
    date_approved: created,
    date_last_updated: created,
  };
  if (typeof body.notification_url === 'string' && body.notification_url !== '') {
    payment.notification_url = body.notification_url;
  }
  state.payments.set(payment.id, payment);
  return payment;
}

/** A refund as the Mercado Pago API answers it. */
export function refundView(refund: Refund): Json {
  return { id: refund.id, payment_id: refund.payment_id, amount: refund.amount, status: refund.status };
}

/** A refund as the control API lists it. */
export function refundRecord(refund: Refund): Json {
  return { ...refundView(refund), origin: refund.origin, date_created: refund.date_created };
}

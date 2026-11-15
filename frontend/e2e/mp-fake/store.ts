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
}

export interface Refund {
  id: number;
  payment_id: number;
  amount: number;
  status: 'approved';
  date_created: string;
  idempotency_key: string | null;
}

export interface WebhookDelivery {
  payment_id: number;
  url: string;
  status: number | null;
  error: string | null;
  date_created: string;
}

export const state = {
  preferences: new Map<string, Preference>(),
  preferenceByIdempotencyKey: new Map<string, string>(),
  payments: new Map<number, Payment>(),
  refunds: [] as Refund[],
  webhooks: [] as WebhookDelivery[],
  nextPaymentId: 1_000_000_001,
  nextRefundId: 2_000_000_001,
};

export function resetState(): void {
  state.preferences.clear();
  state.preferenceByIdempotencyKey.clear();
  state.payments.clear();
  state.refunds = [];
  state.webhooks = [];
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
    collector_id: COLLECTOR_ID,
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
export async function sendWebhook(payment: Payment): Promise<WebhookDelivery> {
  const pref = state.preferences.get(payment.preference_id);
  const notificationURL = pref && typeof pref.body.notification_url === 'string' ? pref.body.notification_url : '';
  const delivery: WebhookDelivery = { payment_id: payment.id, url: '', status: null, error: null, date_created: now() };
  state.webhooks.push(delivery);
  if (notificationURL === '') {
    delivery.error = 'preference has no notification_url';
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
      body: JSON.stringify({ type: 'payment', action: 'payment.created', data: { id: dataID } }),
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

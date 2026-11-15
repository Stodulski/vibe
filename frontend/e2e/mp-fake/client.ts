/**
 * Specs' handle on the fake Mercado Pago (`./server.ts`), which `make e2e`
 * starts and announces through MP_FAKE_URL.
 *
 * The fake is shared by every worker: prefer finding a payment by its
 * `external_reference` (the booking or registration id) over `reset()`, which
 * wipes what parallel specs are still asserting on.
 */

/** Base URL of the fake. */
export const MP_FAKE_URL = (process.env.MP_FAKE_URL ?? 'http://localhost:8090').replace(/\/+$/, '');

/**
 * The seller id the fake puts on every payment as `collector_id`. Fixtures
 * seed it as the complex's `mp_user_id`: the backend drops a payment whose
 * collector does not match. `make e2e` passes the same value to the fake.
 */
export const MP_FAKE_COLLECTOR_ID = process.env.MP_FAKE_COLLECTOR_ID ?? '20260001';

/**
 * The seller tokens the fixtures seal into every complex they connect. The fake
 * knows them from the start (./auth.ts) as the seller MP_FAKE_COLLECTOR_ID.
 */
export const MP_FAKE_SEED_SELLER = {
  access_token: 'fake-e2e-access-token',
  refresh_token: 'fake-e2e-refresh-token',
} as const;

/** The marketplace's own token, as `make e2e` gives it to the API and the fake. */
export const MP_FAKE_PLATFORM_TOKEN = process.env.MP_FAKE_PLATFORM_TOKEN ?? 'e2e-fake-platform-token';

/** The accessible names of the fake checkout's buttons. */
export const MP_FAKE_CHECKOUT = {
  approve: 'Aprobar pago',
  reject: 'Rechazar pago',
  leavePending: 'Dejar pendiente',
} as const;

export interface FakePreference {
  id: string;
  init_point: string;
  date_created: string;
  body: Record<string, unknown>;
}

export interface FakePayment {
  id: number;
  preference_id: string;
  status: string;
  status_detail: string;
  transaction_amount: number;
  transaction_amount_refunded: number;
  net_received_amount: number;
  metadata: Record<string, unknown>;
  external_reference: string;
  collector_id: number;
}

export interface FakeRefund {
  id: number;
  payment_id: number;
  amount: number;
  status: string;
  /** `api`: the backend asked for it; `panel`: made from Mercado Pago's panel (`mpFake.panelRefund`). */
  origin: 'api' | 'panel';
}

export interface FakeSeller {
  collector_id: number;
  access_token: string;
  refresh_token: string;
}

export interface FakeWebhook {
  payment_id: number;
  action: string;
  url: string;
  status: number | null;
  error: string | null;
}

/** One Mercado Pago API call the backend made. `bearer` is platform, seller:<id>, expired:<id>, unknown or none. */
export interface FakeRequest {
  method: string;
  path: string;
  status: number;
  bearer: string;
  date_created: string;
}

async function call<T>(method: 'GET' | 'POST', path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method };
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' };
    init.body = JSON.stringify(body);
  }
  const res = await fetch(`${MP_FAKE_URL}${path}`, init);
  if (!res.ok) {
    throw new Error(`mp-fake ${method} ${path}: HTTP ${String(res.status)} ${await res.text()}`);
  }
  return (await res.json()) as T;
}

export const mpFake = {
  preferences: () => call<FakePreference[]>('GET', '/__fake/preferences'),
  payments: () => call<FakePayment[]>('GET', '/__fake/payments'),
  refunds: () => call<FakeRefund[]>('GET', '/__fake/refunds'),
  webhooks: () => call<FakeWebhook[]>('GET', '/__fake/webhooks'),
  requests: () => call<FakeRequest[]>('GET', '/__fake/requests'),
  reset: () => call<{ ok: boolean }>('POST', '/__fake/reset'),

  /** A new seller with its own collector id and tokens, for a spec that must not share them (e.g. to expire them). */
  registerSeller: () => call<FakeSeller>('POST', '/__fake/sellers'),

  /** Expires every access token the seller holds now: its next API call gets 401 until the backend refreshes. */
  expireSeller: (collectorId: number) =>
    call<{ expired: number }>('POST', `/__fake/sellers/${String(collectorId)}/expire`),

  /**
   * A refund made from Mercado Pago's own panel, outside the app: the fake
   * refunds `amount` pesos (all that remains when omitted) and sends the
   * payment.updated webhook, and the backend learns of it only from that.
   */
  panelRefund: (paymentId: number, amount?: number) =>
    call<{ refund: FakeRefund; payment: FakePayment; webhook: FakeWebhook }>(
      'POST',
      `/__fake/payments/${String(paymentId)}/refunds`,
      amount === undefined ? {} : { amount },
    ),

  /**
   * An approved payment made without a checkout, for rows a fixture writes
   * straight into the database. `transaction_amount` is in pesos.
   */
  seedPayment: (payment: {
    transaction_amount: number;
    external_reference: string;
    metadata?: Record<string, unknown>;
    collector_id?: number;
    notification_url?: string;
  }) => call<FakePayment>('POST', '/__fake/payments', payment),

  /** Webhooks the fake sent for one payment, oldest first. */
  async webhooksOf(paymentId: number): Promise<FakeWebhook[]> {
    return (await this.webhooks()).filter((w) => w.payment_id === paymentId);
  },

  /** Payments whose external_reference is `reference` (a booking or registration id), oldest first. */
  async paymentsFor(reference: string): Promise<FakePayment[]> {
    return (await this.payments()).filter((p) => p.external_reference === reference);
  },

  /** Payments made through one checkout preference (the booking answer's `mp_preference_id`), oldest first. */
  async paymentsOfPreference(preferenceId: string): Promise<FakePayment[]> {
    return (await this.payments()).filter((p) => p.preference_id === preferenceId);
  },

  /** Refunds of one payment. */
  async refundsOf(paymentId: number): Promise<FakeRefund[]> {
    return (await this.refunds()).filter((r) => r.payment_id === paymentId);
  },

  /** Flips a payment's status (e.g. a pending one to approved) and re-sends its webhook. */
  setPaymentStatus: (paymentId: number, status: string) =>
    call<{ payment: FakePayment }>('POST', `/__fake/payments/${String(paymentId)}/status`, { status }),
};

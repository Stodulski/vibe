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
  reset: () => call<{ ok: boolean }>('POST', '/__fake/reset'),

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

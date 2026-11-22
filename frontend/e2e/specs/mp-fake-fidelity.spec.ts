/* eslint-disable max-lines -- one serial story against one seller of its own at the fake, provisioned once */
import { test, expect, request as apiRequest } from '@playwright/test';
import { provisionWorld, type EventsWorld } from '../helpers/events-fixtures';
import { connectOwnFakeSeller, waitForOnlineBooking } from '../helpers/public-booking-fixtures';
import {
  bookAndPay,
  bookStorefrontUpToCheckout,
  ownerBooking,
  paymentOfPreference,
  recordedPayment,
  referenceOfPreference,
  waitForBookingStatus,
  waitForDeliveredWebhook,
  waitForProcessedWebhooks,
  waitForRefundOf,
  type StorefrontClient,
} from '../helpers/mp-checkout';
import {
  MP_FAKE_CHECKOUT,
  MP_FAKE_PLATFORM_TOKEN,
  MP_FAKE_SEED_SELLER,
  MP_FAKE_URL,
  mpFake,
  type FakeRequest,
  type FakeSeller,
} from '../mp-fake/client';

/**
 * What the fake Mercado Pago does the way the real one does, and what the
 * backend does with it (odd/tasks/mp-fake-e2e.md S1f):
 *   1. credentials are checked per call: the collecting seller's token for
 *      preferences and refunds, the platform's for reading a payment, 401/403
 *      otherwise; an expired seller token makes the backend refresh and retry;
 *   2. a preference past its expiry (the backend closes a cancelled booking's
 *      link) cannot be paid;
 *   3. every refund is followed by a signed payment.updated webhook, including
 *      refunds made from Mercado Pago's own panel, partial or full;
 *   4. a link can be paid more than once while open, and the backend refunds
 *      the payment that did not settle the booking (S1g).
 *
 * One complex of its own, connected to a seller of its own at the fake so its
 * tokens can be expired without touching anybody else's checkout. Tests run in
 * order: each storefront booking holds the slot it takes.
 */
test.describe.configure({ mode: 'serial' });

let world: EventsWorld;
let seller: FakeSeller;

function newClient(firstName: string): StorefrontClient {
  const stamp = String(Date.now()).slice(-8);
  return { firstName, lastName: `Fid${stamp}`, phone: `11${stamp}`, email: `fid${stamp}@test.com` };
}

async function fakeCall(method: string, path: string, token: string | null, body?: unknown): Promise<number> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token !== null) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`${MP_FAKE_URL}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  await res.arrayBuffer();
  return res.status;
}

/** A fake API answer: its status and JSON body, cast by the caller to the shape the backend decodes. */
interface Answer<T> {
  status: number;
  body: T;
}

/** A fake API call whose answer the test reads. */
async function fakeJson(method: string, path: string, token: string, body?: unknown): Promise<Answer<unknown>> {
  const res = await fetch(`${MP_FAKE_URL}${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return { status: res.status, body: await res.json() };
}

/** The fields of a searched payment the backend reads (mp.Payment). */
interface SearchedPayment {
  id: number;
  status: string;
  status_detail: string;
  transaction_amount: number;
  transaction_amount_refunded: number;
  external_reference: string;
  date_created: string;
  collector_id: number;
  metadata: Record<string, unknown>;
}

/** A preference as read back (mp.Preference). */
interface ReadPreference {
  id: string;
  init_point: string;
  collector_id: number;
  expires?: boolean;
  expiration_date_to?: string;
}

/** The backend's Mercado Pago calls made after `since` (ms). */
async function requestsSince(since: number): Promise<FakeRequest[]> {
  return (await mpFake.requests()).filter((r) => Date.parse(r.date_created) >= since);
}

const centavos = (pesos: number): number => Math.round(pesos * 100);

test.beforeAll(async () => {
  test.setTimeout(180_000);
  world = await provisionWorld('mp-fidelity', 1, { deposit_percentage: 50, cancellation_hours: 1 });
  seller = await connectOwnFakeSeller(world.complexId);
  const ctx = await apiRequest.newContext();
  await waitForOnlineBooking(ctx, world.slug);
  await ctx.dispose();
});

test('the fake refuses a missing, unknown or wrong party token the way Mercado Pago does', async () => {
  const preference = { items: [{ title: 'x', quantity: 1, unit_price: 100 }] };
  expect(await fakeCall('POST', '/checkout/preferences', null, preference)).toBe(401);
  expect(await fakeCall('POST', '/checkout/preferences', 'APP_USR-not-issued', preference)).toBe(401);
  expect(await fakeCall('POST', '/checkout/preferences', MP_FAKE_PLATFORM_TOKEN, preference)).toBe(403);

  // A payment collected by this spec's seller.
  const payment = await mpFake.seedPayment({
    transaction_amount: 1000,
    external_reference: `fidelity-${String(Date.now())}`,
    collector_id: seller.collector_id,
  });
  const paymentPath = `/v1/payments/${String(payment.id)}`;
  expect(await fakeCall('GET', paymentPath, MP_FAKE_PLATFORM_TOKEN)).toBe(200);
  expect(await fakeCall('GET', paymentPath, seller.access_token)).toBe(200);
  expect(await fakeCall('GET', paymentPath, MP_FAKE_SEED_SELLER.access_token)).toBe(403);
  expect(await fakeCall('GET', paymentPath, null)).toBe(401);

  // Only the collector refunds: the money leaves whoever authorizes it.
  const refund = { amount: 10 };
  expect(await fakeCall('POST', `${paymentPath}/refunds`, MP_FAKE_PLATFORM_TOKEN, refund)).toBe(403);
  expect(await fakeCall('POST', `${paymentPath}/refunds`, MP_FAKE_SEED_SELLER.access_token, refund)).toBe(403);
  expect(await fakeCall('POST', `${paymentPath}/refunds`, 'APP_USR-not-issued', refund)).toBe(401);
  expect(await mpFake.refundsOf(payment.id)).toHaveLength(0);

  // The OAuth client must be the application's own.
  expect(
    await fakeCall('POST', '/oauth/token', null, {
      grant_type: 'refresh_token',
      client_id: 'someone-else',
      client_secret: 'wrong',
      refresh_token: seller.refresh_token,
    }),
  ).toBe(401);
});

test("a payment search by reference answers the seller's own payments, newest first, as the backend reads them", async () => {
  const reference = `fidelity-search-${String(Date.now())}`;
  const older = await mpFake.seedPayment({
    transaction_amount: 1000,
    external_reference: reference,
    collector_id: seller.collector_id,
    metadata: { booking_id: reference },
  });
  const newer = await mpFake.seedPayment({
    transaction_amount: 2500.5,
    external_reference: reference,
    collector_id: seller.collector_id,
  });
  // Same reference, collected by another seller: not this seller's money.
  const foreign = await mpFake.seedPayment({ transaction_amount: 700, external_reference: reference });
  // This seller's, under another reference.
  await mpFake.seedPayment({
    transaction_amount: 900,
    external_reference: `${reference}-other`,
    collector_id: seller.collector_id,
  });

  // The query mp.SearchPaymentsByReference sends.
  const path = `/v1/payments/search?${new URLSearchParams({
    external_reference: reference,
    sort: 'date_created',
    criteria: 'desc',
    limit: '50',
  }).toString()}`;
  const mine = (await fakeJson('GET', path, seller.access_token)) as Answer<{
    paging: { total: number };
    results: SearchedPayment[];
  }>;
  expect(mine.status).toBe(200);
  expect(mine.body.paging.total).toBe(2);
  expect(mine.body.results.map((p) => p.id)).toEqual([newer.id, older.id]);
  const [first, second] = mine.body.results;
  expect(first).toMatchObject({
    status: 'approved',
    status_detail: 'accredited',
    transaction_amount: 2500.5,
    transaction_amount_refunded: 0,
    external_reference: reference,
    collector_id: seller.collector_id,
  });
  expect(Number.isNaN(Date.parse(first?.date_created ?? ''))).toBe(false);
  expect(second?.metadata).toEqual({ booking_id: reference });

  // Another seller sees only what it collected; the platform sees all three.
  const theirs = (await fakeJson('GET', path, MP_FAKE_SEED_SELLER.access_token)) as Answer<{
    results: SearchedPayment[];
  }>;
  expect(theirs.body.results.map((p) => p.id)).toEqual([foreign.id]);
  const platform = (await fakeJson('GET', path, MP_FAKE_PLATFORM_TOKEN)) as Answer<{ results: SearchedPayment[] }>;
  expect(platform.body.results.map((p) => p.id)).toEqual([foreign.id, newer.id, older.id]);
  expect(await fakeCall('GET', path, null)).toBe(401);
});

test('a preference reads back only to its collector, and a closed one reads back as closed', async () => {
  const created = (await fakeJson('POST', '/checkout/preferences', seller.access_token, {
    items: [{ title: 'Reserva', quantity: 1, unit_price: 1000 }],
    external_reference: `fidelity-pref-${String(Date.now())}`,
  })) as Answer<ReadPreference>;
  expect(created.status).toBe(201);
  const path = `/checkout/preferences/${created.body.id}`;

  const open = (await fakeJson('GET', path, seller.access_token)) as Answer<ReadPreference>;
  expect(open.status).toBe(200);
  expect(open.body).toMatchObject({
    id: created.body.id,
    init_point: created.body.init_point,
    collector_id: seller.collector_id,
  });
  expect(open.body.expires ?? false).toBe(false);

  expect(await fakeCall('GET', path, MP_FAKE_SEED_SELLER.access_token)).toBe(403);
  expect(await fakeCall('GET', path, MP_FAKE_PLATFORM_TOKEN)).toBe(403);
  expect(await fakeCall('GET', path, null)).toBe(401);

  // Closed the way mp.UpdatePreferenceExpired closes it: expiry moved into the past.
  const closedAt = new Date(Date.now() - 60_000).toISOString();
  expect(await fakeCall('PUT', path, seller.access_token, { expires: true, expiration_date_to: closedAt })).toBe(200);
  const closed = (await fakeJson('GET', path, seller.access_token)) as Answer<ReadPreference>;
  expect(closed.status).toBe(200);
  expect(closed.body.expires).toBe(true);
  expect(Date.parse(closed.body.expiration_date_to ?? '')).toBeLessThan(Date.now());
});

test('an expired seller token is refreshed through OAuth and the checkout is created on the retry', async ({
  page,
}) => {
  test.slow();
  const since = Date.now();
  const { expired } = await mpFake.expireSeller(seller.collector_id);
  expect(expired).toBeGreaterThan(0);

  const preferenceId = await bookStorefrontUpToCheckout(page, world.slug, newClient('Refresh'));
  const bookingId = await referenceOfPreference(preferenceId);

  const calls = (await requestsSince(since)).map((r) => `${r.method} ${r.path} ${String(r.status)} ${r.bearer}`);
  const c = String(seller.collector_id);
  const rejected = calls.indexOf(`POST /checkout/preferences 401 expired:${c}`);
  const refreshed = calls.indexOf('POST /oauth/token 200 none');
  const created = calls.indexOf(`POST /checkout/preferences 201 seller:${c}`);
  expect(rejected, calls.join('\n')).toBeGreaterThanOrEqual(0);
  expect(refreshed, calls.join('\n')).toBeGreaterThan(rejected);
  expect(created, calls.join('\n')).toBeGreaterThan(refreshed);

  // The refreshed token keeps working: the payment confirms the booking.
  await page.getByRole('button', { name: MP_FAKE_CHECKOUT.approve }).click();
  const payment = await paymentOfPreference(preferenceId);
  expect(payment.collector_id).toBe(seller.collector_id);
  await waitForBookingStatus(world, bookingId, 'confirmed');
});

test('a booking cancelled before payment closes its checkout link, which can no longer be paid', async ({ page }) => {
  test.slow();
  const preferenceId = await bookStorefrontUpToCheckout(page, world.slug, newClient('Vencido'));
  const bookingId = await referenceOfPreference(preferenceId);
  const checkoutURL = page.url();

  const since = Date.now();
  await world.api.cancelBooking(world.complexId, bookingId);
  const calls = (await requestsSince(since)).map((r) => `${r.method} ${r.path} ${String(r.status)} ${r.bearer}`);
  expect(calls, calls.join('\n')).toContain(
    `PUT /checkout/preferences/${preferenceId} 200 seller:${String(seller.collector_id)}`,
  );

  await page.reload();
  await expect(page.getByRole('heading', { name: 'Link de pago vencido' })).toBeVisible();
  await expect(page.getByRole('button', { name: MP_FAKE_CHECKOUT.approve })).toHaveCount(0);

  // Submitting the form the page had before is refused too.
  const submit = await page.request.post(checkoutURL, { form: { outcome: 'approved' }, maxRedirects: 0 });
  expect(submit.status()).toBe(403);
  expect(await mpFake.paymentsOfPreference(preferenceId)).toHaveLength(0);
  expect((await ownerBooking(world, bookingId)).status).toBe('cancelled');
});

test("an in-app refund's webhook is not recorded as a second refund", async ({ page }) => {
  test.slow();
  const { booking, payment } = await bookAndPay(page, world, newClient('Webhook'));

  await world.api.cancelBooking(world.complexId, booking.id);
  await waitForRefundOf(payment.id, payment.transaction_amount);
  await waitForDeliveredWebhook(payment.id, 'payment.updated');
  // payment.created and payment.updated, both worked.
  await waitForProcessedWebhooks(payment.id, 2);

  const refunds = await mpFake.refundsOf(payment.id);
  expect(refunds.map((r) => r.origin)).toEqual(['api']);
  const recorded = recordedPayment(payment.id);
  expect(recorded?.status).toBe('refunded');
  expect(recorded?.refundAmount).toBe(centavos(payment.transaction_amount));
  const after = await ownerBooking(world, booking.id);
  expect(after.status).toBe('cancelled');
  expect(after.refund_status).toBe('full');
});

test('a full refund made from the Mercado Pago panel cancels the booking and is recorded as refunded', async ({
  page,
}) => {
  test.slow();
  const { booking, payment } = await bookAndPay(page, world, newClient('PanelTotal'));

  const { webhook, payment: refunded } = await mpFake.panelRefund(payment.id);
  expect(refunded.status).toBe('refunded');
  expect(refunded.transaction_amount_refunded).toBe(payment.transaction_amount);
  expect(webhook.status).toBe(200);
  await waitForProcessedWebhooks(payment.id, 2);

  const after = await waitForBookingStatus(world, booking.id, 'cancelled');
  expect(after.refund_status).toBe('full');
  const recorded = recordedPayment(payment.id);
  expect(recorded?.status).toBe('refunded');
  expect(recorded?.refundAmount).toBe(centavos(payment.transaction_amount));
  // The backend did not refund it again on its own.
  expect((await mpFake.refundsOf(payment.id)).map((r) => r.origin)).toEqual(['panel']);
});

/** All refunds of the payments made at one preference. */
async function refundsOfPreference(preferenceId: string) {
  const ids = new Set((await mpFake.paymentsOfPreference(preferenceId)).map((p) => p.id));
  return (await mpFake.refunds()).filter((r) => ids.has(r.payment_id));
}

test('paying the same checkout link twice confirms the booking once and refunds the second payment in full', async ({
  page,
}) => {
  test.slow();
  const preferenceId = await bookStorefrontUpToCheckout(page, world.slug, newClient('Doble'));
  const checkoutURL = page.url();
  const bookingId = await referenceOfPreference(preferenceId);
  await page.getByRole('button', { name: MP_FAKE_CHECKOUT.approve }).click();
  await expect(page).toHaveURL(new RegExp(`${world.slug}/book/success`), { timeout: 15_000 });
  const first = await paymentOfPreference(preferenceId);
  await waitForBookingStatus(world, bookingId, 'confirmed');

  // The link is still open (Mercado Pago lets a preference be paid until it expires).
  await page.goto(checkoutURL);
  await page.getByRole('button', { name: MP_FAKE_CHECKOUT.approve }).click();
  await expect(page).toHaveURL(new RegExp(`${world.slug}/book/success`), { timeout: 15_000 });
  // Two payments; the second may already read refunded (the backend refunds it while working its webhook).
  const payments = await mpFake.paymentsOfPreference(preferenceId);
  expect(payments.map((p) => p.id)).toHaveLength(2);
  expect(payments[0]?.status).toBe('approved');
  const second = payments[1];
  if (!second) throw new Error('no second payment');

  await waitForRefundOf(second.id, second.transaction_amount);
  const refunds = await refundsOfPreference(preferenceId);
  expect(refunds.map((r) => [r.payment_id, r.amount])).toEqual([[second.id, second.transaction_amount]]);

  const booking = await ownerBooking(world, bookingId);
  expect(booking.status).toBe('confirmed');
  expect(booking.collection_status).toBe('deposit_paid');
  expect(recordedPayment(first.id)?.status).not.toBe('refunded');
  expect(recordedPayment(first.id)?.refundAmount).toBe(0);
  expect(recordedPayment(second.id)?.status).toBe('refunded');
});

test('a pending payment approved after a second, approved one is refunded in full', async ({ page }) => {
  test.slow();
  const preferenceId = await bookStorefrontUpToCheckout(page, world.slug, newClient('Tardio'));
  const checkoutURL = page.url();
  const bookingId = await referenceOfPreference(preferenceId);
  await page.getByRole('button', { name: MP_FAKE_CHECKOUT.leavePending }).click();
  await expect(page).toHaveURL(new RegExp(`${world.slug}/book/success`), { timeout: 15_000 });
  const pending = await paymentOfPreference(preferenceId);
  expect(pending.status).toBe('pending');

  // The client pays again with another method, which is approved at once.
  await page.goto(checkoutURL);
  await page.getByRole('button', { name: MP_FAKE_CHECKOUT.approve }).click();
  await expect(page).toHaveURL(new RegExp(`${world.slug}/book/success`), { timeout: 15_000 });
  const approved = (await mpFake.paymentsOfPreference(preferenceId)).find((p) => p.status === 'approved');
  if (!approved) throw new Error('no approved payment');
  await waitForBookingStatus(world, bookingId, 'confirmed');

  // Then Mercado Pago approves the first one too.
  await mpFake.setPaymentStatus(pending.id, 'approved');
  await waitForRefundOf(pending.id, pending.transaction_amount);
  const refunds = await refundsOfPreference(preferenceId);
  expect(refunds.map((r) => [r.payment_id, r.amount])).toEqual([[pending.id, pending.transaction_amount]]);

  const booking = await ownerBooking(world, bookingId);
  expect(booking.status).toBe('confirmed');
  expect(booking.collection_status).toBe('deposit_paid');
  expect(recordedPayment(approved.id)?.refundAmount).toBe(0);
  expect(recordedPayment(pending.id)?.status).toBe('refunded');
});

// Last on purpose: it fails against the backend today (reported, odd/tasks/mp-fake-e2e.md T5),
// and a failure in serial mode skips every test after it.
test('a partial refund made from the Mercado Pago panel is recorded, and the booking is not refunded whole', async ({
  page,
}) => {
  test.slow();
  const { booking, payment } = await bookAndPay(page, world, newClient('PanelParcial'));
  const half = Math.round(payment.transaction_amount * 50) / 100;

  const { webhook, payment: refunded } = await mpFake.panelRefund(payment.id, half);
  expect(refunded.status).toBe('approved');
  expect(refunded.status_detail).toBe('partially_refunded');
  expect(refunded.transaction_amount_refunded).toBe(half);
  expect(webhook.status).toBe(200);
  await waitForProcessedWebhooks(payment.id, 2);

  // The product should record the money that went back.
  await expect
    .poll(() => recordedPayment(payment.id)?.refundAmount, {
      timeout: 15_000,
      message: 'the payment row recording the partial refund',
    })
    .toBe(centavos(half));
  const recorded = recordedPayment(payment.id);
  expect(recorded?.status).not.toBe('refunded');
  const after = await ownerBooking(world, booking.id);
  expect(after.status).toBe('confirmed');
  expect(after.refund_status).not.toBe('full');
});

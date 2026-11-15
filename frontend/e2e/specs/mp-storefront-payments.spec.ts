import { test, expect, request as apiRequest, type Browser, type Page } from '@playwright/test';
import { ES_AR } from '../../src/shared/i18n/es_AR';
import { BOOKING_MESSAGES } from '../../../packages/booking/src/messages';
import { landingComplexUrl, waitForStorefrontHydration } from '../helpers/landing';
import { openOwnerPage, provisionWorld, type EventsWorld } from '../helpers/events-fixtures';
import { waitForOnlineBooking } from '../helpers/public-booking-fixtures';
import {
  bookStorefrontUpToCheckout,
  payAtCheckout,
  paymentOfPreference,
  referenceOfPreference,
  waitForRefundOf,
  type StorefrontClient,
} from '../helpers/mp-checkout';
import { mpFake } from '../mp-fake/client';

/**
 * Storefront bookings paid through the fake Mercado Pago (e2e/mp-fake), from
 * the client's checkout to what the owner reads, and the refunds a paid
 * booking's cancellation sends back (odd/tasks/mp-fake-e2e.md S1d).
 *
 * Two complexes of their own, so no other spec takes their slots: one charges
 * a 50% deposit online, the other the full price. Both let a client cancel
 * with a refund up to one hour before the turn, so the first free slot the
 * storefront offers (tomorrow at the earliest) is always inside the window.
 * Tests run in order: each storefront booking holds the slot it takes, and
 * the next one picks the next free one.
 */
test.describe.configure({ mode: 'serial' });

const t = ES_AR.bookings;
const pb = BOOKING_MESSAGES.publicBooking;

interface OwnerBooking {
  id: string;
  date: string;
  status: string;
  collection_status: string;
  refund_status: string;
  deposit_amount: number;
  price: number;
}

let depositWorld: EventsWorld;
let fullWorld: EventsWorld;

/** A client nobody else in the run uses: the backend keys clients by phone. */
function newClient(firstName: string): StorefrontClient {
  const stamp = String(Date.now()).slice(-8);
  return { firstName, lastName: `Mp${stamp}`, phone: `11${stamp}`, email: `mp${stamp}@test.com` };
}

async function ownerBooking(world: EventsWorld, bookingId: string): Promise<OwnerBooking> {
  const { booking } = await world.api.get<{ booking: OwnerBooking }>(
    `/complexes/${world.complexId}/bookings/${bookingId}`,
  );
  return booking;
}

/** Waits until the webhook worked the booking into `status`, and returns it. */
async function waitForBookingStatus(world: EventsWorld, bookingId: string, status: string): Promise<OwnerBooking> {
  await expect
    .poll(async () => (await ownerBooking(world, bookingId)).status, {
      timeout: 30_000,
      message: `booking ${bookingId} reaching ${status}`,
    })
    .toBe(status);
  return ownerBooking(world, bookingId);
}

/** Books, pays "Aprobar pago" and waits for the confirmation. Returns the booking, its payment and the link token. */
async function bookAndPay(page: Page, world: EventsWorld, client: StorefrontClient) {
  const preferenceId = await bookStorefrontUpToCheckout(page, world.slug, client);
  const bookingId = await referenceOfPreference(preferenceId);
  await payAtCheckout(page, 'approve');
  await expect(page).toHaveURL(new RegExp(`${world.slug}/book/success\\?token=`), { timeout: 15_000 });
  const token = new URL(page.url()).searchParams.get('token') ?? '';
  expect(token).not.toBe('');
  const payment = await paymentOfPreference(preferenceId);
  expect(payment.status).toBe('approved');
  const booking = await waitForBookingStatus(world, bookingId, 'confirmed');
  return { booking, payment, token };
}

/** Signs the owner in and opens the booking's detail from the day's grid. */
async function openOwnerDetail(browser: Browser, world: EventsWorld, booking: OwnerBooking, client: StorefrontClient) {
  const owner = await openOwnerPage(browser, world.owner);
  await owner.page.goto(`/bookings?date=${booking.date.slice(0, 10)}`);
  await expect(owner.page.getByRole('heading', { name: t.title, exact: true })).toBeVisible({ timeout: 10_000 });
  await owner.page.getByText(`${client.firstName} ${client.lastName}`).first().click();
  const detail = owner.page.getByRole('dialog').filter({ hasText: ES_AR.publicBooking.bookingRef });
  await expect(detail).toBeVisible({ timeout: 5_000 });
  return { ...owner, detail };
}

test.beforeAll(async () => {
  test.setTimeout(180_000);
  depositWorld = await provisionWorld('mp-deposit', 1, { deposit_percentage: 50, cancellation_hours: 1 });
  fullWorld = await provisionWorld('mp-full', 1, { deposit_percentage: 0, cancellation_hours: 1 });
  const ctx = await apiRequest.newContext();
  await waitForOnlineBooking(ctx, depositWorld.slug);
  await waitForOnlineBooking(ctx, fullWorld.slug);
  await ctx.dispose();
});

test('a deposit paid at checkout confirms the booking and the owner reads "Seña pagada"', async ({ page, browser }) => {
  test.slow();
  const client = newClient('Sena');
  const { booking, payment } = await bookAndPay(page, depositWorld, client);
  await expect(page.getByText(pb.bookingSuccess)).toBeVisible({ timeout: 30_000 });

  expect(booking.collection_status).toBe('deposit_paid');
  expect(booking.deposit_amount).toBe(booking.price / 2);
  expect(payment.transaction_amount).toBeGreaterThanOrEqual(booking.deposit_amount / 100);

  const owner = await openOwnerDetail(browser, depositWorld, booking, client);
  await expect(owner.detail.getByText(t.paymentStatuses.deposit_paid)).toBeVisible();
  await expect(owner.detail.getByText(t.statuses.confirmed)).toBeVisible();
  await owner.context.close();
});

test('"Rechazar pago" lands on the failure page and the booking is not confirmed', async ({ page }) => {
  test.slow();
  const preferenceId = await bookStorefrontUpToCheckout(page, fullWorld.slug, newClient('Rechazo'));
  const bookingId = await referenceOfPreference(preferenceId);
  await payAtCheckout(page, 'reject');

  // back_urls.failure: the storefront's booking page with the payment error.
  await expect(page).toHaveURL(new RegExp(`/c/${fullWorld.slug}/book\\?error=payment_failed`), { timeout: 15_000 });
  await expect(page.getByText(pb.paymentFailed)).toBeVisible({ timeout: 15_000 });
  const payment = await paymentOfPreference(preferenceId);
  expect(payment.status).toBe('rejected');

  // The rejection is worked after the redirect: give it the time an approval
  // takes and check the booking never confirms nor counts money.
  for (let i = 0; i < 6; i++) {
    const booking = await ownerBooking(fullWorld, bookingId);
    expect(booking.status).not.toBe('confirmed');
    expect(booking.collection_status).toBe('unpaid');
    await page.waitForTimeout(500);
  }
});

test('"Dejar pendiente" shows the payment under review, and its later approval confirms the booking', async ({
  page,
}) => {
  test.slow();
  const preferenceId = await bookStorefrontUpToCheckout(page, depositWorld.slug, newClient('Pendiente'));
  const bookingId = await referenceOfPreference(preferenceId);
  await payAtCheckout(page, 'leavePending');

  // back_urls.pending: the success page told the payment is still under review.
  await expect(page).toHaveURL(new RegExp(`${depositWorld.slug}/book/success\\?token=.*status=pending`), {
    timeout: 15_000,
  });
  await expect(page.getByText(pb.paymentUnderReview)).toBeVisible({ timeout: 15_000 });
  const payment = await paymentOfPreference(preferenceId);
  expect(payment.status).toBe('pending');
  expect((await ownerBooking(depositWorld, bookingId)).status).toBe('pending');

  // Mercado Pago approves it later and sends a second webhook for the same payment.
  await mpFake.setPaymentStatus(payment.id, 'approved');
  const booking = await waitForBookingStatus(depositWorld, bookingId, 'confirmed');
  expect(booking.collection_status).toBe('deposit_paid');
});

test('the owner cancels a paid booking: the refund reaches Mercado Pago and the detail reads "Reembolso total"', async ({
  page,
  browser,
}) => {
  test.slow();
  const client = newClient('Anula');
  const { booking, payment } = await bookAndPay(page, depositWorld, client);

  const owner = await openOwnerDetail(browser, depositWorld, booking, client);
  await owner.detail.getByRole('button', { name: t.cancel }).click();
  const cancelDialog = owner.page.getByRole('alertdialog');
  await expect(cancelDialog).toBeVisible({ timeout: 5_000 });
  await cancelDialog.getByRole('button', { name: t.confirmCancel }).click();
  await expect(cancelDialog).toBeHidden({ timeout: 10_000 });

  const refunds = await waitForRefundOf(payment.id, payment.transaction_amount);
  expect(refunds.every((r) => r.status === 'approved')).toBe(true);
  const after = await ownerBooking(depositWorld, booking.id);
  expect(after.status).toBe('cancelled');
  expect(after.refund_status).toBe('full');

  // Cancelled bookings leave the grid: the owner finds it in the client's recent bookings.
  const p = owner.page;
  await p.goto('/clients');
  await expect(p.getByRole('heading', { name: ES_AR.clients.title, exact: true })).toBeVisible({ timeout: 10_000 });
  await p.getByPlaceholder(ES_AR.common.search).first().fill(client.lastName);
  await p
    .getByRole('button', { name: new RegExp(client.lastName) })
    .first()
    .click();
  const recent = p.getByText(ES_AR.clients.recentBookings).locator('..');
  await expect(recent).toBeVisible({ timeout: 10_000 });
  await recent.getByRole('button').filter({ hasText: t.statuses.cancelled }).first().click();
  await expect(
    p.getByRole('dialog').filter({ hasText: ES_AR.publicBooking.bookingRef }).getByText(t.paymentStatuses.refunded),
  ).toBeVisible({ timeout: 10_000 });
  await owner.context.close();
});

test('the client cancels from their link inside the refund window and Mercado Pago refunds them', async ({ page }) => {
  test.slow();
  const { booking, payment, token } = await bookAndPay(page, depositWorld, newClient('Link'));

  await page.goto(landingComplexUrl(depositWorld.slug, `/book/cancel?token=${encodeURIComponent(token)}`));
  await waitForStorefrontHydration(page);
  const main = page.getByRole('main');
  await expect(main.getByRole('heading', { name: pb.cancelBookingQuestion })).toBeVisible({ timeout: 10_000 });
  await main.getByRole('button', { name: pb.cancelBookingConfirm }).click();
  const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog'));
  await expect(confirm.getByText(pb.confirmCancelTitle)).toBeVisible({ timeout: 5_000 });
  await expect(confirm.getByText(pb.confirmCancelRefundDetail)).toBeVisible();

  const response = page.waitForResponse(
    (res) => res.url().endsWith('/book/cancel') && res.request().method() === 'POST',
  );
  await confirm.getByRole('button', { name: pb.confirmCancelYes }).click();
  expect((await response).ok()).toBe(true);
  await expect(main.getByText(pb.cancelBookingSuccess)).toBeVisible({ timeout: 10_000 });

  await waitForRefundOf(payment.id, payment.transaction_amount);
  const after = await ownerBooking(depositWorld, booking.id);
  expect(after.status).toBe('cancelled');
  expect(after.refund_status).toBe('full');
});

test('the full price paid at checkout confirms the booking and the owner reads "Pago completo"', async ({
  page,
  browser,
}) => {
  test.slow();
  const client = newClient('Completo');
  const { booking } = await bookAndPay(page, fullWorld, client);
  await expect(page.getByText(pb.bookingSuccess)).toBeVisible({ timeout: 30_000 });
  // KNOWN PRODUCT BUG (reported to the owner, odd/tasks/mp-fake-e2e.md T3):
  // the webhook settles every approved storefront payment as 'deposit_paid'
  // (payments/process.go processApprovedPayment), even when the complex charges
  // no deposit and the client paid the whole price, so the owner reads "Seña
  // pagada" here. Last in the file so it does not stop the tests after it.
  expect(booking.collection_status).toBe('fully_paid');

  const owner = await openOwnerDetail(browser, fullWorld, booking, client);
  await expect(owner.detail.getByText(t.paymentStatuses.fully_paid)).toBeVisible();
  await owner.context.close();
});

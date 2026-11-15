import { expect, test, type Page } from '@playwright/test';
import { MP_FAKE_CHECKOUT, MP_FAKE_URL, mpFake, type FakePayment, type FakeRefund } from '../mp-fake/client';
import { PublicBookingPage } from '../pages/public-booking.page';
import { findAndClickAvailableSlot } from './public-booking-fixtures';
import { waitForStorefrontHydration } from './landing';
import { queryE2e, type EventsWorld } from './events-fixtures';

/**
 * Paying through the fake Mercado Pago (e2e/mp-fake) from a spec: reaching its
 * checkout, choosing an outcome there and waiting for what the API does with
 * the webhook. The webhook is worked after the API answers it, so everything
 * the backend does with a payment is polled for, never assumed.
 */

export type CheckoutOutcome = keyof typeof MP_FAKE_CHECKOUT;

export interface StorefrontClient {
  firstName: string;
  lastName: string;
  /** Local number as typed on the storefront (e.g. "1155556666"). */
  phone: string;
  email?: string;
}

/**
 * Waits for the browser to land on the fake checkout and returns the
 * preference id its path carries. The answer of the request that created the
 * preference is gone by then (the browser navigated away from it).
 */
export async function waitForCheckout(page: Page): Promise<string> {
  await expect(page).toHaveURL(new RegExp(`^${MP_FAKE_URL}/checkout/`), { timeout: 15_000 });
  return decodeURIComponent(new URL(page.url()).pathname.split('/').pop() ?? '');
}

/** Presses one of the fake checkout's buttons. */
export async function payAtCheckout(page: Page, outcome: CheckoutOutcome): Promise<void> {
  await page.getByRole('button', { name: MP_FAKE_CHECKOUT[outcome] }).click();
}

/**
 * Books the first free storefront slot of `slug` (90 minutes, skipping today)
 * up to the payment step and returns the Mercado Pago preference id. The
 * browser is on the fake checkout when this returns.
 */
export async function bookStorefrontUpToCheckout(page: Page, slug: string, client: StorefrontClient): Promise<string> {
  const publicPage = new PublicBookingPage(page);
  await publicPage.goto(slug);
  await waitForStorefrontHydration(page);
  await page.getByRole('button', { name: '90 min' }).click();
  await expect(page.getByText('¿Cuánto tiempo?')).not.toBeVisible({ timeout: 10_000 });

  const dateBtns = page.getByRole('radiogroup', { name: 'Fecha' }).getByRole('radio').and(page.locator(':enabled'));
  const slotFound = await findAndClickAvailableSlot(dateBtns);
  test.skip(!slotFound, 'No available slots found in the next 7 days');

  const hold = await publicPage.chooseFirstCourt();
  expect(hold.status()).toBe(201);
  await expect(page).toHaveURL(new RegExp(`${slug}/book/confirm`));

  await publicPage.fillBookingForm(client);

  const booked = page.waitForResponse(
    (res) => res.request().method() === 'POST' && new URL(res.url()).pathname.endsWith('/book'),
  );
  await publicPage.submitBooking();
  expect((await booked).status()).toBe(201);

  return waitForCheckout(page);
}

/** The booking or registration a preference was made for (its `external_reference`). */
export async function referenceOfPreference(preferenceId: string): Promise<string> {
  const preference = (await mpFake.preferences()).find((p) => p.id === preferenceId);
  const reference = preference?.body.external_reference;
  if (typeof reference !== 'string' || reference === '') {
    throw new Error(`preference ${preferenceId} carries no external_reference`);
  }
  return reference;
}

/** The (single) payment made at a preference's checkout, once the fake has it. */
export async function paymentOfPreference(preferenceId: string): Promise<FakePayment> {
  let payment: FakePayment | undefined;
  await expect
    .poll(
      async () => {
        payment = (await mpFake.paymentsOfPreference(preferenceId)).at(-1);
        return payment?.id ?? null;
      },
      { timeout: 15_000, message: `a payment at preference ${preferenceId}` },
    )
    .not.toBeNull();
  if (!payment) throw new Error('unreachable');
  return payment;
}

/**
 * Waits until the refunds the API asked the fake for add up to `amount`
 * (pesos) on `paymentId`, and returns them.
 */
export async function waitForRefundOf(paymentId: number, amount: number): Promise<FakeRefund[]> {
  let refunds: FakeRefund[] = [];
  await expect
    .poll(
      async () => {
        refunds = await mpFake.refundsOf(paymentId);
        return Math.round(refunds.reduce((sum, r) => sum + r.amount, 0) * 100) / 100;
      },
      { timeout: 30_000, message: `refunds of fake payment ${String(paymentId)}` },
    )
    .toBe(amount);
  return refunds;
}

/** A booking as the owner's API reads it. */
export interface OwnerBooking {
  id: string;
  date: string;
  status: string;
  collection_status: string;
  refund_status: string;
  deposit_amount: number;
  price: number;
}

export async function ownerBooking(world: EventsWorld, bookingId: string): Promise<OwnerBooking> {
  const { booking } = await world.api.get<{ booking: OwnerBooking }>(
    `/complexes/${world.complexId}/bookings/${bookingId}`,
  );
  return booking;
}

/** Waits until the webhook worked the booking into `status`, and returns it. */
export async function waitForBookingStatus(
  world: EventsWorld,
  bookingId: string,
  status: string,
): Promise<OwnerBooking> {
  await expect
    .poll(async () => (await ownerBooking(world, bookingId)).status, {
      timeout: 30_000,
      message: `booking ${bookingId} reaching ${status}`,
    })
    .toBe(status);
  return ownerBooking(world, bookingId);
}

/** Books, pays "Aprobar pago" and waits for the confirmation. Returns the booking, its payment and the link token. */
export async function bookAndPay(page: Page, world: EventsWorld, client: StorefrontClient) {
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

/** The backend's own record of a Mercado Pago payment (its `payments` row). Amounts are centavos. */
export interface RecordedPayment {
  status: string;
  amount: number;
  serviceFee: number;
  refundAmount: number;
  statusDetail: string;
}

export function recordedPayment(mpPaymentId: number): RecordedPayment | null {
  const [row] = queryE2e(
    `SELECT status || '|' || amount || '|' || service_fee || '|' || refund_amount || '|' || coalesce(status_detail, '')
     FROM payments WHERE mp_payment_id = '${String(mpPaymentId)}';`,
  );
  if (!row) return null;
  const [status = '', amount = '0', serviceFee = '0', refundAmount = '0', statusDetail = ''] = row.split('|');
  return {
    status,
    amount: Number(amount),
    serviceFee: Number(serviceFee),
    refundAmount: Number(refundAmount),
    statusDetail,
  };
}

/**
 * Waits until the backend has worked `count` webhook deliveries for a payment
 * (its durable inbox, `webhook_events`, marks each one processed).
 */
export async function waitForProcessedWebhooks(mpPaymentId: number, count: number): Promise<void> {
  await expect
    .poll(
      () =>
        Number(
          queryE2e(
            `SELECT count(*) FROM webhook_events WHERE external_id = '${String(mpPaymentId)}' AND status = 'processed';`,
          )[0] ?? '0',
        ),
      { timeout: 30_000, message: `${String(count)} processed webhooks of payment ${String(mpPaymentId)}` },
    )
    .toBeGreaterThanOrEqual(count);
}

/**
 * Waits until the fake has delivered a webhook with `action` for a payment and
 * the API accepted it (HTTP 2xx).
 */
export async function waitForDeliveredWebhook(mpPaymentId: number, action: string): Promise<void> {
  await expect
    .poll(
      async () =>
        (await mpFake.webhooksOf(mpPaymentId)).some(
          (w) => w.action === action && w.status !== null && w.status >= 200 && w.status < 300,
        ),
      { timeout: 15_000, message: `a delivered ${action} webhook for payment ${String(mpPaymentId)}` },
    )
    .toBe(true);
}

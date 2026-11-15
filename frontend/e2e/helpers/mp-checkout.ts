import { expect, test, type Page } from '@playwright/test';
import { MP_FAKE_CHECKOUT, MP_FAKE_URL, mpFake, type FakePayment, type FakeRefund } from '../mp-fake/client';
import { PublicBookingPage } from '../pages/public-booking.page';
import { findAndClickAvailableSlot } from './public-booking-fixtures';
import { waitForStorefrontHydration } from './landing';

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

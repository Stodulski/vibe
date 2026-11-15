import { test, expect, request as apiRequest } from '@playwright/test';
import type { Page } from '@playwright/test';
import { TEST_CLIENT } from '../helpers/test-data';
import { waitForStorefrontHydration } from '../helpers/landing';
import { PublicBookingPage } from '../pages/public-booking.page';
import {
  registerAndLoginOwner,
  ensureTestComplex,
  seedTestCourt,
  fakeMercadoPagoConnection,
  findAndClickAvailableSlot,
  pointComplexAtFakeMercadoPago,
  waitForOnlineBooking,
} from '../helpers/public-booking-fixtures';
import { MP_FAKE_CHECKOUT, MP_FAKE_COLLECTOR_ID, MP_FAKE_URL, mpFake } from '../mp-fake/client';

const SLUG = 'complejo-publico-e2e';
const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

/**
 * Books the first free storefront slot up to the payment step and returns the
 * Mercado Pago preference id. The browser is on the fake checkout when this
 * returns.
 */
async function bookUpToCheckout(page: Page): Promise<string> {
  const publicPage = new PublicBookingPage(page);
  await publicPage.goto(SLUG);
  await waitForStorefrontHydration(page);
  await page.getByRole('button', { name: '90 min' }).click();
  await expect(page.getByText('¿Cuánto tiempo?')).not.toBeVisible({ timeout: 10_000 });

  const dateBtns = page.getByRole('radiogroup', { name: 'Fecha' }).getByRole('radio').and(page.locator(':enabled'));
  const slotFound = await findAndClickAvailableSlot(dateBtns);
  test.skip(!slotFound, 'No available slots found in the next 7 days');

  const hold = await publicPage.chooseFirstCourt();
  expect(hold.status()).toBe(201);
  await expect(page).toHaveURL(new RegExp(`${SLUG}/book/confirm`));

  await publicPage.fillBookingForm({
    firstName: TEST_CLIENT.firstName,
    lastName: TEST_CLIENT.lastName,
    phone: '1155556666',
    email: TEST_CLIENT.email,
  });

  const booked = page.waitForResponse(
    (res) => res.request().method() === 'POST' && new URL(res.url()).pathname.endsWith('/book'),
  );
  await publicPage.submitBooking();
  expect((await booked).status()).toBe(201);

  // The storefront follows the preference's init_point to the fake checkout,
  // whose path carries the preference id (the booking answer's body is gone
  // by then: the browser navigated away from it).
  await expect(page).toHaveURL(new RegExp(`^${MP_FAKE_URL}/checkout/`), { timeout: 15_000 });
  const preferenceId = decodeURIComponent(new URL(page.url()).pathname.split('/').pop() ?? '');
  return preferenceId;
}

// Smoke test of the fake Mercado Pago (e2e/mp-fake): a storefront booking is
// paid through the fake checkout, the signed webhook reaches the API, and the
// booking ends up confirmed. The fuller payment matrix lives in its own specs.
test.describe('Mercado Pago fake: storefront booking paid and approved', () => {
  test.beforeAll(async () => {
    test.setTimeout(150_000);
    const ctx = await apiRequest.newContext();
    const headers = await registerAndLoginOwner(ctx);
    const complexId = await ensureTestComplex(ctx, headers, SLUG);
    if (complexId) {
      await seedTestCourt(ctx, headers, complexId);
      await fakeMercadoPagoConnection(complexId);
    }
    pointComplexAtFakeMercadoPago(SLUG);
    await waitForOnlineBooking(ctx, SLUG);
    await ctx.dispose();
  });

  test('"Aprobar pago" returns to the success page and confirms the booking', async ({ page }) => {
    test.slow();
    const preferenceId = await bookUpToCheckout(page);

    await page.getByRole('button', { name: MP_FAKE_CHECKOUT.approve }).click();

    // back_urls.success: the landing's success page, carrying the link token.
    await expect(page).toHaveURL(new RegExp(`${SLUG}/book/success\\?token=`), { timeout: 15_000 });
    const token = new URL(page.url()).searchParams.get('token') ?? '';
    expect(token).not.toBe('');

    const [payment] = await mpFake.paymentsOfPreference(preferenceId);
    expect(payment?.status).toBe('approved');
    expect(String(payment?.collector_id)).toBe(MP_FAKE_COLLECTOR_ID);

    // The webhook is worked after the API answers it, so the confirmation
    // lands a moment after the redirect.
    const ctx = await apiRequest.newContext();
    await expect
      .poll(
        async () => {
          const res = await ctx.get(`${API}/book/status`, { params: { token } });
          if (!res.ok()) return `HTTP ${String(res.status())}`;
          const { booking } = (await res.json()) as { booking: { status: string } };
          return booking.status;
        },
        { timeout: 30_000 },
      )
      .toBe('confirmed');
    await ctx.dispose();

    await expect(page.getByText('Reserva confirmada')).toBeVisible({ timeout: 30_000 });
  });
});

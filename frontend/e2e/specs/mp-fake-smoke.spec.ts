import { test, expect, request as apiRequest } from '@playwright/test';
import { TEST_CLIENT } from '../helpers/test-data';
import {
  registerAndLoginOwner,
  ensureTestComplex,
  seedTestCourt,
  fakeMercadoPagoConnection,
  pointComplexAtFakeMercadoPago,
  waitForOnlineBooking,
} from '../helpers/public-booking-fixtures';
import { MP_FAKE_CHECKOUT, MP_FAKE_COLLECTOR_ID, mpFake } from '../mp-fake/client';
import { bookStorefrontUpToCheckout } from '../helpers/mp-checkout';

const SLUG = 'complejo-publico-e2e';
const API = `${process.env.E2E_BASE_URL ?? 'http://localhost:5173'}/api/v1`;

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
    const preferenceId = await bookStorefrontUpToCheckout(page, SLUG, {
      firstName: TEST_CLIENT.firstName,
      lastName: TEST_CLIENT.lastName,
      phone: '1155556666',
      email: TEST_CLIENT.email,
    });

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

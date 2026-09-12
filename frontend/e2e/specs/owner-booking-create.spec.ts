import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import type { Page } from '@playwright/test';

/** Selects the shared test complex and lands on the (loaded) bookings page. */
async function gotoBookings(page: Page, complexId: string): Promise<void> {
  await page.evaluate((id) => {
    localStorage.setItem('selectedComplexId', id);
  }, complexId);
  await page.goto('/bookings');
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({
    timeout: 10_000,
  });
}

// owner-booking.spec.ts's "can open create booking modal and reach client
// step" walks steps 1-2 and stops there. This carries the full 3-step wizard
// (Cuándo y dónde -> Cliente -> Pago) through to a real POST .../bookings and
// asserts the booking actually lands on the calendar — the flow TST-06
// flagged as uncovered end-to-end.
test.describe('Owner creates a booking end-to-end', () => {
  let complexId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
  });

  test('fills all 3 steps and sees the new booking on the calendar', async ({ authenticatedPage: page }) => {
    await gotoBookings(page, complexId);

    // Tomorrow, not today: the time list omits hours already past, so a slot
    // near the end of the priced window can vanish depending on the time of
    // day this runs.
    await page.getByRole('button', { name: 'Día siguiente' }).click();

    await page.getByRole('button', { name: 'Nueva reserva' }).first().click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    // Step 1: Cuándo y dónde.
    await dialog.getByRole('combobox').filter({ hasText: 'Seleccioná una cancha' }).first().click();
    await page.getByRole('option').first().click();

    const timeSelect = dialog.getByRole('combobox').filter({ hasText: '--:--' });
    await expect(timeSelect).toBeEnabled({ timeout: 10_000 });
    await timeSelect.click();
    // Inside the priced window (08:00-23:00 / weekend 09:00-22:00, see
    // DEFAULT_SCHEDULE/DEFAULT_PRICES in test-data.ts) so a price resolves
    // without the manual-price detour.
    await page
      .getByRole('option', { name: /^(09|1\d|20):[03]0$/ })
      .first()
      .click();
    await dialog.getByRole('button', { name: 'Siguiente' }).click();

    // Step 2: client data. Unique per run — the backend resolves the client
    // by phone, so a reused phone would silently attach to a prior run's
    // client instead of creating this booking's own.
    const uniqueSuffix = String(Date.now()).slice(-9);
    const clientLastName = `E2ECreate${uniqueSuffix}`;
    await expect(dialog.getByText('Datos del cliente')).toBeVisible();
    await dialog.locator('#client-first-name').fill('Nueva');
    await dialog.locator('#client-last-name').fill(clientLastName);
    await dialog.locator('#client-phone').fill(`+549110${uniqueSuffix}`);
    await dialog.getByRole('button', { name: 'Siguiente' }).click();

    // Step 3: payment — defaults to "unpaid" with no extra input needed
    // (PaymentOptionFields renders `value={paymentOption ?? 'unpaid'}`), so
    // submitting directly proves the owner path never requires MercadoPago.
    await dialog.getByRole('button', { name: 'Nueva reserva' }).click();

    await expect(page.getByText('Reserva creada exitosamente')).toBeVisible({ timeout: 10_000 });
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText(`Nueva ${clientLastName}`)).toBeVisible({ timeout: 10_000 });
  });
});

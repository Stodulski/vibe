import type { Locator, Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { typedJson } from '../helpers/api.helper';
import { getFutureDate } from '../helpers/test-data';

/**
 * A booking created with a deposit ("Seña") and its balance collected later.
 * `owner-booking-create.spec.ts` creates with no payment and
 * `confirm-payment.spec.ts` collects an unpaid booking in full; neither goes
 * through the deposit field or the "Confirmar pago restante" path, where the
 * amount charged must be the price minus what the deposit already covered.
 */
interface BookingBody {
  id: string;
  price: number;
  deposit_amount: number;
  collection_status: string;
}

const DEPOSIT_PESOS = 50;

/** Opens "Nueva reserva" for `date` on the shared court, at its first offered time. */
async function startBooking(page: Page, date: string, courtName: string): Promise<Locator> {
  await page.goto(`/bookings?date=${date}`);
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({ timeout: 10_000 });
  await page.getByRole('button', { name: 'Nueva reserva' }).first().click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible({ timeout: 5_000 });

  await dialog.getByRole('combobox').filter({ hasText: 'Seleccioná una cancha' }).first().click();
  // Options read "<court> · <sport>".
  await page.getByRole('option', { name: new RegExp(`^${courtName} ·`) }).click();
  const timeSelect = dialog.getByRole('combobox').filter({ hasText: '--:--' });
  await expect(timeSelect).toBeEnabled({ timeout: 10_000 });
  await timeSelect.click();
  // The day starts at 00:00, which has no price rule; the default prices cover 09:00 to 22:00.
  await page
    .getByRole('option', { name: /^(09|1\d|20):[03]0$/ })
    .first()
    .click();
  await dialog.getByRole('button', { name: 'Siguiente' }).click();
  return dialog;
}

/** Fills the client step and the payment step with a cash deposit, submits, and returns the created booking. */
async function createWithDeposit(page: Page, dialog: Locator, lastName: string): Promise<BookingBody> {
  await expect(dialog.getByText('Datos del cliente')).toBeVisible();
  await dialog.locator('#client-first-name').fill('Seña');
  await dialog.locator('#client-last-name').fill(lastName);
  await dialog.locator('#client-phone').fill(`9110${String(Date.now()).slice(-9)}`);
  await dialog.getByRole('button', { name: 'Siguiente' }).click();

  await dialog.locator('#booking-payment-option').click();
  await page.getByRole('option', { name: 'Seña', exact: true }).click();
  await dialog.locator('#deposit-amount').fill(String(DEPOSIT_PESOS));
  await dialog.getByLabel(/^Método de pago/).click();
  await page.getByRole('option', { name: 'Efectivo', exact: true }).click();

  const created = page.waitForResponse((res) => res.url().endsWith('/bookings') && res.request().method() === 'POST');
  await dialog.getByRole('button', { name: 'Nueva reserva' }).click();
  const response = await created;
  expect(response.ok()).toBe(true);
  await expect(page.getByText('Reserva creada exitosamente')).toBeVisible({ timeout: 10_000 });
  return (await typedJson<{ booking: BookingBody }>(response)).booking;
}

test.describe('Deposit booking', () => {
  test('a booking created with a deposit is collected in full for exactly the remaining balance', async ({
    authenticatedPage: page,
  }) => {
    test.slow();
    const { courtName } = await getSharedSetup();
    const lastName = `Deposito${String(Date.now())}`;

    const dialog = await startBooking(page, getFutureDate(6), courtName);
    const booking = await createWithDeposit(page, dialog, lastName);
    expect(booking.collection_status).toBe('deposit_paid');
    expect(booking.deposit_amount).toBe(DEPOSIT_PESOS * 100);

    await page.getByText(`Seña ${lastName}`).first().click();
    const detailSheet = page.getByRole('dialog').filter({ hasText: 'Reserva' });
    await expect(detailSheet.getByText('Seña pagada')).toBeVisible({ timeout: 5_000 });
    await detailSheet.getByRole('button', { name: 'Confirmar pago' }).click();

    const payDialog = page
      .getByRole('dialog')
      .filter({ has: page.getByRole('heading', { name: 'Confirmar pago restante' }) });
    await expect(payDialog).toBeVisible({ timeout: 5_000 });
    const paid = page.waitForResponse(
      (res) => res.url().includes(`/bookings/${booking.id}/confirm-payment`) && res.request().method() === 'POST',
    );
    await payDialog.getByRole('button', { name: 'Confirmar pago' }).click();
    const response = await paid;
    expect(response.ok()).toBe(true);
    const body = await typedJson<{ booking: BookingBody; payment: { amount: number } }>(response);
    expect(body.booking.collection_status).toBe('fully_paid');
    expect(body.payment.amount).toBe(booking.price - booking.deposit_amount);

    await expect(payDialog).toBeHidden({ timeout: 5_000 });
    await page.getByText(`Seña ${lastName}`).first().click();
    await expect(page.getByRole('dialog').filter({ hasText: 'Reserva' }).getByText('Pago completo')).toBeVisible({
      timeout: 5_000,
    });
  });
});

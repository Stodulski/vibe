import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { bookFirstFreeHour } from '../helpers/booking-fixtures';
import { createApiHelper } from '../helpers/api.helper';
import type { ApiBooking } from '../helpers/api.helper';
import { getFutureDate } from '../helpers/test-data';

/**
 * A booking paid at the counter in cash and then cancelled by staff owes that
 * cash back by hand: nothing automatic can return it, so the backend leaves the
 * booking at refund_status 'partial' and the owner closes it out with "Marcar
 * devolución realizada". Cancelled bookings are hidden from the calendar, so
 * the owner reaches it from the client's recent bookings.
 *
 * The till is open while this happens, so the returned cash must also show on
 * the cash card as "Devoluciones en efectivo".
 */

/** Opens the client's drawer from /clients and the cancelled booking from its recent bookings. */
async function openCancelledBookingFromClient(page: Page, lastName: string) {
  await page.goto('/clients');
  await expect(page.getByRole('heading', { name: 'Clientes', exact: true })).toBeVisible({ timeout: 10_000 });
  await page.getByPlaceholder('Buscar...').first().fill(lastName);
  await page
    .getByRole('button', { name: new RegExp(lastName) })
    .first()
    .click();

  const recent = page.getByText('Reservas recientes').locator('..');
  await expect(recent).toBeVisible({ timeout: 10_000 });
  await recent.getByRole('button').filter({ hasText: 'Cancelada' }).first().click();
}

test.describe('Manual refund', () => {
  let complexId: string;
  let courtId: string;

  test.beforeEach(async () => {
    const setup = await getSharedSetup();
    ({ complexId, courtId } = setup);
    await setup.api.closeAnyOpenCashSession(complexId);
  });

  test.afterEach(async () => {
    await (await createApiHelper()).closeAnyOpenCashSession(complexId);
  });

  test('a cancelled cash-paid booking is marked as returned by hand and the till shows the refund', async ({
    authenticatedPage: page,
  }) => {
    const api = await createApiHelper();
    const lastName = `Devolucion${String(Date.now())}`;
    await api.openCashSession(complexId, 1_000_000);

    const booking = await bookFirstFreeHour(api, complexId, courtId, getFutureDate(4), {
      firstName: 'Reembolso',
      lastName,
    });
    await api.confirmPayment(complexId, booking.id, booking.price);
    const cancelled = await api.cancelBooking(complexId, booking.id);
    expect(cancelled.refund_status).toBe('partial');

    await openCancelledBookingFromClient(page, lastName);
    await page.getByRole('button', { name: 'Marcar devolución realizada' }).click();
    const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog').filter({ hasText: 'en mano' }));
    await expect(confirm).toBeVisible({ timeout: 5_000 });

    const response = page.waitForResponse(
      (res) => res.url().endsWith('/manual-refund') && res.request().method() === 'POST',
    );
    await confirm.getByRole('button', { name: 'Marcar devolución realizada' }).click();
    expect((await response).ok()).toBe(true);
    await expect(page.getByText('Devolución registrada')).toBeVisible({ timeout: 5_000 });

    const after = await api.get<{ booking: ApiBooking }>(`/complexes/${complexId}/bookings/${booking.id}`);
    expect(after.booking.refund_status).toBe('full');

    // Cash handed back during the open session comes off expected cash.
    const price = `-$${(booking.price / 100).toLocaleString('es-AR')}`;
    await page.goto('/cash');
    await expect(page.getByTestId('cash-manual-refunds-row')).toContainText(price, { timeout: 10_000 });
  });
});

import { getFutureDate } from '../helpers/test-data';
import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';

async function openBookingsForDate(page: Page, complexId: string, date: string): Promise<void> {
  await page.evaluate((id) => {
    localStorage.setItem('selectedComplexId', id);
  }, complexId);
  await page.goto(`/bookings?date=${date}`);
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({
    timeout: 10_000,
  });
}

test.describe('Owner cancels a booking', () => {
  let complexId: string;
  let courtId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    courtId = setup.courtId;
  });

  test('cancels from the detail sheet and the calendar reflects it', async ({ authenticatedPage: page }) => {
    // Own request context, not the page's — see confirm-payment.spec.ts's
    // comment on why: logging the helper in through the browser's cookie jar
    // would reuse the refresh token the fixture's session already holds, and
    // the backend revokes both sessions on a reused refresh token.
    const apiHelper = await createApiHelper();
    const uniqueSuffix = String(Date.now()).slice(-9);
    const clientLastName = `E2ECancel${uniqueSuffix}`;
    const date = getFutureDate(2);

    const booking = await apiHelper.createBooking(complexId, courtId, {
      date,
      start_time: '09:00',
      client_first_name: 'Cancelar',
      client_last_name: clientLastName,
      client_phone: `+549111${uniqueSuffix}`,
    });
    expect(booking.collection_status).toBe('unpaid');

    await openBookingsForDate(page, complexId, date);

    await page.getByText(`Cancelar ${clientLastName}`).first().click();
    const detailSheet = page.getByRole('dialog').filter({ hasText: 'Reserva' });
    await expect(detailSheet).toBeVisible({ timeout: 5_000 });

    await detailSheet.getByRole('button', { name: 'Más acciones' }).click();
    await page.getByRole('menuitem', { name: 'Cancelar reserva' }).click();

    const cancelDialog = page.getByRole('alertdialog');
    await expect(cancelDialog).toBeVisible({ timeout: 5_000 });
    await cancelDialog.getByRole('button', { name: 'Confirmar cancelación' }).click();

    await expect(page.getByText('Reserva cancelada')).toBeVisible({ timeout: 10_000 });
    await expect(cancelDialog).not.toBeVisible();

    // The booking's own status badge, not just the toast: reopening the
    // detail sheet should read "Cancelada" instead of leaving the calendar
    // cell showing the previous confirmed/pending state.
    await page.getByText(`Cancelar ${clientLastName}`).first().click();
    await expect(page.getByRole('dialog').filter({ hasText: 'Reserva' }).getByText('Cancelada')).toBeVisible({
      timeout: 10_000,
    });
  });
});

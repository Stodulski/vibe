import { getFutureDate } from '../helpers/test-data';
import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';

async function openBookingsForDate(page: Page, date: string): Promise<void> {
  await page.goto(`/bookings?date=${date}`);
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({
    timeout: 10_000,
  });
}

test.describe('Owner cancels a booking', () => {
  let complexId: string;
  let courtId: string;

  // Its own court, not the shared one every other authenticated spec books
  // against: this spec booked a fixed date+time (getFutureDate(2), 09:00) on
  // the shared court, which another spec running concurrently in a
  // different worker could book first — a real 409 seen against main after
  // this spec merged (POST .../bookings, ErrDuplicateBooking/
  // ErrSlotUnavailable, both reported as the same generic 409 edit-conflict
  // body). A dedicated court makes that collision structurally impossible.
  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    const court = await setup.api.createCourt(complexId, { name: 'Cancha E2E Cancel' });
    await setup.api.setCourtPrices(complexId, court.id);
    courtId = court.id;
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

    await openBookingsForDate(page, date);

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

    // Not by reopening the detail sheet to check a "Cancelada" badge:
    // CourtTimeGrid filters cancelled bookings out of the day view entirely
    // (`bookings.filter((b) => b.status !== 'cancelled')`), so that row
    // never comes back — the previous assertion here re-clicked the same
    // text and hung for the full test timeout waiting for an element that
    // was never going to reappear. Its disappearance from the calendar *is*
    // the observable proof the cancellation took, so assert that instead,
    // and confirm the status directly against the API for the actual value.
    await expect(page.getByText(`Cancelar ${clientLastName}`)).toHaveCount(0);

    const cancelled = await apiHelper.get<{ booking: { status: string } }>(
      `/complexes/${complexId}/bookings/${booking.id}`,
    );
    expect(cancelled.booking.status).toBe('cancelled');
  });
});

import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper, typedJson } from '../helpers/api.helper';
import type { ApiBooking } from '../helpers/api.helper';

// The bookings page defaults to today; navigate forward to reach the
// booking's date. Shared between the initial navigation and the
// post-reload one — a reload always drops the page back to today.
async function navigateToBookingsForDate(page: Page, daysAhead: number) {
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({
    timeout: 10_000,
  });
  const nextDayButton = page.getByRole('button', { name: 'Día siguiente' });
  for (let i = 0; i < daysAhead; i++) {
    await nextDayButton.click();
    await page.waitForTimeout(300);
  }
}

function getFutureDate(daysAhead: number): string {
  const d = new Date();
  d.setDate(d.getDate() + daysAhead);
  return d.toISOString().slice(0, 10);
}

// Picks a pseudo-random half-hour slot between 08:00 and 21:00 so repeated
// local runs don't collide with a previous run's booking on the same slot.
function getRandomSlotStartTime(): string {
  const slotsFromEight = Math.floor(Math.random() * 26); // 0..25 -> 08:00..20:30
  const hour = 8 + Math.floor(slotsFromEight / 2);
  const minute = slotsFromEight % 2 === 0 ? '00' : '30';
  return `${String(hour).padStart(2, '0')}:${minute}`;
}

type ApiHelperInstance = Awaited<ReturnType<typeof createApiHelper>>;

// Retries on a 409 slot conflict: this DB accumulates bookings across local
// runs (no e2e DB reset in this sandbox), so a random date/slot combination
// can collide with a prior run's booking.
async function createUniqueUnpaidBooking(
  apiHelper: ApiHelperInstance,
  complexId: string,
  courtId: string,
  clientLastName: string,
  clientPhone: string,
) {
  const maxAttempts = 15;
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const daysAhead = 1 + (attempt % 5);
    try {
      const booking = await apiHelper.createBooking(complexId, courtId, {
        date: getFutureDate(daysAhead),
        start_time: getRandomSlotStartTime(),
        client_first_name: 'Pago',
        client_last_name: clientLastName,
        client_phone: clientPhone,
      });
      return { booking, daysAhead };
    } catch (e) {
      if (attempt === maxAttempts - 1) throw e;
    }
  }
  throw new Error('unreachable');
}

test.describe('Confirm Payment', () => {
  let complexId: string;
  let courtId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    courtId = setup.courtId;
  });

  test('owner can confirm cash payment for an unpaid booking', async ({ authenticatedPage: page }) => {
    const apiHelper = await createApiHelper(page.context().request);
    // Unique per run: this DB is not reset between local runs (no e2e DB
    // stack in this sandbox — see slice 6/7 apply-progress). The backend
    // resolves the client by phone number, so a reused phone silently
    // reuses the FIRST run's stored client name regardless of the
    // first/last name passed here — both must be unique per run.
    const uniqueSuffix = String(Date.now()).slice(-9);
    const uniqueClientName = `E2E${uniqueSuffix}`;

    const { booking, daysAhead } = await createUniqueUnpaidBooking(
      apiHelper,
      complexId,
      courtId,
      uniqueClientName,
      `+549110${uniqueSuffix}`,
    );
    expect(booking.collection_status).toBe('unpaid');

    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    await page.goto('/bookings');
    await navigateToBookingsForDate(page, daysAhead);

    // Open the booking detail sheet for the client we just created. Scoped
    // by its "Reserva" heading (not position) since a second dialog stacks
    // on top of it once the payment modal opens below.
    await page.getByText(`Pago ${uniqueClientName}`).first().click();
    const detailSheet = page.getByRole('dialog').filter({ hasText: 'Reserva' });
    await expect(detailSheet.getByText('Confirmar pago')).toBeVisible({ timeout: 5_000 });

    // Open the detail's "Confirmar pago" action — this stacks a second
    // dialog (ConfirmPaymentModal) on top of the detail sheet, scoped by its
    // own heading.
    await detailSheet.getByRole('button', { name: 'Confirmar pago' }).click();
    const paymentDialog = page
      .getByRole('dialog')
      .filter({ has: page.getByRole('heading', { name: 'Confirmar pago' }) });
    await expect(paymentDialog).toBeVisible({ timeout: 5_000 });

    const confirmResponse = page.waitForResponse(
      (res) => res.url().includes(`/bookings/${booking.id}/confirm-payment`) && res.request().method() === 'POST',
    );
    await paymentDialog.getByRole('button', { name: 'Confirmar pago' }).click();

    // Real server confirmation: the mutation actually persisted a fully-paid state.
    const res = await confirmResponse;
    expect(res.ok()).toBe(true);
    const resBody = await typedJson<{ booking: ApiBooking }>(res);
    expect(resBody.booking.collection_status).toBe('fully_paid');

    // On success, `useBookingActions`'s `handlePaymentSubmit` closes BOTH
    // the payment dialog and the detail sheet (`setPaymentOpen(false)` +
    // `setDetailOpen(false)`).
    await expect(paymentDialog).toBeHidden({ timeout: 5_000 });
    await expect(detailSheet).toBeHidden({ timeout: 5_000 });

    // Reload to bypass any client-side query cache and verify the UI
    // reflects the server-persisted state independently of cache timing.
    await page.reload();
    await navigateToBookingsForDate(page, daysAhead);
    await page.getByText(`Pago ${uniqueClientName}`).first().click();
    // BookingStatusBadge's fully_paid label is "Pago completo", not "Pagada"
    // (t.bookings.paymentStatusLabels.fully_paid in es_AR/bookings.ts).
    await expect(page.getByRole('dialog').filter({ hasText: 'Reserva' }).getByText('Pago completo')).toBeVisible({
      timeout: 5_000,
    });
  });
});

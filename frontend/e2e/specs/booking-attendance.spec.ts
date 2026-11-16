import type { Page } from '@playwright/test';
import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { bookFirstFreeHour } from '../helpers/booking-fixtures';
import { createApiHelper, type ApiCourt } from '../helpers/api.helper';
import { queryE2e } from '../helpers/events-fixtures';
import { getFutureDate } from '../helpers/test-data';
import { ES_AR } from '../../src/shared/i18n/es_AR';

/**
 * A booking whose hours are over reads "Turno finalizado" until the owner
 * marks whether the client came: "Se presentó" or "No se presentó"
 * (odd/tasks/booking-attendance.md S1).
 *
 * The API books only the future, so each booking is made ahead and then moved
 * a few days back in the database, as if its hours had passed. A court of its
 * own keeps those past hours away from every other spec.
 */
const t = ES_AR.bookings;

interface AttendanceBooking {
  status: string;
  attended_at?: string | null;
}

let complexId: string;
let courtId: string;

/**
 * A booking on this spec's court whose hours ended `daysAgo` days ago. Returns
 * its id and date. Each test takes its own day: the booking keeps its hour, and
 * two of them on one day would overlap.
 */
async function finishedBooking(
  firstName: string,
  lastName: string,
  daysAgo: number,
): Promise<{ id: string; date: string }> {
  const api = await createApiHelper();
  const booking = await bookFirstFreeHour(api, complexId, courtId, getFutureDate(3), { firstName, lastName });
  const [date] = queryE2e(
    `UPDATE bookings SET date = (now() AT TIME ZONE 'America/Argentina/Buenos_Aires')::date - ${String(daysAgo)} WHERE id = '${booking.id}' RETURNING date;`,
  );
  if (!date) throw new Error(`could not move booking ${booking.id} into the past`);
  return { id: booking.id, date };
}

async function openDetail(page: Page, date: string, label: string) {
  await page.goto(`/bookings?date=${date}`);
  await expect(page.getByRole('heading', { name: t.title, exact: true })).toBeVisible({ timeout: 10_000 });
  await page.getByText(label).first().click();
  const detail = page.getByRole('dialog').filter({ hasText: ES_AR.publicBooking.bookingRef });
  await expect(detail).toBeVisible({ timeout: 5_000 });
  return detail;
}

async function bookingOf(id: string): Promise<AttendanceBooking> {
  const api = await createApiHelper();
  return (await api.get<{ booking: AttendanceBooking }>(`/complexes/${complexId}/bookings/${id}`)).booking;
}

const COURT_NAME = 'Cancha E2E Asistencia';

test.describe('Booking attendance', () => {
  // One worker for both tests: each worker runs beforeAll, and the court name is unique per complex.
  test.describe.configure({ mode: 'serial' });

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    // A retry restarts the worker, and the court is already there.
    const { courts } = await setup.api.get<{ courts: ApiCourt[] }>(`/complexes/${complexId}/courts`);
    const existing = courts.find((c) => c.name === COURT_NAME);
    const court = existing ?? (await setup.api.createCourt(complexId, { name: COURT_NAME }));
    if (!existing) await setup.api.setCourtPrices(complexId, court.id);
    courtId = court.id;
  });

  test('a finished booking reads "Turno finalizado" and "Se presentó" marks that the client came', async ({
    authenticatedPage: page,
  }) => {
    const lastName = `Asistio${String(Date.now())}`;
    const { id, date } = await finishedBooking('Vino', lastName, 2);

    const detail = await openDetail(page, date, `Vino ${lastName}`);
    await expect(detail.getByText(t.statuses.finished, { exact: true })).toBeVisible();
    // Played and unpaid: the money is now due.
    await expect(detail.getByText(t.paymentStatuses.outstanding, { exact: true })).toBeVisible();
    await expect(detail.getByRole('button', { name: t.statuses.no_show, exact: true })).toBeVisible();
    // Nothing left to cancel once the hours are over.
    await expect(detail.getByRole('button', { name: t.cancel })).toHaveCount(0);

    const marked = page.waitForResponse(
      (res) => res.url().endsWith(`/bookings/${id}/attended`) && res.request().method() === 'POST',
    );
    await detail.getByRole('button', { name: t.statuses.completed, exact: true }).click();
    expect((await marked).ok()).toBe(true);
    await expect(detail).toBeHidden({ timeout: 5_000 });

    const after = await bookingOf(id);
    expect(after.status).toBe('completed');
    expect(after.attended_at).toBeTruthy();

    const reopened = await openDetail(page, date, `Vino ${lastName}`);
    await expect(reopened.getByText(t.statuses.completed, { exact: true })).toBeVisible();
    await expect(reopened.getByText(t.statuses.finished, { exact: true })).toHaveCount(0);
  });

  test('"No se presentó" marks a finished booking as a no-show', async ({ authenticatedPage: page }) => {
    const lastName = `Falto${String(Date.now())}`;
    const { id, date } = await finishedBooking('NoVino', lastName, 3);

    const detail = await openDetail(page, date, `NoVino ${lastName}`);
    await expect(detail.getByText(t.statuses.finished, { exact: true })).toBeVisible();
    await detail.getByRole('button', { name: t.statuses.no_show, exact: true }).click();
    await expect(detail).toBeHidden({ timeout: 5_000 });

    await expect.poll(async () => (await bookingOf(id)).status, { timeout: 10_000 }).toBe('no_show');
    expect((await bookingOf(id)).attended_at ?? null).toBeNull();

    // A no-show frees the court, so it leaves the grid: the owner finds it in the client's recent bookings.
    await page.goto('/clients');
    await expect(page.getByRole('heading', { name: ES_AR.clients.title, exact: true })).toBeVisible({
      timeout: 10_000,
    });
    await page.getByPlaceholder(ES_AR.common.search).first().fill(lastName);
    await page
      .getByRole('button', { name: new RegExp(lastName) })
      .first()
      .click();
    const recent = page.getByText(ES_AR.clients.recentBookings).locator('..');
    await recent.getByRole('button').filter({ hasText: t.statuses.no_show }).first().click();
    const reopened = page.getByRole('dialog').filter({ hasText: ES_AR.publicBooking.bookingRef });
    await expect(reopened.getByText(t.statuses.no_show, { exact: true })).toBeVisible({ timeout: 10_000 });
    await expect(reopened.getByRole('button', { name: t.statuses.completed, exact: true })).toHaveCount(0);
  });
});

import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';
import { getFutureDate } from '../helpers/test-data';
import type { Page } from '@playwright/test';

/**
 * There is no standalone "Horarios bloqueados" section on the courts page
 * anymore -- `t.courts.blockedSlots` is a dead i18n string unused anywhere
 * in the app. A blocked slot now renders inline in the bookings calendar
 * grid as a `role="button"` block (BlockedBlock.tsx) with an accessible
 * name of "<court>, <start>–<end>, Horario bloqueado". This creates a real
 * block via the API (courts page has no "block" UI to drive from Playwright
 * either) and asserts it actually renders on /bookings.
 *
 * Extracted out of the test body to keep the describe callback under the
 * repo's max-lines-per-function cap.
 */
async function expectBlockedSlotOnCalendar(page: Page, complexId: string, courtId: string): Promise<void> {
  // Own request context: logging in through the page's cookie jar replaces
  // the page's session and races its token refresh.
  const apiHelper = await createApiHelper();
  const futureDate = getFutureDate(21);

  const blocked = await apiHelper.blockSlot(complexId, courtId, {
    date: futureDate,
    start_time: '16:00',
    end_time: '17:00',
    reason: 'Mantenimiento E2E UI',
  });

  try {
    await page.evaluate((id) => {
      localStorage.setItem('selectedComplexId', id);
    }, complexId);
    // The selected day lives in the URL (`?date=`, see useDateNav), so the
    // page is opened on the slot's day directly: 21 rapid clicks on "next
    // day" dropped some of their URL updates and landed short of it.
    await page.goto(`/bookings?date=${futureDate}`);
    await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({
      timeout: 10_000,
    });

    await expect(page.getByRole('button', { name: /Horario bloqueado/ })).toBeVisible({
      timeout: 10_000,
    });
  } finally {
    await apiHelper.deleteBlockedSlot(complexId, blocked.id);
  }
}

test.describe('Blocked Slots', () => {
  let complexId: string;
  let courtId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    courtId = setup.courtId;
  });

  // API only: the helper signs in on its own, so no page is needed.
  test('can block, list, and delete slots via API', async () => {
    const apiHelper = await createApiHelper();

    const futureDate = getFutureDate(14);

    // Block a slot
    const blocked = await apiHelper.blockSlot(complexId, courtId, {
      date: futureDate,
      start_time: '18:00',
      end_time: '19:30',
      reason: 'Mantenimiento E2E',
    });
    expect(blocked.id).toBeTruthy();
    expect(blocked.court_name).toBeTruthy();

    // List blocked slots
    const slots = await apiHelper.listBlockedSlots(complexId, futureDate, futureDate);
    expect(slots.length).toBeGreaterThanOrEqual(1);
    const found = slots.find((s: { id: string }) => s.id === blocked.id);
    expect(found).toBeTruthy();

    // Delete the blocked slot
    await apiHelper.deleteBlockedSlot(complexId, blocked.id);

    // Verify deletion
    const slotsAfter = await apiHelper.listBlockedSlots(complexId, futureDate, futureDate);
    const notFound = slotsAfter.find((s: { id: string }) => s.id === blocked.id);
    expect(notFound).toBeUndefined();
  });

  test('a blocked slot renders on the bookings calendar', async ({ authenticatedPage: page }) => {
    await expectBlockedSlotOnCalendar(page, complexId, courtId);
  });
});

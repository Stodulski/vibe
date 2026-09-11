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
  const apiHelper = await createApiHelper(page.context().request);
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
    await page.goto('/bookings');
    await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({
      timeout: 10_000,
    });

    const nextDayButton = page.getByRole('button', { name: 'Día siguiente' });
    for (let i = 0; i < 21; i++) {
      await nextDayButton.click();
    }

    await expect(page.getByRole('button', { name: /Horario bloqueado/ })).toBeVisible({
      timeout: 10_000,
    });
  } finally {
    // Re-login for a fresh CSRF token rather than reusing `apiHelper`'s: the
    // 21 client-side page navigations above can trigger the app's own
    // session/token refresh on the shared request context, rotating the
    // CSRF token issued at the top of this test and turning a reused one
    // into a 403 "invalid or missing CSRF token".
    const cleanupHelper = await createApiHelper(page.context().request);
    await cleanupHelper.deleteBlockedSlot(complexId, blocked.id);
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

  test('can block, list, and delete slots via API', async ({ authenticatedPage: page }) => {
    const apiHelper = await createApiHelper(page.context().request);

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

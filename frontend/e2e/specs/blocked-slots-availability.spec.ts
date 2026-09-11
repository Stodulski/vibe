import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import { createApiHelper } from '../helpers/api.helper';
import { getFutureDate } from '../helpers/test-data';

// Split out of `blocked-slots.spec.ts` (slice 10, max-lines decomposition)
// — the public-booking-page availability check is a distinct scenario
// (public-facing rendering) from the authenticated CRUD/UI checks that
// remain in `blocked-slots.spec.ts`.
test.describe('Blocked Slots — Public Availability', () => {
  let complexId: string;
  let courtId: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    courtId = setup.courtId;
  });

  test('blocked slot appears in availability as unavailable', async ({ authenticatedPage: page }) => {
    const apiHelper = await createApiHelper();
    const setup = await getSharedSetup();

    const futureDate = getFutureDate(15);

    // Block the 18:00-19:30 slot
    const blocked = await apiHelper.blockSlot(complexId, courtId, {
      date: futureDate,
      start_time: '18:00',
      end_time: '19:30',
      reason: 'Blocked for testing',
    });

    // Navigate to the public booking page and wait for slots to render
    await page.goto(`/${setup.complexSlug}?date=${futureDate}`);

    // Wait for the availability grid to load (court name should appear)
    await expect(page.getByText('Cancha 1')).toBeVisible({ timeout: 15_000 });

    // The 18:00 slot should be marked as unavailable (disabled button or no button)
    // Slots that are available have an interactive button; blocked ones don't
    const slot18 = page.locator('button:has-text("18:00")');
    const count = await slot18.count();
    // If the slot button exists it should be disabled, or it shouldn't exist at all
    if (count > 0) {
      await expect(slot18.first()).toBeDisabled();
    }

    // Cleanup
    await apiHelper.deleteBlockedSlot(complexId, blocked.id);
  });
});

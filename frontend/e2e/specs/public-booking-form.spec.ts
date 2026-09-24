import { test, expect, request as apiRequest } from '@playwright/test';
import { TEST_CLIENT } from '../helpers/test-data';
import { PublicBookingPage } from '../pages/public-booking.page';
import {
  registerAndLoginOwner,
  ensureTestComplex,
  seedTestCourt,
  fakeMercadoPagoConnection,
  findAndClickAvailableSlot,
  waitForOnlineBooking,
} from '../helpers/public-booking-fixtures';

const SLUG = 'complejo-publico-e2e';

// Split out of `public-booking.spec.ts` (slice 10, max-lines decomposition)
// — the full form-fill-and-submit scenario is a distinct concern from the
// navigation/display scenarios that remain in `public-booking.spec.ts`.
test.describe('Public Booking Form Submission', () => {
  let setupDone = false;

  test.beforeAll(async () => {
    // The readiness wait below may outlast Playwright's default hook timeout.
    test.setTimeout(150_000);
    const ctx = await apiRequest.newContext();
    const headers = await registerAndLoginOwner(ctx);
    const complexId = await ensureTestComplex(ctx, headers, SLUG);
    if (complexId) {
      await seedTestCourt(ctx, headers, complexId);
      await fakeMercadoPagoConnection(complexId);
    }
    await waitForOnlineBooking(ctx, SLUG);
    await ctx.dispose();
    setupDone = true;
  });

  test('can fill booking form and submit', async ({ page }) => {
    test.skip(!setupDone, 'Setup failed');
    const publicPage = new PublicBookingPage(page);

    // Set up response listener BEFORE navigation
    const availabilityPromise = page.waitForResponse((res) => res.url().includes('/availability') && res.ok());
    await publicPage.goto(SLUG);
    await availabilityPromise;

    // Date buttons are the enabled radios in the "Fecha" radiogroup.
    const dateBtns = page.getByRole('radiogroup', { name: 'Fecha' }).getByRole('radio').and(page.locator(':enabled'));
    const slotFound = await findAndClickAvailableSlot(dateBtns);
    test.skip(!slotFound, 'No available slots found');

    await publicPage.continueButton.click();
    await expect(page).toHaveURL(new RegExp(`${SLUG}/book/confirm`));

    await publicPage.fillBookingForm({
      firstName: TEST_CLIENT.firstName,
      lastName: TEST_CLIENT.lastName,
      phone: '1155556666',
      email: TEST_CLIENT.email,
    });

    await publicPage.submitBooking();

    // Should navigate away from confirm page (to payment or success)
    // or show a processing state. Since MP is faked, the booking
    // may redirect to payment page or stay on confirm with an error.
    await expect(
      page.getByText(/Redirigiendo|Procesando|Pagar|confirmada/).or(page.locator(`[href*="mercadopago"]`)),
    ).toBeVisible({
      timeout: 15_000,
    });
  });
});

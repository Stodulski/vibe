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

    // The hours only appear once the duration question is answered; the
    // landing hydrates after it server-renders, so let the scripts settle first.
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: '90 min' }).click();
    await expect(page.getByText('¿Cuánto tiempo?')).not.toBeVisible({ timeout: 10_000 });

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

    // MercadoPago is faked, so the API cannot create a preference (403 from
    // MP) and no redirect happens locally: the visitor stays on confirm with
    // the payment-link error (BOOKING_MESSAGES.publicBooking.paymentLinkError).
    await expect(page.getByText('No se pudo generar el link de pago')).toBeVisible({ timeout: 15_000 });
  });
});

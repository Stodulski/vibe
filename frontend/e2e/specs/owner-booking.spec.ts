import { test, expect } from '../helpers/auth.fixture';
import { getSharedSetup } from '../helpers/shared-setup';
import type { Page } from '@playwright/test';

/**
 * Select the shared test complex and land on the (loaded) bookings page.
 * Extracted from 6 duplicated call sites (slice 10, max-lines decomposition
 * — also removes a DRY violation per this repo's own principles).
 */
async function gotoBookings(page: Page, complexId: string): Promise<void> {
  await page.evaluate((id) => {
    localStorage.setItem('selectedComplexId', id);
  }, complexId);
  await page.goto('/bookings');
  await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible({
    timeout: 10_000,
  });
}

test.describe('Owner Booking Management', () => {
  let complexId: string;
  let courtName: string;

  test.beforeAll(async () => {
    const setup = await getSharedSetup();
    complexId = setup.complexId;
    courtName = setup.courtName;
  });

  test('bookings page loads', async ({ authenticatedPage: page }) => {
    await gotoBookings(page, complexId);
  });

  // The create-booking modal is now a 3-step wizard (Cuándo y dónde -> Cliente
  // -> Pago, see CreateBookingSteps.tsx / CreateBookingFormBody.tsx) instead
  // of a single page with every field visible at once — client fields only
  // render on step 2. This walks step 1 (date defaults to the page's selected
  // day; pick a court and a time slot) into step 2 and asserts the real label
  // ("Datos del cliente", not the old literal-uppercase "DATOS DEL CLIENTE").
  test('can open create booking modal and reach client step', async ({ authenticatedPage: page }) => {
    await gotoBookings(page, complexId);

    // The modal's date defaults to the calendar's day, and for today the time
    // list omits hours already past, so a fixed "10:00" only existed while the
    // suite ran in the morning. Tomorrow has the whole priced window.
    await page.getByRole('button', { name: 'Día siguiente' }).click();

    await page.getByRole('button', { name: 'Nueva reserva' }).first().click();
    await expect(page.getByRole('dialog')).toBeVisible({ timeout: 5_000 });
    await expect(page.getByRole('heading', { name: 'Nueva reserva' })).toBeVisible();

    // Step 1: "Cuándo y dónde" — pick a court, then a time slot. The Select
    // triggers aren't wired to their FormField label (no htmlFor/aria-label),
    // so their accessible name is their own text content (the placeholder
    // until a value is picked) rather than "Cancha"/"Horario". Before a court
    // is chosen, TimeField's placeholder also reads "Seleccioná una cancha"
    // (see TimeField.tsx's `timePlaceholder`, `!courtId` branch) — so this
    // filter matches both the court AND (disabled) time selects; `.first()`
    // is the court one, since it renders first in the DOM.
    await expect(page.getByText('Cuándo y dónde')).toBeVisible();
    await page.getByRole('combobox').filter({ hasText: 'Seleccioná una cancha' }).first().click();
    await page.getByRole('option').first().click();

    const timeSelect = page.getByRole('combobox').filter({ hasText: '--:--' });
    await expect(timeSelect).toBeEnabled({ timeout: 10_000 });
    await timeSelect.click();
    // Not `.first()`: the time list spans the full day (00:00-23:xx),
    // starting before the shared complex's priced 08:00-23:00 window
    // (DEFAULT_SCHEDULE/DEFAULT_PRICES in test-data.ts) -- the earliest slots
    // have no price band and force a "no hay tarifa configurada" manual-price
    // detour instead of advancing. And not a fixed hour: the list omits slots
    // other specs in this run already booked on the same day, so the first
    // free one inside the priced window is taken instead. The window is the
    // one every day shares: weekends are priced from 09:00, not 08:00.
    await page
      .getByRole('option', { name: /^(09|1\d|20):[03]0$/ })
      .first()
      .click();

    await page.getByRole('button', { name: 'Siguiente' }).click();

    // Step 2: client fields.
    await expect(page.getByText('Datos del cliente')).toBeVisible();
    await expect(page.locator('#client-first-name')).toBeVisible();
    await expect(page.locator('#client-last-name')).toBeVisible();
    await expect(page.locator('#client-phone')).toBeVisible();
  });

  // The calendar/list tab toggle was removed -- bookings is now calendar-only
  // (see BookingContent.tsx, which renders `BookingCalendar` unconditionally
  // with no tab role anywhere in the page). Assert the calendar renders
  // instead of a "Lista" tab that no longer exists.
  test('calendar view renders with no list-view toggle', async ({ authenticatedPage: page }) => {
    await gotoBookings(page, complexId);

    // The court-time grid actually rendered, not just "the page loaded".
    await expect(page.getByText(courtName, { exact: true })).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole('tab', { name: 'Lista' })).toHaveCount(0);
    await expect(page.getByRole('tab', { name: 'Calendario' })).toHaveCount(0);
  });

  test('can navigate between dates', async ({ authenticatedPage: page }) => {
    await gotoBookings(page, complexId);

    await page.getByRole('button', { name: 'Día siguiente' }).click();
    await page.waitForTimeout(500);

    await page.getByRole('button', { name: 'Día anterior' }).click();
    await page.waitForTimeout(500);

    await expect(page.getByRole('heading', { name: 'Reservas', exact: true })).toBeVisible();
  });
});

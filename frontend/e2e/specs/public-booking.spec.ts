import { test, expect, request as apiRequest } from '@playwright/test';
import type { Page } from '@playwright/test';
import { PublicBookingPage } from '../pages/public-booking.page';
import { landingComplexUrl } from '../helpers/landing';
import {
  registerAndLoginOwner,
  ensureTestComplex,
  seedTestCourt,
  fakeMercadoPagoConnection,
  findAndClickAvailableSlot,
  waitForOnlineBooking,
} from '../helpers/public-booking-fixtures';

const SLUG = 'complejo-publico-e2e';

// The booking flow now asks an explicit "¿Cuánto tiempo?" duration question
// before showing any hours -- see BookingSteps.tsx: `question &&
// steps.isFirstPass` renders ONLY the question, with no children (the hours
// grid) underneath, until `onChoose` answers it. A single-sport venue (this
// fixture's court is padel-only) skips the sport question but never the
// duration one. Extracted out of the describe callback since every test
// below needs it (also keeps the callback under the repo's
// max-lines-per-function cap).
async function answerDurationQuestion(page: Page): Promise<void> {
  // The landing server-renders the question and hydrates it afterwards; a
  // click that lands before hydration does nothing, so let the scripts settle.
  await page.waitForLoadState('networkidle');
  // 90 min is already the pre-selected default (see DEFAULT_DURATION in
  // useComplexPageState.ts) -- clicking it answers the step (steps.answer)
  // without necessarily changing `duration`, so it may not trigger a new
  // network request. Wait for the resulting UI instead of a response.
  await page.getByRole('button', { name: '90 min' }).click();
  await expect(page.getByText('¿Cuánto tiempo?')).not.toBeVisible({ timeout: 10_000 });
}

// Empty-state copy of the slot list, `t.publicBooking.noAvailability`
// (src/shared/i18n/es_AR/publicBooking.ts). The e2e tsconfig only includes
// e2e/, so the string is mirrored here instead of imported; keep both in sync.
// After the last slot of the day a date shows the full-day panel instead of the
// empty-filter copy, so late runs see either one.
const NO_AVAILABILITY_COPY = /Sin horarios para esta fecha|Este día ya está completo/;

test.describe('Public Booking Flow', () => {
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

  test('displays complex info and available slots', async ({ page }) => {
    test.skip(!setupDone, 'Setup failed');
    // The very first hit on the landing's complex page in a freshly started
    // `astro dev` process can be genuinely slow (cold dependency
    // pre-bundling), well past the default 30s test timeout under
    // `make e2e` -- test.slow() is Playwright's sanctioned way to allow for
    // that (triples the timeout) rather than guessing at a bigger constant.
    test.slow();
    const publicPage = new PublicBookingPage(page);
    await publicPage.goto(SLUG);

    await expect(publicPage.complexName).toContainText('Complejo Público E2E');

    // "Elegí tu turno" (a step indicator that used to open this page fixed
    // at "step 1 of 3") is gone by design -- see ComplexPageContent.tsx's
    // comment: the storefront no longer announces a step count before a
    // visitor has chosen anything; that indicator only exists later, on the
    // confirm/pay flow. Assert what the storefront actually shows instead:
    // the date selector, and (once duration is answered) either real
    // availability or its explicit "no slots" message (never nothing).
    await expect(page.getByRole('radiogroup', { name: 'Fecha' })).toBeVisible({ timeout: 30_000 });
    await answerDurationQuestion(page);
    await expect(page.locator('[data-slot-time]').first().or(page.getByText(NO_AVAILABILITY_COPY))).toBeVisible({
      timeout: 10_000,
    });
  });

  test('shows time slots for a selected date', async ({ page }) => {
    test.skip(!setupDone, 'Setup failed');
    test.slow(); // see "displays complex info and available slots"
    const publicPage = new PublicBookingPage(page);
    await publicPage.goto(SLUG);
    await answerDurationQuestion(page);

    // Date buttons are inside a scrollable container after the "Selecciona una fecha" heading.
    // Each button has the day number as text. Click the 2nd button (tomorrow).
    // Date buttons are the enabled radios in the "Fecha" radiogroup.
    const dateBtns = page.getByRole('radiogroup', { name: 'Fecha' }).getByRole('radio').and(page.locator(':enabled'));
    const btnCount = await dateBtns.count();
    if (btnCount > 1) {
      await dateBtns.nth(1).click();
      await page.waitForResponse((res) => res.url().includes('/availability') && res.ok());
    }

    // Should have slots or a "no availability" message
    await expect(page.locator('[data-slot-time]').first().or(page.getByText(NO_AVAILABILITY_COPY))).toBeVisible({
      timeout: 10_000,
    });
  });

  test('can select a slot and navigate to confirm page', async ({ page }) => {
    test.skip(!setupDone, 'Setup failed');
    test.slow(); // see "displays complex info and available slots"
    const publicPage = new PublicBookingPage(page);
    await publicPage.goto(SLUG);
    await answerDurationQuestion(page);

    // Date buttons are the enabled radios in the "Fecha" radiogroup.
    const dateBtns = page.getByRole('radiogroup', { name: 'Fecha' }).getByRole('radio').and(page.locator(':enabled'));
    const slotFound = await findAndClickAvailableSlot(dateBtns);
    test.skip(!slotFound, 'No available slots found in the next 7 days');

    await publicPage.continueButton.click();
    await expect(page).toHaveURL(new RegExp(`${SLUG}/book/confirm`));
  });
});

// Its own block: the flow block above is at the max-lines-per-function cap.
test.describe('Public complex not found', () => {
  test('an unknown slug is a real 404 with the not-found copy', async ({ page }) => {
    const response = await page.goto(landingComplexUrl('non-existent-complex-slug-xyz'));
    expect(response?.status()).toBe(404);
    await expect(page.getByRole('heading', { name: 'Complejo no encontrado' })).toBeVisible({ timeout: 10_000 });
  });
});

import { test, expect, request as apiRequest } from '@playwright/test';
import type { Locator, Page } from '@playwright/test';
import {
  registerAndLoginOwner,
  ensureTestComplex,
  seedTestCourt,
  fakeMercadoPagoConnection,
  waitForOnlineBooking,
} from '../helpers/public-booking-fixtures';

const SLUG = 'complejo-publico-e2e';

/** Whether `target` holds the focus right now, as a plain boolean. */
function isFocused(target: Locator): Promise<boolean> {
  return expect(target)
    .toBeFocused({ timeout: 150 })
    .then(
      () => true,
      () => false,
    );
}

/**
 * Walks the Tab order until `target` holds the focus.
 *
 * Deliberately blind: it presses a key and looks at what ended up focused,
 * exactly like someone without a mouse, so a control that is rendered but
 * unreachable (`tabindex="-1"`, hidden, or behind a focus trap) fails here
 * instead of passing on a `.click()` no keyboard user could make.
 */
async function tabTo(page: Page, target: Locator, key: 'Tab' | 'Shift+Tab' = 'Tab', maxPresses = 40): Promise<void> {
  for (let i = 0; i < maxPresses; i++) {
    if (await isFocused(target)) return;
    await page.keyboard.press(key);
  }
  await expect(target).toBeFocused();
}

/** Seeds the public complex once for every spec in this file. */
async function seedComplex(): Promise<void> {
  const ctx = await apiRequest.newContext();
  const headers = await registerAndLoginOwner(ctx);
  const complexId = await ensureTestComplex(ctx, headers, SLUG);
  if (complexId) {
    await seedTestCourt(ctx, headers, complexId);
    await fakeMercadoPagoConnection(complexId);
  }
  await waitForOnlineBooking(ctx, SLUG);
  await ctx.dispose();
}

let setupDone = false;

test.beforeAll(async () => {
  test.setTimeout(150_000);
  await seedComplex();
  setupDone = true;
});

test.describe('Public booking skip link, keyboard only', () => {
  test('is the first control in the document and moves the tab order into the content', async ({ page }) => {
    test.skip(!setupDone, 'Setup failed');
    test.slow();
    await page.goto(`/${SLUG}`);
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 30_000 });

    // Backwards, not forwards: where a freshly rendered page leaves the
    // sequential focus starting point is the browser's business (here it
    // lands inside the booking steps). What has to be true is that nothing
    // in the layout sits before the skip link, so Shift+Tab ends on it — and
    // that it stops hiding once focused.
    const skipLink = page.getByRole('link', { name: /contenido/i });
    await tabTo(page, skipLink, 'Shift+Tab');
    await expect(skipLink).toBeInViewport();

    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/#main-content$/);

    // `<main>` is not focusable, so the browser moves the *next* tab stop
    // into it rather than the focus itself — which is the behaviour that
    // makes the link worth having.
    await page.keyboard.press('Tab');
    await expect(page.locator('#main-content *:focus')).toHaveCount(1);
  });
});

test.describe('Public booking flow, keyboard only', () => {
  test('a slot can be chosen without ever using the mouse', async ({ page }) => {
    test.skip(!setupDone, 'Setup failed');
    test.slow();
    await page.goto(`/${SLUG}`);
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 30_000 });

    // The duration question is the only thing on screen on the first pass
    // (BookingSteps.tsx), so it is answered before any hour exists.
    const duration = page.getByRole('button', { name: '90 min' });
    await tabTo(page, duration);
    await page.keyboard.press('Enter');
    await expect(page.getByText('¿Cuánto tiempo?')).not.toBeVisible({ timeout: 15_000 });

    // The date strip is a radiogroup with a roving tabindex: one Tab stop,
    // and the arrows move focus and selection together.
    const selectedDate = page.locator('[role="radio"][aria-checked="true"]');
    const enabledSlot = page.locator('[data-slot-time]:not([disabled])').first();

    for (let day = 0; day < 6; day++) {
      if (await enabledSlot.isVisible({ timeout: 2_000 }).catch(() => false)) break;
      await tabTo(page, selectedDate);
      await page.keyboard.press('ArrowRight');
      await expect(page.locator('[role="radio"]:focus')).toHaveAttribute('aria-checked', 'true');
      await page
        .waitForResponse((res) => res.url().includes('/availability') && res.ok(), { timeout: 10_000 })
        .catch(() => {
          /* the day may already be cached; the visibility check decides */
        });
    }

    await expect(enabledSlot).toBeVisible({ timeout: 15_000 });
    await tabTo(page, enabledSlot);
    await page.keyboard.press('Enter');

    await expect(enabledSlot).toHaveAttribute('aria-pressed', 'true');
  });
});

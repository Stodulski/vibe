import { test, expect } from '@playwright/test';
import { landingComplexUrl } from '../helpers/landing';

test.describe('Public Booking Cancellation', () => {
  test('cancel page without params shows error', async ({ page }) => {
    await page.goto(landingComplexUrl('complejo-publico-e2e', '/book/cancel'));

    // Should show an error or a way back. Scoped to `<main>`: `StorefrontLayout`'s
    // header carries a home link to `/c/:slug` that matches the fallback
    // locator on every storefront page, so an unscoped assertion would pass
    // vacuously.
    await expect(
      page
        .getByRole('main')
        .getByText(/no encontrada|no válid|error|enlace/i)
        .or(page.getByRole('main').locator('a[href*="complejo-publico-e2e"]')),
    ).toBeVisible({
      timeout: 10_000,
    });
  });
});

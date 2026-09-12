import { test, expect } from '@playwright/test';

test.describe('Public Booking Cancellation', () => {
  test('cancel page without params shows error', async ({ page }) => {
    await page.goto('/complejo-publico-e2e/book/cancel');

    // Should show an error or a way back. Scoped to `<main>`: `PublicLayout`'s
    // header carries a brand link to `/:slug` that matches the fallback
    // locator on every public page, so an unscoped assertion both passed
    // vacuously and, once the layout stopped being a static import and
    // resolved alongside the page, matched two elements at once.
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

import { test, expect } from '@playwright/test';

test.describe('Public Booking Cancellation', () => {
  test('cancel page without params shows error', async ({ page }) => {
    await page.goto('/complejo-publico-e2e/book/cancel');

    // Should show an error or redirect back
    await expect(
      page.getByText(/no encontrada|no válid|error|enlace/i).or(page.locator('a[href*="complejo-publico-e2e"]')),
    ).toBeVisible({
      timeout: 10_000,
    });
  });
});
